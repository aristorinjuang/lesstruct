package auth_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMFAService_CreateChallenge(t *testing.T) {
	tests := []struct {
		name      string
		store     *stubMFAStore
		wantErr   bool
		wantClear int
	}{
		{
			name:      "success - issues a hashed token and six-digit code",
			store:     &stubMFAStore{},
			wantErr:   false,
			wantClear: 42,
		},
		{
			name:    "error - clearing previous challenges fails",
			store:   &stubMFAStore{deleteErr: errors.New("database error")},
			wantErr: true,
		},
		{
			name:    "error - storing the challenge fails",
			store:   &stubMFAStore{createErr: errors.New("database error")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authdomain.NewMFAService(tt.store, 5*time.Minute, time.Minute, 5)

			result, err := service.CreateChallenge(context.Background(), 42)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, 42, result.UserID, "UserID")
			assert.Len(t, result.Token, 64, "Token length")
			assert.Regexp(t, `^\d{6}$`, result.Code, "Code format")
			assert.Equal(t, tt.wantClear, tt.store.userCleared, "previous challenges cleared for user")
			require.NotNil(t, tt.store.created, "stored challenge")
			assert.Equal(t, authdomain.HashToken(result.Token), tt.store.created.TokenHash, "token hash")
			assert.Equal(t, authdomain.HashToken(result.Code), tt.store.created.CodeHash, "code hash")
			assert.WithinDuration(t, time.Now(), tt.store.created.CreatedAt, time.Second, "created at")
			assert.WithinDuration(t, time.Now().Add(5*time.Minute), result.ExpiresAt, time.Second, "expiry")
		})
	}
}

func TestMFAService_VerifyChallenge(t *testing.T) {
	validChallenge := func(attempts int, expiresAt time.Time) *authdomain.MFAChallenge {
		return &authdomain.MFAChallenge{
			ID:        7,
			UserID:    42,
			CodeHash:  authdomain.HashToken("123456"),
			Attempts:  attempts,
			ExpiresAt: expiresAt,
		}
	}

	tests := []struct {
		name           string
		store          *stubMFAStore
		token          string
		code           string
		wantUser       int
		wantIncrements int
		wantDeletes    int
		wantErr        error
		wantGenericErr bool
	}{
		{
			name:           "success - matching code consumes the challenge",
			store:          &stubMFAStore{challenge: validChallenge(0, time.Now().Add(time.Minute))},
			token:          "challenge-token",
			code:           "123456",
			wantUser:       42,
			wantIncrements: 1,
			wantDeletes:    1,
		},
		{
			name:           "error - a concurrent verifier already consumed the challenge",
			store:          &stubMFAStore{challenge: validChallenge(0, time.Now().Add(time.Minute)), consumeMiss: true},
			token:          "challenge-token",
			code:           "123456",
			wantIncrements: 1,
			wantErr:        authdomain.ErrMFAChallengeInvalid,
		},
		{
			name:    "error - unknown challenge",
			store:   &stubMFAStore{},
			token:   "unknown",
			code:    "123456",
			wantErr: authdomain.ErrMFAChallengeInvalid,
		},
		{
			name:        "error - expired challenge is discarded",
			store:       &stubMFAStore{challenge: validChallenge(0, time.Now().Add(-time.Minute))},
			token:       "challenge-token",
			code:        "123456",
			wantDeletes: 1,
			wantErr:     authdomain.ErrMFAChallengeExpired,
		},
		{
			name:        "error - attempt budget already exhausted",
			store:       &stubMFAStore{challenge: validChallenge(5, time.Now().Add(time.Minute))},
			token:       "challenge-token",
			code:        "123456",
			wantDeletes: 1,
			wantErr:     authdomain.ErrMFACodeMaxAttempts,
		},
		{
			name:           "error - wrong code spends one attempt",
			store:          &stubMFAStore{challenge: validChallenge(0, time.Now().Add(time.Minute))},
			token:          "challenge-token",
			code:           "000000",
			wantIncrements: 1,
			wantErr:        authdomain.ErrMFACodeInvalid,
		},
		{
			name:           "error - final wrong code discards the challenge",
			store:          &stubMFAStore{challenge: validChallenge(4, time.Now().Add(time.Minute))},
			token:          "challenge-token",
			code:           "000000",
			wantIncrements: 1,
			wantDeletes:    1,
			wantErr:        authdomain.ErrMFACodeMaxAttempts,
		},
		{
			name:           "error - lookup fails",
			store:          &stubMFAStore{findErr: errors.New("database error")},
			token:          "challenge-token",
			code:           "123456",
			wantGenericErr: true,
		},
		{
			name:           "error - recording the attempt fails",
			store:          &stubMFAStore{challenge: validChallenge(0, time.Now().Add(time.Minute)), updateErr: errors.New("database error")},
			token:          "challenge-token",
			code:           "000000",
			wantGenericErr: true,
		},
		{
			name:           "error - consuming the challenge fails",
			store:          &stubMFAStore{challenge: validChallenge(0, time.Now().Add(time.Minute)), deleteErr: errors.New("database error")},
			token:          "challenge-token",
			code:           "123456",
			wantGenericErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authdomain.NewMFAService(tt.store, 5*time.Minute, time.Minute, 5)

			userID, err := service.VerifyChallenge(context.Background(), tt.token, tt.code)

			if tt.wantGenericErr {
				require.Error(t, err)
				assert.NotErrorIs(t, err, authdomain.ErrMFAChallengeInvalid)
				assert.NotErrorIs(t, err, authdomain.ErrMFAChallengeExpired)
				assert.NotErrorIs(t, err, authdomain.ErrMFACodeInvalid)
				assert.NotErrorIs(t, err, authdomain.ErrMFACodeMaxAttempts)
				return
			}

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, 0, userID, "UserID on failure")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser, userID, "UserID")
			}

			assert.Equal(t, tt.wantIncrements, tt.store.incremented, "attempt increments")
			assert.Equal(t, tt.wantDeletes, tt.store.deleted, "challenge deletions")
		})
	}
}

func TestMFAService_ResendChallenge(t *testing.T) {
	tests := []struct {
		name    string
		store   *stubMFAStore
		wantErr error
		wantNew bool
	}{
		{
			name: "success - replaces the challenge after the cooldown",
			store: &stubMFAStore{challenge: &authdomain.MFAChallenge{
				ID:        7,
				UserID:    42,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			}},
			wantNew: true,
		},
		{
			name: "error - expired challenge cannot be reissued",
			store: &stubMFAStore{challenge: &authdomain.MFAChallenge{
				ID:        7,
				UserID:    42,
				CreatedAt: time.Now().Add(-10 * time.Minute),
				ExpiresAt: time.Now().Add(-5 * time.Minute),
			}},
			wantErr: authdomain.ErrMFAChallengeExpired,
		},
		{
			name: "error - resend arrives inside the cooldown",
			store: &stubMFAStore{challenge: &authdomain.MFAChallenge{
				ID:        7,
				UserID:    42,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Minute),
			}},
			wantErr: authdomain.ErrMFAResendTooSoon,
		},
		{
			name:    "error - unknown challenge",
			store:   &stubMFAStore{},
			wantErr: authdomain.ErrMFAChallengeInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authdomain.NewMFAService(tt.store, 5*time.Minute, 60*time.Second, 5)

			result, err := service.ResendChallenge(context.Background(), "challenge-token")

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, tt.wantNew, "new challenge expected")
			assert.Equal(t, 42, result.UserID, "UserID")
			assert.Regexp(t, `^\d{6}$`, result.Code, "Code format")
			assert.Equal(t, 42, tt.store.userCleared, "previous challenges cleared")
		})
	}
}

func TestMFAService_ResendChallenge_LookupFailure(t *testing.T) {
	service := authdomain.NewMFAService(&stubMFAStore{findErr: errors.New("database error")}, 5*time.Minute, time.Minute, 5)

	_, err := service.ResendChallenge(context.Background(), "challenge-token")

	require.Error(t, err)
	assert.NotErrorIs(t, err, authdomain.ErrMFAChallengeInvalid)
	assert.NotErrorIs(t, err, authdomain.ErrMFAResendTooSoon)
}

func TestMFAService_ChallengeUserID(t *testing.T) {
	validChallenge := func(expiresAt time.Time) *authdomain.MFAChallenge {
		return &authdomain.MFAChallenge{
			ID:        7,
			UserID:    42,
			ExpiresAt: expiresAt,
		}
	}

	tests := []struct {
		name           string
		store          *stubMFAStore
		wantUser       int
		wantDeletes    int
		wantErr        error
		wantGenericErr bool
	}{
		{
			name:     "success - resolves the challenged user",
			store:    &stubMFAStore{challenge: validChallenge(time.Now().Add(time.Minute))},
			wantUser: 42,
		},
		{
			name:    "error - unknown challenge",
			store:   &stubMFAStore{},
			wantErr: authdomain.ErrMFAChallengeInvalid,
		},
		{
			name:        "error - expired challenge is discarded",
			store:       &stubMFAStore{challenge: validChallenge(time.Now().Add(-time.Minute))},
			wantDeletes: 1,
			wantErr:     authdomain.ErrMFAChallengeExpired,
		},
		{
			name:           "error - lookup fails",
			store:          &stubMFAStore{findErr: errors.New("database error")},
			wantGenericErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authdomain.NewMFAService(tt.store, 5*time.Minute, time.Minute, 5)

			userID, err := service.ChallengeUserID(context.Background(), "challenge-token")

			if tt.wantGenericErr {
				require.Error(t, err)
				assert.NotErrorIs(t, err, authdomain.ErrMFAChallengeInvalid)
				assert.NotErrorIs(t, err, authdomain.ErrMFAChallengeExpired)
				return
			}

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, 0, userID, "UserID on failure")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser, userID, "UserID")
			}

			assert.Equal(t, tt.wantDeletes, tt.store.deleted, "challenge deletions")
		})
	}
}

func TestMFAService_CreateChallenge_EntropyFailures(t *testing.T) {
	tests := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "token generation failure",
			reader: &failAfterReader{err: errors.New("entropy failure")},
		},
		{
			name:   "code generation failure",
			reader: &failAfterReader{data: make([]byte, 32), err: errors.New("entropy failure")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := authdomain.SetRandReaderForTest(tt.reader)
			defer restore()

			service := authdomain.NewMFAService(&stubMFAStore{}, 5*time.Minute, time.Minute, 5)

			result, err := service.CreateChallenge(context.Background(), 42)

			require.Error(t, err)
			assert.Nil(t, result)
		})
	}
}
