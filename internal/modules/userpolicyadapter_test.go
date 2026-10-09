package modules_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserCredentialPolicy(t *testing.T) {
	tests := []struct {
		name    string
		check   func(userdomain.CredentialPolicy) error
		wantErr bool
	}{
		{
			name: "success - valid email passes",
			check: func(p userdomain.CredentialPolicy) error {
				return p.ValidateEmail("test@example.com")
			},
			wantErr: false,
		},
		{
			name: "error - invalid email fails",
			check: func(p userdomain.CredentialPolicy) error {
				return p.ValidateEmail("not-an-email")
			},
			wantErr: true,
		},
		{
			name: "success - valid username passes",
			check: func(p userdomain.CredentialPolicy) error {
				return p.ValidateUsername("testuser")
			},
			wantErr: false,
		},
		{
			name: "error - invalid username fails",
			check: func(p userdomain.CredentialPolicy) error {
				return p.ValidateUsername("invalid user!")
			},
			wantErr: true,
		},
		{
			name: "success - strong password passes",
			check: func(p userdomain.CredentialPolicy) error {
				return p.ValidatePassword("StrongPassword123!")
			},
			wantErr: false,
		},
		{
			name: "error - weak password fails",
			check: func(p userdomain.CredentialPolicy) error {
				return p.ValidatePassword("weak")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := modules.NewUserCredentialPolicy()

			err := tt.check(policy)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestUserCredentialPolicy_PasswordRoundTrip(t *testing.T) {
	policy := modules.NewUserCredentialPolicy()

	password, err := policy.GeneratePassword(16)
	require.NoError(t, err)
	assert.Len(t, password, 16)

	hash, err := policy.HashPassword("CurrentPassword123!")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	require.NoError(t, policy.VerifyPassword(hash, "CurrentPassword123!"))
	require.Error(t, policy.VerifyPassword(hash, "WrongPassword123!"))
}

func TestUserCredentialPolicy_ValidationSentinels(t *testing.T) {
	policy := modules.NewUserCredentialPolicy()

	// The policy translates platform validation failures to the auth
	// module's sentinels. Messages mirror internal/auth byte-for-byte
	// (the first-login handler surfaces them on the wire); the equality
	// pins the mirror so either side rewording fails loudly.
	err := policy.ValidateEmail("not-an-email")
	require.Error(t, err)
	assert.ErrorIs(t, err, authdomain.ErrInvalidEmail)
	assert.Equal(t, auth.ErrEmailInvalid.Error(), err.Error())

	err = policy.ValidatePassword("weak")
	require.Error(t, err)
	assert.ErrorIs(t, err, authdomain.ErrInvalidPassword)
	assert.Equal(t, auth.ErrPasswordInvalid.Error(), err.Error())
}
