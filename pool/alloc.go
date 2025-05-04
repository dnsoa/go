// MIT License
//
// # Copyright (c) 2016-2017 xtaci
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
	"errors"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultMaxBits = 18 // 支持到256K
	DefaultMaxSize = 1 << DefaultMaxBits
)

type Allocator struct {
	buffers   []sync.Pool
	maxBits   byte
	maxSize   int
	zeroOnPut bool    // 回收时是否清零
	objCounts []int64 // 每个池的对象数量
	cleanStop chan struct{}
	cleanOnce sync.Once
}

func NewAllocator(opts ...func(*Allocator)) *Allocator {
	alloc := &Allocator{
		maxBits: DefaultMaxBits,
		maxSize: DefaultMaxSize,
	}
	for _, opt := range opts {
		opt(alloc)
	}
	alloc.buffers = make([]sync.Pool, alloc.maxBits+1)
	alloc.objCounts = make([]int64, alloc.maxBits+1)
	for k := range alloc.buffers {
		k := k
		size := 1 << uint32(k)
		alloc.buffers[k].New = func() any {
			b := make([]byte, size)
			return &b
		}
	}
	return alloc
}

// 可选：设置回收时是否清零
func WithZeroOnPut(zero bool) func(*Allocator) {
	return func(a *Allocator) { a.zeroOnPut = zero }
}

// 可选： 设置自动清理时间
func WithAutoClean(interval time.Duration) func(*Allocator) {
	return func(a *Allocator) {
		if interval <= 0 {
			interval = time.Hour
		}
		a.StartAutoClean(interval)
	}
}

// Get 返回一个合适大小的 []byte 指针
func (alloc *Allocator) Get(size int) *[]byte {
	if size <= 0 {
		panic("Size is negative")
	}
	if size > alloc.maxSize {
		b := make([]byte, size)
		return &b
	}
	bits := bits.Len(uint(size))
	if size == 1<<(bits-1) {
		bits--
	}

	p := alloc.buffers[bits].Get().(*[]byte)
	*p = (*p)[:size]
	atomic.AddInt64(&alloc.objCounts[bits], 1)
	return p
}

// GetBytes 直接返回 []byte，简化调用
func (alloc *Allocator) GetBytes(size int) []byte {
	p := alloc.Get(size)
	return *p
}

// Put 回收 []byte 指针到池
func (alloc *Allocator) Put(p *[]byte) error {
	if p == nil {
		return errors.New("allocator Put() nil pointer")
	}
	c := cap(*p)
	if c == 0 {
		return errors.New("allocator Put() incorrect buffer size")
	}
	if c > alloc.maxSize {
		// 超过最大池管理范围，直接忽略
		return nil
	}
	bits := bits.Len(uint(c)) - 1
	if c != 1<<bits {
		return errors.New("allocator Put() buffer cap must be 2^n")
	}
	if alloc.zeroOnPut {
		for i := range *p {
			(*p)[i] = 0
		}
	}
	alloc.buffers[bits].Put(p)
	atomic.AddInt64(&alloc.objCounts[bits], -1)
	return nil
}

// 统计当前内存占用（单位：字节）
func (alloc *Allocator) CurrentBytes() int64 {
	var total int64
	for k := range alloc.objCounts {
		count := atomic.LoadInt64(&alloc.objCounts[k])
		if count > 0 {
			total += int64(1<<uint32(k)) * count
		}
	}
	return total
}

func (alloc *Allocator) StartAutoClean(interval time.Duration) {
	alloc.cleanOnce.Do(func() {
		alloc.cleanStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					for k := range alloc.buffers {
						k := k
						size := 1 << uint32(k)
						alloc.buffers[k].New = func() any {
							b := make([]byte, size)
							return &b
						}
						// 触发GC，丢弃旧对象,只有重置 `New` 字段并发生 GC，才会清理池中未被引用的对象。
						alloc.buffers[k].Put(nil)
						atomic.StoreInt64(&alloc.objCounts[k], 0)
					}
				case <-alloc.cleanStop:
					return
				}
			}
		}()
	})
}

func (alloc *Allocator) StopAutoClean() {
	if alloc.cleanStop != nil {
		close(alloc.cleanStop)
	}
}
