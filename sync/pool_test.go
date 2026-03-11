package sync

import (
	"bytes"
	"sync"
	"testing"
)

// mockResettable is a test type that implements resettable.
type mockResettable struct {
	data     string
	resetCnt int
}

func (m *mockResettable) Reset() {
	m.data = ""
	m.resetCnt++
}

func TestPool_Basic(t *testing.T) {
	pool := NewPool(func() *mockResettable {
		return &mockResettable{data: "new"}
	})

	// First Get creates a new value
	v1 := pool.Get()
	if v1.data != "new" {
		t.Errorf("expected data='new', got %q", v1.data)
	}

	// Modify the value
	v1.data = "modified"
	v1.resetCnt = 5

	// Put it back
	pool.Put(v1)
	if v1.resetCnt != 6 {
		t.Errorf("expected Reset() to be called, resetCnt=%d", v1.resetCnt)
	}
	if v1.data != "" {
		t.Errorf("expected data to be reset, got %q", v1.data)
	}

	// Get should return the same value (reset)
	v2 := pool.Get()
	if v2 != v1 {
		t.Error("expected to get the same value from pool")
	}
	if v2.data != "" {
		t.Errorf("expected data to remain reset, got %q", v2.data)
	}
	if v2.resetCnt != 6 {
		t.Errorf("resetCnt should not increase on Get, got %d", v2.resetCnt)
	}
}

func TestPool_Concurrent(t *testing.T) {
	pool := NewPool(func() *bytes.Buffer {
		return &bytes.Buffer{}
	})

	const goroutines = 100
	const getsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < getsPerGoroutine; j++ {
				buf := pool.Get()
				buf.WriteString("test")
				pool.Put(buf)
			}
		}()
	}

	wg.Wait()

	// Get a buffer and verify it's been reset
	buf := pool.Get()
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer after reset, got len=%d", buf.Len())
	}
	if buf.String() != "" {
		t.Errorf("expected empty string after reset, got %q", buf.String())
	}
}

func TestPool_AlwaysCreatesNew(t *testing.T) {
	createCnt := 0
	pool := NewPool(func() *mockResettable {
		createCnt++
		return &mockResettable{data: "new"}
	})

	// Multiple gets without puts should create new values each time
	pool.Get()
	pool.Get()
	pool.Get()

	if createCnt != 3 {
		t.Errorf("expected 3 creations, got %d", createCnt)
	}
}

func TestPool_Reuse(t *testing.T) {
	createCnt := 0
	pool := NewPool(func() *mockResettable {
		createCnt++
		return &mockResettable{data: "new"}
	})

	// Get and put to seed the pool
	v := pool.Get()
	pool.Put(v)

	// Get again should reuse, not create new
	v2 := pool.Get()
	if createCnt != 1 {
		t.Errorf("expected 1 creation (reused), got %d", createCnt)
	}
	if v2 != v {
		t.Error("expected to reuse the same value")
	}
}

func TestPool_NilNewPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when New is nil")
		}
	}()

	pool := NewPool[*mockResettable](nil)
	_ = pool.New()
}

// Test with a concrete type that has Reset method
type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Reset() {
	if b.Buffer != nil {
		b.Buffer.Reset()
	}
}

func TestPool_ConcreteType(t *testing.T) {
	pool := NewPool(func() *buffer {
		return &buffer{Buffer: &bytes.Buffer{}}
	})

	b := pool.Get()
	b.WriteString("test data")
	pool.Put(b)

	b2 := pool.Get()
	if b2.String() != "" {
		t.Errorf("expected reset buffer, got %q", b2.String())
	}
}

func BenchmarkPool_GetPut(b *testing.B) {
	pool := NewPool(func() *bytes.Buffer {
		return &bytes.Buffer{}
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.Grow(1024)
		pool.Put(buf)
	}
}

func BenchmarkPool_GetPut_Parallel(b *testing.B) {
	pool := NewPool(func() *bytes.Buffer {
		return &bytes.Buffer{}
	})

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.Grow(1024)
			pool.Put(buf)
		}
	})
}

func BenchmarkPool_NoPool(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		buf.Grow(1024)
		// No pooling, just let it be GC'd
	}
}
