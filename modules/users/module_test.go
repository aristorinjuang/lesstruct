package users_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/users"
	"github.com/aristorinjuang/lesstruct/modules/users/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, users.Name, module.Name())
	assert.Equal(t, users.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestModule_Handlers(t *testing.T) {
	tests := []struct {
		name string
		opts []users.Option
	}{
		{
			name: "success - bare module builds handlers",
			opts: nil,
		},
		{
			name: "success - full options build configured handlers",
			opts: []users.Option{
				users.WithUserManagementService(nil),
				users.WithAdminCreateUserService(nil),
				users.WithProfileService(nil),
				users.WithAccountDeletionService(nil),
				users.WithProfilePictureService(nil),
				users.WithUserStore(nil),
				users.WithSoftDeleteStore(nil),
				users.WithMailer(nil),
				users.WithUserFieldsProvider(nil),
				users.WithProfilePictureURLBuilder(nil),
				users.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := users.New(tt.opts...)

			require.NotNil(t, module.UserManagementHandler())
			require.NotNil(t, module.ProfileHandler())
			require.NotNil(t, module.ProfilePictureHandler())
		})
	}
}
