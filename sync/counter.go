package sync

import (
	"math"
	"sync/atomic"
)

// Counter is a concurrency safe counter.
type Counter struct {
	v uint64
}

// NewCounter creates a new counter.
func NewCounter() *Counter {
	return &Counter{}
}

// Value returns the current value.
func (c *Counter) Value() int {
	val := atomic.LoadUint64(&c.v)
	if val > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(val)
}

// IncBy increments the counter by given delta.
func (c *Counter) IncBy(add uint) {
	atomic.AddUint64(&c.v, uint64(add))
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	c.IncBy(1)
}

// DecBy decrements the counter by given delta.
func (c *Counter) DecBy(dec uint) {
	atomic.AddUint64(&c.v, ^uint64(dec-1))
}

// Dec decrements the counter by 1.
func (c *Counter) Dec() {
	c.DecBy(1)
}
