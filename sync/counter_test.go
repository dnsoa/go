package sync

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCounterBasic(t *testing.T) {
	c := NewCounter()
	if v := c.Value(); v != 0 {
		t.Fatalf("initial value = %d, want 0", v)
	}

	c.Inc()
	if v := c.Value(); v != 1 {
		t.Fatalf("after Inc value = %d, want 1", v)
	}

	c.IncBy(4)
	if v := c.Value(); v != 5 {
		t.Fatalf("after IncBy(4) value = %d, want 5", v)
	}

	c.Dec()
	if v := c.Value(); v != 4 {
		t.Fatalf("after Dec value = %d, want 4", v)
	}

	c.DecBy(2)
	if v := c.Value(); v != 2 {
		t.Fatalf("after DecBy(2) value = %d, want 2", v)
	}
}

func TestCounterConcurrent(t *testing.T) {
	c := NewCounter()
	const goroutines = 100
	const perG = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()
	want := goroutines * perG
	if v := c.Value(); v != want {
		t.Fatalf("concurrent result = %d, want %d", v, want)
	}
}

func TestCounterOverflow(t *testing.T) {
	c := NewCounter()
	// 设置为超过 math.MaxInt 的值，Value 应当返回 math.MaxInt 而不是溢出为负数
	atomic.StoreUint64(&c.v, uint64(math.MaxInt)+100)
	if v := c.Value(); v != math.MaxInt {
		t.Fatalf("overflow value = %d, want %d", v, math.MaxInt)
	}
}
