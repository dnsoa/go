package zmap

import (
	"iter"
	"maps"
	"math/bits"
	"sync"
)

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

func (s *ShardMap[K, V]) Length() int {
	s.lock.RLock()
	total := len(s.items)
	s.lock.RUnlock()
	return total
}

func (s *ShardMap[K, V]) Clear() {
	s.lock.Lock()
	s.items = make(map[K]V)
	s.lock.Unlock()
}

func (s *ShardMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		s.lock.RLock()
		localItems := make(map[K]V, len(s.items))
		maps.Copy(localItems, s.items)
		s.lock.RUnlock()

		for k, v := range localItems {
			if !yield(k, v) {
				return
			}
		}
	}
}

func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	return 1 << bits.Len(uint(n)-1)
}
