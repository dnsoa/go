package zmap

import (
	"hash/maphash"
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	defaultLRUShardNUM = runtime.GOMAXPROCS(0) * 4
	defaultLRUCapacity = 4096
)

type LRUShardMapOption[K comparable, V any] func(*LRUShardMap[K, V])

func WithLRUShardCount[K comparable, V any](shardCount int) LRUShardMapOption[K, V] {
	return func(m *LRUShardMap[K, V]) {
		m.shardCount = nextPowerOfTwo(shardCount)
	}
}

func WithLRUCapacity[K comparable, V any](capacity int) LRUShardMapOption[K, V] {
	return func(m *LRUShardMap[K, V]) {
		m.capacity = nextPowerOfTwo(capacity)
	}
}

func WithLRUOnEvict[K comparable, V any](onEvict func(K, V)) LRUShardMapOption[K, V] {
	return func(m *LRUShardMap[K, V]) {
		m.onEvict = onEvict
	}
}

// lruEntry 是 LRU 缓存中的节点
type lruEntry[K comparable, V any] struct {
	key   K
	value V
	prev  *lruEntry[K, V]
	next  *lruEntry[K, V]
}

// LRUShard 是单个 LRU 分片
type lruShard[K comparable, V any] struct {
	items     map[K]*lruEntry[K, V]
	head      *lruEntry[K, V] // 最近使用的在头部
	tail      *lruEntry[K, V] // 最久未使用的在尾部
	capacity  int             // 当前分片容量
	size      atomic.Int32    // 当前大小
	accessCnt atomic.Uint64   // 访问计数，原子操作
	hitCnt    atomic.Uint64   // 命中计数，原子操作
	mu        sync.RWMutex
}

// LRUShardMap 是一个分片式的 LRU 缓存
type LRUShardMap[K comparable, V any] struct {
	entryPool  sync.Pool
	onEvict    func(K, V) // 淘汰回调
	capacity   int
	shardCount int
	shards     []lruShard[K, V]
	shardMask  int
	seed       maphash.Seed
}

// NewLRUShardMap 创建一个新的分片式 LRU 缓存
// shardCount: 分片数量，默认为16，会向上取整为2的幂
// capacity: 总容量，会平均分配给所有分片
func NewLRUShardMap[K comparable, V any](options ...LRUShardMapOption[K, V]) *LRUShardMap[K, V] {
	m := &LRUShardMap[K, V]{
		shardCount: defaultLRUShardNUM, // 默认分片数
		capacity:   defaultLRUCapacity, // 默认容量
		seed:       maphash.MakeSeed(),
		entryPool:  sync.Pool{New: func() any { return new(lruEntry[K, V]) }},
	}
	for _, option := range options {
		option(m)
	}
	m.shardMask = m.shardCount - 1

	m.shards = make([]lruShard[K, V], m.shardCount)
	perShardCap := m.capacity / m.shardCount
	if perShardCap <= 0 {
		perShardCap = 1
	}

	for i := range m.shards {
		m.shards[i] = lruShard[K, V]{
			items:    make(map[K]*lruEntry[K, V]),
			capacity: perShardCap,
		}
	}

	return m
}

func (m *LRUShardMap[K, V]) getShard(key K) *lruShard[K, V] {
	h := maphash.Comparable(m.seed, key)
	// 使用murmur哈希的简化版本
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return &m.shards[(h & uint64(m.shardMask))]
}

func (m *LRUShardMap[K, V]) Get(key K) (V, bool) {
	shard := m.getShard(key)
	shard.accessCnt.Add(1)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry, ok := shard.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	shard.hitCnt.Add(1)
	value := entry.value

	shard.moveToFront(entry)

	return value, true
}

func (m *LRUShardMap[K, V]) Set(key K, value V) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if entry, ok := shard.items[key]; ok {
		entry.value = value
		shard.moveToFront(entry)
		return
	}

	entry := m.entryPool.Get().(*lruEntry[K, V])
	entry.key = key
	entry.value = value
	entry.prev = nil
	entry.next = nil

	if shard.head == nil {
		shard.head = entry
		shard.tail = entry
	} else {
		entry.next = shard.head
		shard.head.prev = entry
		shard.head = entry
	}

	shard.items[key] = entry
	shard.size.Add(1)

	if int(shard.size.Load()) > shard.capacity {
		oldest := shard.tail
		shard.removeEntry(oldest, m)
	}
}

func (m *LRUShardMap[K, V]) Delete(key K) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry, ok := shard.items[key]
	if !ok {
		return false
	}

	shard.removeEntry(entry, m)
	return true
}

func (s *lruShard[K, V]) moveToFront(entry *lruEntry[K, V]) {
	if entry == nil || s.head == nil {
		if entry != nil && s.head == nil {
			s.head = entry
			s.tail = entry
			entry.prev = nil
			entry.next = nil
		}
		return
	}

	if entry == s.head {
		return
	}

	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
	if entry == s.tail {
		s.tail = entry.prev
	}

	entry.prev = nil
	entry.next = s.head
	s.head.prev = entry
	s.head = entry
}

// 从链表和映射中移除条目
func (s *lruShard[K, V]) removeEntry(entry *lruEntry[K, V], m *LRUShardMap[K, V]) {
	if entry == nil || m == nil {
		return
	}
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

	if m.onEvict != nil {
		m.onEvict(entry.key, entry.value)
	}

	delete(s.items, entry.key)
	s.size.Add(-1)

	// 清理引用，避免内存泄漏
	var zero V
	entry.key, entry.value = *new(K), zero
	entry.prev, entry.next = nil, nil
	m.entryPool.Put(entry)
}

// Len 返回当前缓存中的项目数
func (m *LRUShardMap[K, V]) Len() int {
	total := 0
	for i := range m.shards {
		shard := &m.shards[i]
		shard.mu.RLock()
		total += int(shard.size.Load())
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
		shard.size.Store(0)
		shard.accessCnt.Store(0)
		shard.hitCnt.Store(0)
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

// Stats 返回缓存统计信息,包括命中率和每个分片的负载
func (m *LRUShardMap[K, V]) Stats() (hitRate float64, shardLoad []float64) {
	access := uint64(0)
	hits := uint64(0)
	shardLoad = make([]float64, len(m.shards))
	for i := range m.shards {
		shard := &m.shards[i]
		access += shard.accessCnt.Load()
		hits += shard.hitCnt.Load()
		shardLoad[i] = float64(shard.size.Load()) / float64(shard.capacity)
	}
	hitRate = 0.0
	if access > 0 {
		hitRate = float64(hits) / float64(access)
	}
	return
}
