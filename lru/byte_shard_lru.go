package lru

import (
	"fmt"
	"hash/maphash"
	"sync"
	"sync/atomic"
)

const (
	defaultByteShardCount   = 32
	defaultByteShardMaxBytes = 64 * 1024 * 1024 // 64MB per shard
)

// byteShard is a single shard with its own map, linked list, and mutex.
type byteShard[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]*byteEntry[K, V]
	head     *byteEntry[K, V]
	tail     *byteEntry[K, V]
	maxBytes int
	curBytes atomic.Int64
	sizer    ByteSizer[V]
	onEvict  func(K, V)
}

// ByteShardLRUOption is a functional option for ByteShardLRU.
type ByteShardLRUOption[K comparable, V any] func(*ByteShardLRU[K, V])

// WithShardMaxBytes sets the per-shard maximum byte limit.
// Total capacity = shardCount * shardMaxBytes.
func WithShardMaxBytes[K comparable, V any](maxBytes int) ByteShardLRUOption[K, V] {
	return func(lru *ByteShardLRU[K, V]) {
		lru.shardMaxBytes = maxBytes
	}
}

// WithShardByteSizer sets the byte size function.
func WithShardByteSizer[K comparable, V any](sizer ByteSizer[V]) ByteShardLRUOption[K, V] {
	return func(lru *ByteShardLRU[K, V]) {
		lru.sizer = sizer
	}
}

// WithShardByteOnEvict sets the eviction callback.
func WithShardByteOnEvict[K comparable, V any](onEvict func(K, V)) ByteShardLRUOption[K, V] {
	return func(lru *ByteShardLRU[K, V]) {
		lru.onEvict = onEvict
	}
}

// WithShardCountByte sets the number of shards.
func WithShardCountByte[K comparable, V any](count int) ByteShardLRUOption[K, V] {
	return func(lru *ByteShardLRU[K, V]) {
		lru.shardCount = nextPowerOfTwo(count)
	}
}

// WithTotalMaxBytes sets the total byte limit across all shards.
// The per-shard limit is computed as totalMaxBytes / shardCount.
func WithTotalMaxBytes[K comparable, V any](totalMaxBytes int) ByteShardLRUOption[K, V] {
	return func(lru *ByteShardLRU[K, V]) {
		lru.totalMaxBytes = totalMaxBytes
	}
}

// ByteShardLRU is a sharded byte-budget LRU cache.
// Each shard has independent locking, allowing concurrent Get/Set on different keys.
type ByteShardLRU[K comparable, V any] struct {
	shards        []byteShard[K, V]
	shardCount    int
	shardMask     int
	shardMaxBytes int
	totalMaxBytes int // if set, overrides shardMaxBytes
	sizer         ByteSizer[V]
	onEvict       func(K, V)
	seed          maphash.Seed
}

// NewByteShardLRU creates a new sharded byte-budget LRU.
func NewByteShardLRU[K comparable, V any](options ...ByteShardLRUOption[K, V]) (*ByteShardLRU[K, V], error) {
	m := &ByteShardLRU[K, V]{
		shardCount:    defaultByteShardCount,
		shardMaxBytes: defaultByteShardMaxBytes,
		seed:          maphash.MakeSeed(),
	}
	for _, opt := range options {
		opt(m)
	}
	m.shardCount = nextPowerOfTwo(m.shardCount)
	m.shardMask = m.shardCount - 1

	// If totalMaxBytes was set, derive per-shard limit from it
	if m.totalMaxBytes > 0 {
		m.shardMaxBytes = m.totalMaxBytes / m.shardCount
		if m.shardMaxBytes <= 0 {
			m.shardMaxBytes = 1
		}
	}

	if m.shardMaxBytes <= 0 {
		return nil, fmt.Errorf("shardMaxBytes must be positive, got %d", m.shardMaxBytes)
	}

	// Default sizer
	if m.sizer == nil {
		var v V
		switch any(v).(type) {
		case string:
			m.sizer = any(ByteSizer[string](func(v string) int { return len(v) })).(ByteSizer[V])
		case []byte:
			m.sizer = any(ByteSizer[[]byte](func(v []byte) int { return len(v) })).(ByteSizer[V])
		default:
			return nil, fmt.Errorf("sizer must be set for type %T, use WithShardByteSizer option", v)
		}
	}

	m.shards = make([]byteShard[K, V], m.shardCount)
	for i := range m.shards {
		m.shards[i] = byteShard[K, V]{
			items:    make(map[K]*byteEntry[K, V]),
			maxBytes: m.shardMaxBytes,
			sizer:    m.sizer,
			onEvict:  m.onEvict,
		}
	}
	return m, nil
}

func (lru *ByteShardLRU[K, V]) getShard(key K) *byteShard[K, V] {
	h := maphash.Comparable(lru.seed, key)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return &lru.shards[h&uint64(lru.shardMask)]
}

// Get retrieves a value and promotes it to the front of its shard's LRU.
func (lru *ByteShardLRU[K, V]) Get(key K) (V, bool) {
	var zero V
	s := lru.getShard(key)
	s.mu.RLock()
	_, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return zero, false
	}
	s.mu.Lock()
	// double-check: entry may have been removed between RUnlock and Lock
	if e, ok := s.items[key]; ok {
		s.moveToFront(e)
		value := e.value
		s.mu.Unlock()
		return value, true
	}
	s.mu.Unlock()
	return zero, false
}

// Set stores a key-value pair, evicting LRU entries if the shard exceeds its byte budget.
func (lru *ByteShardLRU[K, V]) Set(key K, value V) {
	var evictedEntries []struct {
		k K
		v V
	}

	s := lru.getShard(key)
	s.mu.Lock()
	newSize := s.sizer(value)
	if newSize < 0 || newSize > s.maxBytes {
		s.mu.Unlock()
		return
	}

	if entry, ok := s.items[key]; ok {
		oldSize := entry.size
		s.curBytes.Add(int64(newSize - oldSize))
		entry.value = value
		entry.size = newSize
		s.moveToFront(entry)
		evictedEntries = s.evictIfNeededLocked()
		s.mu.Unlock()
	} else {
		entry := &byteEntry[K, V]{
			key:   key,
			value: value,
			size:  newSize,
		}
		for s.curBytes.Load()+int64(newSize) > int64(s.maxBytes) && s.tail != nil {
			evk, evv, _ := s.evictOneLocked()
			evictedEntries = append(evictedEntries, struct {
				k K
				v V
			}{k: evk, v: evv})
		}
		if s.head == nil {
			s.head = entry
			s.tail = entry
		} else {
			entry.next = s.head
			s.head.prev = entry
			s.head = entry
		}
		s.items[key] = entry
		s.curBytes.Add(int64(newSize))
		s.mu.Unlock()
	}

	if s.onEvict != nil {
		for _, e := range evictedEntries {
			s.onEvict(e.k, e.v)
		}
	}
}

// Delete removes a key from the cache.
func (lru *ByteShardLRU[K, V]) Delete(key K) bool {
	s := lru.getShard(key)
	s.mu.Lock()
	entry, ok := s.items[key]
	if !ok {
		s.mu.Unlock()
		return false
	}
	var evk K
	var evv V
	var hasEvict bool
	if s.onEvict != nil {
		evk, evv = entry.key, entry.value
		hasEvict = true
	}
	s.removeEntry(entry)
	s.mu.Unlock()

	if hasEvict && s.onEvict != nil {
		s.onEvict(evk, evv)
	}
	return true
}

// EvictExpired removes all entries for which shouldEvict returns true.
// Each shard is locked independently to minimize contention.
func (lru *ByteShardLRU[K, V]) EvictExpired(shouldEvict func(V) bool) int {
	total := 0
	for i := range lru.shards {
		s := &lru.shards[i]
		var evictedEntries []struct {
			k K
			v V
		}
		s.mu.Lock()
		for k, entry := range s.items {
			if shouldEvict(entry.value) {
				if s.onEvict != nil {
					evictedEntries = append(evictedEntries, struct {
						k K
						v V
					}{k: k, v: entry.value})
				}
				s.removeEntry(entry)
			}
		}
		s.mu.Unlock()
		total += len(evictedEntries)
		if s.onEvict != nil {
			for _, e := range evictedEntries {
				s.onEvict(e.k, e.v)
			}
		}
	}
	return total
}

// Contains checks if a key exists without updating access order.
func (lru *ByteShardLRU[K, V]) Contains(key K) bool {
	s := lru.getShard(key)
	s.mu.RLock()
	_, ok := s.items[key]
	s.mu.RUnlock()
	return ok
}

// Len returns the total number of entries across all shards.
func (lru *ByteShardLRU[K, V]) Len() int {
	total := 0
	for i := range lru.shards {
		s := &lru.shards[i]
		s.mu.RLock()
		total += len(s.items)
		s.mu.RUnlock()
	}
	return total
}

// Stats returns aggregated statistics across all shards.
func (lru *ByteShardLRU[K, V]) Stats() (count int, curBytes int64, maxBytes int) {
	var c int
	var cb int64
	for i := range lru.shards {
		s := &lru.shards[i]
		s.mu.RLock()
		c += len(s.items)
		s.mu.RUnlock()
		cb += s.curBytes.Load()
	}
	return c, cb, lru.shardCount * lru.shardMaxBytes
}

// Clear removes all entries from all shards.
func (lru *ByteShardLRU[K, V]) Clear() {
	for i := range lru.shards {
		s := &lru.shards[i]
		var evictedEntries []struct {
			k K
			v V
		}
		s.mu.Lock()
		if s.onEvict != nil {
			evictedEntries = make([]struct {
				k K
				v V
			}, 0, len(s.items))
			for k, entry := range s.items {
				evictedEntries = append(evictedEntries, struct {
					k K
					v V
				}{k: k, v: entry.value})
				entry.prev, entry.next = nil, nil
			}
		}
		s.items = make(map[K]*byteEntry[K, V])
		s.head = nil
		s.tail = nil
		s.curBytes.Store(0)
		s.mu.Unlock()

		if s.onEvict != nil {
			for _, e := range evictedEntries {
				s.onEvict(e.k, e.v)
			}
		}
	}
}

// --- shard-internal methods ---

func (s *byteShard[K, V]) moveToFront(entry *byteEntry[K, V]) {
	if entry == nil || s.head == nil {
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

func (s *byteShard[K, V]) removeEntry(entry *byteEntry[K, V]) {
	if entry == nil {
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
	delete(s.items, entry.key)
	s.curBytes.Add(int64(-entry.size))
	entry.prev, entry.next = nil, nil
}

func (s *byteShard[K, V]) evictOneLocked() (K, V, int) {
	var zeroK K
	var zeroV V
	if s.tail == nil {
		return zeroK, zeroV, 0
	}
	oldest := s.tail
	k, v, size := oldest.key, oldest.value, oldest.size
	s.removeEntry(oldest)
	return k, v, size
}

func (s *byteShard[K, V]) evictIfNeededLocked() []struct {
	k K
	v V
} {
	var evicted []struct {
		k K
		v V
	}
	for s.curBytes.Load() > int64(s.maxBytes) && s.tail != nil {
		evk, evv, _ := s.evictOneLocked()
		evicted = append(evicted, struct {
			k K
			v V
		}{k: evk, v: evv})
	}
	return evicted
}
