package fasttime

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestUnixDate(t *testing.T) {
	dateExpected := time.Now().Unix() / SecondsPerDay
	date := UnixDate()
	diff := date - dateExpected
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Fatalf("unexpected UnixDate; got %d; want %d", date, dateExpected)
	}
}

func TestUnixHour(t *testing.T) {
	hourExpected := time.Now().Unix() / SecondsPerHour
	hour := UnixHour()
	diff := hour - hourExpected
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Fatalf("unexpected UnixHour; got %d; want %d", hour, hourExpected)
	}
}

func TestUnixTime(t *testing.T) {
	tsExpected := time.Now().Unix()
	ts := UnixTime()
	if ts-tsExpected > 1 {
		t.Fatalf("unexpected Unix; got %d; want %d", ts, tsExpected)
	}
	tsExpected = time.Now().UnixNano()
	ts = Now().UnixNano()
	if tsExpected-ts > 1e9 {
		t.Fatalf("unexpected UnixNano; got %d; want %d", ts, tsExpected)
	}

	time.Sleep(time.Second)
	diff := time.Since(Now())
	// Use the default update interval for tolerance
	allowed := DefaultUpdateInterval + 100*time.Millisecond
	if diff > allowed {
		t.Errorf("time is not correct %v (allowed %v)", diff, allowed)
	}
	for range 5 {
		nowUnix := time.Now().Unix()
		fastNowUnix := Now().Unix()
		d := nowUnix - fastNowUnix
		if d < 0 {
			d = -d
		}
		if d > 1 {
			t.Errorf("Unix() and Now().Unix() differ by %d seconds", d)
		}
		dateDiff := UnixDate() - time.Now().Unix()/SecondsPerDay
		if dateDiff < 0 {
			dateDiff = -dateDiff
		}
		if dateDiff > 1 {
			t.Errorf("UnixDate() and Now().Unix()/SecondsPerDay differ by %d days", dateDiff)
		}
		secDiff := time.Now().Unix() - UnixTime()
		if secDiff < 0 {
			secDiff = -secDiff
		}
		if secDiff > 1 {
			t.Errorf("Unix() =%d and Now().Unix()=%d are not equal (diff %d)", UnixTime(), time.Now().Unix(), secDiff)
		}
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(1500)))
	}
}

func TestSince(t *testing.T) {
	start := Now().Add(time.Millisecond * -100)
	diff := Since(start)
	if diff > time.Millisecond*110 { // 100ms tolerance
		t.Errorf("time is not correct %v", diff)
	}
}

func TestUntil(t *testing.T) {
	start := Now().Add(time.Millisecond * 100)
	diff := Until(start)
	if diff < time.Millisecond*90 { // 100ms tolerance
		t.Errorf("time is not correct %v", diff)
	}
}

func TestConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	const goroutines = 100
	const opsPerGoroutine = 1000

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = UnixNano()
				_ = UnixTime()
				_ = Now()
				_ = UnixDate()
				_ = UnixHour()
			}
		}()
	}
	wg.Wait()
}

func BenchmarkUnixTimestamp(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ts uint64
		for pb.Next() {
			ts += uint64(UnixTime())
		}
		Sink.Store(ts)
	})
}

func BenchmarkTimeNowUnix(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ts uint64
		for pb.Next() {
			ts += uint64(time.Now().Unix())
		}
		Sink.Store(ts)
	})
}

func BenchmarkFastTimeNow(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var sum int64
		for pb.Next() {
			sum += Now().UnixNano()
		}
		Sink.Store(uint64(sum))
	})
}

func BenchmarkTimeNow(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var sum int64
		for pb.Next() {
			sum += time.Now().UnixNano()
		}
		Sink.Store(uint64(sum))
	})
}

func BenchmarkUnixNano(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var sum int64
		for pb.Next() {
			sum += UnixNano()
		}
		Sink.Store(uint64(sum))
	})
}

// testTicker is a helper for benchmark testing that simulates additional
// background updaters by writing into package-level currentTime. This makes
// the benchmark more realistic by increasing contention on the shared atomic
// variable used by the package.
type testTicker struct {
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func startTestTicker(interval time.Duration) *testTicker {
	tt := &testTicker{
		stopCh: make(chan struct{}),
	}
	tt.wg.Add(1)
	go func() {
		defer tt.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case tm := <-ticker.C:
				currentTime.Store(tm.UnixNano())
			case <-tt.stopCh:
				return
			}
		}
	}()
	return tt
}

func (tt *testTicker) stop() {
	close(tt.stopCh)
	tt.wg.Wait()
}

func BenchmarkHighPrecision(b *testing.B) {
	os.Setenv("FASTTIME_HIGH_PRECISION", "true")
	defer os.Unsetenv("FASTTIME_HIGH_PRECISION")

	// Warmup
	runtime.GC()
	b.ResetTimer()

	// Test with different worker counts
	for _, workers := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			interval := HighPrecisionUpdateInterval
			var tickers []*testTicker
			for i := 0; i < workers; i++ {
				tickers = append(tickers, startTestTicker(interval))
			}
			defer func() {
				for _, tt := range tickers {
					tt.stop()
				}
			}()

			// Measure CPU and memory
			var memStatsStart, memStatsEnd runtime.MemStats
			var cpuStart, cpuEnd uint64

			runtime.ReadMemStats(&memStatsStart)
			start := time.Now()
			cpuStart = getCPUTime()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					// Simulate actual workload using public API
					_ = UnixNano()
					time.Sleep(10 * time.Microsecond)
				}
			})

			cpuEnd = getCPUTime()
			runtime.ReadMemStats(&memStatsEnd)

			// Calculate metrics
			duration := time.Since(start).Seconds()
			cpuUsage := float64(cpuEnd-cpuStart) / (duration * 1e9) * 100
			memAlloc := memStatsEnd.TotalAlloc - memStatsStart.TotalAlloc

			b.ReportMetric(cpuUsage, "CPU%")
			b.ReportMetric(float64(memAlloc)/1024/1024, "MB/op")
		})
	}
}

// getCPUTime returns the process cumulative CPU time (Linux/Mac compatible).
func getCPUTime() uint64 {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return 0
	}
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err == nil {
		return uint64(rusage.Utime.Sec)*1e9 + uint64(rusage.Utime.Usec)*1e3
	}
	return 0
}

// Sink prevents compiler from optimizing away benchmark code.
var Sink atomic.Uint64
