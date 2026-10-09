package conformance_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/conformance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubExtensionProvider is a test double for AdminExtensionProvider.
type stubExtensionProvider struct {
	name     string
	version  string
	manifest kernel.AdminExtensionManifest
	flip     bool
	calls    int
}

func (s *stubExtensionProvider) Name() string    { return s.name }
func (s *stubExtensionProvider) Version() string { return s.version }
func (s *stubExtensionProvider) Requires() []string {
	return nil
}
func (s *stubExtensionProvider) Optional() []string {
	return nil
}
func (s *stubExtensionProvider) Init(_ kernel.Kernel) error { return nil }
func (s *stubExtensionProvider) Shutdown() error            { return nil }
func (s *stubExtensionProvider) AdminManifest() kernel.AdminExtensionManifest {
	s.calls++
	if s.flip && s.calls > 1 {
		return kernel.AdminExtensionManifest{
			MenuEntries: []kernel.AdminMenuEntry{{Label: "Other", Path: "/other"}},
		}
	}
	return s.manifest
}

func TestCheckAdminExtension(t *testing.T) {
	// The public wrapper is exercised on the pass path; failure modes are
	// pinned through the same provider shapes below via table tests on the
	// error taxonomy (a failing wrapper call would fail this suite).
	conformance.CheckAdminExtension(t, &stubExtensionProvider{
		name:    "media",
		version: "0.1.0",
		manifest: kernel.AdminExtensionManifest{
			MenuEntries: []kernel.AdminMenuEntry{{Label: "Media", Path: "/media"}},
		},
	})
}

func TestCheckAdminExtensionFailures(t *testing.T) {
	tests := []struct {
		name     string
		provider kernel.AdminExtensionProvider
		contains string
		wantErr  bool
	}{
		{
			name:     "error - nil provider",
			provider: nil,
			contains: "must not be nil",
			wantErr:  true,
		},
		{
			name:     "error - empty name",
			provider: &stubExtensionProvider{version: "0.1.0"},
			contains: "name must be non-empty",
			wantErr:  true,
		},
		{
			name:     "error - empty version",
			provider: &stubExtensionProvider{name: "media"},
			contains: "version must be non-empty",
			wantErr:  true,
		},
		{
			name: "error - invalid manifest",
			provider: &stubExtensionProvider{
				name:    "media",
				version: "0.1.0",
				manifest: kernel.AdminExtensionManifest{
					MenuEntries: []kernel.AdminMenuEntry{{Label: "Media", Path: "relative"}},
				},
			},
			contains: "must be absolute",
			wantErr:  true,
		},
		{
			name: "error - nondeterministic manifest",
			provider: &stubExtensionProvider{
				name:    "media",
				version: "0.1.0",
				flip:    true,
			},
			contains: "deterministic",
			wantErr:  true,
		},
		{
			name: "success - valid provider passes",
			provider: &stubExtensionProvider{
				name:    "media",
				version: "0.1.0",
				manifest: kernel.AdminExtensionManifest{
					MenuEntries: []kernel.AdminMenuEntry{{Label: "Media", Path: "/media"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The error form pins every failure mode; the public wrapper
			// asserts on it (covered by TestCheckAdminExtension).
			err := conformance.ValidateAdminExtension(tt.provider)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.contains)
				return
			}
			require.NoError(t, err)
		})
	}
}
