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

package pool

import (
	"math/bits"
	"math/rand/v2"
	"testing"
	"time"
)

func TestAlloc(t *testing.T) {
	alloc := NewAllocator(WithZeroOnPut(true))
	t.Log(alloc.CurrentBytes())
	for range 10 {
		alloc.Get(DefaultMaxSize)
	}
	if alloc.CurrentBytes() != DefaultMaxSize*10 {
		t.Fatal("CurrentBytes() misbehavior")
	}
	alloc.StartAutoClean(time.Second)
	defer alloc.StopAutoClean()
	time.Sleep(1 * time.Second)
	if alloc.CurrentBytes() != 0 {
		t.Fatal("CurrentBytes() misbehavior")
	}
	a := alloc.GetBytes(3)
	if len(a) != 3 {
		t.Fatal("GetBytes() misbehavior")
	}
	if cap(a) != 4 {
		t.Fatal("GetBytes() misbehavior")
	}
	a[0] = 1
	a[1] = 2
	a[2] = 3
	alloc.Put(&a)

	if a[0] != 0 || a[1] != 0 || a[2] != 0 {
		t.Fatal("Put() misbehavior")
	}
}

func TestAllocGet(t *testing.T) {
	alloc := NewAllocator()
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
	alloc := NewAllocator()
	if err := alloc.Put(nil); err == nil {
		t.Fatal("put nil misbehavior")
	}
	b := make([]byte, 3)
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
	alloc := NewAllocator()
	data := alloc.Get(4)
	alloc.Put(data)
	newData := alloc.Get(4)
	if cap(*data) != cap(*newData) {
		t.Fatal("different cap while alloc.Get()")
	}
}

var (
	debruijinPos = [...]byte{0, 9, 1, 10, 13, 21, 2, 29, 11, 14, 16, 18, 22, 25, 3, 30, 8, 12, 20, 28, 15, 17, 24, 7, 19, 27, 23, 6, 26, 5, 4, 31}
)

// msb return the pos of most significiant bit
func msb(size int) byte {
	v := uint32(size)
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	return debruijinPos[(v*0x07C4ACDD)>>27]
}

func BenchmarkMSB(b *testing.B) {
	b.Run("msb", func(b *testing.B) {
		for b.Loop() {
			msb(rand.Int())
		}
	})
	b.Run("bits.Len", func(b *testing.B) {
		for b.Loop() {
			bits.Len(uint(rand.Int()))
		}
	})
}

func BenchmarkAlloc(b *testing.B) {
	alloc := NewAllocator()
	for i := 0; b.Loop(); i++ {
		size := i % (DefaultMaxSize + 1)
		if size == 0 {
			size = 1
		}
		pbuf := alloc.Get(size)
		alloc.Put(pbuf)
	}
}
