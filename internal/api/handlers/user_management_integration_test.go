package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	usermocks "github.com/aristorinjuang/lesstruct/internal/domain/user/mocks"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

// setupTestRouter sets up a test router with user management routes
func setupTestRouter(
	t *testing.T,
	userRepo *repomocks.MockUserRepo,
	blockedEmailRepo *repomocks.MockBlockedEmailRepo,
	emailService *emailmocks.MockEmailService,
	softDeleteRepo *repomocks.MockSoftDeleteRepo,
) *chi.Mux {
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := handlers.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	r := chi.NewRouter()
	r.Get("/api/admin/pending-users", userManagementHandler.GetPendingUsers)
	r.Post("/api/admin/users/{id}/approve", userManagementHandler.ApproveUser)
	r.Post("/api/admin/users/{id}/reject", userManagementHandler.RejectUser)
	r.Post("/api/admin/users/{id}/mark-spam", userManagementHandler.MarkUserAsSpam)

	// Account administration routes
	r.Post("/api/admin/users", userManagementHandler.CreateUser)
	r.Get("/api/admin/users", userManagementHandler.GetAllUsers)
	r.Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)
	r.Post("/api/admin/users/{id}/unsuspend", userManagementHandler.UnsuspendUser)
	r.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)
	r.Put("/api/admin/users/{id}", userManagementHandler.UpdateUser)
	r.Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)
	r.Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	return r
}

// TestApproveUserFlow tests the complete approve user flow
func TestApproveUserFlow(t *testing.T) {
	// Setup
	var approvedUserID int

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// GetPendingUsers expectation
	userRepo.EXPECT().
		GetPendingUsers(mock.Anything, mock.Anything, mock.Anything).
		Return([]*repository.User{
			{
				ID:       123,
				Username: "pendinguser",
				Email:    "pending@example.com",
				Status:   "pending",
				Role:     "Contributor",
			},
		}, nil)

	// ApproveUser: handler calls GetUserByID, service calls UpdateUserStatusIfCurrentStatus
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "pendinguser",
			Email:    "pending@example.com",
			Status:   "pending",
			Role:     "Contributor",
		}, nil)
	userRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "pending", "verified").
		Run(func(_ context.Context, userID int, _ string, _ string) {
			approvedUserID = userID
		}).
		Return(nil)

	// SendUserApprovedEmail expectation
	emailService.EXPECT().
		SendUserApprovedEmail(mock.Anything, "pending@example.com", "pendinguser").
		Return(nil)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Test 1: Get pending users
	req := httptest.NewRequest(http.MethodGet, "/api/admin/pending-users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data handlers.GetPendingUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v. Body: %s", err, w.Body.String())
	}

	resp := wrapper.Data
	t.Logf("Response: %+v", resp)

	if len(resp.Data) != 1 {
		t.Fatalf("Expected 1 pending user, got %d", len(resp.Data))
	}

	if resp.Data[0].Username != "pendinguser" {
		t.Errorf("Expected username 'pendinguser', got '%s'", resp.Data[0].Username)
	}

	if resp.Meta.Count != 1 {
		t.Errorf("Expected count 1, got %d", resp.Meta.Count)
	}

	// Test 2: Approve user
	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/123/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var approveRespWrapper struct {
		Data handlers.ApproveUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&approveRespWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	approveResp := approveRespWrapper.Data

	if approvedUserID != 123 {
		t.Errorf("Expected user ID 123 to be approved, got %d", approvedUserID)
	}

	if approveResp.Message != "User approved successfully" {
		t.Errorf("Expected message 'User approved successfully', got '%s'", approveResp.Message)
	}
}

// TestRejectUserFlow tests the complete reject user flow
func TestRejectUserFlow(t *testing.T) {
	// Setup
	var deletedUserID int

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// RejectUser: handler calls GetUserByID, service calls GetUserByID + DeleteUser
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 456).
		Return(&repository.User{
			ID:       456,
			Username: "rejectuser",
			Email:    "reject@example.com",
			Status:   "pending",
			Role:     "Contributor",
		}, nil)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 456).
		Return(&repository.User{
			ID:       456,
			Username: "rejectuser",
			Email:    "reject@example.com",
			Status:   "pending",
			Role:     "Contributor",
		}, nil)
	userRepo.EXPECT().
		DeleteUser(mock.Anything, mock.Anything).
		Run(func(_ context.Context, userID int) {
			deletedUserID = userID
		}).
		Return(nil)

	// SendUserRejectedEmail expectation
	emailService.EXPECT().
		SendUserRejectedEmail(mock.Anything, "reject@example.com", "rejectuser").
		Return(nil)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Reject user
	rejectReqBody := map[string]bool{
		"confirmed": true,
	}
	bodyBytes, _ := json.Marshal(rejectReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/456/reject", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Data handlers.RejectUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	resp := respWrapper.Data

	if deletedUserID != 456 {
		t.Errorf("Expected user ID 456 to be deleted, got %d", deletedUserID)
	}

	if resp.Message != "User rejected successfully. Rejection email sent." {
		t.Errorf("Expected message 'User rejected successfully. Rejection email sent.', got '%s'", resp.Message)
	}
}

// TestMarkUserAsSpamFlow tests the complete mark as spam flow
func TestMarkUserAsSpamFlow(t *testing.T) {
	// Setup
	var deletedUserID int
	var blockedEmail string

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// MarkUserAsSpam: handler calls GetUserByID, service calls GetUserByID + IsEmailBlocked + BlockEmail + DeleteUser
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 789).
		Return(&repository.User{
			ID:       789,
			Username: "spamuser",
			Email:    "spam@example.com",
			Status:   "pending",
			Role:     "Contributor",
		}, nil)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 789).
		Return(&repository.User{
			ID:       789,
			Username: "spamuser",
			Email:    "spam@example.com",
			Status:   "pending",
			Role:     "Contributor",
		}, nil)
	userRepo.EXPECT().
		DeleteUser(mock.Anything, mock.Anything).
		Run(func(_ context.Context, userID int) {
			deletedUserID = userID
		}).
		Return(nil)

	// Blocked email repo: service's MarkUserAsSpam calls IsEmailBlocked once, then BlockEmail
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, nil)
	blockedEmailRepo.EXPECT().
		BlockEmail(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, email string, _ string) {
			blockedEmail = email
		}).
		Return(nil)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Mark user as spam
	markSpamReqBody := map[string]bool{
		"confirmed": true,
	}
	bodyBytes, _ := json.Marshal(markSpamReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/789/mark-spam", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Data handlers.MarkUserAsSpamResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	resp := respWrapper.Data

	if deletedUserID != 789 {
		t.Errorf("Expected user ID 789 to be deleted, got %d", deletedUserID)
	}

	if blockedEmail != "spam@example.com" {
		t.Errorf("Expected email 'spam@example.com' to be blocked, got '%s'", blockedEmail)
	}

	if resp.Message != "User marked as spam and email blocked" {
		t.Errorf("Expected message 'User marked as spam and email blocked', got '%s'", resp.Message)
	}
}

// TestBlockedEmailPreventsRegistration tests that blocked emails cannot register
func TestBlockedEmailPreventsRegistration(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword("admin")
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	loginService := authdomain.NewLoginService(nil, repomocks.NewMockFailedLoginAttemptRepo(t), nil)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	// Configure blocked email repo to block specific email
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	blockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "blocked@example.com").
		Return(true, nil)

	passwordResetUserRepo := repomocks.NewMockUserRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(passwordResetUserRepo, passwordResetTokenRepo, 1)

	authHandler := handlers.NewAuthHandler(
		authService,
		jwtManager,
		logger,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		nil,
		failedLoginRepo,
		notificationRepo,
		emailService,
		blockedEmailRepo,
	)

	// Try to register with blocked email
	reqBody := handlers.RegisterUserRequest{
		Username: "blockeduser",
		Email:    "blocked@example.com",
		Password: "SecurePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.RegisterUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for blocked email, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Message == "" {
		t.Error("Expected error message in response")
	}

	if resp.Error.Message != "This email address has been blocked" {
		t.Errorf("Expected 'This email address has been blocked' error, got '%s'", resp.Error.Message)
	}
}

// TestSuspendUserFlow tests the complete suspend user flow
func TestSuspendUserFlow(t *testing.T) {
	// Setup
	var suspendedUserID int

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// GetAllUsers expectation (Test 1)
	userRepo.EXPECT().
		GetAllUsers(mock.Anything, "", mock.Anything, mock.Anything).
		Return([]*repository.User{
			{
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "verified",
				Role:     "Author",
			},
		}, nil)

	// SuspendUser: handler calls GetUserByID, service calls GetUserStatus + UpdateUserStatusIfCurrentStatus,
	// handler calls GetUserByID again, then SendUserSuspendedEmail
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "testuser",
			Email:    "test@example.com",
			Status:   "verified",
			Role:     "Author",
		}, nil).Twice()
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)
	userRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "verified", "suspended").
		Run(func(_ context.Context, userID int, _ string, _ string) {
			suspendedUserID = userID
		}).
		Return(nil)

	emailService.EXPECT().
		SendUserSuspendedEmail(mock.Anything, "test@example.com", "testuser", "Violating community guidelines").
		Return(nil)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Test 1: Get all users (verify user is verified)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var usersWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&usersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	users := usersWrapper.Data.Data
	if len(users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(users))
	}

	if users[0].Status != "verified" {
		t.Errorf("Expected status 'verified', got '%s'", users[0].Status)
	}

	// Test 2: Suspend user
	suspendReqBody := map[string]string{
		"reason": "Violating community guidelines",
	}
	bodyBytes, _ := json.Marshal(suspendReqBody)

	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/123/suspend", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var suspendWrapper struct {
		Data handlers.SuspendUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&suspendWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	suspendResp := suspendWrapper.Data
	if suspendedUserID != 123 {
		t.Errorf("Expected user ID 123 to be suspended, got %d", suspendedUserID)
	}

	if suspendResp.Message != "User suspended successfully" {
		t.Errorf("Expected message 'User suspended successfully', got '%s'", suspendResp.Message)
	}
}

// TestUnsuspendUserFlow tests the complete unsuspend user flow
func TestUnsuspendUserFlow(t *testing.T) {
	// Setup
	var unsuspendedUserID int

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// UnsuspendUser: handler calls GetUserByID, service calls GetUserStatus + UpdateUserStatusIfCurrentStatus,
	// handler calls GetUserByID again, then SendUserUnsuspendedEmail
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 456).
		Return(&repository.User{
			ID:       456,
			Username: "suspendeduser",
			Email:    "suspended@example.com",
			Status:   "suspended",
			Role:     "Author",
		}, nil).Twice()
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 456).
		Return("suspended", nil)
	userRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "suspended", "verified").
		Run(func(_ context.Context, userID int, _ string, _ string) {
			unsuspendedUserID = userID
		}).
		Return(nil)

	emailService.EXPECT().
		SendUserUnsuspendedEmail(mock.Anything, "suspended@example.com", "suspendeduser").
		Return(nil)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Unsuspend user
	unsuspendReqBody := map[string]string{
		"reason": "Suspension lifted after review",
	}
	bodyBytes, _ := json.Marshal(unsuspendReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/456/unsuspend", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var unsuspendWrapper struct {
		Data handlers.UnsuspendUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&unsuspendWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	unsuspendResp := unsuspendWrapper.Data
	if unsuspendedUserID != 456 {
		t.Errorf("Expected user ID 456 to be unsuspended, got %d", unsuspendedUserID)
	}

	if unsuspendResp.Message != "User unsuspended successfully" {
		t.Errorf("Expected message 'User unsuspended successfully', got '%s'", unsuspendResp.Message)
	}
}

// TestSoftDeleteUserFlow tests the complete soft delete user flow
func TestSoftDeleteUserFlow(t *testing.T) {
	// Setup
	var deletedUserID int

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// SoftDeleteUser: handler calls GetUserByID, service calls GetUserStatus + UpdateUserStatusIfCurrentStatus,
	// handler calls GetUserByID again, then SendUserSoftDeletedEmail
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 789).
		Return(&repository.User{
			ID:       789,
			Username: "deleteuser",
			Email:    "delete@example.com",
			Status:   "verified",
			Role:     "Author",
		}, nil).Twice()
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 789).
		Return("verified", nil)
	userRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "verified", "soft_deleted").
		Run(func(_ context.Context, userID int, _ string, _ string) {
			deletedUserID = userID
		}).
		Return(nil)

	emailService.EXPECT().
		SendUserSoftDeletedEmail(mock.Anything, "delete@example.com", "deleteuser", "User requested account deletion").
		Return(nil)

	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := handlers.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	r := chi.NewRouter()
	r.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Soft delete user
	softDeleteReqBody := map[string]any{
		"confirmed": true,
		"reason":    "User requested account deletion",
	}
	bodyBytes, _ := json.Marshal(softDeleteReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/789/soft-delete", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var softDeleteWrapper struct {
		Data handlers.SoftDeleteUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&softDeleteWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	softDeleteResp := softDeleteWrapper.Data
	if deletedUserID != 789 {
		t.Errorf("Expected user ID 789 to be soft deleted, got %d", deletedUserID)
	}

	if softDeleteResp.Message != "User account and content soft deleted successfully" {
		t.Errorf("Expected message 'User account and content soft deleted successfully', got '%s'", softDeleteResp.Message)
	}

	// Note: DeletedContentCount is 0 because content management is not yet implemented
	// This is a placeholder for Story 2.x when content types are added
	if softDeleteResp.DeletedContentCount != 0 {
		t.Errorf("Expected deleted content count 0 (not implemented), got %d", softDeleteResp.DeletedContentCount)
	}
}

// TestSuspendedUserCannotLogin tests that suspended users cannot log in
func TestSuspendedUserCannotLogin(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword("admin")
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	// Configure user repository mock to return suspended user
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, "suspendeduser").
		Return(&repository.User{
			ID:           123,
			Username:     "suspendeduser",
			Email:        "suspended@example.com",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz123456",
			Status:       "suspended",
			Role:         "Author",
		}, nil)

	// LoginService calls IsLocked before checking status
	failedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)

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

	// Attempt login with suspended user
	reqBody := map[string]string{
		"username": "suspendeduser",
		"password": "SomePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for suspended user, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != "ACCOUNT_SUSPENDED" {
		t.Errorf("Expected error code 'ACCOUNT_SUSPENDED', got '%s'", resp.Error.Code)
	}

	if resp.Error.Message != "Your account has been suspended. Please contact an administrator." {
		t.Errorf("Expected message 'Your account has been suspended. Please contact an administrator.', got '%s'", resp.Error.Message)
	}
}

// TestSoftDeletedUserCannotLogin tests that soft deleted users cannot log in
func TestSoftDeletedUserCannotLogin(t *testing.T) {
	// Setup
	defaultPasswordHash, _ := auth.HashPassword("admin")
	authService := authdomain.NewAuthService(defaultPasswordHash)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	// Configure user repository mock to return soft deleted user
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByUsername(mock.Anything, "deleteduser").
		Return(&repository.User{
			ID:           456,
			Username:     "deleteduser",
			Email:        "deleted@example.com",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz123456",
			Status:       "soft_deleted",
			Role:         "Author",
		}, nil)

	// LoginService calls IsLocked before checking status
	failedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 456).
		Return(false, nil, nil)

	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, nil)

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

	// Attempt login with soft deleted user
	reqBody := map[string]string{
		"username": "deleteduser",
		"password": "SomePassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for soft deleted user, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != "ACCOUNT_DELETED" {
		t.Errorf("Expected error code 'ACCOUNT_DELETED', got '%s'", resp.Error.Code)
	}

	if resp.Error.Message != "Your account has been deleted." {
		t.Errorf("Expected message 'Your account has been deleted.', got '%s'", resp.Error.Message)
	}
}

// TestStatusFilteringInUserList tests status filtering in user list
func TestStatusFilteringInUserList(t *testing.T) {
	// Setup
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// GetAllUsers expectation with RunAndReturn to handle dynamic filtering
	userRepo.EXPECT().
		GetAllUsers(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, status string, limit int, offset int) ([]*repository.User, error) {
			var users []*repository.User

			// Filter by status
			if status == "" || status == "suspended" {
				users = append(users, &repository.User{
					ID:       111,
					Username: "suspendeduser1",
					Email:    "suspended1@example.com",
					Status:   "suspended",
					Role:     "Author",
				})
				users = append(users, &repository.User{
					ID:       222,
					Username: "suspendeduser2",
					Email:    "suspended2@example.com",
					Status:   "suspended",
					Role:     "Contributor",
				})
			}

			if status == "" || status == "verified" {
				users = append(users, &repository.User{
					ID:       333,
					Username: "activeuser",
					Email:    "active@example.com",
					Status:   "verified",
					Role:     "Admin",
				})
			}

			if status == "" || status == "soft_deleted" {
				users = append(users, &repository.User{
					ID:       444,
					Username: "deleteduser",
					Email:    "deleted@example.com",
					Status:   "soft_deleted",
					Role:     "Author",
				})
			}

			// Apply limit
			if limit > 0 && len(users) > limit {
				users = users[:limit]
			}

			// Apply offset
			if offset > 0 && offset < len(users) {
				users = users[offset:]
			} else if offset > 0 && offset >= len(users) {
				users = []*repository.User{}
			}

			return users, nil
		})

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Test 1: Get all users (no filter)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var allUsersWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&allUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	allUsers := allUsersWrapper.Data.Data
	if len(allUsers) != 4 {
		t.Errorf("Expected 4 users, got %d", len(allUsers))
	}

	// Test 2: Filter by suspended status
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?status=suspended", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var suspendedUsersWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&suspendedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	suspendedUsers := suspendedUsersWrapper.Data.Data
	if len(suspendedUsers) != 2 {
		t.Errorf("Expected 2 suspended users, got %d", len(suspendedUsers))
	}

	for _, u := range suspendedUsers {
		if u.Status != "suspended" {
			t.Errorf("Expected all users to have status 'suspended', got '%s'", u.Status)
		}
	}

	// Test 3: Filter by verified status
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?status=verified", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var verifiedUsersWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&verifiedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	verifiedUsers := verifiedUsersWrapper.Data.Data
	if len(verifiedUsers) != 1 {
		t.Errorf("Expected 1 verified user, got %d", len(verifiedUsers))
	}

	if verifiedUsers[0].Status != "verified" {
		t.Errorf("Expected user status 'verified', got '%s'", verifiedUsers[0].Status)
	}

	// Test 4: Filter by soft_deleted status
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?status=soft_deleted", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var deletedUsersWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&deletedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	deletedUsers := deletedUsersWrapper.Data.Data
	if len(deletedUsers) != 1 {
		t.Errorf("Expected 1 soft_deleted user, got %d", len(deletedUsers))
	}

	if deletedUsers[0].Status != "soft_deleted" {
		t.Errorf("Expected user status 'soft_deleted', got '%s'", deletedUsers[0].Status)
	}

	// Test 5: Pagination with limit
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?limit=1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var paginatedUsersWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&paginatedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	paginatedUsers := paginatedUsersWrapper.Data.Data
	if len(paginatedUsers) != 1 {
		t.Errorf("Expected 1 user with limit=1, got %d", len(paginatedUsers))
	}

	if paginatedUsersWrapper.Data.Meta.Limit != 1 {
		t.Errorf("Expected meta.limit=1, got %d", paginatedUsersWrapper.Data.Meta.Limit)
	}

	// Test offset parameter skips results correctly
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?offset=1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var offsetWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&offsetWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	offsetUsers := offsetWrapper.Data.Data
	if len(offsetUsers) != 3 {
		t.Errorf("Expected 3 users with offset=1 (skipping first of 4), got %d", len(offsetUsers))
	}

	if offsetWrapper.Data.Meta.Offset != 1 {
		t.Errorf("Expected meta.offset=1, got %d", offsetWrapper.Data.Meta.Offset)
	}

	// Test max limit is capped at 1000
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?limit=5000", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var maxLimitWrapper struct {
		Data handlers.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&maxLimitWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if maxLimitWrapper.Data.Meta.Limit != 1000 {
		t.Errorf("Expected meta.limit capped to 1000, got %d", maxLimitWrapper.Data.Meta.Limit)
	}
}

// TestGetSoftDeletedContent tests retrieving soft deleted content for a user
func TestGetSoftDeletedContent(t *testing.T) {
	// Setup
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// GetSoftDeletedContent: handler calls GetUserByID, then softDeleteRepo.GetSoftDeletedContentByUser
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "testuser",
			Email:    "test@example.com",
			Status:   "soft_deleted",
			Role:     "Author",
		}, nil)
	// Handler calls GetUserByID for each content item's DeletedBy to get admin username
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:       1,
			Username: "admin",
			Email:    "admin@example.com",
			Status:   "verified",
			Role:     "Admin",
		}, nil).Times(2)

	softDeleteRepo.EXPECT().
		GetSoftDeletedContentByUser(mock.Anything, 123).
		Return([]*repository.SoftDeletedContent{
			{
				ID:          1,
				ContentType: "post",
				ContentID:   101,
				UserID:      123,
				DeletedAt:   "2026-03-28T14:30:00Z",
				DeletedBy:   1,
				Reason:      sql.NullString{String: "User requested account deletion", Valid: true},
			},
			{
				ID:          2,
				ContentType: "comment",
				ContentID:   202,
				UserID:      123,
				DeletedAt:   "2026-03-28T14:30:00Z",
				DeletedBy:   1,
				Reason:      sql.NullString{String: "User requested account deletion", Valid: true},
			},
		}, nil)

	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := handlers.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	r := chi.NewRouter()
	r.Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Get soft deleted content
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users/123/deleted-content", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data handlers.GetSoftDeletedContentResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resp := wrapper.Data
	if len(resp.Data) != 2 {
		t.Errorf("Expected 2 deleted content items, got %d", len(resp.Data))
	}

	if resp.Data[0].ContentType != "post" {
		t.Errorf("Expected content type 'post', got '%s'", resp.Data[0].ContentType)
	}

	if resp.Data[1].ContentType != "comment" {
		t.Errorf("Expected content type 'comment', got '%s'", resp.Data[1].ContentType)
	}

	if resp.Data[0].DeletedBy != "admin" {
		t.Errorf("Expected deletedBy 'admin', got '%s'", resp.Data[0].DeletedBy)
	}

	if resp.Meta.Count != 2 {
		t.Errorf("Expected count 2, got %d", resp.Meta.Count)
	}
}

// TestRestoreSoftDeletedContent tests restoring soft deleted content
func TestRestoreSoftDeletedContent(t *testing.T) {
	// Setup
	var restoredContentType string
	var restoredContentID int

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// RestoreContent: handler calls GetSoftDeletedContentByID, then RestoreContent
	softDeleteRepo.EXPECT().
		GetSoftDeletedContentByID(mock.Anything, 1).
		Return(&repository.SoftDeletedContent{
			ID:          1,
			ContentType: "post",
			ContentID:   101,
			UserID:      123,
			DeletedAt:   "2026-03-28T14:30:00Z",
			DeletedBy:   1,
			Reason:      sql.NullString{String: "User requested account deletion", Valid: true},
		}, nil)
	softDeleteRepo.EXPECT().
		RestoreContent(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, contentType string, contentID int) {
			restoredContentType = contentType
			restoredContentID = contentID
		}).
		Return(nil)

	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := handlers.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	r := chi.NewRouter()
	r.Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Restore content
	req := httptest.NewRequest(http.MethodPost, "/api/admin/content/1/restore", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data handlers.RestoreContentResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resp := wrapper.Data
	if resp.Message != "Content restored successfully" {
		t.Errorf("Expected message 'Content restored successfully', got '%s'", resp.Message)
	}

	if resp.Content == nil {
		t.Fatal("Expected content field in response, got nil")
	} else {
		if resp.Content.ID != 101 {
			t.Errorf("Expected content.ID=101, got %d", resp.Content.ID)
		}

		if resp.Content.Type != "post" {
			t.Errorf("Expected content.Type='post', got '%s'", resp.Content.Type)
		}
	}

	if restoredContentType != "post" {
		t.Errorf("Expected content type 'post' to be restored, got '%s'", restoredContentType)
	}

	if restoredContentID != 101 {
		t.Errorf("Expected content ID 101 to be restored, got %d", restoredContentID)
	}
}

// TestNonAdminCannotAccessAccountAdministration tests that non-admin users cannot access account administration endpoints
func TestNonAdminCannotAccessAccountAdministration(t *testing.T) {
	// Setup - mocks that won't have methods called because middleware rejects first
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	// Setup with admin middleware
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := handlers.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	r := chi.NewRouter()
	// Apply admin middleware to all routes
	r.With(adminMiddleware.AdminOnly).Get("/api/admin/users", userManagementHandler.GetAllUsers)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/users/{id}/unsuspend", userManagementHandler.UnsuspendUser)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)
	r.With(adminMiddleware.AdminOnly).Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	// Create non-admin token
	authorToken, err := jwtManager.GenerateToken("123", "testuser", "Author")
	if err != nil {
		t.Fatalf("Failed to generate author token: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		url            string
		body           []byte
		expectedStatus int
	}{
		{
			name:           "Get all users",
			method:         http.MethodGet,
			url:            "/api/admin/users",
			body:           nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Suspend user",
			method:         http.MethodPost,
			url:            "/api/admin/users/123/suspend",
			body:           []byte(`{"reason":"test"}`),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Unsuspend user",
			method:         http.MethodPost,
			url:            "/api/admin/users/123/unsuspend",
			body:           []byte(`{"reason":"test"}`),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Soft delete user",
			method:         http.MethodPost,
			url:            "/api/admin/users/123/soft-delete",
			body:           []byte(`{"confirmed":true,"reason":"test"}`),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Get soft deleted content",
			method:         http.MethodGet,
			url:            "/api/admin/users/123/deleted-content",
			body:           nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Restore content",
			method:         http.MethodPost,
			url:            "/api/admin/content/1/restore",
			body:           nil,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer "+authorToken)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d. Body: %s", tt.name, tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestEmailNotificationsSentOnAccountActions tests that email notifications are sent on suspend/unsuspend/soft delete
func TestEmailNotificationsSentOnAccountActions(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	// Create admin token
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Test 1: Suspend user sends email
	t.Run("Suspend user sends email", func(t *testing.T) {
		var suspendEmailTo string
		var suspendUsername string
		var suspendReason string

		userRepo := repomocks.NewMockUserRepo(t)
		blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
		emailService := emailmocks.NewMockEmailService(t)
		softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

		// SuspendUser: handler calls GetUserByID, service calls GetUserStatus + UpdateUserStatusIfCurrentStatus,
		// handler calls GetUserByID again, then SendUserSuspendedEmail
		userRepo.EXPECT().
			GetUserByID(mock.Anything, 123).
			Return(&repository.User{
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "verified",
				Role:     "Author",
			}, nil).Twice()
		userRepo.EXPECT().
			GetUserStatus(mock.Anything, 123).
			Return("verified", nil)
		userRepo.EXPECT().
			UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "verified", "suspended").
			Return(nil)

		emailService.EXPECT().
			SendUserSuspendedEmail(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(_ context.Context, to string, username string, reason string) {
				suspendEmailTo = to
				suspendUsername = username
				suspendReason = reason
			}).
			Return(nil)

		userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
		userManagementHandler := handlers.NewUserManagementHandler(
			userManagementService,
			nil,
			userRepo,
			softDeleteRepo,
			jwtManager,
			emailService,
			logger,
   nil,
		)

		r := chi.NewRouter()
		r.Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)

		reqBody := map[string]string{"reason": "Test suspension"}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/users/123/suspend", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if suspendEmailTo != "test@example.com" {
			t.Errorf("Expected email to be sent to 'test@example.com', got '%s'", suspendEmailTo)
		}

		if suspendUsername != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", suspendUsername)
		}

		if suspendReason != "Test suspension" {
			t.Errorf("Expected reason 'Test suspension', got '%s'", suspendReason)
		}
	})

	// Test 2: Unsuspend user sends email
	t.Run("Unsuspend user sends email", func(t *testing.T) {
		var unsuspendEmailTo string
		var unsuspendUsername string

		unsuspendUserRepo := repomocks.NewMockUserRepo(t)
		unsuspendBlockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
		unsuspendEmailService := emailmocks.NewMockEmailService(t)
		unsuspendSoftDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

		// UnsuspendUser: handler calls GetUserByID, service calls GetUserStatus + UpdateUserStatusIfCurrentStatus,
		// handler calls GetUserByID again, then SendUserUnsuspendedEmail
		unsuspendUserRepo.EXPECT().
			GetUserByID(mock.Anything, 123).
			Return(&repository.User{
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "suspended",
				Role:     "Author",
			}, nil).Twice()
		unsuspendUserRepo.EXPECT().
			GetUserStatus(mock.Anything, 123).
			Return("suspended", nil)
		unsuspendUserRepo.EXPECT().
			UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "suspended", "verified").
			Return(nil)

		unsuspendEmailService.EXPECT().
			SendUserUnsuspendedEmail(mock.Anything, mock.Anything, mock.Anything).
			Run(func(_ context.Context, to string, username string) {
				unsuspendEmailTo = to
				unsuspendUsername = username
			}).
			Return(nil)

		unsuspendService := user.NewUserManagementService(unsuspendUserRepo, unsuspendBlockedEmailRepo)
		unsuspendHandler := handlers.NewUserManagementHandler(
			unsuspendService,
			nil,
			unsuspendUserRepo,
			unsuspendSoftDeleteRepo,
			jwtManager,
			unsuspendEmailService,
			logger,
   nil,
		)

		unsuspendRouter := chi.NewRouter()
		unsuspendRouter.Post("/api/admin/users/{id}/unsuspend", unsuspendHandler.UnsuspendUser)

		reqBody := map[string]string{"reason": "Test unsuspension"}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/users/123/unsuspend", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		unsuspendRouter.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if unsuspendEmailTo != "test@example.com" {
			t.Errorf("Expected email to be sent to 'test@example.com', got '%s'", unsuspendEmailTo)
		}

		if unsuspendUsername != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", unsuspendUsername)
		}
	})

	// Test 3: Soft delete user sends email
	t.Run("Soft delete user sends email", func(t *testing.T) {
		var softDeleteEmailTo string
		var softDeleteUsername string
		var softDeleteReason string

		userRepo := repomocks.NewMockUserRepo(t)
		blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
		emailService := emailmocks.NewMockEmailService(t)
		softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

		// SoftDeleteUser: handler calls GetUserByID, service calls GetUserStatus + UpdateUserStatusIfCurrentStatus,
		// handler calls GetUserByID again, then SendUserSoftDeletedEmail
		userRepo.EXPECT().
			GetUserByID(mock.Anything, 123).
			Return(&repository.User{
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "verified",
				Role:     "Author",
			}, nil).Twice()
		userRepo.EXPECT().
			GetUserStatus(mock.Anything, 123).
			Return("verified", nil)
		userRepo.EXPECT().
			UpdateUserStatusIfCurrentStatus(mock.Anything, mock.Anything, "verified", "soft_deleted").
			Return(nil)

		emailService.EXPECT().
			SendUserSoftDeletedEmail(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(_ context.Context, to string, username string, reason string) {
				softDeleteEmailTo = to
				softDeleteUsername = username
				softDeleteReason = reason
			}).
			Return(nil)

		userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
		userManagementHandler := handlers.NewUserManagementHandler(
			userManagementService,
			nil,
			userRepo,
			softDeleteRepo,
			jwtManager,
			emailService,
			logger,
   nil,
		)

		r2 := chi.NewRouter()
		r2.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)

		reqBody := map[string]any{
			"confirmed": true,
			"reason":    "Test soft delete",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/users/123/soft-delete", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r2.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if softDeleteEmailTo != "test@example.com" {
			t.Errorf("Expected email to be sent to 'test@example.com', got '%s'", softDeleteEmailTo)
		}

		if softDeleteUsername != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", softDeleteUsername)
		}

		if softDeleteReason != "Test soft delete" {
			t.Errorf("Expected reason 'Test soft delete', got '%s'", softDeleteReason)
		}
	})
}

func TestCreateUser_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	adminCreateRepoMock := usermocks.NewMockAdminCreateUserRepo(t)
	adminCreateBlockedMock := usermocks.NewMockBlockedEmailRepo(t)

	logger := util.NewLogger(os.Stdout)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	adminCreateService := user.NewAdminCreateUserService(adminCreateRepoMock, adminCreateBlockedMock)

	userManagementHandler := handlers.NewUserManagementHandler(
		userManagementService,
		adminCreateService,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	adminCreateBlockedMock.EXPECT().IsEmailBlocked(mock.Anything, "newuser@example.com").Return(false, nil)
	adminCreateRepoMock.EXPECT().CheckUsernameExists(mock.Anything, "newuser").Return(false, nil)
	adminCreateRepoMock.EXPECT().CheckEmailExists(mock.Anything, "newuser@example.com").Return(false, nil)
	adminCreateRepoMock.EXPECT().CreateUser(mock.Anything, mock.AnythingOfType("*repository.User")).
		RunAndReturn(func(ctx context.Context, u *repository.User) error {
			u.ID = 99
			u.CreatedAt = "2026-04-26T12:00:00Z"
			return nil
		})
	emailService.EXPECT().SendAccountCreatedEmail(
		mock.Anything,
		"newuser@example.com",
		"newuser",
		mock.Anything,
	).Return(nil).Maybe()

	r := chi.NewRouter()
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	r.With(authMiddleware.RequireAuth).Post("/api/admin/users", userManagementHandler.CreateUser)

	adminToken, _ := jwtManager.GenerateToken("1", "admin", "Admin")

	body := `{"username":"newuser","email":"newuser@example.com","role":"Contributor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)

	if data["message"] != "User created successfully" {
		t.Errorf("Expected 'User created successfully', got '%v'", data["message"])
	}

	userData := data["user"].(map[string]any)
	if userData["username"] != "newuser" {
		t.Errorf("Expected username 'newuser', got '%v'", userData["username"])
	}
	if userData["status"] != "verified" {
		t.Errorf("Expected status 'verified', got '%v'", userData["status"])
	}
	if data["password"] == nil || data["password"].(string) == "" {
		t.Error("Expected non-empty password in response")
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	jwtManager := auth.NewJWTManager("test-secret-key")
	adminToken, _ := jwtManager.GenerateToken("1", "admin", "Admin")

	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"missing all", `{"username":"","email":"","role":""}`, "MISSING_FIELDS"},
		{"missing username", `{"username":"","email":"test@example.com","role":"Contributor"}`, "MISSING_FIELDS"},
		{"missing email", `{"username":"testuser","email":"","role":"Contributor"}`, "MISSING_FIELDS"},
		{"missing role", `{"username":"testuser","email":"test@example.com","role":""}`, "MISSING_FIELDS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}

			var resp map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			errInfo := resp["error"].(map[string]any)
			if errInfo["code"] != tt.wantCode {
				t.Errorf("Expected error code '%s', got '%v'", tt.wantCode, errInfo["code"])
			}
		})
	}
}

func TestCreateUser_InvalidRequest(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	r := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	jwtManager := auth.NewJWTManager("test-secret-key")
	adminToken, _ := jwtManager.GenerateToken("1", "admin", "Admin")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_Success tests successful user profile update
func TestUpdateUser_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Old Name", Role: "Commentator",
		}, nil).Once()

	userRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "new@example.com").
		Return(false, nil)

	userRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "New Name", "new@example.com", "Contributor", mock.Anything).
		Return(nil)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "new@example.com",
			Name: "New Name", Role: "Contributor", Status: "verified", CreatedAt: "2025-01-01",
		}, nil).Once()

	body, _ := json.Marshal(map[string]string{
		"name":  "New Name",
		"email": "new@example.com",
		"role":  "Contributor",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateUser_InvalidUserID tests invalid user ID
func TestUpdateUser_InvalidUserID(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/abc", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_NegativeUserID tests negative user ID
func TestUpdateUser_NegativeUserID(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_UserNotFound tests updating non-existent user
func TestUpdateUser_UserNotFound(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 999).
		Return(nil, errors.New("user not found"))

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/999", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestUpdateUser_InvalidEmail tests invalid email format
func TestUpdateUser_InvalidEmail(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	body, _ := json.Marshal(map[string]string{"email": "not-an-email"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_InvalidRole tests invalid role
func TestUpdateUser_InvalidRole(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	body, _ := json.Marshal(map[string]string{"role": "SuperAdmin"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_EmailExists tests email already in use
func TestUpdateUser_EmailExists(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	userRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "taken@example.com").
		Return(true, nil)

	body, _ := json.Marshal(map[string]string{"email": "taken@example.com"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

// TestUpdateUser_InvalidJSON tests invalid JSON body
func TestUpdateUser_InvalidJSON(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_PartialUpdate tests partial update (only name)
func TestUpdateUser_PartialUpdate(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Old Name", Role: "Admin",
		}, nil).Once()

	userRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "Only Name", "test@example.com", "Admin", mock.Anything).
		Return(nil)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Only Name", Role: "Admin", Status: "verified", CreatedAt: "2025-01-01",
		}, nil).Once()

	body, _ := json.Marshal(map[string]string{"name": "Only Name"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}
