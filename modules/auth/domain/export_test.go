package auth

import "io"

// SetRandReaderForTest swaps the package entropy source and returns a restore
// function, so tests can force generation failures deterministically.
func SetRandReaderForTest(reader io.Reader) func() {
	previous := randReader
	randReader = reader
	return func() {
		randReader = previous
	}
}
