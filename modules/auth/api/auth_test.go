package api_test

import (
	"bytes"
	"context"
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

func adminFixture() *userdomain.User {
	return &userdomain.User{
		ID:           1,
		Username:     "admin",
		PasswordHash: "hashed:admin",
		Email:        "admin@example.com",
		Role:         "Admin",
		Status:       "verified",
		TokenVersion: 3,
	}
}

func decodeData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var envelope map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&envelope), "Failed to decode response")
	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok, "Response data should be an object")
	return data
}

func decodeErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var envelope map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&envelope), "Failed to decode response")
	require.NotNil(t, envelope["error"], "Response error should not be nil")
	errorMap, ok := envelope["error"].(map[string]any)
	require.True(t, ok, "Error is not a map")
	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	return code
}

func TestLogin_ValidCredentials(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{token: "test-jwt"}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{"admin": adminFixture()},
		admin:      adminFixture(),
	}
	store.byUsername["admin"].PasswordHash = "hashed:SecurePassword123!"
	store.admin.PasswordHash = "hashed:SecurePassword123!"
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler, deliverer := newAuthHandlerWithMFA(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create request with a non-default password so the password step always
	// starts the MFA challenge (default credentials go to first-login setup).
	reqBody := api.LoginRequest{
		Username: "admin",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute the password step
	handler.Login(w, req)

	// Assert the password step issued a challenge, not a session
	assert.Equal(t, http.StatusOK, w.Code, "Status")

	data := decodeData(t, w)

	dataBytes, _ := json.Marshal(data)
	var challenge api.MFARequiredResponse
	require.NoError(t, json.Unmarshal(dataBytes, &challenge), "Failed to parse challenge response")

	assert.True(t, challenge.MFARequired, "MFARequired")
	assert.NotEmpty(t, challenge.ChallengeToken, "ChallengeToken should not be empty")
	assert.Equal(t, "a***@example.com", challenge.EmailHint, "EmailHint")
	assert.NotEmpty(t, deliverer.code, "Code should have been delivered")

	// Execute the code step
	verifyBody := api.MFAVerifyRequest{
		ChallengeToken: challenge.ChallengeToken,
		Code:           deliverer.code,
	}
	verifyBytes, _ := json.Marshal(verifyBody)

	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(verifyBytes))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyW := httptest.NewRecorder()

	handler.VerifyMFA(verifyW, verifyReq)

	assert.Equal(t, http.StatusOK, verifyW.Code, "Verify status")

	verifyData := decodeData(t, verifyW)

	assert.NotContains(t, verifyW.Body.String(), `"token"`, "no token in login JSON")

	verifyDataBytes, _ := json.Marshal(verifyData)
	var loginResp api.LoginResponse
	require.NoError(t, json.Unmarshal(verifyDataBytes, &loginResp), "Failed to parse login response")

	assert.Equal(t, "admin", loginResp.User.Username, "Username")
	assert.Equal(t, "Admin", loginResp.User.Role, "Role")

	cookies := verifyW.Result().Cookies()
	require.Len(t, cookies, 1, "one session cookie")
	assert.Equal(t, "lesstruct_jwt", cookies[0].Name, "session cookie name")
	assert.True(t, cookies[0].HttpOnly, "session cookie is HttpOnly")
}

func TestLogin_InvalidCredentials(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	store := &stubUserStore{}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create request with wrong password (user missing -> invalid credentials
	// without reaching password verification)
	reqBody := api.LoginRequest{
		Username: "admin",
		Password: "wrongpassword",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.Login(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Status")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")

	assert.Nil(t, resp["data"], "Response data should be nil for error response")
	require.NotNil(t, resp["error"], "Response error should not be nil")
}

// newEmailLoginHandler builds the login handler for identifier-resolution
// cases. The mailer records the asynchronous lockout notification.
func newEmailLoginHandler(
	store *stubUserStore,
	failed *stubFailedStore,
	mailer *stubMailer,
) *api.AuthHandler {
	policy := stubPolicy{}
	passwordResetService := authdomain.NewPasswordResetService(store, &stubResetStore{}, policy, 1)
	loginService := authdomain.NewLoginService(store, failed, policy, nil)

	return newAuthHandler(
		authdomain.NewAuthService("hashed:admin", policy),
		&stubIssuer{token: "test-jwt"},
		nil,
		authdomain.NewFirstLoginService(policy),
		authdomain.NewRegistrationService(nil, policy, true),
		authdomain.NewVerificationService(nil, nil, 24),
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		&stubBlockedChecker{},
	)
}

func TestLogin_EmailIdentifier_ValidCredentials(t *testing.T) {
	admin := adminFixture()
	admin.PasswordHash = "hashed:SecurePassword123!"
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{"admin@example.com": admin},
		admin:   admin,
	}
	handler := newEmailLoginHandler(store, &stubFailedStore{}, &stubMailer{})

	bodyBytes, _ := json.Marshal(api.LoginRequest{
		Username: "admin@example.com",
		Password: "SecurePassword123!",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Status")

	data := decodeData(t, w)
	dataBytes, _ := json.Marshal(data)
	var challenge api.MFARequiredResponse
	require.NoError(t, json.Unmarshal(dataBytes, &challenge), "Failed to parse challenge response")

	assert.True(t, challenge.MFARequired, "MFARequired")
	assert.NotEmpty(t, challenge.ChallengeToken, "ChallengeToken should not be empty")
	assert.Equal(t, "a***@example.com", challenge.EmailHint, "EmailHint")
}

func TestLogin_EmailIdentifier_UnknownEmail(t *testing.T) {
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{"admin@example.com": adminFixture()},
	}
	handler := newEmailLoginHandler(store, &stubFailedStore{}, &stubMailer{})

	bodyBytes, _ := json.Marshal(api.LoginRequest{
		Username: "missing@example.com",
		Password: "SecurePassword123!",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Status")
	assert.Equal(t, "INVALID_CREDENTIALS", decodeErrorCode(t, w), "Error code")
}

func TestLogin_EmailIdentifier_LockoutEmailResolved(t *testing.T) {
	admin := adminFixture()
	admin.PasswordHash = "hashed:SecurePassword123!"
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{"admin@example.com": admin},
	}
	mailer := &stubMailer{}
	failed := &stubFailedStore{
		locked:           map[int]bool{admin.ID: true},
		sendLockoutEmail: true,
	}
	handler := newEmailLoginHandler(store, failed, mailer)

	bodyBytes, _ := json.Marshal(api.LoginRequest{
		Username: "admin@example.com",
		Password: "SecurePassword123!",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, "Status")
	assert.Equal(t, "ACCOUNT_LOCKED", decodeErrorCode(t, w), "Error code")

	require.Eventually(t, func() bool {
		return len(mailer.sentMails()) == 1
	}, time.Second, 10*time.Millisecond, "Expected a lockout email")

	sent := mailer.sentMails()
	require.Len(t, sent, 1)
	assert.Equal(t, "admin@example.com", sent[0].to, "Lockout email recipient")
	assert.Equal(t, "admin", sent[0].username, "Lockout email username")
}

func TestLogin_InvalidRequestBody(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	store := &stubUserStore{}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create invalid request
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.Login(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")
}

func TestLogin_DefaultCredentialsAfterFirstLogin(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	// Simulate setup completion: admin password has been changed from default
	changed := adminFixture()
	changed.PasswordHash = "hashed:ChangedPassword456!"

	store := &stubUserStore{admin: changed}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create request with default credentials (admin/admin)
	reqBody := api.LoginRequest{
		Username: authdomain.DefaultUsername,
		Password: authdomain.DefaultPassword,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.Login(w, req)

	// Assert - should return 401 with DEFAULT_CREDENTIALS_INVALID error
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Status")
	assert.Equal(t, "DEFAULT_CREDENTIALS_INVALID", decodeErrorCode(t, w), "Error code")
}

func TestLogin_FirstLoginBootstrap(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{token: "test-jwt"}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	// The seeded admin has no email address until first-login setup finishes.
	admin := adminFixture()
	admin.Email = ""
	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{"admin": admin},
		admin:      admin,
	}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	passwordResetService := authdomain.NewPasswordResetService(store, &stubResetStore{}, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		&stubMailer{},
		&stubBlockedChecker{},
	)

	bodyBytes, _ := json.Marshal(api.LoginRequest{
		Username: authdomain.DefaultUsername,
		Password: authdomain.DefaultPassword,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Status")

	data := decodeData(t, w)
	assert.Equal(t, true, data["firstLoginRequired"], "firstLoginRequired")
	assert.NotContains(t, data, "token", "bootstrap must not issue a session")
}

func TestLogin_DefaultCredentialsAdminLookupFailure(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	store := &stubUserStore{
		admin:    adminFixture(),
		adminErr: errors.New("database error"),
	}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	passwordResetService := authdomain.NewPasswordResetService(store, &stubResetStore{}, policy, 1)
	handler := newAuthHandler(
		authService,
		&stubIssuer{},
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		&stubMailer{},
		&stubBlockedChecker{},
	)

	bodyBytes, _ := json.Marshal(api.LoginRequest{
		Username: authdomain.DefaultUsername,
		Password: authdomain.DefaultPassword,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "Status")
	assert.Equal(t, "LOGIN_FAILED", decodeErrorCode(t, w), "Error code")
}

func TestVerifyEmail_ValidToken(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)

	// Create user to verify
	testUser := &userdomain.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
		Status:   "pending",
	}

	// Store tokens in memory for the stub.
	storedTokens := map[string]*authdomain.VerificationToken{}
	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{"testuser": testUser},
		byID:       map[int]*userdomain.User{testUser.ID: testUser},
	}
	tokens := &stubTokenStore{tokens: storedTokens}

	verificationService := authdomain.NewVerificationService(store, tokens, 24)

	recorder := &stubRecorder{}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		recorder,
		mailer,
		blocked,
	)

	// Create a verification token
	tokenResult, err := verificationService.CreateVerificationToken(context.Background(), testUser.ID)
	require.NoError(t, err, "Failed to create verification token")

	// Create request with valid token
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token="+tokenResult.Token, nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Status")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")

	assert.NotNil(t, resp["data"], "Response data should not be nil")

	// Verify user status was updated
	assert.Equal(t, "verified", testUser.Status, "User status")

	// Verify the admin notification fired
	require.Len(t, recorder.verified, 1)
	assert.Equal(t, "testuser", recorder.verified[0])
}

func TestVerifyEmail_EmptyToken(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	store := &stubUserStore{}
	tokens := &stubTokenStore{}
	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create request without token
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")
	assert.Equal(t, "INVALID_TOKEN", decodeErrorCode(t, w), "Error code")
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	store := &stubUserStore{}
	tokens := &stubTokenStore{}
	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create request with invalid token
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=invalidtoken123", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")
	assert.Equal(t, "INVALID_TOKEN", decodeErrorCode(t, w), "Error code")
}

func TestVerifyEmail_ExpiredToken(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)

	store := &stubUserStore{}
	tokens := &stubTokenStore{
		tokens: map[string]*authdomain.VerificationToken{
			"expired-hash": {
				ID:        1,
				UserID:    1,
				TokenHash: "expired-hash",
				ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedAt: time.Date(2019, 12, 31, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Create request with a token (will be found but expired). The stub looks
	// tokens up by exact hash, so seed the expired token under the hash the
	// service computes for the presented value.
	tokens.tokens[authdomain.HashToken("sometoken")] = tokens.tokens["expired-hash"]
	delete(tokens.tokens, "expired-hash")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=sometoken", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")
	assert.Equal(t, "TOKEN_EXPIRED", decodeErrorCode(t, w), "Error code")
}

func TestRegisterUser_Success(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)

	store := &stubUserStore{}
	tokens := &stubTokenStore{}

	registrationService := authdomain.NewRegistrationService(store, policy, true)
	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	recorder := &stubRecorder{}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		recorder,
		mailer,
		blocked,
	)

	// Create request
	reqBody := api.RegisterUserRequest{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.RegisterUser(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Status")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "Response data should be an object")
	assert.Equal(t, "Registration successful. Please check your email to verify your account.", data["message"], "Message")
	assert.NotContains(t, resp["data"], "userId", "the reply must not include an account ID")

	// Registration notifies the admin; the verification email is sent from a
	// background goroutine (asserted best-effort via the async send below).
	require.Len(t, recorder.registered, 1)
	assert.Equal(t, "newuser", recorder.registered[0])
}

func TestRegisterUser_CommentsDisabled_ReturnsForbidden(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)

	// Comments disabled → registration blocked with 403 before any repo call
	// (the blocked-email repo must NOT be queried).
	registrationService := authdomain.NewRegistrationService(nil, policy, false)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(nil, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(nil, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		nil,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	reqBody := api.RegisterUserRequest{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterUser(w, req)

	// The gate must short-circuit before the blocked-email lookup.
	assert.False(t, blocked.checked, "blocked-email repo must not be queried")

	assert.Equal(t, http.StatusForbidden, w.Code, "registration must be 403 when comments are disabled")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")
	require.NotNil(t, resp["error"], "Response error should not be nil")
	errorMap, ok := resp["error"].(map[string]any)
	require.True(t, ok, "error should be a JSON object")
	assert.Equal(t, "REGISTRATION_DISABLED", errorMap["code"], "error code")
}

// TestRegisterUser_DuplicateIsAcceptedLikeSuccess proves the anti-enumeration
// contract: a duplicate username or email gets the exact same 200 reply as a
// fresh registration, and the notice email goes to the account holder — never
// back to the requester.
func TestRegisterUser_DuplicateIsAcceptedLikeSuccess(t *testing.T) {
	existing := &userdomain.User{
		ID:       7,
		Username: "existinguser",
		Email:    "existing@example.com",
		Status:   "verified",
	}

	tests := []struct {
		name          string
		username      string
		email         string
		store         *stubUserStore
		wantNoticeTo  string
		wantNoticeFor string
	}{
		{
			name:     "success - duplicate username notifies the username's account",
			username: "existinguser",
			email:    "newuser@example.com",
			store: &stubUserStore{
				byUsername:     map[string]*userdomain.User{"existinguser": existing},
				usernameExists: map[string]bool{"existinguser": true},
			},
			wantNoticeTo:  "existing@example.com",
			wantNoticeFor: "existinguser",
		},
		{
			name:     "success - duplicate email notifies the email's account",
			username: "newuser",
			email:    "existing@example.com",
			store: &stubUserStore{
				byEmail:     map[string]*userdomain.User{"existing@example.com": existing},
				emailExists: map[string]bool{"existing@example.com": true},
			},
			wantNoticeTo:  "existing@example.com",
			wantNoticeFor: "existinguser",
		},
		{
			name:     "success - unresolvable collision still reads as accepted",
			username: "ghost",
			email:    "ghost@example.com",
			store: &stubUserStore{
				usernameExists: map[string]bool{"ghost": true},
			},
			wantNoticeTo: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := stubPolicy{}
			authService := authdomain.NewAuthService("hashed:admin", policy)
			issuer := &stubIssuer{}
			firstLoginService := authdomain.NewFirstLoginService(policy)

			tokens := &stubTokenStore{}
			registrationService := authdomain.NewRegistrationService(tt.store, policy, true)
			verificationService := authdomain.NewVerificationService(tt.store, tokens, 24)
			failed := &stubFailedStore{}
			loginService := authdomain.NewLoginService(tt.store, failed, policy, nil)
			mailer := &stubMailer{}
			blocked := &stubBlockedChecker{blocked: map[string]bool{}}
			resets := &stubResetStore{}
			passwordResetService := authdomain.NewPasswordResetService(tt.store, resets, policy, 1)
			handler := newAuthHandler(
				authService,
				issuer,
				nil,
				firstLoginService,
				registrationService,
				verificationService,
				loginService,
				passwordResetService,
				tt.store,
				failed,
				&stubRecorder{},
				mailer,
				blocked,
			)

			reqBody := api.RegisterUserRequest{
				Username: tt.username,
				Email:    tt.email,
				Password: "SecurePassword123!",
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.RegisterUser(w, req)

			// The reply is byte-identical to a fresh registration.
			assert.Equal(t, http.StatusOK, w.Code, "Status")
			assert.NotContains(t, w.Body.String(), "ACCOUNT_EXISTS", "error envelope must not leak the conflict")
			assert.NotContains(t, w.Body.String(), "userId", "the reply must not include an account ID")

			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")
			data, ok := resp["data"].(map[string]any)
			require.True(t, ok, "Response data should be an object")
			assert.Equal(t, "Registration successful. Please check your email to verify your account.", data["message"], "Message")

			if tt.wantNoticeTo == "" {
				return
			}

			// The notice rides a background goroutine.
			require.Eventually(t, func() bool {
				return len(mailer.sentMails()) == 1
			}, time.Second, 10*time.Millisecond, "Expected a registration attempt notice")

			sent := mailer.sentMails()
			require.Len(t, sent, 1)
			assert.Equal(t, tt.wantNoticeTo, sent[0].to, "Notice recipient")
			assert.Equal(t, tt.wantNoticeFor, sent[0].username, "Notice account")
			assert.Empty(t, sent[0].token, "The notice must carry no action token")
		})
	}
}

func TestRegisterUser_InvalidPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantCode string
		wantMsg  string
	}{
		{
			name:     "success - weak password maps to rule text",
			password: "weak",
			wantCode: "INVALID_PASSWORD",
			wantMsg:  "password must be at least 12 characters",
		},
		{
			name:     "success - short password maps to rule text",
			password: "short",
			wantCode: "INVALID_PASSWORD",
			wantMsg:  "password must be at least 12 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := stubPolicy{passwordErr: authdomain.ErrInvalidPassword}
			authService := authdomain.NewAuthService("hashed:admin", policy)
			issuer := &stubIssuer{}
			firstLoginService := authdomain.NewFirstLoginService(policy)

			store := &stubUserStore{}
			tokens := &stubTokenStore{}

			registrationService := authdomain.NewRegistrationService(store, policy, true)
			verificationService := authdomain.NewVerificationService(store, tokens, 24)
			failed := &stubFailedStore{}
			loginService := authdomain.NewLoginService(store, failed, policy, nil)
			mailer := &stubMailer{}
			blocked := &stubBlockedChecker{}
			resets := &stubResetStore{}
			passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
			handler := newAuthHandler(
				authService,
				issuer,
				nil,
				firstLoginService,
				registrationService,
				verificationService,
				loginService,
				passwordResetService,
				store,
				failed,
				&stubRecorder{},
				mailer,
				blocked,
			)

			reqBody := api.RegisterUserRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
				Password: tt.password,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.RegisterUser(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, "Status")
			raw := w.Body.String()
			assert.Contains(t, raw, tt.wantCode, "Error code present")
			assert.Contains(t, raw, tt.wantMsg, "Error message states the rule")
		})
	}
}

func TestRegisterUser_MissingFields(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)

	store := &stubUserStore{}
	tokens := &stubTokenStore{}

	registrationService := authdomain.NewRegistrationService(store, policy, true)
	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	tests := []struct {
		name     string
		reqBody  api.RegisterUserRequest
		expected int
	}{
		{
			name: "Missing username",
			reqBody: api.RegisterUserRequest{
				Username: "",
				Email:    "test@example.com",
				Password: "SecurePassword123!",
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "Missing email",
			reqBody: api.RegisterUserRequest{
				Username: "testuser",
				Email:    "",
				Password: "SecurePassword123!",
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "Missing password",
			reqBody: api.RegisterUserRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "",
			},
			expected: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			handler.RegisterUser(w, req)

			// Assert
			assert.Equal(t, tt.expected, w.Code, "Status")
		})
	}
}

func TestRegisterUser_BlockedEmail(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	store := &stubUserStore{}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{blocked: map[string]bool{"blocked@example.com": true}}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Try to register with blocked email
	reqBody := api.RegisterUserRequest{
		Username: "blockeduser",
		Email:    "blocked@example.com",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterUser(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "Status")

	var resp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")
	assert.Equal(t, "This email address has been blocked", resp.Error.Message, "Error message")
}

func TestLogin_SuspendedUser(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{
			"suspendeduser": {
				ID:           123,
				Username:     "suspendeduser",
				Email:        "suspended@example.com",
				PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz123456",
				Status:       "suspended",
				Role:         "Author",
			},
		},
	}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Attempt login with suspended user
	reqBody := api.LoginRequest{
		Username: "suspendeduser",
		Password: "SomePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "Status")

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")
	assert.Equal(t, "ACCOUNT_SUSPENDED", resp.Error.Code, "Error code")
	assert.Equal(t, "Your account has been suspended. Please contact an administrator.", resp.Error.Message, "Error message")
}

func TestLogin_SoftDeletedUser(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)
	registrationService := authdomain.NewRegistrationService(nil, policy, true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{
			"deleteduser": {
				ID:           124,
				Username:     "deleteduser",
				Email:        "deleted@example.com",
				PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz123456",
				Status:       "soft_deleted",
				Role:         "Author",
			},
		},
	}
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := newAuthHandler(
		authService,
		issuer,
		nil,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		store,
		failed,
		&stubRecorder{},
		mailer,
		blocked,
	)

	// Attempt login with soft-deleted user
	reqBody := api.LoginRequest{
		Username: "deleteduser",
		Password: "SomePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "Status")
}
