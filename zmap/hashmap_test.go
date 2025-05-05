package zmap

import (
	"sync"
	"testing"
)

func TestHashMap(t *testing.T) {
	m := NewHashMap(WithShardCount[int, string](8))
	m.Set(1, "one")
	v, ok := m.Get(1)
	if !ok || v != "one" {
		t.Errorf("expected value to be 'one', got %s", v)
	}
	if m.Length() != 1 {
		t.Errorf("expected length to be 1, got %d", m.Length())
	}
	m.Delete(1)
	_, ok = m.Get(1)
	if ok {
		t.Errorf("expected value to be deleted")
	}
	if m.Length() != 0 {
		t.Errorf("expected length to be 0, got %d", m.Length())
	}
	for i := 0; i < 1000; i++ {
		m.Set(i, "value")
	}
	if m.Length() != 1000 {
		t.Errorf("expected length to be 1000, got %d", m.Length())
	}
	for k, v := range m.All() {
		if v != "value" {
			t.Errorf("expected value to be 'value', got %s", v)
		}
		if k < 0 || k >= 1000 {
			t.Errorf("expected key to be in range [0, 1000), got %d", k)
		}
	}
	m.Clear()
	if m.Length() != 0 {
		t.Errorf("expected length to be 0, got %d", m.Length())
	}
}

func BenchmarkHashMap(b *testing.B) {
	m := NewHashMap[int, string]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, "value")
	}
}

func BenchmarkShardMap(b *testing.B) {
	m := NewShardMap[int, string]()

	for i := 0; b.Loop(); i++ {
		m.Set(i, "value")
	}
}

func BenchmarkSyncMap(b *testing.B) {
	m := sync.Map{}
	for i := 0; b.Loop(); i++ {
		m.Store(i, "value")
	}
}
