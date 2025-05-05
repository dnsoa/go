package zmap

import "testing"

func TestShardMap(t *testing.T) {
	m := NewShardMap[int, string]()
	m.Set(1, "one")
	v, ok := m.Get(1)
	if !ok || v != "one" {
		t.Errorf("expected value to be 'one', got %s", v)
	}
	if m.Len() != 1 {
		t.Errorf("expected size to be 1, got %d", m.Len())
	}
	m.Delete(1)
	if m.Len() != 0 {
		t.Errorf("expected size to be 0, got %d", m.Len())
	}
	m.Delete(2)

	m.Set(1, "one")
	v, ok = m.Get(1)
	if !ok || v != "one" {
		t.Errorf("expected value to be 'one', got %s", v)
	}
	for i := 0; i < 1000; i++ {
		m.Set(i, "value")
	}
	if m.Len() != 1000 {
		t.Errorf("expected size to be 1000, got %d", m.Len())
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
	if m.Len() != 0 {
		t.Errorf("expected size to be 0, got %d", m.Len())
	}
	m.Set(1, "one")
	v, ok = m.Get(1)
	if !ok || v != "one" {
		t.Errorf("expected value to be 'one', got %s", v)
	}
	if m.Len() != 1 {
		t.Errorf("expected size to be 1, got %d", m.Len())
	}
}

func BenchmarkShardMap(b *testing.B) {
	m := NewShardMap[int, string]()

	for i := 0; b.Loop(); i++ {
		m.Set(i, "value")
	}
}
