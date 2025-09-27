package sync

import (
	"sync"
)

// OnceFunc returns a function wrapping f which ensures f is only executed once.
// If f is nil, it panics.
// The returned function may block if called concurrently until f completes.
func OnceFunc(f func()) func() {
	if f == nil {
		panic("nil function provided")
	}
	var once sync.Once
	return func() { once.Do(f) }
}

// OnceValue wraps a function returning a value, ensuring it's called only once.
func OnceValue[T any](f func() T) func() T {
	if f == nil {
		panic("nil function provided")
	}
	var once sync.Once
	var result T
	return func() T {
		once.Do(func() { result = f() })
		return result
	}
}
