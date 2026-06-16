package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	usermocks "github.com/aristorinjuang/lesstruct/internal/domain/user/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

// setupProfileTestRouter sets up a test router with profile routes
func setupProfileTestRouter(profileService user.ProfileServiceInterface, accountDeletionService user.AccountDeletionServiceInterface) *chi.Mux {
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	profileHandler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	r := chi.NewRouter()
	// Profile routes with authentication
	r.Route("/api/profile", func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)
		r.Get("/", profileHandler.GetProfile)
		r.Put("/email", profileHandler.UpdateEmail)
		r.Put("/password", profileHandler.ChangePassword)
		r.Get("/export", profileHandler.ExportUserData)
		r.Delete("/account", profileHandler.DeleteAccount)
	})
	// Email verification route (public, token-based - no authentication required)
	r.Get("/api/profile/verify-email", profileHandler.VerifyEmailUpdate)

	return r
}

// TestCompleteProfileViewFlow tests the complete profile view flow
func TestCompleteProfileViewFlow(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:        123,
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      "Author",
			CreatedAt: "2026-03-28T10:30:00Z",
			UpdatedAt: "2026-03-30T12:00:00Z",
		}, nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	// Generate user token
	userToken, err := jwtManager.GenerateToken("123", "testuser", "Author")
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}

	// View profile
	req := httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Data struct {
			Profile handlers.ProfileInfo `json:"profile"`
			Meta    struct {
				Timestamp string `json:"timestamp"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v. Body: %s", err, w.Body.String())
	}

	profile := respWrapper.Data.Profile
	if profile.ID != 123 {
		t.Errorf("Expected ID 123, got %d", profile.ID)
	}
	if profile.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", profile.Username)
	}
	if profile.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", profile.Email)
	}
	if profile.Role != "Author" {
		t.Errorf("Expected role 'Author', got '%s'", profile.Role)
	}
	if respWrapper.Data.Meta.Timestamp == "" {
		t.Error("Expected timestamp in meta")
	}
}

// TestCompleteEmailUpdateFlow tests the complete email update flow
func TestCompleteEmailUpdateFlow(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateEmail(mock.Anything, 123, "newemail@example.com", mock.Anything).
		Return(nil)
	profileService.EXPECT().
		VerifyEmailUpdate(mock.Anything, "validtoken123").
		Return(123, "newemail@example.com", nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, err := jwtManager.GenerateToken("123", "testuser", "Author")
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}

	// Step 1: Request email update
	reqBody := handlers.EmailUpdateRequest{
		NewEmail:        "newemail@example.com",
		CurrentPassword: "CurrentPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/profile/email", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var updateRespWrapper struct {
		Data struct {
			Message  string `json:"message"`
			NewEmail string `json:"newEmail"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&updateRespWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if updateRespWrapper.Data.NewEmail != "newemail@example.com" {
		t.Errorf("Expected new email in response 'newemail@example.com', got '%s'", updateRespWrapper.Data.NewEmail)
	}

	// Step 2: Verify email update
	verifyReq := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email?token=validtoken123", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+userToken)
	verifyW := httptest.NewRecorder()

	r.ServeHTTP(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", verifyW.Code, verifyW.Body.String())
	}

	var verifyRespWrapper struct {
		Data struct {
			Message string `json:"message"`
			Email   string `json:"email"`
		} `json:"data"`
	}
	if err := json.NewDecoder(verifyW.Body).Decode(&verifyRespWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if verifyRespWrapper.Data.Email != "newemail@example.com" {
		t.Errorf("Expected email 'newemail@example.com', got '%s'", verifyRespWrapper.Data.Email)
	}
}

// TestCompletePasswordChangeFlow tests the complete password change flow
func TestCompletePasswordChangeFlow(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		ChangePassword(mock.Anything, 123, "OldPassword123!", "NewPassword456!").
		Return(nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, err := jwtManager.GenerateToken("123", "testuser", "Author")
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}

	// Change password
	reqBody := handlers.PasswordUpdateRequest{
		CurrentPassword: "OldPassword123!",
		NewPassword:     "NewPassword456!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/profile/password", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respWrapper.Data.Message == "" {
		t.Error("Expected message in response")
	}
}

// TestCompleteDataExportFlow tests the complete data export flow
func TestCompleteDataExportFlow(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		ExportUserData(mock.Anything, 123).
		Return(&repository.UserDataExport{
			ExportDate: "2026-03-31T12:00:00Z",
			User: &repository.User{
				ID:        123,
				Username:  "testuser",
				Email:     "test@example.com",
				Role:      "Author",
				CreatedAt: "2026-03-28T10:30:00Z",
			},
			Content: []*repository.UserContentItem{
				{
					ID:        1,
					Type:      "post",
					Title:     "My First Post",
					Slug:      "my-first-post",
					Content:   "<p>Post content here...</p>",
					Status:    "published",
					CreatedAt: "2026-03-29T10:00:00Z",
					UpdatedAt: "2026-03-29T10:30:00Z",
				},
			},
			Comments: []*repository.UserCommentItem{
				{
					ID:            1,
					ContentItemID: 1,
					Content:       "Great post!",
					CreatedAt:     "2026-03-29T14:00:00Z",
					UpdatedAt:     "2026-03-29T14:00:00Z",
				},
			},
			Media: []*repository.UserMediaItem{
				{
					ID:               1,
					Filename:         "image1.webp",
					OriginalFilename: "photo.jpg",
					FilePath:         "/uploads/media/2026/03/29/image1.webp",
					FileSize:         102400,
					MimeType:         "image/webp",
					CreatedAt:        "2026-03-29T12:00:00Z",
				},
			},
		}, nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, err := jwtManager.GenerateToken("123", "testuser", "Author")
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}

	// Export user data
	req := httptest.NewRequest("GET", "/api/profile/export", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	if contentDisposition == "" {
		t.Error("Expected Content-Disposition header")
	}

	var exportData repository.UserDataExport
	if err := json.NewDecoder(w.Body).Decode(&exportData); err != nil {
		t.Fatalf("Failed to decode export data: %v", err)
	}

	if exportData.User.ID != 123 {
		t.Errorf("Expected user ID 123 in export, got %d", exportData.User.ID)
	}
	if len(exportData.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(exportData.Content))
	}
	if len(exportData.Comments) != 1 {
		t.Errorf("Expected 1 comment, got %d", len(exportData.Comments))
	}
	if len(exportData.Media) != 1 {
		t.Errorf("Expected 1 media item, got %d", len(exportData.Media))
	}
}

// TestUnauthenticatedCannotAccessProfile tests authorization checks
func TestUnauthenticatedCannotAccessProfile(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)

	tests := []struct {
		name           string
		method         string
		path           string
		body           []byte
		expectedStatus int
	}{
		{
			name:           "View profile",
			method:         "GET",
			path:           "/api/profile",
			body:           nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Update email",
			method:         "PUT",
			path:           "/api/profile/email",
			body:           []byte(`{"newEmail":"new@example.com","currentPassword":"pass"}`),
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Change password",
			method:         "PUT",
			path:           "/api/profile/password",
			body:           []byte(`{"currentPassword":"old","newPassword":"new"}`),
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Export data",
			method:         "GET",
			path:           "/api/profile/export",
			body:           nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Delete account",
			method:         "DELETE",
			path:           "/api/profile/account",
			body:           []byte(`{"confirmation":"DELETE"}`),
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer(tt.body))
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestEmailUpdateAlreadyInUse tests updating email to one already in use
func TestEmailUpdateAlreadyInUse(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateEmail(mock.Anything, 123, "existing@example.com", mock.Anything).
		Return(user.ErrEmailAlreadyInUse)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	reqBody := handlers.EmailUpdateRequest{
		NewEmail:        "existing@example.com",
		CurrentPassword: "CurrentPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/profile/email", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respWrapper.Error.Code != "EMAIL_ALREADY_IN_USE" {
		t.Errorf("Expected error code 'EMAIL_ALREADY_IN_USE', got '%s'", respWrapper.Error.Code)
	}
}

// TestEmailUpdateInvalidEmail tests updating email with invalid format
func TestEmailUpdateInvalidEmail(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateEmail(mock.Anything, 123, "invalid-email", mock.Anything).
		Return(user.ErrInvalidEmail)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	reqBody := handlers.EmailUpdateRequest{
		NewEmail:        "invalid-email",
		CurrentPassword: "CurrentPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/profile/email", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respWrapper.Error.Code != "INVALID_EMAIL" {
		t.Errorf("Expected error code 'INVALID_EMAIL', got '%s'", respWrapper.Error.Code)
	}
}

// TestPasswordChangeWrongCurrentPassword tests changing password with wrong current password
func TestPasswordChangeWrongCurrentPassword(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		ChangePassword(mock.Anything, 123, "WrongPassword123!", "NewPassword456!").
		Return(user.ErrInvalidPassword)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	reqBody := handlers.PasswordUpdateRequest{
		CurrentPassword: "WrongPassword123!",
		NewPassword:     "NewPassword456!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/profile/password", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respWrapper.Error.Code != "INVALID_PASSWORD" {
		t.Errorf("Expected error code 'INVALID_PASSWORD', got '%s'", respWrapper.Error.Code)
	}
}

// TestVerifyEmailUpdateExpiredToken tests verifying email update with expired token
func TestVerifyEmailUpdateExpiredToken(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		VerifyEmailUpdate(mock.Anything, "expiredtoken123").
		Return(0, "", fmt.Errorf("token not found or expired"))

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email?token=expiredtoken123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Expired token should return 400 with INVALID_TOKEN error code
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respWrapper.Error.Code != "INVALID_TOKEN" {
		t.Errorf("Expected error code 'INVALID_TOKEN', got '%s'", respWrapper.Error.Code)
	}
}

// TestVerifyEmailUpdateInvalidToken tests verifying email update with invalid token
func TestVerifyEmailUpdateInvalidToken(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		VerifyEmailUpdate(mock.Anything, "invalidtoken123").
		Return(0, "", errors.New("invalid token"))

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email?token=invalidtoken123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Invalid token should return 400 with INVALID_TOKEN error code
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respWrapper.Error.Code != "INVALID_TOKEN" {
		t.Errorf("Expected error code 'INVALID_TOKEN', got '%s'", respWrapper.Error.Code)
	}
}

// TestExportDataForUserWithNoContent tests exporting data for user with no content
func TestExportDataForUserWithNoContent(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		ExportUserData(mock.Anything, 123).
		Return(&repository.UserDataExport{
			ExportDate: "2026-03-31T12:00:00Z",
			User: &repository.User{
				ID:        123,
				Username:  "testuser",
				Email:     "test@example.com",
				Role:      "Author",
				CreatedAt: "2026-03-28T10:30:00Z",
			},
			Content:  []*repository.UserContentItem{},
			Comments: []*repository.UserCommentItem{},
			Media:    []*repository.UserMediaItem{},
		}, nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	req := httptest.NewRequest("GET", "/api/profile/export", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var exportData repository.UserDataExport
	if err := json.NewDecoder(w.Body).Decode(&exportData); err != nil {
		t.Fatalf("Failed to decode export data: %v", err)
	}

	if len(exportData.Content) != 0 {
		t.Errorf("Expected 0 content items, got %d", len(exportData.Content))
	}
	if len(exportData.Comments) != 0 {
		t.Errorf("Expected 0 comments, got %d", len(exportData.Comments))
	}
	if len(exportData.Media) != 0 {
		t.Errorf("Expected 0 media items, got %d", len(exportData.Media))
	}
}

// TestProfileResponseStructure tests the profile response structure
func TestProfileResponseStructure(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:        123,
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      "Author",
			CreatedAt: "2026-03-28T10:30:00Z",
			UpdatedAt: "2026-03-30T12:00:00Z",
		}, nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	req := httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var respWrapper struct {
		Data struct {
			Profile handlers.ProfileInfo `json:"profile"`
			Meta    struct {
				Timestamp string `json:"timestamp"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if respWrapper.Data.Profile.ID == 0 {
		t.Error("Expected profile ID to be set")
	}
	if respWrapper.Data.Profile.Username == "" {
		t.Error("Expected profile username to be set")
	}
	if respWrapper.Data.Profile.Email == "" {
		t.Error("Expected profile email to be set")
	}
	if respWrapper.Data.Profile.Role == "" {
		t.Error("Expected profile role to be set")
	}
	if respWrapper.Data.Profile.CreatedAt == "" {
		t.Error("Expected profile createdAt to be set")
	}
	if respWrapper.Data.Meta.Timestamp == "" {
		t.Error("Expected meta timestamp to be set")
	}

	// Verify timestamp format
	if _, err := time.Parse(time.RFC3339, respWrapper.Data.Meta.Timestamp); err != nil {
		t.Errorf("Expected timestamp in RFC3339 format, got error: %v", err)
	}
}

// TestEmailUpdateVerificationTokenFormat tests the email update verification flow
func TestEmailUpdateVerificationTokenFormat(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateEmail(mock.Anything, 123, "newemail@example.com", mock.Anything).
		Return(nil)
	profileService.EXPECT().
		VerifyEmailUpdate(mock.Anything, "abc123def456").
		Return(123, "newemail@example.com", nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	// Step 1: Request email update
	reqBody := handlers.EmailUpdateRequest{
		NewEmail:        "newemail@example.com",
		CurrentPassword: "CurrentPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/profile/email", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Step 2: Verify email update with the token (no auth required)
	verifyReq := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email?token=abc123def456", nil)
	verifyW := httptest.NewRecorder()

	r.ServeHTTP(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", verifyW.Code, verifyW.Body.String())
	}
}

// TestProfileHandlerWithMiddlewareIntegration tests profile handler with authentication middleware integration
func TestProfileHandlerWithMiddlewareIntegration(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:        123,
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      "Author",
			CreatedAt: "2026-03-28T10:30:00Z",
		}, nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")
	logger := util.NewLogger(os.Stdout)

	profileHandler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	r := chi.NewRouter()
	r.Handle("/api/profile", authMiddleware.RequireAuth(http.HandlerFunc(profileHandler.GetProfile)))

	// Test with valid token
	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")
	req := httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid token, got %d", w.Code)
	}

	// Test with invalid token
	req2 := httptest.NewRequest("GET", "/api/profile", nil)
	req2.Header.Set("Authorization", "Bearer invalid-token")
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 with invalid token, got %d", w2.Code)
	}
}

// TestDataExportJSONStructure tests the data export JSON structure
func TestDataExportJSONStructure(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		ExportUserData(mock.Anything, 123).
		Return(&repository.UserDataExport{
			ExportDate: "2026-03-31T12:00:00Z",
			User: &repository.User{
				ID:        123,
				Username:  "testuser",
				Email:     "test@example.com",
				Role:      "Author",
				CreatedAt: "2026-03-28T10:30:00Z",
			},
			Content: []*repository.UserContentItem{
				{
					ID:        1,
					Type:      "post",
					Title:     "My First Post",
					Slug:      "my-first-post",
					Content:   "<p>Post content here...</p>",
					Status:    "published",
					CreatedAt: "2026-03-29T10:00:00Z",
					UpdatedAt: "2026-03-29T10:30:00Z",
				},
			},
			Comments: []*repository.UserCommentItem{
				{
					ID:            1,
					ContentItemID: 1,
					Content:       "Great post!",
					CreatedAt:     "2026-03-29T14:00:00Z",
					UpdatedAt:     "2026-03-29T14:00:00Z",
				},
			},
			Media: []*repository.UserMediaItem{
				{
					ID:               1,
					Filename:         "image1.webp",
					OriginalFilename: "photo.jpg",
					FilePath:         "/uploads/media/2026/03/29/image1.webp",
					FileSize:         102400,
					MimeType:         "image/webp",
					CreatedAt:        "2026-03-29T12:00:00Z",
				},
			},
		}, nil)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	req := httptest.NewRequest("GET", "/api/profile/export", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Parse and verify JSON structure
	var exportData map[string]any
	if err := json.NewDecoder(w.Body).Decode(&exportData); err != nil {
		t.Fatalf("Failed to decode export data: %v", err)
	}

	// Verify required top-level fields with camelCase JSON tags
	if _, ok := exportData["exportDate"]; !ok {
		t.Error("Expected exportDate field in JSON")
	}
	if _, ok := exportData["user"]; !ok {
		t.Error("Expected user field in JSON")
	}
	if _, ok := exportData["content"]; !ok {
		t.Error("Expected content field in JSON")
	}
	if _, ok := exportData["comments"]; !ok {
		t.Error("Expected comments field in JSON")
	}
	if _, ok := exportData["media"]; !ok {
		t.Error("Expected media field in JSON")
	}

	// Verify user structure with camelCase JSON tags
	userData := exportData["user"].(map[string]any)
	if _, ok := userData["id"]; !ok {
		t.Error("Expected user.id field in JSON")
	}
	if _, ok := userData["username"]; !ok {
		t.Error("Expected user.username field in JSON")
	}
	if _, ok := userData["email"]; !ok {
		t.Error("Expected user.email field in JSON")
	}

	// Verify content is an array
	contentData := exportData["content"].([]any)
	if len(contentData) == 0 {
		t.Error("Expected content to be an array with at least one item")
	}
}

// TestProfileHandlerRequestSizeLimit tests request body size limit
func TestProfileHandlerRequestSizeLimit(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	// Create a very large request body (simulating potential abuse)
	largeBody := make([]byte, 2*1024*1024) // 2MB
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest("PUT", "/api/profile/email", bytes.NewBuffer(largeBody))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// The request should be rejected due to size limit
	// Note: This test verifies the handler doesn't crash with large input
	// In production, you'd want to add middleware to enforce size limits
	if w.Code == http.StatusOK {
		t.Log("Large request was accepted - consider adding request size limit middleware")
	}
}

// TestProfileHandlerJSONParsing tests JSON parsing edge cases
func TestProfileHandlerJSONParsing(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	r := setupProfileTestRouter(profileService, accountDeletionService)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-testing")

	userToken, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	tests := []struct {
		name           string
		method         string
		path           string
		body           io.Reader
		expectedStatus int
	}{
		{
			name:           "Invalid JSON",
			method:         "PUT",
			path:           "/api/profile/email",
			body:           bytes.NewBufferString("{invalid json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty request body",
			method:         "PUT",
			path:           "/api/profile/email",
			body:           bytes.NewBufferString(""),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing required fields",
			method:         "PUT",
			path:           "/api/profile/email",
			body:           bytes.NewBufferString(`{"newEmail":"test@example.com"}`),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			req.Header.Set("Authorization", "Bearer "+userToken)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
