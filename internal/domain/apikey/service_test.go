package apikey_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// longName builds a name that exceeds the max length (121+ runes).
func longName() string {
	return strings.Repeat("x", 121)
}

func TestService_Create(t *testing.T) {
	tests := []struct {
		name      string
		userID    int
		keyName   string
		pepper    string
		setupMock func(*mocks.MockRepository)
		wantErr   error
	}{
		{
			name:    "success - valid name, no pepper",
			userID:  1,
			keyName: "My CI Key",
			pepper:  "",
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					CountByUserAndName(mock.Anything, 1, "My CI Key").
					Return(0, nil)
				m.EXPECT().
					Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
					Run(func(_ context.Context, key *apikey.APIKey) { key.ID = 42 }).
					Return(nil)
			},
			wantErr: nil,
		},
		{
			name:    "success - valid name, with pepper (HMAC path)",
			userID:  2,
			keyName: "Deploy Bot",
			pepper:  "super-secret-pepper",
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					CountByUserAndName(mock.Anything, 2, "Deploy Bot").
					Return(0, nil)
				m.EXPECT().
					Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
					Run(func(_ context.Context, key *apikey.APIKey) { key.ID = 7 }).
					Return(nil)
			},
			wantErr: nil,
		},
		{
			name:     "error - empty name returns ErrInvalidKeyName",
			userID:   1,
			keyName:  "",
			pepper:   "",
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:  apikey.ErrInvalidKeyName,
		},
		{
			name:     "error - whitespace-only name returns ErrInvalidKeyName",
			userID:   1,
			keyName:  "   ",
			pepper:   "",
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:  apikey.ErrInvalidKeyName,
		},
		{
			name:     "error - name exceeding 120 chars returns ErrInvalidKeyName",
			userID:   1,
			keyName:  longName(),
			pepper:   "",
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:  apikey.ErrInvalidKeyName,
		},
		{
			name:    "error - duplicate name returns ErrDuplicateKeyName",
			userID:  3,
			keyName: "dupe",
			pepper:  "",
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					CountByUserAndName(mock.Anything, 3, "dupe").
					Return(1, nil)
			},
			wantErr: apikey.ErrDuplicateKeyName,
		},
		{
			name:    "error - repo Count failure wraps",
			userID:  4,
			keyName: "count-fail",
			pepper:  "",
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					CountByUserAndName(mock.Anything, 4, "count-fail").
					Return(0, errors.New("db offline"))
			},
			wantErr: errors.New("failed to check duplicate key name"),
		},
		{
			name:    "error - repo Create failure wraps",
			userID:  5,
			keyName: "create-fail",
			pepper:  "",
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					CountByUserAndName(mock.Anything, 5, "create-fail").
					Return(0, nil)
				m.EXPECT().
					Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
					Return(errors.New("disk full"))
			},
			wantErr: errors.New("failed to create api key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			svc := apikey.NewService(mockRepo, tt.pepper)
			keyStr, key, err := svc.Create(context.Background(), tt.userID, tt.keyName)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, apikey.ErrInvalidKeyName) ||
					errors.Is(tt.wantErr, apikey.ErrDuplicateKeyName) {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					// wrapped errors - check message substring
					assert.Contains(t, err.Error(), tt.wantErr.Error())
				}
				assert.Empty(t, keyStr)
				assert.Nil(t, key)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, keyStr)
			require.NotNil(t, key)

			// Key format: lesstruct_<12hex>_<32hex>
			assert.True(t, strings.HasPrefix(keyStr, apikey.KeyPrefix))

			payload := strings.TrimPrefix(keyStr, apikey.KeyPrefix)
			parts := strings.Split(payload, "_")
			require.Len(t, parts, 2, "key must have exactly keyID and secret segments")
			assert.Len(t, parts[0], 12, "keyID must be 12 hex chars")
			assert.Len(t, parts[1], 32, "secret must be 32 hex chars")

			assert.Len(t, key.KeyHash, 64, "KeyHash must be 64 hex chars")
			assert.NotEqual(t, parts[1], key.KeyHash, "secret must never equal the stored hash")

			// Entity fields
			assert.Equal(t, tt.userID, key.UserID)
			assert.Equal(t, parts[0], key.KeyID)
			assert.False(t, key.CreatedAt.IsZero(), "CreatedAt must be populated")
			// Name is trimmed per service implementation
			assert.Equal(t, strings.TrimSpace(tt.keyName), key.Name)
		})
	}
}

// fixedBytesReader returns the same pattern on every Read, filling p completely
// (repeating the pattern as needed). It pins key-material generation so hashing
// can be verified against an independently computed digest (known-answer test).
type fixedBytesReader struct{ pattern []byte }

func (f fixedBytesReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		n += copy(p[n:], f.pattern)
	}
	return n, nil
}

// TestService_Hashing_KnownAnswer verifies the no-pepper (SHA-256) and pepper
// (HMAC-SHA256) hashing paths produce the EXACT expected digest for a pinned
// secret. This is a real known-answer test: the expected digests are computed
// independently in the test, so a swapped HMAC argument, double-hashing, or
// hashing the wrong input would fail.
func TestService_Hashing_KnownAnswer(t *testing.T) {
	// Pin the entropy source: 22 bytes (6 for keyID, 16 for secret).
	known := make([]byte, 22)
	for i := range known {
		known[i] = byte(i)
	}
	original := apikey.Reader
	t.Cleanup(func() { apikey.Reader = original })
	apikey.Reader = fixedBytesReader{pattern: known}

	expectedSecret := hex.EncodeToString(known[6:]) // 32 hex chars
	// Independently compute the expected digests.
	plainSum := sha256.Sum256([]byte(expectedSecret))
	expectedPlain := hex.EncodeToString(plainSum[:])

	const pepper = "pepper-x"
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(expectedSecret))
	expectedPepper := hex.EncodeToString(mac.Sum(nil))

	// Capture the persisted hash via the repo mock for each service variant.
	var plainHash, pepperHash string

	plainRepo := mocks.NewMockRepository(t)
	plainRepo.EXPECT().
		CountByUserAndName(mock.Anything, 99, "KAT").
		Return(0, nil)
	plainRepo.EXPECT().
		Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
		Run(func(_ context.Context, k *apikey.APIKey) { plainHash = k.KeyHash }).
		Return(nil)

	plainSvc := apikey.NewService(plainRepo, "")
	_, _, err := plainSvc.Create(context.Background(), 99, "KAT")
	require.NoError(t, err)

	pepperRepo := mocks.NewMockRepository(t)
	pepperRepo.EXPECT().
		CountByUserAndName(mock.Anything, 99, "KAT").
		Return(0, nil)
	pepperRepo.EXPECT().
		Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
		Run(func(_ context.Context, k *apikey.APIKey) { pepperHash = k.KeyHash }).
		Return(nil)

	pepperSvc := apikey.NewService(pepperRepo, pepper)
	_, _, err = pepperSvc.Create(context.Background(), 99, "KAT")
	require.NoError(t, err)

	assert.Len(t, plainHash, 64, "plain hash must be 64 hex chars")
	assert.Len(t, pepperHash, 64, "pepper hash must be 64 hex chars")
	assert.Equal(t, expectedPlain, plainHash, "no-pepper path must equal hex(sha256(secret))")
	assert.Equal(t, expectedPepper, pepperHash, "pepper path must equal hex(hmac-sha256(pepper, secret))")
	assert.NotEqual(t, plainHash, pepperHash, "HMAC-peppered hash must differ from plain SHA-256 hash")
	assert.NotEqual(t, expectedSecret, plainHash, "stored hash must never equal the secret")
}

// TestDisplayPrefix verifies the masked display form used by list views. The
// secret is never part of this string; the full keyID (public lookup prefix)
// is shown followed by four bullets representing the hidden secret.
func TestDisplayPrefix(t *testing.T) {
	tests := []struct {
		name    string
		keyID   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard 12-hex keyID",
			keyID: "aabbccddeeff",
			want:  "lesstruct_aabbccddeeff••••",
			wantErr: false,
		},
		{
			name:  "empty keyID still prefixed and bulleted",
			keyID: "",
			want:  "lesstruct_••••",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apikey.DisplayPrefix(tt.keyID)
			assert.Equal(t, tt.want, got)
			// The secret marker is always present and the key never leaks.
			assert.True(t, strings.HasSuffix(got, "••••"))
		})
	}
}

// TestValidateKeyName exercises the validator directly for 100% coverage of
// the boundary conditions (empty vs over-limit).
func TestValidateKeyName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid single char", input: "a", wantErr: false},
		{name: "valid 120 chars", input: strings.Repeat("n", 120), wantErr: false},
		{name: "empty after trim", input: "   ", wantErr: true},
		{name: "over limit", input: strings.Repeat("n", 121), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apikey.ValidateKeyName(tt.input)
			if tt.wantErr {
				assert.ErrorIs(t, err, apikey.ErrInvalidKeyName)
				return
			}
			require.NoError(t, err)
		})
	}
}

// failingReader is an io.Reader that always returns an error, used to exercise
// the key-generation error path of Service.Create.
type failingReader struct{}

func (failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated entropy source failure")
}

// TestService_Create_RandError verifies that when the cryptographic reader
// fails, Service.Create returns ErrKeyGeneration and no repo write occurs.
func TestService_Create_RandError(t *testing.T) {
	original := apikey.Reader
	t.Cleanup(func() { apikey.Reader = original })

	apikey.Reader = failingReader{}

	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		CountByUserAndName(mock.Anything, 1, "Entropy Fail").
		Return(0, nil)

	svc := apikey.NewService(mockRepo, "")
	keyStr, key, err := svc.Create(context.Background(), 1, "Entropy Fail")

	require.Error(t, err)
	assert.ErrorIs(t, err, apikey.ErrKeyGeneration)
	assert.Empty(t, keyStr)
	assert.Nil(t, key)
}

// shortReader returns one byte fewer than requested with no error — a legal but
// undesirable io.Reader behavior. The service must reject it rather than
// silently producing predictable key material.
type shortReader struct{}

func (shortReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

// TestService_Create_ShortRead exercises the n != len(combined) guard.
func TestService_Create_ShortRead(t *testing.T) {
	original := apikey.Reader
	t.Cleanup(func() { apikey.Reader = original })
	apikey.Reader = shortReader{}

	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		CountByUserAndName(mock.Anything, 1, "Short Read").
		Return(0, nil)

	svc := apikey.NewService(mockRepo, "")
	keyStr, key, err := svc.Create(context.Background(), 1, "Short Read")

	require.Error(t, err)
	assert.ErrorIs(t, err, apikey.ErrKeyGeneration)
	assert.Empty(t, keyStr)
	assert.Nil(t, key)
}

// TestService_Create_UniqueConstraint_NameRace verifies the DB-level UNIQUE
// backstop: a concurrent insert that wins the race causes Create to fail with
// ErrUniqueConstraint, the service re-checks the name, and returns
// ErrDuplicateKeyName (not an opaque 500).
func TestService_Create_UniqueConstraint_NameRace(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	// Single expectation with stateful returns: pre-check sees 0, re-check sees 1.
	countCalls := 0
	mockRepo.EXPECT().
		CountByUserAndName(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ int, _ string) (int, error) {
			countCalls++
			if countCalls == 1 {
				return 0, nil // pre-check: no duplicate seen yet
			}
			return 1, nil // re-check after race: now the name exists
		}).Times(2)
	mockRepo.EXPECT().
		Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
		Return(apikey.ErrUniqueConstraint).Times(1)

	svc := apikey.NewService(mockRepo, "")
	keyStr, key, err := svc.Create(context.Background(), 1, "race")

	require.Error(t, err)
	assert.ErrorIs(t, err, apikey.ErrDuplicateKeyName)
	assert.Empty(t, keyStr)
	assert.Nil(t, key)
}

// TestService_Create_KeyIDCollision_RetriesThenSucceeds verifies that a key_id
// collision (not a name duplicate) triggers a bounded regenerate-and-retry that
// ultimately succeeds.
func TestService_Create_KeyIDCollision_RetriesThenSucceeds(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		CountByUserAndName(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ int, _ string) (int, error) {
			return 0, nil // never a name duplicate
		}).Times(2)
	createCalls := 0
	mockRepo.EXPECT().
		Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
		RunAndReturn(func(_ context.Context, k *apikey.APIKey) error {
			createCalls++
			if createCalls == 1 {
				return apikey.ErrUniqueConstraint // first attempt: key_id collision
			}
			k.ID = 77
			return nil // second attempt: succeeds
		}).Times(2)

	svc := apikey.NewService(mockRepo, "")
	keyStr, key, err := svc.Create(context.Background(), 1, "collide")

	require.NoError(t, err)
	require.NotNil(t, key)
	assert.Equal(t, 77, key.ID)
	assert.True(t, strings.HasPrefix(keyStr, apikey.KeyPrefix))
}

// TestService_Create_KeyIDCollision_Exhausts verifies that persistent key_id
// collisions give up after maxCreateAttempts and surface a wrapped
// ErrKeyGeneration rather than looping forever.
func TestService_Create_KeyIDCollision_Exhausts(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	// 1 pre-check + 3 re-checks after each collision = 4 total Count calls.
	mockRepo.EXPECT().
		CountByUserAndName(mock.Anything, 1, "stuck").
		Return(0, nil).Times(4)
	// All 3 insert attempts collide.
	mockRepo.EXPECT().
		Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
		Return(apikey.ErrUniqueConstraint).Times(3)

	svc := apikey.NewService(mockRepo, "")
	keyStr, key, err := svc.Create(context.Background(), 1, "stuck")

	require.Error(t, err)
	assert.ErrorIs(t, err, apikey.ErrKeyGeneration)
	assert.Empty(t, keyStr)
	assert.Nil(t, key)
}

func TestService_List(t *testing.T) {
	twoKeys := []*apikey.APIKey{
		{ID: 1, UserID: 5, Name: "Alpha", KeyID: "aaaaaaaaaaaa"},
		{ID: 2, UserID: 5, Name: "Beta", KeyID: "bbbbbbbbbbbb"},
	}

	tests := []struct {
		name      string
		userID    int
		setupMock func(*mocks.MockRepository)
		wantKeys  []*apikey.APIKey
		wantErr   bool
		errSubstr string
	}{
		{
			name:   "success - returns slice as-is",
			userID: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					ListByUserID(mock.Anything, 5).
					Return(twoKeys, nil)
			},
			wantKeys: twoKeys,
			wantErr:  false,
		},
		{
			name:   "success - empty list returns non-nil empty slice",
			userID: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					ListByUserID(mock.Anything, 5).
					Return([]*apikey.APIKey{}, nil)
			},
			wantKeys: []*apikey.APIKey{},
			wantErr:  false,
		},
		{
			name:   "error - repo failure wraps",
			userID: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					ListByUserID(mock.Anything, 5).
					Return(nil, errors.New("db offline"))
			},
			wantErr:   true,
			errSubstr: "failed to list api keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			svc := apikey.NewService(mockRepo, "")
			keys, err := svc.List(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.Nil(t, keys)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKeys, keys)
		})
	}
}

func TestService_Revoke(t *testing.T) {
	tests := []struct {
		name              string
		id                int
		userID            int
		setupMock         func(*mocks.MockRepository, *time.Time)
		wantErr           bool
		errIs             error
		errSubstr         string
		wantRevoked       bool
		assertCapturedNow bool
	}{
		{
			name:   "success - active key revoked",
			id:     10,
			userID: 7,
			setupMock: func(m *mocks.MockRepository, capturedNow *time.Time) {
				m.EXPECT().
					FindByIDAndUserID(mock.Anything, 10, 7).
					Return(&apikey.APIKey{
						ID:     10,
						UserID: 7,
						Name:   "Active",
						KeyID:  "cccccccccccc",
					}, nil)
				m.EXPECT().
					RevokeByIDAndUserID(mock.Anything, 10, 7, mock.AnythingOfType("time.Time")).
					Run(func(_ context.Context, _, _ int, now time.Time) { *capturedNow = now }).
					Return(nil)
			},
			wantErr:           false,
			wantRevoked:       true,
			assertCapturedNow: true,
		},
		{
			name:   "idempotent - already-revoked key returns success without DB write",
			id:     11,
			userID: 7,
			setupMock: func(m *mocks.MockRepository, _ *time.Time) {
				prev := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				m.EXPECT().
					FindByIDAndUserID(mock.Anything, 11, 7).
					Return(&apikey.APIKey{
						ID:        11,
						UserID:    7,
						Name:      "Revoked",
						KeyID:     "dddddddddddd",
						RevokedAt: &prev,
					}, nil)
				// RevokeByIDAndUserID deliberately NOT expected — proves the
				// idempotent guard skips the DB write.
			},
			wantErr:     false,
			wantRevoked: true,
		},
		{
			name:   "not-found - FindByIDAndUserID returns ErrKeyNotFound",
			id:     12,
			userID: 7,
			setupMock: func(m *mocks.MockRepository, _ *time.Time) {
				m.EXPECT().
					FindByIDAndUserID(mock.Anything, 12, 7).
					Return(nil, apikey.ErrKeyNotFound)
			},
			wantErr: true,
			errIs:   apikey.ErrKeyNotFound,
		},
		{
			name:   "error - FindByIDAndUserID returns non-sentinel error",
			id:     13,
			userID: 7,
			setupMock: func(m *mocks.MockRepository, _ *time.Time) {
				m.EXPECT().
					FindByIDAndUserID(mock.Anything, 13, 7).
					Return(nil, errors.New("connection lost"))
			},
			wantErr:   true,
			errSubstr: "failed to find api key for revoke",
		},
		{
			name:   "error - RevokeByIDAndUserID returns error",
			id:     14,
			userID: 7,
			setupMock: func(m *mocks.MockRepository, _ *time.Time) {
				m.EXPECT().
					FindByIDAndUserID(mock.Anything, 14, 7).
					Return(&apikey.APIKey{
						ID:     14,
						UserID: 7,
						Name:   "FailRevoke",
						KeyID:  "eeeeeeeeeeee",
					}, nil)
				m.EXPECT().
					RevokeByIDAndUserID(mock.Anything, 14, 7, mock.AnythingOfType("time.Time")).
					Return(errors.New("disk write fail"))
			},
			wantErr:   true,
			errSubstr: "failed to revoke api key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			var capturedNow time.Time
			if tt.setupMock != nil {
				tt.setupMock(mockRepo, &capturedNow)
			}

			svc := apikey.NewService(mockRepo, "")
			key, err := svc.Revoke(context.Background(), tt.id, tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				} else {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				assert.Nil(t, key)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, key)
			if tt.wantRevoked {
				require.NotNil(t, key.RevokedAt, "RevokedAt must be populated")
			}
			// Deterministic timestamp check for the active-revoke success path:
			// the returned RevokedAt must equal the now value the Service passed
			// to the repo (captured via the mock Run callback).
			if tt.assertCapturedNow {
				assert.Equal(t, capturedNow, *key.RevokedAt)
			}
		})
	}
}

// seedKey runs the real Service.Create flow to produce a valid key whose
// KeyHash was computed by the service's own pepper-aware hashSecret. This avoids
// reaching into the private hasher from tests (no private-field access) and gives
// Verify a full key whose secret deterministically matches a stored hash. The
// returned keyID/keyHash seed the mock's FindByKeyID return value.
func seedKey(t *testing.T, pepper string) (fullKey, keyID, keyHash string) {
	t.Helper()
	seedRepo := mocks.NewMockRepository(t)
	seedRepo.EXPECT().
		CountByUserAndName(mock.Anything, mock.Anything, mock.Anything).
		Return(0, nil)
	seedRepo.EXPECT().
		Create(mock.Anything, mock.AnythingOfType("*apikey.APIKey")).
		Return(nil)

	svc := apikey.NewService(seedRepo, pepper)
	fk, key, err := svc.Create(context.Background(), 1, "verify-seed")
	require.NoError(t, err)
	require.NotNil(t, key)
	return fk, key.KeyID, key.KeyHash
}

func TestService_Verify(t *testing.T) {
	// Seed one real key (no pepper) so Verify's constant-time compare has a known
	// matching hash. The seed's fullKey/keyID/keyHash feed the mock return values.
	fullKey, keyID, keyHash := seedKey(t, "")

	// A well-formed key with a WRONG secret (same keyID, valid 32-hex length, but
	// a different secret than was seeded). Its hash will not match keyHash.
	wrongSecretKey := apikey.KeyPrefix + keyID + "_" + strings.Repeat("9", 32)

	// storedKey builds a persisted key matching the seed (hash matches fullKey's
	// secret); overrides mutate it (e.g. set RevokedAt/ExpiresAt).
	storedKey := func(overrides ...func(*apikey.APIKey)) *apikey.APIKey {
		k := &apikey.APIKey{
			ID:      42,
			UserID:  1,
			Name:    "verify-seed",
			KeyID:   keyID,
			KeyHash: keyHash,
		}
		for _, o := range overrides {
			o(k)
		}
		return k
	}

	tests := []struct {
		name      string
		input     string
		setupMock func(*mocks.MockRepository)
		wantErr   error  // sentinel checked via errors.Is; nil = expect success
		errSubstr string // checked when wantErr is a wrapped (non-sentinel) error
		wantKey   bool   // expect a non-nil *APIKey back (revoked/expired/success)
	}{
		{
			name:      "malformed - missing lesstruct_ prefix",
			input:     "deadbeef",
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:   apikey.ErrMalformedKey,
		},
		{
			name:      "malformed - wrong segment structure (no secret segment)",
			input:     "lesstruct_onlykeyid",
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:   apikey.ErrMalformedKey,
		},
		{
			name:      "malformed - keyID wrong length (11 chars)",
			input:     "lesstruct_" + strings.Repeat("a", 11) + "_" + strings.Repeat("b", 32),
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:   apikey.ErrMalformedKey,
		},
		{
			name:      "malformed - secret wrong length (31 chars)",
			input:     "lesstruct_" + strings.Repeat("a", 12) + "_" + strings.Repeat("b", 31),
			setupMock: func(m *mocks.MockRepository) {},
			wantErr:   apikey.ErrMalformedKey,
		},
		{
			name:  "not-found - FindByKeyID returns ErrKeyNotFound",
			input: fullKey,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(nil, apikey.ErrKeyNotFound)
			},
			wantErr: apikey.ErrKeyNotFound,
		},
		{
			name:  "wrong-secret - stored hash does not match presented secret",
			input: wrongSecretKey,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(storedKey(), nil)
			},
			wantErr: apikey.ErrKeyNotFound,
		},
		{
			name:  "revoked - secret matches but RevokedAt set",
			input: fullKey,
			setupMock: func(m *mocks.MockRepository) {
				revoked := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(storedKey(func(k *apikey.APIKey) { k.RevokedAt = &revoked }), nil)
			},
			wantErr: apikey.ErrKeyRevoked,
			wantKey: true,
		},
		{
			name:  "expired - secret matches, ExpiresAt in the past, not revoked",
			input: fullKey,
			setupMock: func(m *mocks.MockRepository) {
				expired := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(storedKey(func(k *apikey.APIKey) { k.ExpiresAt = &expired }), nil)
			},
			wantErr: apikey.ErrKeyExpired,
			wantKey: true,
		},
		{
			name:  "success - secret matches, no revocation, no expiry",
			input: fullKey,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(storedKey(), nil)
			},
			wantErr: nil,
			wantKey: true,
		},
		{
			name:  "success - secret matches, ExpiresAt in the future",
			input: fullKey,
			setupMock: func(m *mocks.MockRepository) {
				future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(storedKey(func(k *apikey.APIKey) { k.ExpiresAt = &future }), nil)
			},
			wantErr: nil,
			wantKey: true,
		},
		{
			name:  "repo-error - FindByKeyID returns non-sentinel error",
			input: fullKey,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().
					FindByKeyID(mock.Anything, keyID).
					Return(nil, errors.New("connection lost"))
			},
			errSubstr: "failed to verify api key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setupMock(mockRepo)

			svc := apikey.NewService(mockRepo, "")
			key, err := svc.Verify(context.Background(), tt.input)

			if tt.errSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.Nil(t, key)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			if tt.wantKey {
				require.NotNil(t, key)
				assert.Equal(t, keyID, key.KeyID)
			} else {
				assert.Nil(t, key)
			}
		})
	}
}

func TestService_UpdateLastUsed(t *testing.T) {
	tests := []struct {
		name      string
		id        int
		ip        string
		setupMock func(*mocks.MockRepository, *time.Time, *string)
		wantErr   bool
		errSubstr string
	}{
		{
			name: "success - repo UpdateLastUsed returns nil",
			id:   42,
			ip:   "1.2.3.4",
			setupMock: func(m *mocks.MockRepository, capturedNow *time.Time, capturedIP *string) {
				m.EXPECT().
					UpdateLastUsed(mock.Anything, 42, mock.AnythingOfType("time.Time"), "1.2.3.4").
					Run(func(_ context.Context, _ int, now time.Time, ip string) {
						*capturedNow = now
						*capturedIP = ip
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "repo-error - repo UpdateLastUsed returns error",
			id:   42,
			ip:   "1.2.3.4",
			setupMock: func(m *mocks.MockRepository, _ *time.Time, _ *string) {
				m.EXPECT().
					UpdateLastUsed(mock.Anything, 42, mock.AnythingOfType("time.Time"), "1.2.3.4").
					Return(errors.New("db write failed"))
			},
			wantErr:   true,
			errSubstr: "failed to update last used",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			var capturedNow time.Time
			var capturedIP string
			tt.setupMock(mockRepo, &capturedNow, &capturedIP)

			svc := apikey.NewService(mockRepo, "")
			err := svc.UpdateLastUsed(context.Background(), tt.id, tt.ip)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}

			require.NoError(t, err)
			// now is stamped internally (time.Now().UTC()); assert it is recent and
			// non-zero, and that the ip arg was forwarded verbatim.
			assert.False(t, capturedNow.IsZero(), "now must be stamped")
			assert.True(t, time.Since(capturedNow) < 5*time.Second, "now must be recent")
			assert.Equal(t, tt.ip, capturedIP)
		})
	}
}

