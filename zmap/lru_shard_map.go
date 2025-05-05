package zmap

import (
	"hash/maphash"
	"math/bits"
	"sync"
)

// lruEntry 是 LRU 缓存中的节点
type lruEntry[K comparable, V any] struct {
	key   K
	value V
	prev  *lruEntry[K, V]
	next  *lruEntry[K, V]
}

// LRUShard 是单个 LRU 分片
type lruShard[K comparable, V any] struct {
	mu        sync.RWMutex
	items     map[K]*lruEntry[K, V]
	head      *lruEntry[K, V] // 最近使用的在头部
	tail      *lruEntry[K, V] // 最久未使用的在尾部
	capacity  int             // 当前分片容量
	size      int             // 当前大小
	accessCnt uint64          // 访问计数
	hitCnt    uint64          // 命中计数
}

// LRUShardMap 是一个分片式的 LRU 缓存
type LRUShardMap[K comparable, V any] struct {
	shards    []lruShard[K, V]
	shardMask int
	seed      maphash.Seed
	entryPool sync.Pool
	onEvict   func(K, V) // 淘汰回调
	stopChan  chan struct{}
}

// NewLRUShardMap 创建一个新的分片式 LRU 缓存
// shardCount: 分片数量，默认为16，会向上取整为2的幂
// capacity: 总容量，会平均分配给所有分片
func NewLRUShardMap[K comparable, V any](shardCount, capacity int) *LRUShardMap[K, V] {
	if shardCount <= 0 {
		shardCount = 16
	}
	if capacity <= 0 {
		capacity = 1024
	}

	// 向上取整为2的幂
	shardCount = 1 << bits.Len(uint(shardCount-1))
	shardMask := shardCount - 1

	shards := make([]lruShard[K, V], shardCount)
	perShardCap := capacity / shardCount
	if perShardCap <= 0 {
		perShardCap = 1
	}

	for i := range shards {
		shards[i] = lruShard[K, V]{
			items:    make(map[K]*lruEntry[K, V]),
			capacity: perShardCap,
		}
	}

	return &LRUShardMap[K, V]{
		shards:    shards,
		shardMask: shardMask,
		seed:      maphash.MakeSeed(),
		entryPool: sync.Pool{New: func() interface{} { return new(lruEntry[K, V]) }},
		stopChan:  make(chan struct{}),
	}
}

func (m *LRUShardMap[K, V]) getShard(key K) *lruShard[K, V] {
	hash := maphash.Comparable(m.seed, key)
	return &m.shards[int(hash)&m.shardMask]
}

func (m *LRUShardMap[K, V]) Get(key K) (V, bool) {
	shard := m.getShard(key)
	shard.mu.RLock()
	entry, ok := shard.items[key]
	if !ok {
		shard.mu.RUnlock()
		shard.accessCnt++
		var zero V
		return zero, false
	}

	// 在读锁下只读取值，不修改链表
	value := entry.value
	shard.mu.RUnlock()

	// 获取写锁以更新位置
	shard.mu.Lock()
	shard.moveToFront(entry)
	shard.accessCnt++
	shard.hitCnt++
	shard.mu.Unlock()

	return value, true
}

// Set 设置键值对，如果键已存在则更新值
func (m *LRUShardMap[K, V]) Set(key K, value V) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if entry, ok := shard.items[key]; ok {
		// 更新现有条目
		entry.value = value
		shard.moveToFront(entry)
		return
	}

	entry := &lruEntry[K, V]{
		key:   key,
		value: value,
	}

	// 添加到链表头部
	if shard.head == nil {
		shard.head = entry
		shard.tail = entry
	} else {
		entry.next = shard.head
		shard.head.prev = entry
		shard.head = entry
	}

	// 添加到映射
	shard.items[key] = entry
	shard.size++

	// 如果超出容量，移除最久未使用的条目
	if shard.size > shard.capacity {
		oldest := shard.tail
		shard.removeEntry(oldest)
	}
}

// Del 删除键值对，如果键存在返回true，否则返回false
func (m *LRUShardMap[K, V]) Del(key K) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry, ok := shard.items[key]
	if !ok {
		return false
	}

	shard.removeEntry(entry)
	return true
}

// 移动条目到链表头部
func (s *lruShard[K, V]) moveToFront(entry *lruEntry[K, V]) {
	if entry == s.head {
		return
	}

	// 从原位置移除
	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
	if entry == s.tail {
		s.tail = entry.prev
	}

	// 添加到头部
	entry.prev = nil
	entry.next = s.head
	s.head.prev = entry
	s.head = entry
}

// 从链表和映射中移除条目
func (s *lruShard[K, V]) removeEntry(entry *lruEntry[K, V]) {
	// 从链表中移除
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		s.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		s.tail = entry.prev
	}

	// 从映射中移除
	delete(s.items, entry.key)
	s.size--
}

// Len 返回当前缓存中的项目数
func (m *LRUShardMap[K, V]) Len() int {
	total := 0
	for i := range m.shards {
		shard := &m.shards[i]
		shard.mu.RLock()
		total += shard.size
		shard.mu.RUnlock()
	}
	return total
}

// Clear 清空缓存
func (m *LRUShardMap[K, V]) Clear() {
	for i := range m.shards {
		shard := &m.shards[i]
		shard.mu.Lock()
		shard.items = make(map[K]*lruEntry[K, V])
		shard.head = nil
		shard.tail = nil
		shard.size = 0
		shard.mu.Unlock()
	}
}

// Contains 检查键是否存在
func (m *LRUShardMap[K, V]) Contains(key K) bool {
	shard := m.getShard(key)
	shard.mu.RLock()
	_, ok := shard.items[key]
	shard.mu.RUnlock()
	return ok
}
