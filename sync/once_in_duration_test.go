package sync_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	syncpkg "github.com/dnsoa/go/sync"
)

func TestOnceInDurationBasic(t *testing.T) {
	var o syncpkg.OnceInDuration
	var cnt int32

	o.Do(50*time.Millisecond, func() {
		atomic.AddInt32(&cnt, 1)
	})

	// 再次立即调用不应执行
	o.Do(50*time.Millisecond, func() {
		atomic.AddInt32(&cnt, 1)
	})

	if v := atomic.LoadInt32(&cnt); v != 1 {
		t.Fatalf("expected 1 execution, got %d", v)
	}

	// 等待冷却结束后再次调用应执行
	time.Sleep(75 * time.Millisecond)
	o.Do(50*time.Millisecond, func() {
		atomic.AddInt32(&cnt, 1)
	})

	if v := atomic.LoadInt32(&cnt); v != 2 {
		t.Fatalf("expected 2 executions after cooldown, got %d", v)
	}
}

func TestOnceInDurationConcurrent(t *testing.T) {
	var o syncpkg.OnceInDuration
	var cnt int32
	var wg sync.WaitGroup
	workers := 100
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			o.Do(100*time.Millisecond, func() {
				atomic.AddInt32(&cnt, 1)
			})
		}()
	}
	wg.Wait()

	if v := atomic.LoadInt32(&cnt); v != 1 {
		t.Fatalf("expected exactly 1 execution under concurrency, got %d", v)
	}

	// 等待冷却结束后再调用一次
	time.Sleep(125 * time.Millisecond)
	o.Do(50*time.Millisecond, func() {
		atomic.AddInt32(&cnt, 1)
	})

	if v := atomic.LoadInt32(&cnt); v != 2 {
		t.Fatalf("expected 2 executions after cooldown, got %d", v)
	}
}

func TestOnceInDurationPanicSchedulesReset(t *testing.T) {
	var o syncpkg.OnceInDuration
	var cnt int32

	// 调用会 panic，但在 goroutine 中捕获 panic 以不影响测试流程
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		o.Do(50*time.Millisecond, func() {
			panic("boom")
		})
	}()
	wg.Wait()

	// 等待比 duration 稍长的时间，确保 reset 已执行
	time.Sleep(75 * time.Millisecond)

	// 现在正常执行一个函数，应当成功
	o.Do(50*time.Millisecond, func() {
		atomic.AddInt32(&cnt, 1)
	})

	if v := atomic.LoadInt32(&cnt); v != 1 {
		t.Fatalf("expected 1 execution after panic cooldown, got %d", v)
	}
}

func BenchmarkOnceInDuration_SequentialReset(b *testing.B) {
	var o syncpkg.OnceInDuration
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Do(1*time.Millisecond, func() {})
		// 确保冷却到期再下一次循环
		time.Sleep(2 * time.Millisecond)
	}
}

func BenchmarkOnceInDuration_Concurrent(b *testing.B) {
	var o syncpkg.OnceInDuration
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			o.Do(100*time.Millisecond, func() {})
		}
	})
}

func BenchmarkOnceInDuration_HitCooldown(b *testing.B) {
	var o syncpkg.OnceInDuration
	b.ReportAllocs()
	// 大多数调用都会快速命中冷却路径
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Do(10*time.Second, func() {})
	}
}
