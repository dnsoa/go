// Package fasttime provides a high-performance alternative to time.Now() by caching
// the current time and updating it periodically in a background goroutine.
//
// The cached time reduces system call overhead significantly, making it suitable for
// high-frequency operations where exact millisecond precision is not required.
//
// By default, time is updated every 200ms (configurable via FASTTIME_HIGH_PRECISION
// environment variable). All times are returned in UTC for consistency across
// distributed systems.
package fasttime

import (
	"os"
	"sync/atomic"
	"time"
)

const (
	// DefaultUpdateInterval is the default time update interval.
	DefaultUpdateInterval = 200 * time.Millisecond

	// HighPrecisionUpdateInterval is the interval used when FASTTIME_HIGH_PRECISION is set.
	HighPrecisionUpdateInterval = 10 * time.Millisecond

	// SecondsPerDay is the number of seconds in a day.
	SecondsPerDay = 24 * 3600

	// SecondsPerHour is the number of seconds in an hour.
	SecondsPerHour = 3600

	// nanosPerSecond is the number of nanoseconds in a second.
	nanosPerSecond = int64(time.Second)
)

// currentTime holds unix nano timestamp updated periodically.
var currentTime atomic.Int64

func init() {
	interval := DefaultUpdateInterval
	if os.Getenv("FASTTIME_HIGH_PRECISION") == "true" {
		interval = HighPrecisionUpdateInterval
	}

	// Initialize with current time
	currentTime.Store(time.Now().UnixNano())
	// Start background updater.
	//
	// We read time.Now() at store-time rather than using the time delivered
	// on the ticker channel: that value is the scheduled tick time, which may
	// already be stale by up to one interval if the goroutine is woken late
	// under load. time.NewTicker is used over time.Tick because the latter's
	// underlying Ticker can never be reclaimed by the GC.
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			currentTime.Store(time.Now().UnixNano())
		}
	}()
}

// Now returns current time in UTC.
func Now() time.Time {
	return time.Unix(0, currentTime.Load()).UTC()
}

// UnixNano returns the current unix timestamp in nanoseconds.
// It is faster than time.Now().UnixNano().
func UnixNano() int64 {
	return currentTime.Load()
}

// UnixTime returns the current unix timestamp in seconds.
func UnixTime() int64 {
	return currentTime.Load() / nanosPerSecond
}

// UnixDate returns date from the current unix timestamp.
func UnixDate() int64 {
	return UnixTime() / SecondsPerDay
}

// UnixHour returns hour from the current unix timestamp.
func UnixHour() int64 {
	return UnixTime() / SecondsPerHour
}

// Since returns the time elapsed since t.
// It is shorthand for time.Now().Sub(t) but uses the cached time.
//
// Note: If t is in the future, this will return a negative duration.
// For past times, the result may be up to UpdateInterval ahead of the actual elapsed time.
func Since(t time.Time) time.Duration {
	return time.Duration(currentTime.Load() - t.UnixNano())
}

// Until returns the duration until t.
// It is shorthand for t.Sub(time.Now()) but uses the cached time.
//
// Note: If t is in the past, this will return a negative duration.
// For future times, the result may be up to UpdateInterval less than the actual remaining time.
func Until(t time.Time) time.Duration {
	return time.Duration(t.UnixNano() - currentTime.Load())
}
