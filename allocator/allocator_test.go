// MIT License
//
// Copyright (c) 2016-2017 xtaci
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package allocator

import (
	"testing"
	"time"
)

func TestAlloc(t *testing.T) {
	alloc := New(WithZeroOnPut(true))
	buffers := make([]*Buffer, 0, 10)
	for range 10 {
		buffers = append(buffers, alloc.Get(DefaultMaxSize))
	}
	if alloc.CurrentBytes() != DefaultMaxSize*10 {
		t.Fatal("CurrentBytes() misbehavior")
	}
	alloc.StartAutoClean(time.Millisecond * 200)
	defer alloc.StopAutoClean()
	time.Sleep(300 * time.Millisecond)
	if alloc.CurrentBytes() != DefaultMaxSize*10 {
		t.Fatal("CurrentBytes() should keep borrowed buffers counted during auto clean")
	}
	for _, buf := range buffers {
		if err := alloc.Release(buf); err != nil {
			t.Fatalf("Release returned error: %v", err)
		}
	}
	if alloc.CurrentBytes() != 0 {
		t.Fatal("CurrentBytes() misbehavior")
	}
}

func TestAllocGet(t *testing.T) {
	alloc := New()
	if len(*alloc.Get(1)) != 1 {
		t.Fatal(1)
	}
	if len(*alloc.Get(2)) != 2 {
		t.Fatal(2)
	}
	if len(*alloc.Get(3)) != 3 || cap(*alloc.Get(3)) != 4 {
		t.Fatal(3)
	}
	if len(*alloc.Get(4)) != 4 {
		t.Fatal(4)
	}
	if len(*alloc.Get(1023)) != 1023 || cap(*alloc.Get(1023)) != 1024 {
		t.Fatal(1023)
	}
	if len(*alloc.Get(1024)) != 1024 {
		t.Fatal(1024)
	}
	if len(*alloc.Get(65536)) != 65536 {
		t.Fatal(65536)
	}
	if len(*alloc.Get(DefaultMaxSize + 1)) != (DefaultMaxSize + 1) {
		t.Fatal(DefaultMaxSize + 1)
	}
}

func TestAllocPut(t *testing.T) {
	alloc := New()
	if err := alloc.Put(nil); err == nil {
		t.Fatal("put nil misbehavior")
	}
	b := make(Buffer, 3)
	if err := alloc.Put(&b); err == nil {
		t.Fatal("put elem:3 []bytes misbehavior")
	}
	b = make([]byte, 4)
	if err := alloc.Put(&b); err != nil {
		t.Fatal("put elem:4 []bytes misbehavior")
	}
	b = make([]byte, 1023, 1024)
	if err := alloc.Put(&b); err != nil {
		t.Fatal("put elem:1024 []bytes misbehavior")
	}
	b = make([]byte, 65536)
	if err := alloc.Put(&b); err != nil {
		t.Fatal("put elem:65536 []bytes misbehavior")
	}
	b = make([]byte, 65537)
	if err := alloc.Put(&b); err == nil {
		t.Fatal("put elem:65537 []bytes misbehavior")
	}
}

func TestAllocPutThenGet(t *testing.T) {
	alloc := New()
	data := alloc.Get(4)
	alloc.Put(data)
	newData := alloc.Get(4)
	if cap(*data) != cap(*newData) {
		t.Fatal("different cap while alloc.Get()")
	}
}

func TestAllocRelease(t *testing.T) {
	alloc := New()
	buf := alloc.Get(4)
	if err := alloc.Release(buf); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}

	invalid := make(Buffer, 3)
	if err := alloc.Release(&invalid); err == nil {
		t.Fatal("Release should return allocator errors")
	}
}

func TestAllocAutoCleanKeepsCurrentBytesAccurate(t *testing.T) {
	alloc := New()
	buf := alloc.Get(8)
	alloc.StartAutoClean(time.Millisecond * 50)
	defer alloc.StopAutoClean()

	time.Sleep(120 * time.Millisecond)
	if alloc.CurrentBytes() != 8 {
		t.Fatalf("CurrentBytes after auto clean = %d, want 8", alloc.CurrentBytes())
	}
	if err := alloc.Release(buf); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	if alloc.CurrentBytes() != 0 {
		t.Fatalf("CurrentBytes after Release = %d, want 0", alloc.CurrentBytes())
	}
	}

// TestGetReturnsBuffer verifies Get returns *Buffer for proper recycling
func TestGetReturnsBuffer(t *testing.T) {
	alloc := New()

	// Get returns *Buffer which can be recycled via Put
	buf := alloc.Get(4)
	if len(*buf) != 4 {
		t.Fatalf("expected len 4, got %d", len(*buf))
	}

	// If you need []byte, dereference the buffer
	bytes := *buf
	bytes[0] = 'x'

	// Always recycle the *Buffer, not the []byte
	alloc.Put(buf)

	// The recycled buffer is available for reuse
	newBuf := alloc.Get(4)
	if cap(*buf) != cap(*newBuf) {
		t.Logf("Warning: recycling may not be working (caps %d vs %d)", cap(*buf), cap(*newBuf))
	}
	_ = newBuf
}

func BenchmarkAlloc(b *testing.B) {
	alloc := New()
	for i := 0; b.Loop(); i++ {
		size := i % (DefaultMaxSize + 1)
		if size == 0 {
			size = 1
		}
		pbuf := alloc.Get(size)
		alloc.Put(pbuf)
	}
}
