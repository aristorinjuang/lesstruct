package media

import "time"

// SetNowForTest pins the package clock and returns a restore function, so
// table rows can exercise weekday and month boundaries deterministically.
func SetNowForTest(fn func() time.Time) func() {
	previous := now
	now = fn
	return func() {
		now = previous
	}
}
