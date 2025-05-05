package zmap

import (
	"hash/maphash"
	"iter"
	"runtime"
)

type HashMap[K comparable, V any] struct {
	shards     []ShardMap[K, V]
	shardCount int
	seed       maphash.Seed
}

type HashMapOption[K comparable, V any] func(*HashMap[K, V])

func WithShardCount[K comparable, V any](shardCount int) HashMapOption[K, V] {
	return func(m *HashMap[K, V]) {
		m.shardCount = nextPowerOfTwo(shardCount)
	}
}

func NewHashMap[K comparable, V any](options ...HashMapOption[K, V]) *HashMap[K, V] {
	m := &HashMap[K, V]{
		shardCount: nextPowerOfTwo(runtime.GOMAXPROCS(0) * 16),
		seed:       maphash.MakeSeed(),
	}
	for _, option := range options {
		option(m)
	}
	m.shards = make([]ShardMap[K, V], m.shardCount)
	for i := range m.shards {
		m.shards[i] = NewShardMap[K, V]()
	}
	return m
}

func (m *HashMap[K, V]) getShard(key K) *ShardMap[K, V] {
	hash := maphash.Comparable(m.seed, key)
	return &m.shards[int(hash)&(m.shardCount-1)]
}

func (m *HashMap[K, V]) Set(k K, v V) {
	m.getShard(k).Set(k, v)
}

func (m *HashMap[K, V]) Get(k K) (V, bool) {
	return m.getShard(k).Get(k)
}

func (m *HashMap[K, V]) Delete(k K) {
	m.getShard(k).Delete(k)
}

func (m *HashMap[K, V]) Len() int {
	total := 0
	for i := range m.shards {
		total += m.shards[i].Len()
	}
	return total
}

func (m *HashMap[K, V]) Clear() {
	for i := range m.shards {
		m.shards[i].Clear()
	}
}

func (m *HashMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := range m.shards {
			shard := &m.shards[i]

			shard.All()(func(k K, v V) bool {
				return yield(k, v)
			})
		}
	}
}
