// Package conformance ships embeddable test suites that third-party module
// authors run against their kernel-contract implementations.
package conformance

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AuthorizerFixture labels the principals a resolver implementation is
// checked against. Admin must hold the "admin" capability; NonAdmin must
// not; Anonymous (the zero Principal) must hold nothing.
type AuthorizerFixture struct {
	Admin     kernel.Principal
	NonAdmin  kernel.Principal
	Anonymous kernel.Principal
}

// CheckAuthorizer verifies the structural contract of a CapabilityResolver:
// determinism, fail-closed anonymous/unknown handling, and the admin grant.
// Capability names beyond "admin" are implementation-defined and not probed.
func CheckAuthorizer(
	t *testing.T,
	resolve kernel.CapabilityResolver,
	fixture AuthorizerFixture,
) {
	t.Helper()
	require.NotNil(t, resolve)

	assert.Equal(t, resolve(fixture.Admin, "admin"), resolve(fixture.Admin, "admin"), "resolver must be deterministic")

	assert.True(t, resolve(fixture.Admin, "admin"), "admin principal must hold the admin capability")

	assert.False(t, resolve(fixture.NonAdmin, "admin"), "non-admin principal must not hold the admin capability")

	for _, capability := range []string{"admin", "media", "comments", "content.publish", "content.manage:post", "unknown-capability"} {
		assert.False(t, resolve(fixture.Anonymous, capability), "anonymous must not hold %q", capability)
	}

	for _, principal := range []kernel.Principal{fixture.Admin, fixture.NonAdmin} {
		assert.False(t, resolve(principal, "unknown-capability"), "unknown capabilities must fail closed")
	}
}
