package fasttime

import (
	"os"
	"sync/atomic"
	"time"
)

var updateInterval = func() time.Duration {
	if os.Getenv("FASTTIME_HIGH_PRECISION") == "true" {
		return time.Millisecond * 10
	}
	return 200 * time.Millisecond
}()

// currentTime holds unix nano timestamp updated periodically
var currentTime atomic.Int64

// nowFunc holds the function used to obtain current time. Tests can replace it.
var nowFunc atomic.Value // stores func() time.Time

func defaultNow() time.Time {
	return time.Unix(0, currentTime.Load()).In(time.Local)
}

func init() {
	// initialize currentTime and nowFunc
	currentTime.Store(time.Now().UnixNano())
	nowFunc.Store(func() time.Time { return defaultNow() })

	go func() {
		ticker := time.NewTicker(updateInterval)
		defer ticker.Stop()
		for tm := range ticker.C {
			currentTime.Store(tm.UnixNano())
		}
	}()
}

// SetNowFunc sets a custom function to produce current time (useful for tests).
func SetNowFunc(f func() time.Time) {
	nowFunc.Store(f)
}

// ResetNowFunc restores the default (cached) now function.
func ResetNowFunc() {
	nowFunc.Store(func() time.Time { return defaultNow() })
}

// Now returns current time in Local timezone
func Now() time.Time {
	f := nowFunc.Load().(func() time.Time)
	return f()
}

// UnixNano returns the current unix timestamp in nanoseconds.
//
// It is faster than time.Now().UnixNano()
func UnixNano() int64 {
	f := nowFunc.Load().(func() time.Time)
	return f().UnixNano()
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
	// Use unix-nano arithmetic to avoid mixing underlying Now implementations
	return time.Duration(t.UnixNano() - UnixNano())
}
