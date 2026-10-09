package conformance_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/conformance"
	"github.com/stretchr/testify/assert"
)

func stubAllowAdmin(principal kernel.Principal, capability string) bool {
	return capability == "admin" && principal.ID == "admin-id"
}

func stubAllowAdminRole(principal kernel.Principal, capability string) bool {
	return capability == "admin" && principal.HasRole("Admin")
}

func TestCheckAuthorizer(t *testing.T) {
	fixture := conformance.AuthorizerFixture{
		Admin:     kernel.Principal{ID: "admin-id", Username: "admin", Roles: []string{"Admin"}},
		NonAdmin:  kernel.Principal{ID: "user-1", Username: "ed", Roles: []string{"User"}},
		Anonymous: kernel.Principal{},
	}

	tests := []struct {
		name    string
		resolve kernel.CapabilityResolver
	}{
		{
			name:    "id based implementation satisfies the contract",
			resolve: stubAllowAdmin,
		},
		{
			name:    "role based implementation satisfies the contract",
			resolve: stubAllowAdminRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conformance.CheckAuthorizer(t, tt.resolve, fixture)
			assert.True(t, true, "contract suite passed")
		})
	}
}
