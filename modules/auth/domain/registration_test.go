package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		wantError error
	}{
		{
			name:      "Valid username with letters",
			username:  "testuser",
			wantError: nil,
		},
		{
			name:      "Valid username with numbers",
			username:  "user123",
			wantError: nil,
		},
		{
			name:      "Valid username with underscore",
			username:  "test_user",
			wantError: nil,
		},
		{
			name:      "Valid username with hyphen",
			username:  "test-user",
			wantError: nil,
		},
		{
			name:      "Valid username with mixed characters",
			username:  "Test_User-123",
			wantError: nil,
		},
		{
			name:      "Valid single character username",
			username:  "a",
			wantError: nil,
		},
		{
			name:      "Valid 50 character username",
			username:  strings.Repeat("a", 50),
			wantError: nil,
		},
		{
			name:      "Empty username",
			username:  "",
			wantError: authdomain.ErrUsernameInvalid,
		},
		{
			name:      "Username with spaces",
			username:  "test user",
			wantError: authdomain.ErrUsernameInvalid,
		},
		{
			name:      "Username with special characters",
			username:  "test@user",
			wantError: authdomain.ErrUsernameInvalid,
		},
		{
			name:      "Username with dots",
			username:  "test.user",
			wantError: authdomain.ErrUsernameInvalid,
		},
		{
			name:      "Username with 51 characters (too long)",
			username:  strings.Repeat("a", 51),
			wantError: authdomain.ErrUsernameInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authdomain.ValidateUsername(tt.username)
			if tt.wantError != nil {
				assert.ErrorIs(t, err, tt.wantError, "ValidateUsername() should return expected error")
			} else {
				assert.NoError(t, err, "ValidateUsername() should not return error")
			}
		})
	}
}

func TestRegistrationService_RegisterUser_Success(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	result, err := service.RegisterUser(context.Background(), req)
	require.NoError(t, err, "RegisterUser() should succeed")
	require.NotNil(t, result, "RegisterUser() should return result")

	assert.Equal(t, 123, result.UserID, "UserID should match")
	assert.NotEmpty(t, result.Message, "Message should not be empty")
	assert.NotNil(t, store.created, "CreateUser must persist the user")
	assert.Equal(t, "testuser", store.created.Username)
	assert.Equal(t, "pending", store.created.Status)
	assert.Equal(t, "Commentator", store.created.Role)
	assert.NotEmpty(t, store.created.PasswordHash)
}

func TestRegistrationService_RegisterUser_Consent(t *testing.T) {
	terms := legal.Document{Page: legal.PageTermsOfService, PageID: 3, Slug: "terms-of-service", Language: "en"}
	privacy := legal.Document{Page: legal.PagePrivacyPolicy, PageID: 4, Slug: "privacy-policy", Language: "en"}
	policy := authdomain.ConsentPolicy{Required: true, Terms: terms, Privacy: privacy}

	tests := []struct {
		name            string
		policy          authdomain.ConsentPolicy
		accepted        bool
		wantErr         error
		wantEvidence    bool
		wantRepoQueries bool
	}{
		{
			name:            "required consent refuses an unchecked registration before repository work",
			policy:          policy,
			accepted:        false,
			wantErr:         authdomain.ErrConsentRequired,
			wantEvidence:    false,
			wantRepoQueries: false,
		},
		{
			name:            "required consent stores the document evidence",
			policy:          policy,
			accepted:        true,
			wantErr:         nil,
			wantEvidence:    true,
			wantRepoQueries: true,
		},
		{
			name:            "a disabled gate ignores the flag and stores no evidence",
			policy:          authdomain.ConsentPolicy{},
			accepted:        false,
			wantErr:         nil,
			wantEvidence:    false,
			wantRepoQueries: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubUserStore{}
			service := authdomain.NewRegistrationService(store, stubPolicy{}, true)
			service.SetConsentPolicy(tt.policy)

			result, err := service.RegisterUser(context.Background(), authdomain.RegisterRequest{
				Username:        "testuser",
				Email:           "test@example.com",
				Password:        "SecurePassword123!",
				ConsentAccepted: tt.accepted,
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
				assert.Nil(t, store.created, "a refused registration must not create a user")
				assert.Equal(t, tt.wantRepoQueries, store.usernameChecked)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, store.created)
			if !tt.wantEvidence {
				assert.Nil(t, store.created.ConsentAt)
				assert.Empty(t, store.created.ConsentEvidence)
				return
			}
			require.NotNil(t, store.created.ConsentAt)
			assert.False(t, store.created.ConsentAt.IsZero())
			require.NotEmpty(t, store.created.ConsentEvidence)

			var evidence legal.Consent
			require.NoError(t, json.Unmarshal([]byte(store.created.ConsentEvidence), &evidence))
			assert.Equal(t, terms, evidence.Terms)
			assert.Equal(t, privacy, evidence.Privacy)
			assert.Equal(t, *store.created.ConsentAt, evidence.AcceptedAt)
		})
	}
}

func TestRegistrationService_RegisterUser_InvalidUsername(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(store, stubPolicy{usernameErr: errors.New("bad username")}, true)

	req := authdomain.RegisterRequest{
		Username: "test user",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, authdomain.ErrUsernameInvalid, "Should return ErrUsernameInvalid")
	assert.False(t, store.usernameChecked, "No repository lookup must happen after validation failure")
}

func TestRegistrationService_RegisterUser_InvalidEmail(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(store, stubPolicy{emailErr: errors.New("bad email")}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "invalidemail",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error for invalid email")
	assert.False(t, store.usernameChecked, "No repository lookup must happen after validation failure")
}

func TestRegistrationService_RegisterUser_InvalidPassword(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(store, stubPolicy{passwordErr: errors.New("weak password")}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "weak",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error for invalid password")
}

func TestRegistrationService_RegisterUser_UsernameExists(t *testing.T) {
	store := &stubUserStore{
		usernameExists: map[string]bool{"existinguser": true},
	}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "existinguser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, authdomain.ErrUsernameExists, "Should return ErrUsernameExists")
}

func TestRegistrationService_RegisterUser_EmailExists(t *testing.T) {
	store := &stubUserStore{
		emailExists: map[string]bool{"existing@example.com": true},
	}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "existing@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, authdomain.ErrEmailExists, "Should return ErrEmailExists")
}

func TestRegistrationService_RegisterUser_CreateUserFailure(t *testing.T) {
	store := &stubUserStore{createErr: errors.New("database error")}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error when CreateUser fails")
	assert.ErrorIs(t, err, authdomain.ErrRegistrationFailed, "Should wrap error in ErrRegistrationFailed")
}

func TestRegistrationService_RegisterUser_CheckUsernameExistsError(t *testing.T) {
	store := &stubUserStore{usernameErr: errors.New("database connection lost")}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error when CheckUsernameExists fails")
}

func TestRegistrationService_RegisterUser_CheckEmailExistsError(t *testing.T) {
	store := &stubUserStore{emailErr: errors.New("database connection lost")}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error when CheckEmailExists fails")
}

func TestRegistrationService_RegisterUser_CommentsDisabled(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, false)

	req := authdomain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	// No repo interaction happens: the gate rejects before any lookup.
	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, authdomain.ErrRegistrationDisabled, "Should return ErrRegistrationDisabled")
	assert.False(t, store.usernameChecked, "No repository lookup must happen when disabled")
}

func TestRegistrationService_RegistrationEnabled(t *testing.T) {
	tests := []struct {
		name            string
		commentsEnabled bool
		options         []authdomain.RegistrationOption
		expected        bool
	}{
		{
			name:            "enabled when comments are on",
			commentsEnabled: true,
			expected:        true,
		},
		{
			name:            "disabled when comments are off",
			commentsEnabled: false,
			expected:        false,
		},
		{
			name:            "explicit enabled overrides comments-off",
			commentsEnabled: false,
			options:         []authdomain.RegistrationOption{authdomain.WithEnabled(true)},
			expected:        true,
		},
		{
			name:            "explicit disabled overrides comments-on",
			commentsEnabled: true,
			options:         []authdomain.RegistrationOption{authdomain.WithEnabled(false)},
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authdomain.NewRegistrationService(nil, stubPolicy{}, tt.commentsEnabled, tt.options...)
			assert.Equal(t, tt.expected, service.RegistrationEnabled())
		})
	}
}

func TestRegistrationService_RegisterUser_CustomDefaultRole(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(
		store,
		stubPolicy{},
		true,
		authdomain.WithDefaultRole("Journalist"),
	)

	req := authdomain.RegisterRequest{
		Username: "reporter",
		Email:    "reporter@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, store.created)
	assert.Equal(t, "Journalist", store.created.Role)
	assert.Equal(t, "pending", store.created.Status)
}

func TestRegistrationService_RegisterUser_AlwaysPending(t *testing.T) {
	store := &stubUserStore{}

	service := authdomain.NewRegistrationService(store, stubPolicy{}, true)

	req := authdomain.RegisterRequest{
		Username: "quickuser",
		Email:    "quick@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, store.created)
	// Email verification is mandatory: registrants are always created pending,
	// regardless of options.
	assert.Equal(t, "pending", store.created.Status)
}

func TestRegistrationService_RegisterUser_DisabledExplicitly(t *testing.T) {
	store := &stubUserStore{}

	// Comments are on but [registration] enabled=false must win.
	service := authdomain.NewRegistrationService(store, stubPolicy{}, true, authdomain.WithEnabled(false))

	_, err := service.RegisterUser(context.Background(), authdomain.RegisterRequest{
		Username: "blocked",
		Email:    "blocked@example.com",
		Password: "SecurePassword123!",
	})
	assert.ErrorIs(t, err, authdomain.ErrRegistrationDisabled)
	assert.False(t, store.usernameChecked, "No repository lookup must happen when disabled")
}
