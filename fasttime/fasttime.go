package fasttime

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var updateInterval = func() time.Duration {
	if os.Getenv("FASTTIME_HIGH_PRECISION") == "true" {
		return time.Millisecond * 10
	}
	return 200 * time.Millisecond
}()

func init() {
	go func() {
		ticker := time.NewTicker(updateInterval)
		defer ticker.Stop()
		for tm := range ticker.C {
			currentTime.Store(tm.UnixNano())
		}
	}()
}

var currentTime = func() *atomic.Int64 {
	var x atomic.Int64
	t := time.Now()
	x.Store(t.UnixNano())
	return &x
}()

// Now returns current time in Local timezone
func Now() time.Time {
	if testing.Testing() {
		// When executing inside the tests, use the time package directly.
		// This allows to override time using synctest package.
		return time.Now()
	}
	return time.Unix(0, UnixNano()).In(time.Local)
}

// UnixNano returns the current unix timestamp in nanoseconds.
//
// It is faster than time.Now().UnixNano()
func UnixNano() int64 {
	if testing.Testing() {
		return time.Now().UnixNano()
	}
	return currentTime.Load()
}

// UnixTime returns the current unix timestamp in seconds.
//
// The timestamp is calculated by dividing unix timestamp in nanoseconds by 1e9
func UnixTime() int64 {
	return UnixNano() / 1e9
}

// UnixDate returns date from the current unix timestamp.
//
// The date is calculated by dividing unix timestamp by (24*3600)
func UnixDate() int64 {
	return UnixTime() / (24 * 3600)
}

// UnixHour returns hour from the current unix timestamp.
//
// The hour is calculated by dividing unix timestamp by 3600
func UnixHour() int64 {
	return UnixTime() / 3600
}

// Since returns the time elapsed since t.
// It is shorthand for time.Now().Sub(t).
func Since(t time.Time) time.Duration {
	return time.Duration(UnixNano() - t.UnixNano())
}

// Until returns the duration until t.
// It is shorthand for t.Sub(time.Now()).
func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}
