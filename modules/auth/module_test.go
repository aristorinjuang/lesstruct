package auth_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/auth"
	"github.com/aristorinjuang/lesstruct/modules/auth/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, auth.Name, module.Name())
	assert.Equal(t, auth.Version, module.Version())
	assert.Equal(t, []string{"users"}, module.Requires())
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
		opts []auth.Option
	}{
		{
			name: "success - bare module builds handlers",
			opts: nil,
		},
		{
			name: "success - full options build configured handlers",
			opts: []auth.Option{
				auth.WithAuthService(nil),
				auth.WithFirstLoginService(nil),
				auth.WithRegistrationService(nil),
				auth.WithVerificationService(nil),
				auth.WithLoginService(nil),
				auth.WithPasswordResetService(nil),
				auth.WithTokenIssuer(nil),
				auth.WithUserStore(nil),
				auth.WithFailedLoginStore(nil),
				auth.WithNotificationRecorder(nil),
				auth.WithMailer(nil),
				auth.WithBlockedEmailChecker(nil),
				auth.WithCredentialPolicy(nil),
				auth.WithLogger(nil),
				auth.WithAdminApprovalRequired(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := auth.New(tt.opts...)

			require.NotNil(t, module.AuthHandler())
			require.NotNil(t, module.FirstLoginHandler())
		})
	}
}
