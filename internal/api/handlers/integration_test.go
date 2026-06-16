package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/mock"
)

// TestCompleteLoginFlow tests the full login flow from HTTP request to JWT token
func TestCompleteLoginFlow(t *testing.T) {
	// Setup
	defaultPasswordHash, err := auth.HashPassword(constants.DefaultPassword)
	if err != nil {
		t.Fatalf("Failed to hash default password: %v", err)
	}

	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: defaultPasswordHash,
			Email:        "admin@example.com",
			Role:         "Admin",
			Status:       "verified",
		}, nil)
	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, "admin").
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: defaultPasswordHash,
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
		IsLocked(mock.Anything, 1).
		Return(false, (*time.Time)(nil), nil)
	failedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, 1).
		Return(nil)

	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
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

	// Test login with correct credentials
	reqBody := handlers.LoginRequest{
		Username: "admin",
		Password: "admin",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	// Assert response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify JWT token is returned
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("Response data should be an object")
	}

	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Error("JWT token should be returned in response")
	}

	// Verify token can be validated
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		t.Errorf("Failed to validate JWT token: %v", err)
	}

	if claims.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", claims.Username)
	}

	if claims.Role != "Admin" {
		t.Errorf("Expected role 'Admin', got '%s'", claims.Role)
	}
}

// TestLoginWithEmptyCredentials tests that empty credentials are rejected
func TestLoginWithEmptyCredentials(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
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

	authHandler := handlers.NewAuthHandler(
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

	tests := []struct {
		name     string
		username string
		password string
		expected int
	}{
		{"Empty username and password", "", "", http.StatusBadRequest},
		{"Empty username only", "", "admin", http.StatusBadRequest},
		{"Empty password only", "admin", "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := handlers.LoginRequest{
				Username: tt.username,
				Password: tt.password,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			authHandler.Login(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

// TestLoginWithInvalidJSON tests that malformed JSON is rejected
func TestLoginWithInvalidJSON(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
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

	authHandler := handlers.NewAuthHandler(
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

	// Test with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}
}

// TestLoginResponseStructure tests the structure of the login response
func TestLoginResponseStructure(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)

	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: defaultPasswordHash,
			Email:        "admin@example.com",
			Role:         "Admin",
			Status:       "verified",
		}, nil)
	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, "admin").
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: defaultPasswordHash,
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
		IsLocked(mock.Anything, 1).
		Return(false, (*time.Time)(nil), nil)
	failedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, 1).
		Return(nil)

	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
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

	reqBody := handlers.LoginRequest{
		Username: "admin",
		Password: "admin",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check response structure
	if _, ok := resp["data"]; !ok {
		t.Error("Response should have 'data' field")
	}

	if _, ok := resp["error"]; ok {
		t.Error("Successful response should not have 'error' field")
	}

	// Check data structure
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("Response data should be an object")
	}

	if _, ok := data["token"]; !ok {
		t.Error("Response data should have 'token' field")
	}

	if _, ok := data["user"]; !ok {
		t.Error("Response data should have 'user' field")
	}

	// Check user structure
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatal("User should be an object")
	}

	if _, ok := user["id"]; !ok {
		t.Error("User should have 'id' field")
	}

	if _, ok := user["username"]; !ok {
		t.Error("User should have 'username' field")
	}

	if _, ok := user["role"]; !ok {
		t.Error("User should have 'role' field")
	}
}

// TestCompleteRegistrationFlow tests the full registration flow from HTTP request to database
func TestCompleteRegistrationFlow(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)

	// Store created user for verification (with mutex for race condition)
	var createdUser *repository.User
	var mu sync.Mutex

	// Configure user repository mock
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(false, nil)
	userRepo.EXPECT().
		CheckEmailExists(mock.Anything, mock.Anything).
		Return(false, nil)
	userRepo.EXPECT().
		CreateUser(mock.Anything, mock.Anything).
		Run(func(_ context.Context, user *repository.User) {
			user.ID = 123 // Simulate database assigning ID
			mu.Lock()
			createdUser = user
			mu.Unlock()
		}).
		Return(nil)

	// Configure verification token repository mock
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
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, mock.Anything).
		Return(false, nil)

	// Configure email service mock - use Maybe since it runs in a goroutine
	emailService := emailmocks.NewMockEmailService(t)
	emailService.EXPECT().
		SendVerificationEmail(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
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

	// Execute registration request
	reqBody := handlers.RegisterUserRequest{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.RegisterUser(w, req)

	// Wait for asynchronous email sending to complete
	time.Sleep(100 * time.Millisecond)

	// Assert response status
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	// Verify user was created with pending status
	mu.Lock()
	if createdUser == nil {
		mu.Unlock()
		t.Fatal("User should have been created")
	}

	if createdUser.Status != "pending" {
		status := createdUser.Status
		mu.Unlock()
		t.Errorf("User status = %s; want pending", status)
	} else if createdUser.Role != "Commentator" {
		role := createdUser.Role
		mu.Unlock()
		t.Errorf("User role = %s; want Commentator", role)
	} else {
		mu.Unlock()
	}

	// Verify response structure
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("Response should have data field")
	}

	if _, ok := data["message"]; !ok {
		t.Error("Response data should have message field")
	}

	if _, ok := data["userId"]; !ok {
		t.Error("Response data should have userId field")
	}
}

// TestCompleteVerificationFlow tests the full verification flow
func TestCompleteVerificationFlow(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)

	// Create user to verify
	testUser := &repository.User{
		ID:       123,
		Username: "testuser",
		Email:    "test@example.com",
		Status:   "pending",
	}

	// Configure user repository mock
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, testUser.ID).
		Return(testUser, nil)
	userRepo.EXPECT().
		UpdateUserStatus(mock.Anything, testUser.ID, "verified").
		Run(func(_ context.Context, _ int, status string) {
			testUser.Status = status
		}).
		Return(nil)

	// Configure verification token repository mock
	var storedToken *repository.VerificationToken

	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, testUser.ID).
		Return(nil)
	verificationTokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Run(func(_ context.Context, token *repository.VerificationToken) {
			token.ID = 1
			storedToken = token
		}).
		Return(nil)
	verificationTokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, tokenHash string) (*repository.VerificationToken, error) {
			if storedToken != nil && storedToken.TokenHash == tokenHash {
				return storedToken, nil
			}
			return nil, nil
		})
	verificationTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, testUser.ID).
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
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
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

	// Create verification token
	tokenResult, err := verificationService.CreateVerificationToken(context.Background(), testUser.ID)
	if err != nil {
		t.Fatalf("Failed to create verification token: %v", err)
	}

	// Execute verification request
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token="+tokenResult.Token, nil)
	w := httptest.NewRecorder()

	authHandler.VerifyEmail(w, req)

	// Assert response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify user status was updated to verified
	if testUser.Status != "verified" {
		t.Errorf("User status = %s; want verified", testUser.Status)
	}

	// Verify response structure
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("Response should have data field")
	}

	if _, ok := data["message"]; !ok {
		t.Error("Response data should have message field")
	}

	if _, ok := data["redirect"]; !ok {
		t.Error("Response data should have redirect field")
	}
}

// TestExpiredTokenFlow tests the expired token scenario
func TestExpiredTokenFlow(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)

	// Configure user repository mock
	userRepo := repomocks.NewMockUserRepo(t)

	// Configure verification token repository mock to return expired token
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.VerificationToken{
			ID:        1,
			UserID:    123,
			TokenHash: "expiredtoken",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
			CreatedAt: time.Now().Add(-25 * time.Hour),
		}, nil)

	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
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

	// Execute verification request with expired token
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=expiredtoken", nil)
	w := httptest.NewRecorder()

	authHandler.VerifyEmail(w, req)

	// Assert response status
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify response error structure
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	errorData, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("Response should have error field")
	}

	if errorData["code"] != "TOKEN_EXPIRED" {
		t.Errorf("Error code = %v; want TOKEN_EXPIRED", errorData["code"])
	}
}

// TestResendVerificationFlow tests the resend verification flow
func TestResendVerificationFlow(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)

	// Create pending user
	pendingUser := &repository.User{
		ID:       123,
		Username: "pendinguser",
		Email:    "pending@example.com",
		Status:   "pending",
	}

	// Configure user repository mock
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, pendingUser.Email).
		Return(pendingUser, nil)

	// Configure verification token repository mock
	var newVerificationTokenHash string

	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	verificationTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, pendingUser.ID).
		Return(nil)
	verificationTokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Run(func(_ context.Context, token *repository.VerificationToken) {
			newVerificationTokenHash = token.TokenHash
		}).
		Return(nil)

	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)

	// Configure email service mock - use Maybe since it runs in a goroutine
	emailService := emailmocks.NewMockEmailService(t)
	emailService.EXPECT().
		SendVerificationEmail(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
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

	// Execute resend verification request
	reqBody := handlers.ResendVerificationRequest{
		Email: pendingUser.Email,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.ResendVerificationEmail(w, req)

	// Wait for asynchronous email sending to complete
	time.Sleep(100 * time.Millisecond)

	// Assert response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify new verification token was created
	if newVerificationTokenHash == "" {
		t.Error("New verification token should have been created")
	}

	// Verify response structure
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("Response should have data field")
	}

	if _, ok := data["message"]; !ok {
		t.Error("Response data should have message field")
	}
}

// TestForgotPasswordAndResetFlow tests forgot-password -> reset-password -> login with new password
func TestForgotPasswordAndResetFlow(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)

	newPasswordHash, _ := auth.HashPassword("NewP@ssw0rd!12345")

	userRepo := repomocks.NewMockUserRepo(t)
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	emailService := emailmocks.NewMockEmailService(t)
	emailService.EXPECT().
		SendPasswordResetEmail(mock.Anything, "user@example.com", "TestUser", mock.Anything).
		Return(nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	authHandler := handlers.NewAuthHandler(
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

	// Step 1: Forgot password request
	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(&repository.User{
			ID:     1,
			Email:  "user@example.com",
			Name:   "TestUser",
			Status: "verified",
		}, nil)

	var capturedTokenHash string
	passwordResetTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 1).
		Return(nil)
	passwordResetTokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Run(func(_ context.Context, token *repository.PasswordResetToken) {
			capturedTokenHash = token.TokenHash
		}).
		Return(nil)

	forgotBody, _ := json.Marshal(handlers.ForgotPasswordRequest{Email: "user@example.com"})
	forgotReq := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(forgotBody))
	forgotReq.Header.Set("Content-Type", "application/json")
	forgotW := httptest.NewRecorder()

	authHandler.ForgotPassword(forgotW, forgotReq)

	time.Sleep(100 * time.Millisecond)

	if forgotW.Code != http.StatusOK {
		t.Errorf("Forgot password: expected 200, got %d. Body: %s", forgotW.Code, forgotW.Body.String())
	}

	if capturedTokenHash == "" {
		t.Fatal("Expected a reset token to be created")
	}

	// Step 2: Reset password with the token
	passwordResetTokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.PasswordResetToken{
			ID:        1,
			UserID:    1,
			TokenHash: capturedTokenHash,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}, nil)

	userRepo.EXPECT().
		UpdatePasswordByUserID(mock.Anything, 1, mock.Anything).
		Run(func(_ context.Context, _ int, hash string) {
			newPasswordHash = hash
		}).
		Return(nil)

	passwordResetTokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 1).
		Return(nil)

	resetBody, _ := json.Marshal(handlers.ResetPasswordRequest{
		Token:       "test-plain-token",
		NewPassword: "NewP@ssw0rd!12345",
	})
	resetReq := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(resetBody))
	resetReq.Header.Set("Content-Type", "application/json")
	resetW := httptest.NewRecorder()

	authHandler.ResetPassword(resetW, resetReq)

	if resetW.Code != http.StatusOK {
		t.Errorf("Reset password: expected 200, got %d. Body: %s", resetW.Code, resetW.Body.String())
	}

	// Verify reset response
	var resetResp map[string]any
	if err := json.NewDecoder(resetW.Body).Decode(&resetResp); err != nil {
		t.Fatalf("Failed to decode reset response: %v", err)
	}
	resetData, _ := resetResp["data"].(map[string]any)
	if resetData["message"] != "Password reset successfully" {
		t.Errorf("Expected reset success message, got: %v", resetData["message"])
	}

	// Step 3: Login with the new password
	failedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 1).
		Return(false, nil, nil)

	failedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, 1).
		Return(nil)

	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           1,
			Username:     "testuser",
			PasswordHash: newPasswordHash,
			Email:        "user@example.com",
			Role:         "Admin",
			Status:       "verified",
		}, nil)
	userRepo.EXPECT().
		UpdateLastLoginAt(mock.Anything, 1).
		Return(nil)

	loginBody, _ := json.Marshal(handlers.LoginRequest{
		Username: "testuser",
		Password: "NewP@ssw0rd!12345",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()

	authHandler.Login(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Errorf("Login after reset: expected 200, got %d. Body: %s", loginW.Code, loginW.Body.String())
	}

	var loginResp map[string]any
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Failed to decode login response: %v", err)
	}
	loginData, _ := loginResp["data"].(map[string]any)
	if _, ok := loginData["token"]; !ok {
		t.Error("Login response should contain token")
	}
}

// TestForgotPasswordNonexistentEmail tests that forgot-password returns same response for unknown emails
func TestForgotPasswordNonexistentEmail(t *testing.T) {
	defaultPasswordHash, _ := auth.HashPassword(constants.DefaultPassword)
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)

	userRepo := repomocks.NewMockUserRepo(t)
	verificationTokenRepo := repomocks.NewMockVerificationTokenRepo(t)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, verificationTokenRepo, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	authHandler := handlers.NewAuthHandler(
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

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "nonexistent@example.com").
		Return(nil, nil)

	body, _ := json.Marshal(handlers.ForgotPasswordRequest{Email: "nonexistent@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for nonexistent email, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	msg, _ := data["message"].(string)
	if !strings.Contains(msg, "If an account exists") {
		t.Errorf("Expected generic message, got: %s", msg)
	}
}
