// Package sync provides synchronization utilities extensions beyond the standard library.
package sync

import (
	"sync"
)

// resettable is an interface for types that can be reset for reuse.
type resettable interface {
	Reset()
}

// Pool is a generic sync.Pool that optionally resets values before reuse.
// T must be a type that implements resettable (has a Reset method).
type Pool[T resettable] struct {
	pool sync.Pool
	New  func() T
}

// NewPool creates a new Pool with the given constructor function.
func NewPool[T resettable](new func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{New: func() any { return new() }},
		New:  new,
	}
}

// Put returns value to the pool, resetting it first.
// The caller should ensure value is not the zero value for T.
func (p *Pool[T]) Put(value T) {
	value.Reset()
	p.pool.Put(value)
}

// Get returns a value from the pool or creates a new one via New.
func (p *Pool[T]) Get() T {
	if v := p.pool.Get(); v != nil {
		return v.(T)
	}
	return p.New()
}
