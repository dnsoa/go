package lru

import (
	"hash/maphash"
	"math/bits"
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	// 确保默认分片数量为 2 的幂，避免位掩码分片偏斜
	defaultShardNUM = nextPowerOfTwo(runtime.GOMAXPROCS(0) * 4)
	defaultCapacity = 4096
)

type ShardLRUOption[K comparable, V any] func(*ShardLRU[K, V])

func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	return 1 << bits.Len(uint(n)-1)
}

func WithShardCount[K comparable, V any](shardCount int) ShardLRUOption[K, V] {
	return func(m *ShardLRU[K, V]) {
		m.shardCount = nextPowerOfTwo(shardCount)
	}
}

func WithCapacity[K comparable, V any](capacity int) ShardLRUOption[K, V] {
	return func(m *ShardLRU[K, V]) {
		m.capacity = nextPowerOfTwo(capacity)
	}
}

func WithLRUOnEvict[K comparable, V any](onEvict func(K, V)) ShardLRUOption[K, V] {
	return func(m *ShardLRU[K, V]) {
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

// ShardLRU 是一个分片式的 LRU 缓存
type ShardLRU[K comparable, V any] struct {
	entryPool  sync.Pool
	onEvict    func(K, V) // 淘汰回调
	capacity   int
	shardCount int
	shards     []lruShard[K, V]
	shardMask  int
	seed       maphash.Seed
}

// NewShardLRU 创建一个新的分片式 LRU 缓存
// shardCount: 分片数量，默认为16，会向上取整为2的幂
// capacity: 总容量，会平均分配给所有分片
func NewShardLRU[K comparable, V any](options ...ShardLRUOption[K, V]) *ShardLRU[K, V] {
	m := &ShardLRU[K, V]{
		shardCount: defaultShardNUM, // 默认分片数
		capacity:   defaultCapacity, // 默认容量
		seed:       maphash.MakeSeed(),
		entryPool:  sync.Pool{New: func() any { return new(lruEntry[K, V]) }},
	}
	for _, option := range options {
		option(m)
	}
	// 兜底强制 2 的幂
	m.shardCount = nextPowerOfTwo(m.shardCount)
	m.capacity = nextPowerOfTwo(m.capacity)
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

func (lru *ShardLRU[K, V]) getShard(key K) *lruShard[K, V] {
	h := maphash.Comparable(lru.seed, key)
	// 使用murmur哈希的简化版本
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return &lru.shards[(h & uint64(lru.shardMask))]
}

func (lru *ShardLRU[K, V]) Get(key K) (V, bool) {
	shard := lru.getShard(key)
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

func (lru *ShardLRU[K, V]) Set(key K, value V) {
	shard := lru.getShard(key)
	shard.mu.Lock()

	if entry, ok := shard.items[key]; ok {
		entry.value = value
		shard.moveToFront(entry)
		shard.mu.Unlock()
		return
	}

	entry := lru.entryPool.Get().(*lruEntry[K, V])
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

	var evk K
	var evv V
	var evicted bool
	if int(shard.size.Load()) > shard.capacity {
		oldest := shard.tail
		evk, evv, evicted = shard.removeEntry(oldest, lru)
	}

	shard.mu.Unlock()

	if evicted && lru.onEvict != nil {
		lru.onEvict(evk, evv)
	}
}

func (lru *ShardLRU[K, V]) Delete(key K) bool {
	shard := lru.getShard(key)
	shard.mu.Lock()

	entry, ok := shard.items[key]
	if !ok {
		shard.mu.Unlock()
		return false
	}

	evk, evv, evicted := shard.removeEntry(entry, lru)
	shard.mu.Unlock()

	if evicted && lru.onEvict != nil {
		lru.onEvict(evk, evv)
	}
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

// 从链表和映射中移除条目（不在持锁期间触发 onEvict）
func (s *lruShard[K, V]) removeEntry(entry *lruEntry[K, V], m *ShardLRU[K, V]) (K, V, bool) {
	var zeroK K
	var zeroV V
	if entry == nil || m == nil {
		return zeroK, zeroV, false
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

	k := entry.key
	v := entry.value

	delete(s.items, k)
	s.size.Add(-1)

	// 清理引用，避免内存泄漏
	entry.key, entry.value = *new(K), zeroV
	entry.prev, entry.next = nil, nil
	m.entryPool.Put(entry)

	return k, v, true
}

// Len 返回当前缓存中的项目数
func (lru *ShardLRU[K, V]) Len() int {
	total := 0
	for i := range lru.shards {
		shard := &lru.shards[i]
		total += int(shard.size.Load())
	}
	return total
}

// Clear 清空缓存
func (lru *ShardLRU[K, V]) Clear() {
	for i := range lru.shards {
		shard := &lru.shards[i]
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
func (lru *ShardLRU[K, V]) Contains(key K) bool {
	shard := lru.getShard(key)
	shard.mu.RLock()
	_, ok := shard.items[key]
	shard.mu.RUnlock()
	return ok
}

// Stats 返回缓存统计信息,包括命中率和每个分片的负载
func (lru *ShardLRU[K, V]) Stats() (hitRate float64, shardLoad []float64) {
	access := uint64(0)
	hits := uint64(0)
	shardLoad = make([]float64, len(lru.shards))
	for i := range lru.shards {
		shard := &lru.shards[i]
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
