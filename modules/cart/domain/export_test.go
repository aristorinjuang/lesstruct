package cart

import "io"

// SetRandSourceForTest swaps the token entropy source and returns a
// restore function. Tests cover the mint failure branch with a failing
// reader; production always runs on crypto/rand.
func SetRandSourceForTest(source io.Reader) (restore func()) {
	previous := randSource
	randSource = source
	return func() { randSource = previous }
}
