package conformance

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/require"
)

// CheckAdminExtension verifies the structural contract of an
// AdminExtensionProvider: a non-nil provider with a name and version, a
// valid manifest, and a deterministic manifest (the shell renders it
// without rebuilding). Third-party module authors call it from their own
// module test; ValidateAdminExtension exposes the same check in
// error form for non-testify harnesses.
func CheckAdminExtension(t *testing.T, provider kernel.AdminExtensionProvider) {
	t.Helper()
	require.NoError(t, ValidateAdminExtension(provider))
}

// ValidateAdminExtension is the error-returning core of CheckAdminExtension:
// every failure mode is a descriptive error.
func ValidateAdminExtension(provider kernel.AdminExtensionProvider) error {
	if provider == nil {
		return errors.New("conformance: provider must not be nil")
	}
	if provider.Name() == "" {
		return errors.New("conformance: module name must be non-empty")
	}
	if provider.Version() == "" {
		return errors.New("conformance: module version must be non-empty")
	}
	manifest := provider.AdminManifest()
	if err := manifest.Validate(); err != nil {
		return err
	}
	if !reflect.DeepEqual(manifest, provider.AdminManifest()) {
		return errors.New("conformance: manifest must be deterministic across calls")
	}
	return nil
}
