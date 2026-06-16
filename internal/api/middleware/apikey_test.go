package middleware_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyAuthMiddleware_Handler exercises the Bearer API-key middleware end
// to end across every acceptance criterion: success, revoked, expired, malformed,
// wrong-secret (safe disclosure), missing/non-Bearer header, user-lookup failure,
// the defensive nil-key guard, and best-effort last_used_at. The middleware
// depends only on the narrow APIKeyVerifier + UserLookup interfaces, so the
// *apikey.Service hashing path is covered by the domain tests; here we mock the
// verifier and assert the middleware's own wiring (context parity, error codes,
// and logging hygiene).
func TestAPIKeyAuthMiddleware_Handler(t *testing.T) {
	const (
		validKeyID = "abcdef123456"
		secret     = "deadbeefdeadbeefdeadbeefdeadbeef" // 32 hex; never logged
		validToken = "lesstruct_" + validKeyID + "_" + secret
		userID     = 42
	)

	verifiedKey := &apikey.APIKey{
		ID:     7,
		UserID: userID,
		Name:   "CI Key",
		KeyID:  validKeyID,
	}
	owningUser := &repository.User{
		ID:       userID,
		Username: "alice",
		Role:     "Editor",
	}

	tests := []struct {
		name               string
		header             string // raw Authorization header value
		omitHeader         bool   // true → do not send an Authorization header at all
		setupVerifier      func(*mocks.MockAPIKeyVerifier)
		setupUser          func(*mocks.MockUserLookup)
		captureLogs        bool // use a bytes.Buffer logger for log assertions
		wantStatus         int
		wantBodyCode       string // expected error code substring ("" = skip)
		wantNextCalled     bool
		wantUserID         string
		wantUpdateLastUsed bool // true → middleware MUST call UpdateLastUsed
		disclosureTag      string // "malformed"/"wrongSecret" for byte-identity check
	}{
		{
			name:   "valid key - authenticates and injects owning user into context",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(verifiedKey, nil)
				v.EXPECT().UpdateLastUsed(mock.Anything, 7, "1.2.3.4").Return(nil)
			},
			setupUser: func(u *mocks.MockUserLookup) {
				u.EXPECT().GetUserByID(mock.Anything, userID).Return(owningUser, nil)
			},
			wantStatus:         http.StatusOK,
			wantNextCalled:     true,
			wantUserID:         strconv.Itoa(userID),
			wantUpdateLastUsed: true,
		},
		{
			name:   "revoked - 401 REVOKED_KEY, next and UpdateLastUsed not called",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(verifiedKey, apikey.ErrKeyRevoked)
			},
			captureLogs:    true,
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "REVOKED_KEY",
			wantNextCalled: false,
		},
		{
			name:   "expired - 401 EXPIRED_KEY, next and UpdateLastUsed not called",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(verifiedKey, apikey.ErrKeyExpired)
			},
			captureLogs:    true,
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "EXPIRED_KEY",
			wantNextCalled: false,
		},
		{
			name:   "malformed format - 401 INVALID_API_KEY",
			header: "Bearer lesstruct_short_bad",
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, "lesstruct_short_bad").Return(nil, apikey.ErrMalformedKey)
			},
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "INVALID_API_KEY",
			wantNextCalled: false,
			disclosureTag:  "malformed",
		},
		{
			name:   "wrong secret - 401 INVALID_API_KEY, byte-identical to malformed",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(nil, apikey.ErrKeyNotFound)
			},
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "INVALID_API_KEY",
			wantNextCalled: false,
			disclosureTag:  "wrongSecret",
		},
		{
			name:           "missing Authorization header - 401 UNAUTHORIZED",
			omitHeader:     true,
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "UNAUTHORIZED",
			wantNextCalled: false,
		},
		{
			name:           "non-Bearer scheme - 401 UNAUTHORIZED",
			header:         "Basic xyz",
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "UNAUTHORIZED",
			wantNextCalled: false,
		},
		{
			name:   "user-lookup failure - 401 INVALID_API_KEY, UpdateLastUsed not called",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(verifiedKey, nil)
			},
			setupUser: func(u *mocks.MockUserLookup) {
				u.EXPECT().GetUserByID(mock.Anything, userID).Return(nil, errors.New("user not found with ID 42"))
			},
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "INVALID_API_KEY",
			wantNextCalled: false,
		},
		{
			name:   "defensive - Verify returns nil key without error - 500 INTERNAL_ERROR",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(nil, nil)
			},
			wantStatus:     http.StatusInternalServerError,
			wantBodyCode:   "INTERNAL_ERROR",
			wantNextCalled: false,
		},
		{
			name:   "best-effort - UpdateLastUsed error does not block a valid request",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(verifiedKey, nil)
				v.EXPECT().UpdateLastUsed(mock.Anything, 7, "1.2.3.4").Return(errors.New("db down"))
			},
			setupUser: func(u *mocks.MockUserLookup) {
				u.EXPECT().GetUserByID(mock.Anything, userID).Return(owningUser, nil)
			},
			captureLogs:        true,
			wantStatus:         http.StatusOK,
			wantNextCalled:     true,
			wantUserID:         strconv.Itoa(userID),
			wantUpdateLastUsed: true,
		},
		{
			// Exercises the writeAuthError default branch: a non-sentinel Verify
			// error (e.g. a wrapped DB fault, nil key) must map to 401
			// INVALID_API_KEY (fail closed) and never reach next/UpdateLastUsed.
			name:   "unexpected verify error - 401 INVALID_API_KEY (fail closed)",
			header: "Bearer " + validToken,
			setupVerifier: func(v *mocks.MockAPIKeyVerifier) {
				v.EXPECT().Verify(mock.Anything, validToken).Return(nil, errors.New("db connection lost"))
			},
			wantStatus:     http.StatusUnauthorized,
			wantBodyCode:   "INVALID_API_KEY",
			wantNextCalled: false,
		},
	}

	disclosureBodies := map[string]string{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := mocks.NewMockAPIKeyVerifier(t)
			if tt.setupVerifier != nil {
				tt.setupVerifier(verifier)
			}
			userLookup := mocks.NewMockUserLookup(t)
			if tt.setupUser != nil {
				tt.setupUser(userLookup)
			}

			var buf *bytes.Buffer
			logger := util.NewLogger(io.Discard)
			if tt.captureLogs {
				buf = &bytes.Buffer{}
				logger = util.NewLogger(buf)
			}

			mw := appmiddleware.NewAPIKeyAuthMiddleware(verifier, userLookup, logger)

			var (
				nextCalled bool
				gotUserID  string
			)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotUserID, _ = appmiddleware.GetUserID(r)
				gotUsername, _ := appmiddleware.GetUsername(r)
				gotRole, _ := appmiddleware.GetRole(r)
				assert.Equal(t, "alice", gotUsername)
				assert.Equal(t, "Editor", gotRole)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/content", nil)
			if !tt.omitHeader {
				req.Header.Set("Authorization", tt.header)
			}
			req.RemoteAddr = "1.2.3.4:5678"

			rr := httptest.NewRecorder()
			mw.Handler(nextHandler).ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantBodyCode != "" {
				assert.Contains(t, rr.Body.String(), tt.wantBodyCode)
			}
			assert.Equal(t, tt.wantNextCalled, nextCalled)
			if tt.wantNextCalled {
				assert.Equal(t, tt.wantUserID, gotUserID)
			}

			// When no verifier/user expectations were set, the corresponding
			// methods must never be reached (regression guard on the control flow).
			if tt.setupVerifier == nil {
				assert.True(t, verifier.AssertNotCalled(t, "Verify"), "Verify must not be called")
			}
			if tt.setupUser == nil {
				assert.True(t, userLookup.AssertNotCalled(t, "GetUserByID"), "GetUserByID must not be called")
			}
			if !tt.wantUpdateLastUsed {
				assert.True(t, verifier.AssertNotCalled(t, "UpdateLastUsed"), "UpdateLastUsed must not be called")
			}

			// Log hygiene (AC6 / NFR-A1): only the keyID is ever logged; the secret
			// and full key string are never written to logs.
			if buf != nil {
				logged := buf.String()
				assert.Contains(t, logged, validKeyID, "log must contain the keyID")
				assert.NotContains(t, logged, secret, "log must never contain the secret")
			}

			if tt.disclosureTag != "" {
				disclosureBodies[tt.disclosureTag] = rr.Body.String()
			}
		})
	}

	// Safe disclosure (AC5): a wrong secret and a malformed key produce
	// byte-identical INVALID_API_KEY responses — no keyID-existence leak.
	require.Contains(t, disclosureBodies, "malformed")
	require.Contains(t, disclosureBodies, "wrongSecret")
	assert.Equal(t,
		disclosureBodies["malformed"],
		disclosureBodies["wrongSecret"],
		"wrong-secret and malformed responses must be byte-identical",
	)
}
