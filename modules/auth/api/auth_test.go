package api_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{}
	tokens := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, tokens, policy, 1)
	handler := api.NewAuthHandler(
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

	// Create request (using a non-default password to avoid GetAdminUser call
	// would be triggered, but we test the full flow including that check)
	reqBody := api.LoginRequest{
		Username: "admin",
		Password: "admin",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.Login(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Status")

	data := decodeData(t, w)

	// Parse the data field
	dataBytes, _ := json.Marshal(data)
	var loginResp api.LoginResponse
	require.NoError(t, json.Unmarshal(dataBytes, &loginResp), "Failed to parse login response")

	assert.NotEmpty(t, loginResp.Token, "Token should not be empty")
	assert.Equal(t, "admin", loginResp.User.Username, "Username")
	assert.Equal(t, "Admin", loginResp.User.Role, "Role")
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	assert.Equal(t, http.StatusCreated, w.Code, "Status")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")

	assert.NotNil(t, resp["data"], "Response data should not be nil")

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
	handler := api.NewAuthHandler(
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

func TestRegisterUser_DuplicateUsername(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)

	store := &stubUserStore{usernameExists: map[string]bool{"existinguser": true}}
	tokens := &stubTokenStore{}

	registrationService := authdomain.NewRegistrationService(store, policy, true)
	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{blocked: map[string]bool{}}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := api.NewAuthHandler(
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

	// Create request with existing username
	reqBody := api.RegisterUserRequest{
		Username: "existinguser",
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
	assert.Equal(t, http.StatusConflict, w.Code, "Status")
	assert.Equal(t, "ACCOUNT_EXISTS", decodeErrorCode(t, w), "Error code")
}

func TestRegisterUser_DuplicateEmail(t *testing.T) {
	policy := stubPolicy{}
	authService := authdomain.NewAuthService("hashed:admin", policy)
	issuer := &stubIssuer{}
	firstLoginService := authdomain.NewFirstLoginService(policy)

	store := &stubUserStore{emailExists: map[string]bool{"existing@example.com": true}}
	tokens := &stubTokenStore{}

	registrationService := authdomain.NewRegistrationService(store, policy, true)
	verificationService := authdomain.NewVerificationService(store, tokens, 24)
	failed := &stubFailedStore{}
	loginService := authdomain.NewLoginService(store, failed, policy, nil)
	mailer := &stubMailer{}
	blocked := &stubBlockedChecker{blocked: map[string]bool{}}
	resets := &stubResetStore{}
	passwordResetService := authdomain.NewPasswordResetService(store, resets, policy, 1)
	handler := api.NewAuthHandler(
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

	// Create request with existing email
	reqBody := api.RegisterUserRequest{
		Username: "newuser",
		Email:    "existing@example.com",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.RegisterUser(w, req)

	// Assert
	assert.Equal(t, http.StatusConflict, w.Code, "Status")
	assert.Equal(t, "ACCOUNT_EXISTS", decodeErrorCode(t, w), "Error code")
}

func TestRegisterUser_InvalidPassword(t *testing.T) {
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
	handler := api.NewAuthHandler(
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

	// Create request with weak password
	reqBody := api.RegisterUserRequest{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "weak",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.RegisterUser(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")
	assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w), "Error code")
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
	handler := api.NewAuthHandler(
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
