package modules_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRoleCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		service *role.Service
		wantNil bool
	}{
		{
			name:    "success - wraps registry",
			service: role.NewService(),
			wantNil: false,
		},
		{
			name:    "success - nil service yields nil",
			service: nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := modules.NewRoleCapabilities(tt.service)
			if tt.wantNil {
				assert.Nil(t, adapter)
				return
			}
			require.NotNil(t, adapter)
			assert.True(t, adapter.IsAdmin("Admin"))
			assert.False(t, adapter.IsAdmin("Contributor"))
			assert.True(t, adapter.CanPublish("Contributor"))
			assert.False(t, adapter.CanMedia("Commentator"))
			assert.True(t, adapter.CanComment("Commentator"))
			assert.NotEmpty(t, adapter.ManageablePostTypes("Admin", []string{"post"}))
			names := make([]string, 0)
			for _, role := range adapter.GetAll() {
				names = append(names, role.Name)
			}
			assert.Contains(t, names, "Admin")
		})
	}
}
