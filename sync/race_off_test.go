//go:build !race

package sync

// raceEnabled reports whether the binary was built with the race detector.
const raceEnabled = false
