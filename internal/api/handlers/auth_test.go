package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	appresponse "github.com/aristorinjuang/lesstruct/internal/api/response"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLogin_ValidCredentials(t *testing.T) {
	// Setup
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password for test")

	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: hash,
			Role:         "Admin",
		}, nil)
	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, "admin").
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: hash,
			Email:        "admin@example.com",
			Role:         "Admin",
			Status:       "verified",
		}, nil)

	userRepo.EXPECT().
		UpdateLastLoginAt(mock.Anything, mock.Anything).
		Return(nil)

	notificationRepo := repomocks.NewMockNotificationRepo(t)

	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	failedLoginRepo.EXPECT().
		IsLocked(mock.Anything, mock.Anything).
		Return(false, (*time.Time)(nil), nil)
	failedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, mock.Anything).
		Return(nil)

	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(
		authService,
		jwtManager,
		logger,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		userRepo,
		failedLoginRepo,
		notificationRepo,
		emailService,
		blockedEmailRepo,
	)

	// Create request (using a non-default password to avoid GetAdminUser call
	// would be triggered, but we test the full flow including that check)
	reqBody := handlers.LoginRequest{
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

	var resp appresponse.Response
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	assert.NotNil(t, resp.Data, "Response data should not be nil")

	// Parse the data field
	dataBytes, _ := json.Marshal(resp.Data)
	var loginResp handlers.LoginResponse
	err = json.Unmarshal(dataBytes, &loginResp)
	require.NoError(t, err, "Failed to parse login response")

	assert.NotEmpty(t, loginResp.Token, "Token should not be empty")
	assert.Equal(t, "admin", loginResp.User.Username, "Username")
	assert.Equal(t, "Admin", loginResp.User.Role, "Role")
}

func TestLogin_InvalidCredentials(t *testing.T) {
	// Setup
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password for test")

	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, mock.Anything).
		Return((*repository.User)(nil), nil)

	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with wrong password
	reqBody := handlers.LoginRequest{
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

	var resp appresponse.Response
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	assert.Nil(t, resp.Data, "Response data should be nil for error response")
	require.NotNil(t, resp.Error, "Response error should not be nil")
}

func TestLogin_InvalidRequestBody(t *testing.T) {
	// Setup
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password for test")

	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	userRepo := repomocks.NewMockUserRepo(t)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

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
	// Setup - first-login is complete
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password for test")

	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	// Simulate setup completion: admin password has been changed from default
	changedHash, err := appauth.HashPassword("ChangedPassword456!")
	require.NoError(t, err, "Failed to hash changed password for test")

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: changedHash,
			Role:         "Admin",
		}, nil)

	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with default credentials (admin/admin)
	reqBody := handlers.LoginRequest{
		Username: constants.DefaultUsername,
		Password: constants.DefaultPassword,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.Login(w, req)

	// Assert - should return 401 with DEFAULT_CREDENTIALS_INVALID error
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Status")

	var resp appresponse.Response
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")

	// When decoded from JSON, error is a map[string]any
	errorMap, ok := resp.Error.(map[string]any)
	require.True(t, ok, "Error is not a map")

	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	assert.Equal(t, "DEFAULT_CREDENTIALS_INVALID", code, "Error code")
}

func TestVerifyEmail_ValidToken(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)

	// Create user to verify
	testUser := &repository.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
		Status:   "pending",
	}

	// Store tokens in memory for mock
	storedTokens := make(map[string]*repository.VerificationToken)

	userRepo := repomocks.NewMockUserRepo(t)

	// Configure verification token mock to store and retrieve tokens.
	// Order matters: CreateVerificationToken calls DeleteUserTokens then CreateToken.
	// VerifyEmail calls FindValidToken, UpdateUserStatus, then DeleteUserTokens.
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, userID int) error {
			// Delete all tokens for this user (called by CreateVerificationToken)
			for hash, token := range storedTokens {
				if token.UserID == userID {
					delete(storedTokens, hash)
				}
			}
			return nil
		})
	verificationTokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, token *repository.VerificationToken) {
			storedTokens[token.TokenHash] = token
		}).
		Return(nil)
	verificationTokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, tokenHash string) (*repository.VerificationToken, error) {
			if token, exists := storedTokens[tokenHash]; exists {
				return token, nil
			}
			return nil, nil
		})
	verificationTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, userID int) error {
			// Delete all tokens for this user (post-verification cleanup)
			for hash, token := range storedTokens {
				if token.UserID == userID {
					delete(storedTokens, hash)
				}
			}
			return nil
		})

	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)

	// UserRepo expectations: called by VerifyEmail handler after verification succeeds.
	// Order: UpdateUserStatus (from VerifyEmail service), then GetUserByID (from handler).
	userRepo.EXPECT().
		UpdateUserStatus(mock.Anything, testUser.ID, "verified").
		Run(func(ctx context.Context, userID int, status string) {
			testUser.Status = status
		}).
		Return(nil)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, testUser.ID).
		Return(testUser, nil)

	notificationRepo := repomocks.NewMockNotificationRepo(t)
	notificationRepo.EXPECT().
		CreateNotification(mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

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

	var resp appresponse.Response
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	assert.NotNil(t, resp.Data, "Response data should not be nil")

	// Verify user status was updated
	assert.Equal(t, "verified", testUser.Status, "User status")
}

func TestVerifyEmail_EmptyToken(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	userRepo := repomocks.NewMockUserRepo(t)
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request without token
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")

	errorMap, ok := resp.Error.(map[string]any)
	require.True(t, ok, "Error is not a map")

	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	assert.Equal(t, "INVALID_TOKEN", code, "Error code")
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	userRepo := repomocks.NewMockUserRepo(t)
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return((*repository.VerificationToken)(nil), nil)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with invalid token
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=invalidtoken123", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")

	errorMap, ok := resp.Error.(map[string]any)
	require.True(t, ok, "Error is not a map")

	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	assert.Equal(t, "INVALID_TOKEN", code, "Error code")
}

func TestVerifyEmail_ExpiredToken(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)

	// Configure user mock
	userRepo := repomocks.NewMockUserRepo(t)

	// Configure verification token mock to return an expired token
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.VerificationToken{
			ID:        1,
			UserID:    1,
			TokenHash: "sometokenhash",
			ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), // Past date
			CreatedAt: time.Date(2019, 12, 31, 0, 0, 0, 0, time.UTC),
		}, nil)

	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with a token (will be found but expired)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=sometoken", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.VerifyEmail(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")

	errorMap, ok := resp.Error.(map[string]any)
	require.True(t, ok, "Error is not a map")

	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	assert.Equal(t, "TOKEN_EXPIRED", code, "Error code")
}

func TestRegisterUser_Success(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		CheckUsernameExists(mock.Anything, "newuser").
		Return(false, nil)
	userRepo.EXPECT().
		CheckEmailExists(mock.Anything, "newuser@example.com").
		Return(false, nil)
	userRepo.EXPECT().
		CreateUser(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, user *repository.User) {
			user.ID = 1 // Simulate database assigning ID
		}).
		Return(nil)

	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, mock.Anything).
		Return(nil)
	verificationTokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Return(nil)

	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	notificationRepo.EXPECT().
		CreateNotification(mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	emailService.EXPECT().
		SendVerificationEmail(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "newuser@example.com").
		Return(false, nil)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request
	reqBody := handlers.RegisterUserRequest{
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

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	assert.NotNil(t, resp.Data, "Response data should not be nil")
}

func TestRegisterUser_DuplicateUsername(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		CheckUsernameExists(mock.Anything, "existinguser").
		Return(true, nil)

	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "newuser@example.com").
		Return(false, nil)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with existing username
	reqBody := handlers.RegisterUserRequest{
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

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")

	errorMap, ok := resp.Error.(map[string]any)
	require.True(t, ok, "Error is not a map")

	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	assert.Equal(t, "ACCOUNT_EXISTS", code, "Error code")
}

func TestRegisterUser_DuplicateEmail(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		CheckUsernameExists(mock.Anything, "newuser").
		Return(false, nil)
	userRepo.EXPECT().
		CheckEmailExists(mock.Anything, "existing@example.com").
		Return(true, nil)

	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "existing@example.com").
		Return(false, nil)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with existing email
	reqBody := handlers.RegisterUserRequest{
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

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")

	errorMap, ok := resp.Error.(map[string]any)
	require.True(t, ok, "Error is not a map")

	code, ok := errorMap["code"].(string)
	require.True(t, ok, "Error code is not a string")
	assert.Equal(t, "ACCOUNT_EXISTS", code, "Error code")
}

func TestRegisterUser_InvalidPassword(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)

	userRepo := repomocks.NewMockUserRepo(t)
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "newuser@example.com").
		Return(false, nil)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	// Create request with weak password
	reqBody := handlers.RegisterUserRequest{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "weak", // Too short, doesn't meet requirements
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.RegisterUser(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status")

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	require.NotNil(t, resp.Error, "Response error should not be nil")
}

func TestRegisterUser_MissingFields(t *testing.T) {
	// Setup
	hash, _ := appauth.HashPassword("admin")
	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)

	userRepo := repomocks.NewMockUserRepo(t)
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	handler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, userRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)

	tests := []struct {
		name     string
		reqBody  handlers.RegisterUserRequest
		expected int
	}{
		{
			name: "Missing username",
			reqBody: handlers.RegisterUserRequest{
				Username: "",
				Email:    "test@example.com",
				Password: "SecurePassword123!",
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "Missing email",
			reqBody: handlers.RegisterUserRequest{
				Username: "testuser",
				Email:    "",
				Password: "SecurePassword123!",
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "Missing password",
			reqBody: handlers.RegisterUserRequest{
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
