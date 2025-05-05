package zmap

import (
	"iter"
	"maps"
	"math/bits"
	"sync"
)

type ShardMap[K comparable, V any] struct {
	items map[K]V
	mu    sync.RWMutex
}

func NewShardMap[K comparable, V any]() ShardMap[K, V] {
	return ShardMap[K, V]{
		mu:    sync.RWMutex{},
		items: make(map[K]V),
	}
}

func (s *ShardMap[K, V]) Get(key K) (value V, ok bool) {
	s.mu.RLock()
	value, ok = s.items[key]
	s.mu.RUnlock()
	return
}

func (s *ShardMap[K, V]) Set(key K, value V) {
	s.mu.Lock()
	s.items[key] = value
	s.mu.Unlock()
}

func (s *ShardMap[K, V]) Delete(key K) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

func (s *ShardMap[K, V]) Len() int {
	s.mu.RLock()
	total := len(s.items)
	s.mu.RUnlock()
	return total
}

func (s *ShardMap[K, V]) Clear() {
	s.mu.Lock()
	s.items = make(map[K]V)
	s.mu.Unlock()
}

func (s *ShardMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		s.mu.RLock()
		localItems := make(map[K]V, len(s.items))
		maps.Copy(localItems, s.items)
		s.mu.RUnlock()

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
