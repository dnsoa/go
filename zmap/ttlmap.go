package zmap

import (
	"runtime"
	"time"
)

const (
	defaultTTL = 15 * time.Minute
)

type ttlValue[V any] struct {
	value V
	exp   time.Time
}
type TtlMap[K comparable, V any] struct {
	janitor         *janitor
	hashMap         *HashMap[K, ttlValue[V]]
	nowFn           func() time.Time
	defaultTTL      time.Duration
	cleanupInterval time.Duration
}

type TtlMapOption[K comparable, V any] func(*TtlMap[K, V])

func WithTTL[K comparable, V any](ttl time.Duration) TtlMapOption[K, V] {
	return func(m *TtlMap[K, V]) {
		m.defaultTTL = ttl
	}
}

func WithNowFn[K comparable, V any](nowFn func() time.Time) TtlMapOption[K, V] {
	return func(m *TtlMap[K, V]) {
		m.nowFn = nowFn
	}
}

func WithCleanupInterval[K comparable, V any](interval time.Duration) TtlMapOption[K, V] {
	return func(m *TtlMap[K, V]) {
		m.cleanupInterval = interval
	}
}

func NewTtlMap[K comparable, V any](options ...TtlMapOption[K, V]) *TtlMap[K, V] {
	m := &TtlMap[K, V]{
		hashMap:         NewHashMap[K, ttlValue[V]](),
		defaultTTL:      defaultTTL,
		nowFn:           time.Now,
		cleanupInterval: time.Minute,
	}
	for _, option := range options {
		option(m)
	}
	runJanitor(m, m.cleanupInterval)
	runtime.SetFinalizer(m, stopJanitor)
	return m
}

func (m *TtlMap[K, V]) Set(k K, v V) {
	m.SetWithTTL(k, v, m.defaultTTL)
}

func (m *TtlMap[K, V]) SetWithTTL(k K, v V, ttl time.Duration) {
	m.hashMap.Set(k, ttlValue[V]{value: v, exp: m.nowFn().Add(ttl)})
}

func (m *TtlMap[K, V]) Get(k K) (V, bool) {
	ttlValue, ok := m.hashMap.Get(k)
	if !ok {
		return ttlValue.value, false
	}
	if m.nowFn().After(ttlValue.exp) {
		m.Delete(k)
		return ttlValue.value, false
	}
	return ttlValue.value, true
}

func (m *TtlMap[K, V]) Delete(k K) {
	m.hashMap.Delete(k)
}

func (m *TtlMap[K, V]) Length() int {
	return m.hashMap.Length()
}

func (m *TtlMap[K, V]) DeleteExpired() {
	currentTime := m.nowFn()
	for i := range m.hashMap.shards {
		shard := &m.hashMap.shards[i]
		expiredKeys := []K{}

		shard.lock.RLock()
		for k, v := range shard.items {
			if v.exp.Before(currentTime) {
				expiredKeys = append(expiredKeys, k)
			}
		}
		shard.lock.RUnlock()

		if len(expiredKeys) > 0 {
			shard.lock.Lock()
			for _, k := range expiredKeys {
				delete(shard.items, k)
			}
			shard.lock.Unlock()
		}
	}
}

func (m *TtlMap[K, V]) SetJanitor(j *janitor) {
	m.janitor = j
}

func (m *TtlMap[K, V]) Janitor() *janitor {
	return m.janitor
}
