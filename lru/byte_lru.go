package lru

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ByteSizer 用于计算值的字节大小
type ByteSizer[V any] func(V) int

// ByteLRUOption 是 ByteLRU 的选项函数
type ByteLRUOption[K comparable, V any] func(*ByteLRU[K, V]) error

// byteEntry 是 ByteLRU 缓存中的节点
type byteEntry[K comparable, V any] struct {
	key   K
	value V
	size  int // 该条目的字节大小
	prev  *byteEntry[K, V]
	next  *byteEntry[K, V]
}

// ByteLRU 是一个基于字节数的 LRU 缓存
// 当总字节数超过限制时，移除最久未使用的条目，直到可以插入新条目
type ByteLRU[K comparable, V any] struct {
	mu       sync.RWMutex
	maxBytes int          // 最大字节数
	curBytes atomic.Int64 // 当前字节数
	items    map[K]*byteEntry[K, V]
	head     *byteEntry[K, V] // 最近使用的在头部
	tail     *byteEntry[K, V] // 最久未使用的在尾部
	sizer    ByteSizer[V]     // 计算值大小的函数
	onEvict  func(K, V)
}

// WithMaxBytes 设置最大字节数限制
func WithMaxBytes[K comparable, V any](maxBytes int) ByteLRUOption[K, V] {
	return func(lru *ByteLRU[K, V]) error {
		if maxBytes <= 0 {
			return fmt.Errorf("maxBytes must be positive, got %d", maxBytes)
		}
		lru.maxBytes = maxBytes
		return nil
	}
}

// WithSizer 设置字节大小计算函数
func WithSizer[K comparable, V any](sizer ByteSizer[V]) ByteLRUOption[K, V] {
	return func(lru *ByteLRU[K, V]) error {
		if sizer == nil {
			return fmt.Errorf("sizer must not be nil")
		}
		lru.sizer = sizer
		return nil
	}
}

// WithByteOnEvict 设置淘汰回调函数
func WithByteOnEvict[K comparable, V any](onEvict func(K, V)) ByteLRUOption[K, V] {
	return func(lru *ByteLRU[K, V]) error {
		lru.onEvict = onEvict
		return nil
	}
}

// NewByteLRU 创建一个基于字节数的 LRU 缓存
// 必须通过 WithMaxBytes 和 WithSizer 选项设置必要参数
func NewByteLRU[K comparable, V any](options ...ByteLRUOption[K, V]) (*ByteLRU[K, V], error) {
	lru := &ByteLRU[K, V]{
		items: make(map[K]*byteEntry[K, V]),
	}

	for _, option := range options {
		if err := option(lru); err != nil {
			return nil, err
		}
	}

	// 验证必要参数
	if lru.maxBytes <= 0 {
		return nil, fmt.Errorf("maxBytes must be set and positive, use WithMaxBytes option")
	}

	// 如果 sizer 为 nil，尝试使用默认 sizer
	if lru.sizer == nil {
		var v V
		switch any(v).(type) {
		case string:
			lru.sizer = any(ByteSizer[string](func(v string) int { return len(v) })).(ByteSizer[V])
		case []byte:
			lru.sizer = any(ByteSizer[[]byte](func(v []byte) int { return len(v) })).(ByteSizer[V])
		default:
			return nil, fmt.Errorf("sizer must be set for type %T, use WithSizer option", v)
		}
	}

	return lru, nil
}

// MaxBytes 返回最大字节数限制
func (lru *ByteLRU[K, V]) MaxBytes() int {
	return lru.maxBytes
}

// CurBytes 返回当前已使用的字节数
func (lru *ByteLRU[K, V]) CurBytes() int64 {
	return lru.curBytes.Load()
}

// Len 返回当前缓存中的条目数
func (lru *ByteLRU[K, V]) Len() int {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return len(lru.items)
}

// OnEvict 获取淘汰回调函数
func (lru *ByteLRU[K, V]) OnEvict() func(K, V) {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return lru.onEvict
}

// SetOnEvict 设置淘汰回调函数
func (lru *ByteLRU[K, V]) SetOnEvict(onEvict func(K, V)) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.onEvict = onEvict
}

// Clear 清空缓存
func (lru *ByteLRU[K, V]) Clear() {
	var evicted []struct {
		k K
		v V
	}

	lru.mu.Lock()
	onEvict := lru.onEvict
	if onEvict != nil {
		evicted = make([]struct {
			k K
			v V
		}, 0, len(lru.items))
		for k, entry := range lru.items {
			evicted = append(evicted, struct {
				k K
				v V
			}{k: k, v: entry.value})
		}
	}
	lru.items = make(map[K]*byteEntry[K, V])
	lru.head = nil
	lru.tail = nil
	lru.curBytes.Store(0)
	lru.mu.Unlock()

	if onEvict != nil {
		for _, e := range evicted {
			onEvict(e.k, e.v)
		}
	}
}

// Get 获取键对应的值，如果存在则将节点移到链表头部
func (lru *ByteLRU[K, V]) Get(key K) (V, bool) {
	var zero V
	lru.mu.Lock()
	defer lru.mu.Unlock()
	entry, ok := lru.items[key]
	if !ok {
		return zero, false
	}

	lru.moveToFront(entry)
	return entry.value, true
}

// Set 设置键值对
// 如果键已存在则更新值并移到头部
// 如果总大小超过限制，则淘汰最久未使用的条目直到可以插入
func (lru *ByteLRU[K, V]) Set(key K, value V) {
	var evictedEntries []struct {
		k K
		v V
	}

	lru.mu.Lock()
	onEvict := lru.onEvict
	newSize := lru.sizer(value)

	// 如果大小为负数或超过最大限制，直接返回不插入
	if newSize < 0 || newSize > lru.maxBytes {
		lru.mu.Unlock()
		return
	}

	// 如果键已存在，更新值并调整位置和大小
	if entry, ok := lru.items[key]; ok {
		oldSize := entry.size
		lru.curBytes.Add(int64(newSize - oldSize))
		entry.value = value
		entry.size = newSize
		lru.moveToFront(entry)
		// 如果新值更大，可能需要淘汰
		evictedEntries = lru.evictIfNeededLocked()
		lru.mu.Unlock()
	} else {
		// 新条目，需要淘汰直到有足够空间
		entry := &byteEntry[K, V]{
			key:   key,
			value: value,
			size:  newSize,
		}

		// 先淘汰直到有足够空间
		for lru.curBytes.Load()+int64(newSize) > int64(lru.maxBytes) && lru.tail != nil {
			evk, evv, evSize := lru.evictOneLocked()
			if onEvict != nil {
				evictedEntries = append(evictedEntries, struct {
					k K
					v V
				}{k: evk, v: evv})
			}
			_ = evSize // 避免未使用变量警告
		}

		// 插入新条目
		if lru.head == nil {
			lru.head = entry
			lru.tail = entry
		} else {
			entry.next = lru.head
			lru.head.prev = entry
			lru.head = entry
		}

		lru.items[key] = entry
		lru.curBytes.Add(int64(newSize))
		lru.mu.Unlock()
	}

	// 在锁外调用回调
	if onEvict != nil {
		for _, e := range evictedEntries {
			onEvict(e.k, e.v)
		}
	}
}

// Delete 删除键值对
func (lru *ByteLRU[K, V]) Delete(key K) bool {
	var evk K
	var evv V
	var evicted bool

	lru.mu.Lock()
	onEvict := lru.onEvict
	entry, ok := lru.items[key]
	if !ok {
		lru.mu.Unlock()
		return false
	}
	if onEvict != nil {
		evk, evv = entry.key, entry.value
		evicted = true
	}
	lru.removeEntry(entry)
	lru.mu.Unlock()

	if evicted && onEvict != nil {
		onEvict(evk, evv)
	}
	return true
}

// Contains 检查键是否存在（不更新访问顺序）
func (lru *ByteLRU[K, V]) Contains(key K) bool {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	_, ok := lru.items[key]
	return ok
}

// moveToFront 将节点移到链表头部
func (lru *ByteLRU[K, V]) moveToFront(entry *byteEntry[K, V]) {
	if entry == nil || lru.head == nil {
		return
	}

	if entry == lru.head {
		return
	}

	// 从原位置移除
	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
	if entry == lru.tail {
		lru.tail = entry.prev
	}

	// 移到头部
	entry.prev = nil
	entry.next = lru.head
	lru.head.prev = entry
	lru.head = entry
}

// removeEntry 从链表和映射中移除条目
func (lru *ByteLRU[K, V]) removeEntry(entry *byteEntry[K, V]) {
	if entry == nil {
		return
	}

	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		lru.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		lru.tail = entry.prev
	}

	delete(lru.items, entry.key)
	lru.curBytes.Add(int64(-entry.size))
}

// evictOneLocked 淘汰最久未使用的条目（调用时必须持有锁）
func (lru *ByteLRU[K, V]) evictOneLocked() (K, V, int) {
	var zeroK K
	var zeroV V
	if lru.tail == nil {
		return zeroK, zeroV, 0
	}

	oldest := lru.tail
	k, v, size := oldest.key, oldest.value, oldest.size
	lru.removeEntry(oldest)
	return k, v, size
}

// evictIfNeededLocked 如果当前大小超过限制，淘汰条目直到在限制内（调用时必须持有锁）
func (lru *ByteLRU[K, V]) evictIfNeededLocked() []struct {
	k K
	v V
} {
	var evicted []struct {
		k K
		v V
	}
	for lru.curBytes.Load() > int64(lru.maxBytes) && lru.tail != nil {
		evk, evv, _ := lru.evictOneLocked()
		evicted = append(evicted, struct {
			k K
			v V
		}{k: evk, v: evv})
	}
	return evicted
}

// Stats 返回缓存统计信息
func (lru *ByteLRU[K, V]) Stats() (count int, curBytes int64, maxBytes int) {
	lru.mu.RLock()
	count = len(lru.items)
	lru.mu.RUnlock()
	curBytes = lru.curBytes.Load()
	maxBytes = lru.maxBytes
	return
}

// EvictExpired removes all entries for which shouldEvict returns true.
// Deletion and eviction callbacks happen under a single Lock acquisition.
// Returns the number of entries removed.
func (lru *ByteLRU[K, V]) EvictExpired(shouldEvict func(V) bool) int {
	var evictedEntries []struct {
		k K
		v V
	}

	lru.mu.Lock()
	onEvict := lru.onEvict
	for k, entry := range lru.items {
		if shouldEvict(entry.value) {
			if onEvict != nil {
				evictedEntries = append(evictedEntries, struct {
					k K
					v V
				}{k: k, v: entry.value})
			}
			lru.removeEntry(entry)
		}
	}
	lru.mu.Unlock()

	if onEvict != nil {
		for _, e := range evictedEntries {
			onEvict(e.k, e.v)
		}
	}
	return len(evictedEntries)
}
