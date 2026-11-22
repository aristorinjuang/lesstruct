package modgen

import (
	"slices"
	"strings"
)

const (
	// FindingSeverityError blocks install.
	FindingSeverityError = "error"
	// FindingSeverityWarning surfaces in the UI without blocking.
	FindingSeverityWarning = "warning"
)

var (
	knownSurfaces = []string{
		"routes",
		"public_routes",
		"migrations",
		"posttypes",
		"roles",
		"themes",
		"admin",
	}
)

// IsKnownSurface reports whether surface names a generatable kernel
// capability surface.
func IsKnownSurface(surface string) bool {
	trimmed := strings.TrimSpace(surface)
	return slices.Contains(knownSurfaces, trimmed)
}

// KnownSurfaces returns the canonical generatable surface list.
func KnownSurfaces() []string {
	return slices.Clone(knownSurfaces)
}
