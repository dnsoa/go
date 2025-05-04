package maps

import (
	"testing"
	"unsafe"
)

func TestRuntimeStructAlignment(t *testing.T) {
	a := any((map[int]struct{})(nil))
	eface := (*rtEface)(unsafe.Pointer(&a))
	if eface.typ.Hasher == nil {
		t.Fatal("Go runtime structure incompatible!")
	}
	hash := eface.typ.Hasher(unsafe.Pointer(&a), uintptr(0))
	if hash == 0 {
		t.Fatal("Go runtime structure incompatible!")
	}
	t.Log(hash)
}

func TestHashMap(t *testing.T) {
	m := NewHashMap[int, string]()
	m.Set(1, "one")
	v, ok := m.Get(1)
	if !ok || v != "one" {
		t.Errorf("expected value to be 'one', got %s", v)
	}
	m.Delete(1)
	_, ok = m.Get(1)
	if ok {
		t.Errorf("expected value to be deleted")
	}
}

func BenchmarkHashMap(b *testing.B) {
	m := NewHashMap[int, string]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, "value")
	}
}

func BenchmarkHashMapWithHasher(b *testing.B) {
	m := NewHashMap[int, string](WithHasher[int, string](func(key unsafe.Pointer, seed uintptr) uintptr {
		return uintptr(key)
	}))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, "value")
	}
}

func BenchmarkShardMap(b *testing.B) {
	m := NewShardMap[int, string]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, "value")
	}
}
