package maps

import "sync"

type ShardMap[K comparable, V any] struct {
	items map[K]V
	lock  sync.RWMutex
}

func NewShardMap[K comparable, V any]() ShardMap[K, V] {
	return ShardMap[K, V]{
		lock:  sync.RWMutex{},
		items: make(map[K]V),
	}
}

func (s *ShardMap[K, V]) Get(key K) (value V, ok bool) {
	s.lock.RLock()
	value, ok = s.items[key]
	s.lock.RUnlock()
	return
}

func (s *ShardMap[K, V]) Set(key K, value V) {
	s.lock.Lock()
	s.items[key] = value
	s.lock.Unlock()
}

func (s *ShardMap[K, V]) Delete(key K) {
	s.lock.Lock()
	delete(s.items, key)
	s.lock.Unlock()
}

func nextPowerOfTwo(n int) int {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}
