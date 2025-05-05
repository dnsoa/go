package zmap

import (
	"testing"
	"time"
)

func TestTtlMap(t *testing.T) {
	m := NewTtlMap(WithTTL[int, string](1*time.Second), WithCleanupInterval[int, string](time.Second))
	m.Set(1, "one")
	v, ok := m.Get(1)
	if !ok || v != "one" {
		t.Errorf("expected value to be 'one', got %s", v)
	}
	if m.Len() != 1 {
		t.Errorf("expected length to be 1, got %d", m.Len())
	}
	time.Sleep(time.Second * 2)
	if m.Len() != 0 {
		t.Errorf("expected length to be 0, got %d", m.Len())
	}
	_, ok = m.Get(1)
	if ok {
		t.Errorf("expected value to be expired")
	}
}

func BenchmarkTtlMap(b *testing.B) {
	m := NewTtlMap[int, string]()
	b.ResetTimer()
	mockTime := time.Now()
	m.nowFn = func() time.Time {
		return mockTime
	}
	for i := 0; i < b.N; i++ {
		m.Set(i, "value")
	}
}
