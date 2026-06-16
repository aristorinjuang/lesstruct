package thumbnail

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidMaxWidth is returned when max_width is not a positive integer.
	ErrInvalidMaxWidth = errors.New("max_width must be an integer greater than 0")
	// ErrInvalidSuffix is returned when suffix is empty or does not start with '_'.
	ErrInvalidSuffix = errors.New("suffix must be non-empty and start with '_'")
	// ErrDuplicateSuffix is returned when suffix is duplicated across config entries.
	ErrDuplicateSuffix = errors.New("suffix must be unique across all thumbnail entries")
	// ErrSuffixNotFound is returned when no thumbnail config matches the given suffix.
	ErrSuffixNotFound = errors.New("thumbnail config not found for suffix")
)

// ThumbnailConfig represents a single thumbnail size configuration.
type ThumbnailConfig struct {
	MaxWidth int    `toml:"max_width"`
	Suffix   string `toml:"suffix"`
}

// Validate checks the ThumbnailConfig fields.
func (tc ThumbnailConfig) Validate() error {
	if tc.MaxWidth <= 0 {
		return ErrInvalidMaxWidth
	}

	suffix := strings.TrimSpace(tc.Suffix)
	if suffix == "" || !strings.HasPrefix(suffix, "_") {
		return fmt.Errorf("%w: %q", ErrInvalidSuffix, tc.Suffix)
	}

	return nil
}

// ValidateUnique checks configs for duplicate suffixes.
func ValidateUnique(configs []ThumbnailConfig) error {
	seen := make(map[string]bool, len(configs))
	for _, c := range configs {
		s := strings.TrimSpace(c.Suffix)
		if seen[s] {
			return fmt.Errorf("%w: %q", ErrDuplicateSuffix, s)
		}
		seen[s] = true
	}
	return nil
}
