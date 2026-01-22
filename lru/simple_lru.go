package lru

import "sync/atomic"

type SimpleLRU[K comparable, V any] struct {
	capacity int
	size     atomic.Int32
	items    map[K]*lruEntry[K, V]
	head     *lruEntry[K, V] // 最近使用的在头部
	tail     *lruEntry[K, V] // 最久未使用的在尾部
	onEvict  func(K, V)
}

func NewSimpleLRU[K comparable, V any](capacity int, onEvict func(K, V)) *SimpleLRU[K, V] {
	if capacity <= 0 {
		capacity = defaultCapacity
	}

	return &SimpleLRU[K, V]{
		capacity: capacity,
		items:    make(map[K]*lruEntry[K, V]),
		onEvict:  onEvict,
	}
}

func (lru *SimpleLRU[K, V]) Size() int {
	return int(lru.size.Load())
}

func (lru *SimpleLRU[K, V]) Capacity() int {
	return lru.capacity
}

func (lru *SimpleLRU[K, V]) OnEvict() func(K, V) {
	return lru.onEvict
}

func (lru *SimpleLRU[K, V]) SetOnEvict(onEvict func(K, V)) {
	lru.onEvict = onEvict
}

func (lru *SimpleLRU[K, V]) Clear() {
	for k, entry := range lru.items {
		if lru.onEvict != nil {
			lru.onEvict(k, entry.value)
		}
	}
	lru.items = make(map[K]*lruEntry[K, V])
	lru.head = nil
	lru.tail = nil
	lru.size.Store(0)
}

// Get 获取键对应的值，如果存在则将节点移到链表头部
func (lru *SimpleLRU[K, V]) Get(key K) (V, bool) {
	var zero V
	entry, ok := lru.items[key]
	if !ok {
		return zero, false
	}

	lru.moveToFront(entry)
	return entry.value, true
}

// Set 设置键值对，如果键已存在则更新值并移到头部
func (lru *SimpleLRU[K, V]) Set(key K, value V) {
	if entry, ok := lru.items[key]; ok {
		entry.value = value
		lru.moveToFront(entry)
		return
	}

	entry := &lruEntry[K, V]{
		key:   key,
		value: value,
	}

	if lru.head == nil {
		lru.head = entry
		lru.tail = entry
	} else {
		entry.next = lru.head
		lru.head.prev = entry
		lru.head = entry
	}

	lru.items[key] = entry
	lru.size.Add(1)

	if int(lru.size.Load()) > lru.capacity {
		lru.evict()
	}
}

// Delete 删除键值对
func (lru *SimpleLRU[K, V]) Delete(key K) bool {
	entry, ok := lru.items[key]
	if !ok {
		return false
	}

	var evk K
	var evv V
	if lru.onEvict != nil {
		evk, evv = entry.key, entry.value
	}

	lru.removeEntry(entry)

	if lru.onEvict != nil {
		lru.onEvict(evk, evv)
	}
	return true
}

// Contains 检查键是否存在（不更新访问顺序）
func (lru *SimpleLRU[K, V]) Contains(key K) bool {
	_, ok := lru.items[key]
	return ok
}

// Len 返回当前缓存中的项目数（别名方法）
func (lru *SimpleLRU[K, V]) Len() int {
	return int(lru.size.Load())
}

// moveToFront 将节点移到链表头部
func (lru *SimpleLRU[K, V]) moveToFront(entry *lruEntry[K, V]) {
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
func (lru *SimpleLRU[K, V]) removeEntry(entry *lruEntry[K, V]) {
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
	lru.size.Add(-1)
}

// evict 淘汰最久未使用的条目
func (lru *SimpleLRU[K, V]) evict() {
	if lru.tail == nil {
		return
	}

	oldest := lru.tail
	var evk K
	var evv V
	if lru.onEvict != nil {
		evk, evv = oldest.key, oldest.value
	}

	lru.removeEntry(oldest)

	if lru.onEvict != nil {
		lru.onEvict(evk, evv)
	}
}
