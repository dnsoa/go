package fasttime

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestUnixDate(t *testing.T) {
	dateExpected := time.Now().Unix() / (24 * 3600)
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
	hourExpected := time.Now().Unix() / 3600
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
	// 使用基于包内 updateInterval 的动态容差，避免固定 100ms 导致在较大更新间隔时失败
	allowed := updateInterval + 100*time.Millisecond
	if diff > allowed { // 动态容差
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
		dateDiff := UnixDate() - time.Now().Unix()/86400
		if dateDiff < 0 {
			dateDiff = -dateDiff
		}
		if dateDiff > 1 {
			t.Errorf("UnixDate() and Now().Unix()/86400 differ by %d days", dateDiff)
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

var _currentTime atomic.Int64

func startTicker(interval time.Duration) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case tm := <-ticker.C:
				_currentTime.Store(tm.UnixNano())
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

func BenchmarkHighPrecision(b *testing.B) {
	os.Setenv("FASTTIME_HIGH_PRECISION", "true")
	defer os.Unsetenv("FASTTIME_HIGH_PRECISION")

	// 预热
	runtime.GC()
	b.ResetTimer()

	// 测试不同并发度下的影响
	for _, workers := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			var stopFuncs []func()
			for i := 0; i < workers; i++ {
				stop := startTicker(updateInterval)
				stopFuncs = append(stopFuncs, stop)
			}
			defer func() {
				for _, stop := range stopFuncs {
					stop()
				}
			}()

			// 测量CPU和内存
			var memStatsStart, memStatsEnd runtime.MemStats
			var cpuStart, cpuEnd uint64

			runtime.ReadMemStats(&memStatsStart)
			start := time.Now()
			cpuStart = getCPUTime()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					// 模拟实际工作负载
					_ = currentTime.Load()
					time.Sleep(10 * time.Microsecond)
				}
			})

			cpuEnd = getCPUTime()
			runtime.ReadMemStats(&memStatsEnd)

			// 计算指标
			duration := time.Since(start).Seconds()
			cpuUsage := float64(cpuEnd-cpuStart) / (duration * 1e9) * 100 // CPU利用率百分比
			memAlloc := memStatsEnd.TotalAlloc - memStatsStart.TotalAlloc

			b.ReportMetric(cpuUsage, "CPU%")
			b.ReportMetric(float64(memAlloc)/1024/1024, "MB/op")
		})
	}
}

// 获取进程累计CPU时间（Linux/Mac兼容）
func getCPUTime() uint64 {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return 0
	}
	var cpuTime uint64
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err == nil {
		cpuTime = uint64(rusage.Utime.Sec)*1e9 + uint64(rusage.Utime.Usec)*1e3
	} else {
		cpuTime = 0
	}
	return cpuTime
}

// Sink should prevent from code elimination by optimizing compiler
var Sink atomic.Uint64
