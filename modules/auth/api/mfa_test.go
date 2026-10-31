package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mfaHandler builds a handler carrying only the collaborators the MFA
// endpoints touch.
func mfaHandler(
	service *authdomain.MFAService,
	deliverer api.MFACodeDeliverer,
	store api.UserStore,
	issuer api.TokenIssuer,
) *api.AuthHandler {
	return api.NewAuthHandler(
		nil,
		issuer,
		nil,
		nil,
		nil,
		nil,
		nil,
		service,
		nil,
		store,
		nil,
		nil,
		nil,
		deliverer,
		nil,
	)
}

// seededMFARepo stores one challenge under its hashed token.
func seededMFARepo(challenge *authdomain.MFAChallenge) *stubMFARepo {
	if challenge.TokenHash == "" {
		challenge.TokenHash = authdomain.HashToken("challenge-token")
	}
	if challenge.CodeHash == "" {
		challenge.CodeHash = authdomain.HashToken("123456")
	}

	return &stubMFARepo{
		challenges: map[string]*authdomain.MFAChallenge{challenge.TokenHash: challenge},
		nextID:     challenge.ID,
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "success - masks the local part", input: "admin@example.com", want: "a***@example.com"},
		{name: "success - single letter local part", input: "a@example.com", want: "***@example.com"},
		{name: "success - multi-byte local part", input: "éclair@example.com", want: "é***@example.com"},
		{name: "success - single multi-byte local part", input: "😀@example.com", want: "***@example.com"},
		{name: "success - missing domain", input: "no-domain", want: "***"},
		{name: "success - empty address", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, api.MaskEmail(tt.input))
		})
	}
}

func TestVerifyMFA(t *testing.T) {
	verifiedUser := adminFixture()

	tests := []struct {
		name        string
		challenge   *authdomain.MFAChallenge
		store       *stubUserStore
		issuer      *stubIssuer
		body        string
		wantStatus  int
		wantCode    string
		wantSession bool
	}{
		{
			name: "success - matching code sets the session cookie",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:       &stubUserStore{admin: verifiedUser},
			issuer:      &stubIssuer{token: "session-jwt"},
			body:        `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus:  http.StatusOK,
			wantSession: true,
		},
		{
			name:       "error - malformed body",
			store:      &stubUserStore{},
			issuer:     &stubIssuer{},
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_REQUEST",
		},
		{
			name:       "error - missing challenge token",
			store:      &stubUserStore{},
			issuer:     &stubIssuer{},
			body:       `{"code":"123456"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "MISSING_CHALLENGE",
		},
		{
			name:       "error - code is not six digits",
			store:      &stubUserStore{},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"abc"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_CODE_FORMAT",
		},
		{
			name: "error - wrong code",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"000000"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_CODE_INVALID",
		},
		{
			name: "error - expired challenge",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(-time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_CHALLENGE_EXPIRED",
		},
		{
			name: "error - attempt budget exhausted",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				Attempts:  5,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_MAX_ATTEMPTS",
		},
		{
			name:       "error - unknown challenge",
			store:      &stubUserStore{},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"unknown","code":"123456"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_CHALLENGE_INVALID",
		},
		{
			name: "error - user lookup fails",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{getErr: errors.New("database error")},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "LOGIN_FAILED",
		},
		{
			name: "error - user disappeared",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_CREDENTIALS",
		},
		{
			name: "error - account suspended after the password step",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store: &stubUserStore{admin: &userdomain.User{
				ID:     1,
				Status: "suspended",
			}},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_SUSPENDED",
		},
		{
			name: "error - account pending after the password step",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store: &stubUserStore{admin: &userdomain.User{
				ID:     1,
				Status: "pending",
			}},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "EMAIL_NOT_VERIFIED",
		},
		{
			name: "error - account soft-deleted after the password step",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store: &stubUserStore{admin: &userdomain.User{
				ID:     1,
				Status: "soft_deleted",
			}},
			issuer:     &stubIssuer{},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_DELETED",
		},
		{
			name: "error - token generation fails",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			issuer:     &stubIssuer{err: errors.New("signing error")},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "TOKEN_GENERATION_FAILED",
		},
		{
			name: "error - session expiry resolution fails",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			issuer:     &stubIssuer{token: "session-jwt", expiryErr: errors.New("expiry error")},
			body:       `{"challengeToken":"challenge-token","code":"123456"}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "TOKEN_GENERATION_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubMFARepo{}
			if tt.challenge != nil {
				repo = seededMFARepo(tt.challenge)
			}

			service := authdomain.NewMFAService(repo, 5*time.Minute, time.Minute, 5)
			handler := mfaHandler(service, &stubDeliverer{}, tt.store, tt.issuer)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.VerifyMFA(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "status")

			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, decodeErrorCode(t, w), "error code")
				return
			}

			data := decodeData(t, w)
			if tt.wantSession {
				assert.NotContains(t, w.Body.String(), `"token"`, "no token in login JSON")
				user, ok := data["user"].(map[string]any)
				require.True(t, ok, "response carries the user")
				assert.Equal(t, "admin", user["username"], "username")
				assert.Equal(t, tt.store.admin.TokenVersion, tt.issuer.tokenVersion,
					"the issued token must carry the user's current token version")
				cookies := w.Result().Cookies()
				require.Len(t, cookies, 1, "one session cookie")
				cookie := cookies[0]
				assert.Equal(t, "lesstruct_jwt", cookie.Name, "session cookie name")
				assert.Equal(t, "session-jwt", cookie.Value, "session cookie value")
				assert.True(t, cookie.HttpOnly, "session cookie is HttpOnly")
				assert.Equal(t, "/", cookie.Path, "session cookie path")
				assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite, "session cookie is SameSite=Lax")
				assert.Greater(t, cookie.MaxAge, 23*60*60, "cookie lifetime follows the token expiry")
				assert.WithinDuration(t, time.Now().Add(24*time.Hour), cookie.Expires, 5*time.Minute, "cookie expiry follows the token expiry")
			}
		})
	}
}

func TestVerifyMFA_Unconfigured(t *testing.T) {
	handler := mfaHandler(nil, &stubDeliverer{}, &stubUserStore{}, &stubIssuer{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader([]byte(`{"challengeToken":"challenge-token","code":"123456"}`)))
	w := httptest.NewRecorder()

	handler.VerifyMFA(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "status")
	assert.Equal(t, "MFA_UNAVAILABLE", decodeErrorCode(t, w), "error code")
}

func TestResendMFA(t *testing.T) {
	verifiedUser := adminFixture()

	tests := []struct {
		name         string
		challenge    *authdomain.MFAChallenge
		store        *stubUserStore
		delivererErr error
		body         string
		wantStatus   int
		wantCode     string
		wantHint     string
	}{
		{
			name: "success - issues a fresh code after the cooldown",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusOK,
			wantHint:   "a***@example.com",
		},
		{
			name: "error - expired challenge",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-10 * time.Minute),
				ExpiresAt: time.Now().Add(-5 * time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_CHALLENGE_EXPIRED",
		},
		{
			name:       "error - malformed body",
			store:      &stubUserStore{},
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_REQUEST",
		},
		{
			name:       "error - missing challenge token",
			store:      &stubUserStore{},
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "MISSING_CHALLENGE",
		},
		{
			name:       "error - unknown challenge",
			store:      &stubUserStore{},
			body:       `{"challengeToken":"unknown"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_CHALLENGE_INVALID",
		},
		{
			name: "error - resend within the cooldown",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Minute),
			},
			store:      &stubUserStore{admin: verifiedUser},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "MFA_RESEND_COOLDOWN",
		},
		{
			name: "error - user lookup fails",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			},
			store:      &stubUserStore{getErr: errors.New("database error")},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MFA_CHALLENGE_INVALID",
		},
		{
			name: "error - account suspended after the password step",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			},
			store: &stubUserStore{admin: &userdomain.User{
				ID:     1,
				Status: "suspended",
			}},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_SUSPENDED",
		},
		{
			name: "error - account pending after the password step",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			},
			store: &stubUserStore{admin: &userdomain.User{
				ID:     1,
				Status: "pending",
			}},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "EMAIL_NOT_VERIFIED",
		},
		{
			name: "error - account soft-deleted after the password step",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			},
			store: &stubUserStore{admin: &userdomain.User{
				ID:     1,
				Status: "soft_deleted",
			}},
			body:       `{"challengeToken":"challenge-token"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_DELETED",
		},
		{
			name: "error - delivery fails",
			challenge: &authdomain.MFAChallenge{
				ID:        1,
				UserID:    1,
				CreatedAt: time.Now().Add(-2 * time.Minute),
				ExpiresAt: time.Now().Add(4 * time.Minute),
			},
			store:        &stubUserStore{admin: verifiedUser},
			delivererErr: errors.New("smtp down"),
			body:         `{"challengeToken":"challenge-token"}`,
			wantStatus:   http.StatusServiceUnavailable,
			wantCode:     "MFA_UNAVAILABLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubMFARepo{}
			if tt.challenge != nil {
				repo = seededMFARepo(tt.challenge)
			}

			service := authdomain.NewMFAService(repo, 5*time.Minute, time.Minute, 5)
			handler := mfaHandler(service, &stubDeliverer{err: tt.delivererErr}, tt.store, &stubIssuer{})

			req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/resend", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ResendMFA(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "status")

			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, decodeErrorCode(t, w), "error code")
				return
			}

			data := decodeData(t, w)
			assert.Equal(t, tt.wantHint, data["emailHint"], "email hint")
			assert.NotEmpty(t, data["challengeToken"], "challenge token")
		})
	}
}

func TestResendMFA_Unconfigured(t *testing.T) {
	handler := mfaHandler(nil, &stubDeliverer{}, &stubUserStore{}, &stubIssuer{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/resend", bytes.NewReader([]byte(`{"challengeToken":"challenge-token"}`)))
	w := httptest.NewRecorder()

	handler.ResendMFA(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "status")
	assert.Equal(t, "MFA_UNAVAILABLE", decodeErrorCode(t, w), "error code")
}

func TestResendMFA_NoDeliverer(t *testing.T) {
	service := authdomain.NewMFAService(&stubMFARepo{}, 5*time.Minute, time.Minute, 5)
	handler := mfaHandler(service, nil, &stubUserStore{}, &stubIssuer{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/resend", bytes.NewReader([]byte(`{"challengeToken":"challenge-token"}`)))
	w := httptest.NewRecorder()

	handler.ResendMFA(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "status")
	assert.Equal(t, "MFA_UNAVAILABLE", decodeErrorCode(t, w), "error code")
}

func TestLogin_MFADeliveryFailure(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{"admin": adminFixture()},
		admin:      adminFixture(),
	}
	store.byUsername["admin"].PasswordHash = "hashed:SecurePassword123!"
	store.admin.PasswordHash = "hashed:SecurePassword123!"
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mfaService := authdomain.NewMFAService(&stubMFARepo{}, 5*time.Minute, time.Minute, 5)
	handler := api.NewAuthHandler(
		authService,
		&stubIssuer{},
		nil,
		authdomain.NewFirstLoginService(policy),
		authdomain.NewRegistrationService(nil, policy, true),
		authdomain.NewVerificationService(nil, nil, 24),
		loginService,
		mfaService,
		authdomain.NewPasswordResetService(store, &stubResetStore{}, policy, 1),
		store,
		failed,
		&stubRecorder{},
		&stubMailer{},
		&stubDeliverer{err: errors.New("smtp down")},
		&stubBlockedChecker{},
	)

	bodyBytes, _ := json.Marshal(api.LoginRequest{Username: "admin", Password: "SecurePassword123!"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "status")
	assert.Equal(t, "MFA_UNAVAILABLE", decodeErrorCode(t, w), "error code")
}

func TestLogin_MFAUnconfigured(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{"admin": adminFixture()},
		admin:      adminFixture(),
	}
	store.byUsername["admin"].PasswordHash = "hashed:SecurePassword123!"
	store.admin.PasswordHash = "hashed:SecurePassword123!"
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	handler := api.NewAuthHandler(
		authService,
		&stubIssuer{},
		nil,
		authdomain.NewFirstLoginService(policy),
		authdomain.NewRegistrationService(nil, policy, true),
		authdomain.NewVerificationService(nil, nil, 24),
		loginService,
		nil,
		authdomain.NewPasswordResetService(store, &stubResetStore{}, policy, 1),
		store,
		failed,
		&stubRecorder{},
		&stubMailer{},
		nil,
		&stubBlockedChecker{},
	)

	bodyBytes, _ := json.Marshal(api.LoginRequest{Username: "admin", Password: "SecurePassword123!"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "status")
	assert.Equal(t, "MFA_UNAVAILABLE", decodeErrorCode(t, w), "error code")
}
