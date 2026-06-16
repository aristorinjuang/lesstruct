package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

// setupStatusTestRouter sets up a test router for status transition tests
func setupStatusTestRouter(t *testing.T, userRepo repository.UserRepo, blockedEmailRepo user.BlockedEmailRepo, softDeleteRepo repository.SoftDeleteRepo) *chi.Mux {
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	emailService := emailmocks.NewMockEmailService(t)
	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := NewUserManagementHandler(userManagementService, nil, userRepo, softDeleteRepo, jwtManager, emailService, logger, nil)

	r := chi.NewRouter()
	r.Get("/api/admin/users", userManagementHandler.GetAllUsers)
	r.Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)
	r.Post("/api/admin/users/{id}/unsuspend", userManagementHandler.UnsuspendUser)
	r.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)
	r.Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)
	r.Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	return r
}

// TestSuspendAlreadySuspendedUser tests that suspending an already-suspended user fails
func TestSuspendAlreadySuspendedUser(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "suspendeduser",
			Email:    "suspended@example.com",
			Status:   "suspended",
			Role:     "Author",
		}, nil)
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("suspended", nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	reqBody := map[string]string{"reason": "Already suspended"}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/admin/users/123/suspend", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409 for suspending already-suspended user, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestSuspendSoftDeletedUser tests that suspending a soft-deleted user fails
func TestSuspendSoftDeletedUser(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "deleteduser",
			Email:    "deleted@example.com",
			Status:   "soft_deleted",
			Role:     "Author",
		}, nil)
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("soft_deleted", nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/admin/users/123/suspend", bytes.NewReader([]byte(`{"reason":"test"}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409 for suspending soft-deleted user, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestUnsuspendActiveUser tests that unsuspending an active (verified) user fails
func TestUnsuspendActiveUser(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "activeuser",
			Email:    "active@example.com",
			Status:   "verified",
			Role:     "Author",
		}, nil)
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/admin/users/123/unsuspend", bytes.NewReader([]byte(`{"reason":"test"}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409 for unsuspending active user, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestSoftDeleteAlreadySoftDeletedUser tests that soft-deleting an already-soft-deleted user fails
func TestSoftDeleteAlreadySoftDeletedUser(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetUserByID(mock.Anything, 123).
		Return(&repository.User{
			ID:       123,
			Username: "deleteduser",
			Email:    "deleted@example.com",
			Status:   "soft_deleted",
			Role:     "Author",
		}, nil)
	userRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("soft_deleted", nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/admin/users/123/soft-delete", bytes.NewReader([]byte(`{"confirmed":true,"reason":"test"}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409 for soft-deleting already-soft-deleted user, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestSoftDeleteWithoutConfirmation tests that soft-delete requires confirmation
func TestSoftDeleteWithoutConfirmation(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Test with confirmed: false
	req := httptest.NewRequest("POST", "/api/admin/users/123/soft-delete", bytes.NewReader([]byte(`{"confirmed":false}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for soft-delete without confirmation, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Test with missing confirmed field
	req2 := httptest.NewRequest("POST", "/api/admin/users/123/soft-delete", bytes.NewReader([]byte(`{"reason":"test"}`)))
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for soft-delete without confirmed field, got %d. Body: %s", w2.Code, w2.Body.String())
	}
}

// TestRestoreNonExistentContent tests that restoring non-existent content returns 404
func TestRestoreNonExistentContent(t *testing.T) {
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	softDeleteRepo.EXPECT().
		GetSoftDeletedContentByID(mock.Anything, 999).
		Return(nil, errors.New("content not found"))

	userRepo := repomocks.NewMockUserRepo(t)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/admin/content/999/restore", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent content, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestEmptyUserList tests that an empty user list returns an empty array
func TestEmptyUserList(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	userRepo.EXPECT().
		GetAllUsers(mock.Anything, "", mock.Anything, mock.Anything).
		Return([]*repository.User{}, nil)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if wrapper.Data.Data == nil {
		t.Error("Expected empty array, got nil")
	}

	if len(wrapper.Data.Data) != 0 {
		t.Errorf("Expected 0 users, got %d", len(wrapper.Data.Data))
	}

	if wrapper.Data.Meta.Count != 0 {
		t.Errorf("Expected count 0, got %d", wrapper.Data.Meta.Count)
	}
}

// TestUnauthenticatedCannotAccessAccountAdministration tests that requests without auth get 401
func TestUnauthenticatedCannotAccessAccountAdministration(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)
	emailService := emailmocks.NewMockEmailService(t)
	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	userManagementHandler := NewUserManagementHandler(userManagementService, nil, userRepo, softDeleteRepo, jwtManager, emailService, logger, nil)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	r := chi.NewRouter()
	r.With(adminMiddleware.AdminOnly).Get("/api/admin/users", userManagementHandler.GetAllUsers)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/users/{id}/unsuspend", userManagementHandler.UnsuspendUser)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)
	r.With(adminMiddleware.AdminOnly).Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)
	r.With(adminMiddleware.AdminOnly).Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	endpoints := []struct {
		name   string
		method string
		url    string
	}{
		{"Get all users", "GET", "/api/admin/users"},
		{"Suspend user", "POST", "/api/admin/users/123/suspend"},
		{"Unsuspend user", "POST", "/api/admin/users/123/unsuspend"},
		{"Soft delete user", "POST", "/api/admin/users/123/soft-delete"},
		{"Get deleted content", "GET", "/api/admin/users/123/deleted-content"},
		{"Restore content", "POST", "/api/admin/content/1/restore"},
	}

	for _, tt := range endpoints {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.method == "POST" {
				req = httptest.NewRequest(tt.method, tt.url, bytes.NewReader([]byte(`{"reason":"test","confirmed":true}`)))
			} else {
				req = httptest.NewRequest(tt.method, tt.url, nil)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401 for unauthenticated request to %s, got %d. Body: %s", tt.name, w.Code, w.Body.String())
			}
		})
	}
}

// TestInvalidUserID tests that invalid user ID formats return 400
func TestInvalidUserID(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	tests := []struct {
		name string
		url  string
	}{
		{"Non-numeric ID", "/api/admin/users/abc/suspend"},
		{"Zero ID", "/api/admin/users/0/suspend"},
		{"Negative ID", "/api/admin/users/-1/suspend"},
		{"Non-numeric restore ID", "/api/admin/content/abc/restore"},
		{"Zero restore ID", "/api/admin/content/0/restore"},
		{"Negative restore ID", "/api/admin/content/-5/restore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, bytes.NewReader([]byte(`{"reason":"test","confirmed":true}`)))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for %s, got %d. Body: %s", tt.name, w.Code, w.Body.String())
			}
		})
	}
}

// TestInvalidStatusParameter tests that invalid status query parameter returns 400
func TestInvalidStatusParameter(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/users?status=invalid_status", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid status parameter, got %d. Body: %s", w.Code, w.Body.String())
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if errResp.Error.Code != "INVALID_STATUS" {
		t.Errorf("Expected error code 'INVALID_STATUS', got '%s'", errResp.Error.Code)
	}
}

// TestRestoreAlreadyRestoredContent tests that restoring already-restored content returns 404
func TestRestoreAlreadyRestoredContent(t *testing.T) {
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	softDeleteRepo.EXPECT().
		GetSoftDeletedContentByID(mock.Anything, 1).
		Return(nil, nil)

	userRepo := repomocks.NewMockUserRepo(t)

	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)

	r := setupStatusTestRouter(t, userRepo, blockedEmailRepo, softDeleteRepo)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	adminToken, err := jwtManager.GenerateToken("1", "admin", "Admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/admin/content/1/restore", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for already-restored content, got %d. Body: %s", w.Code, w.Body.String())
	}
}
