package zmap_test

import (
	"hash/maphash"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// 添加性能监控指标
type Metrics struct {
	HitRate   float64
	ShardLoad []float64
}

// 条目结构优化：内存对齐 + 时间戳
type lruEntry[K comparable, V any] struct {
	key      K
	value    V
	expireAt int64 // 纳秒时间戳
	prev     unsafe.Pointer
	next     unsafe.Pointer
}

// 分片结构优化：原子计数器 + 异步队列
type lruShard[K comparable, V any] struct {
	mu        sync.RWMutex
	items     map[K]*lruEntry[K, V]
	head      unsafe.Pointer // 原子指针
	tail      unsafe.Pointer // 原子指针
	capacity  int
	size      int32 // 原子操作
	accessCnt atomic.Uint64
	hitCnt    atomic.Uint64
	moveChan  chan *lruEntry[K, V] // 异步移动队列
	parent    *LRUShardMap[K, V]
}

// 主结构增加扩展功能
type LRUShardMap[K comparable, V any] struct {
	shards    []*lruShard[K, V]
	shardMask int
	seed      maphash.Seed
	entryPool sync.Pool
	onEvict   func(K, V) // 淘汰回调
	stopChan  chan struct{}
}

const (
	asyncMoveBuffer = 128       // 异步队列缓冲区
	defaultTTL      = time.Hour // 默认过期时间
)

func NewLRUShardMap[K comparable, V any](shardCount, capacity int) *LRUShardMap[K, V] {
	if shardCount <= 0 {
		shardCount = 16
	}
	if capacity <= 0 {
		capacity = 1024
	}
	shardCount = 1 << bits.Len(uint(shardCount-1))
	shardMask := shardCount - 1

	m := &LRUShardMap[K, V]{
		shards:    make([]*lruShard[K, V], shardCount),
		shardMask: shardMask,
		seed:      maphash.MakeSeed(),
		entryPool: sync.Pool{New: func() interface{} { return new(lruEntry[K, V]) }},
		stopChan:  make(chan struct{}),
	}

	perShardCap := max(capacity/shardCount, 1)
	for i := range m.shards {
		s := &lruShard[K, V]{
			items:    make(map[K]*lruEntry[K, V], perShardCap),
			capacity: perShardCap,
			moveChan: make(chan *lruEntry[K, V], asyncMoveBuffer),
			parent:   m,
		}
		m.shards[i] = s
		go s.asyncMoveProcessor()
	}

	go m.backgroundExpirationChecker()
	return m
}

// 异步处理链表移动
func (s *lruShard[K, V]) asyncMoveProcessor() {
	for entry := range s.moveChan {
		s.mu.Lock()
		if _, exists := s.items[entry.key]; exists {
			s.moveToFront(entry)
		}
		s.mu.Unlock()
	}
}

// 后台过期检查
func (m *LRUShardMap[K, V]) backgroundExpirationChecker() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixNano()
			for _, shard := range m.shards {
				shard.checkExpiration(now)
			}
		case <-m.stopChan:
			return
		}
	}
}

// 过期检查（批量处理）
func (s *lruShard[K, V]) checkExpiration(now int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, entry := range s.items {
		if entry.expireAt > 0 && entry.expireAt < now {
			//s.removeEntry(entry)
			if s.parent.onEvict != nil {
				s.parent.onEvict(k, entry.value)
			}
		}
	}
}

// 完整的Set方法实现
func (m *LRUShardMap[K, V]) Set(key K, value V) {
	m.SetWithTTL(key, value, 0) // 默认不过期
}

func (m *LRUShardMap[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 如果已存在，更新值并移动位置
	if entry, exists := shard.items[key]; exists {
		entry.value = value
		if ttl > 0 {
			entry.expireAt = time.Now().Add(ttl).UnixNano()
		} else {
			entry.expireAt = 0
		}
		shard.moveToFront(entry)
		return
	}

	// 创建新条目
	entry := m.getEntry(key, value)
	if ttl > 0 {
		entry.expireAt = time.Now().Add(ttl).UnixNano()
	}

	// 添加到链表头部
	if atomic.LoadPointer(&shard.head) == nil {
		atomic.StorePointer(&shard.head, unsafe.Pointer(entry))
		atomic.StorePointer(&shard.tail, unsafe.Pointer(entry))
	} else {
		oldHead := (*lruEntry[K, V])(atomic.LoadPointer(&shard.head))
		entry.next = unsafe.Pointer(oldHead)
		oldHead.prev = unsafe.Pointer(entry)
		atomic.StorePointer(&shard.head, unsafe.Pointer(entry))
	}

	// 添加到map
	shard.items[key] = entry
	atomic.AddInt32(&shard.size, 1)

	// 淘汰逻辑
	if int(atomic.LoadInt32(&shard.size)) > shard.capacity {
		tail := (*lruEntry[K, V])(atomic.LoadPointer(&shard.tail))
		if tail != nil {
			shard.removeEntry(tail)
		}
	}
}

// 完整的Del方法实现
func (m *LRUShardMap[K, V]) Del(key K) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry, exists := shard.items[key]
	if !exists {
		return false
	}

	shard.removeEntry(entry)
	return true
}

// 改进的removeEntry方法
func (s *lruShard[K, V]) removeEntry(entry *lruEntry[K, V]) {
	// 调整前驱节点
	if prev := entry.prev; prev != nil {
		prevEntry := (*lruEntry[K, V])(prev)
		prevEntry.next = entry.next
	} else {
		atomic.StorePointer(&s.head, entry.next)
	}

	// 调整后继节点
	if next := entry.next; next != nil {
		nextEntry := (*lruEntry[K, V])(next)
		nextEntry.prev = entry.prev
	} else {
		atomic.StorePointer(&s.tail, entry.prev)
	}

	// 清理数据
	delete(s.items, entry.key)
	atomic.AddInt32(&s.size, -1)

	// 触发回调
	if s.parent.onEvict != nil {
		s.parent.onEvict(entry.key, entry.value)
	}

	// 回收到对象池
	s.parent.entryPool.Put(entry)
}

// 补充缺失的getShard方法
func (m *LRUShardMap[K, V]) getShard(key K) *lruShard[K, V] {
	hash := maphash.Comparable(m.seed, key)
	return m.shards[int(hash)&m.shardMask]
}

// 补充缺失的moveToFront原子操作实现
func (s *lruShard[K, V]) moveToFront(entry *lruEntry[K, V]) {
	currentHead := atomic.LoadPointer(&s.head)
	if unsafe.Pointer(entry) == currentHead {
		return
	}

	// 原子交换实现无锁移动
	for {
		oldHead := (*lruEntry[K, V])(currentHead)
		entry.next = currentHead
		if atomic.CompareAndSwapPointer(&s.head, currentHead, unsafe.Pointer(entry)) {
			if oldHead != nil {
				oldHead.prev = unsafe.Pointer(entry)
			}
			break
		}
		currentHead = atomic.LoadPointer(&s.head)
	}
}

// 使用内存池获取条目
func (m *LRUShardMap[K, V]) getEntry(key K, value V) *lruEntry[K, V] {
	entry := m.entryPool.Get().(*lruEntry[K, V])
	entry.key = key
	entry.value = value
	entry.expireAt = 0
	entry.prev = nil
	entry.next = nil
	return entry
}

// 优化后的Get方法
func (m *LRUShardMap[K, V]) Get(key K) (V, bool) {
	shard := m.getShard(key)
	shard.accessCnt.Add(1)

	shard.mu.RLock()
	entry, ok := shard.items[key]
	shard.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	// 异步触发位置更新
	select {
	case shard.moveChan <- entry:
	default: // 避免阻塞
	}

	shard.hitCnt.Add(1)
	return entry.value, !entry.isExpired()
}

// 性能监控接口
func (m *LRUShardMap[K, V]) Metrics() Metrics {
	totalAccess := uint64(0)
	totalHit := uint64(0)
	shardLoad := make([]float64, len(m.shards))

	for i, s := range m.shards {
		access := s.accessCnt.Load()
		hit := s.hitCnt.Load()
		totalAccess += access
		totalHit += hit
		shardLoad[i] = float64(s.size) / float64(s.capacity)
	}

	hitRate := 0.0
	if totalAccess > 0 {
		hitRate = float64(totalHit) / float64(totalAccess)
	}

	return Metrics{
		HitRate:   hitRate,
		ShardLoad: shardLoad,
	}
}

// 条目过期检查
func (e *lruEntry[K, V]) isExpired() bool {
	if e.expireAt == 0 {
		return false
	}
	return e.expireAt < time.Now().UnixNano()
}

// Len 返回当前缓存中的项目数
func (m *LRUShardMap[K, V]) Len() int {
	total := int32(0)
	for i := range m.shards {
		shard := m.shards[i]
		shard.mu.RLock()
		total += atomic.LoadInt32(&shard.size)
		shard.mu.RUnlock()
	}
	return int(total)
}

// Clear 清空缓存
func (m *LRUShardMap[K, V]) Clear() {
	for i := range m.shards {
		shard := m.shards[i]
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
