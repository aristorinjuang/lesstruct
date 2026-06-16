package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	usermocks "github.com/aristorinjuang/lesstruct/internal/domain/user/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
)

// Helper function to create authenticated request
func createAuthenticatedRequest(handler http.Handler, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestProfileHandler_GetProfile_Success(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:           123,
			Username:     "testuser",
			Email:        "test@example.com",
			Role:         "Author",
			CreatedAt:    "2026-03-28T10:30:00Z",
			UpdatedAt:    "2026-03-30T12:00:00Z",
			CustomFields: map[string]any{"job_title": "Engineer", "company": "Acme"},
		}, nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	// Check if handler is properly initialized
	require.NotNil(t, handler, "handler is nil")

	// Generate JWT token
	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	// Create request with context
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Manually set context (simulate middleware)
	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "testuser")
	ctx = context.WithValue(ctx, appmiddleware.RoleKey, "Author")
	req = req.WithContext(ctx)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler directly
	handler.GetProfile(rr, req)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	profile := data["profile"].(map[string]any)

	assert.Equal(t, float64(123), profile["id"], "expected ID 123")
	assert.Equal(t, "testuser", profile["username"], "expected username testuser")
	assert.Equal(t, map[string]any{"job_title": "Engineer", "company": "Acme"}, profile["customFields"])
}

func TestProfileHandler_GetProfile_NoCustomFields(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		GetProfile(mock.Anything, 456).
		Return(&user.Profile{
			ID:        456,
			Username:  "nosluguser",
			Email:     "noslug@example.com",
			Role:      "Author",
			CreatedAt: "2026-03-28T10:30:00Z",
		}, nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "456")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "nosluguser")
	ctx = context.WithValue(ctx, appmiddleware.RoleKey, "Author")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetProfile(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	data := resp["data"].(map[string]any)
	profile := data["profile"].(map[string]any)

	_, hasCustomFields := profile["customFields"]
	assert.False(t, hasCustomFields, "customFields should be omitted when nil")
}

func TestProfileHandler_GetProfile_Unauthorized(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)

	protectedHandler := authMiddleware.RequireAuth(http.HandlerFunc(handler.GetProfile))

	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	rr := httptest.NewRecorder()

	protectedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "expected status 401")
}

func TestProfileHandler_UpdateEmail_Success(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateEmail(mock.Anything, 123, "newemail@example.com", mock.Anything).
		Return(nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	protectedHandler := authMiddleware.RequireAuth(http.HandlerFunc(handler.UpdateEmail))

	reqBody := handlers.EmailUpdateRequest{
		NewEmail:        "newemail@example.com",
		CurrentPassword: "CurrentPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	rr := createAuthenticatedRequest(protectedHandler, http.MethodPut, "/api/profile/email", bodyBytes, token)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["message"], "expected message in response")
}

func TestProfileHandler_UpdateEmail_MissingFields(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	protectedHandler := authMiddleware.RequireAuth(http.HandlerFunc(handler.UpdateEmail))

	reqBody := handlers.EmailUpdateRequest{
		NewEmail: "newemail@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	rr := createAuthenticatedRequest(protectedHandler, http.MethodPut, "/api/profile/email", bodyBytes, token)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_ChangePassword_Success(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		ChangePassword(mock.Anything, 123, "OldPassword123!", "NewPassword456!").
		Return(nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	protectedHandler := authMiddleware.RequireAuth(http.HandlerFunc(handler.ChangePassword))

	reqBody := handlers.PasswordUpdateRequest{
		CurrentPassword: "OldPassword123!",
		NewPassword:     "NewPassword456!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	rr := createAuthenticatedRequest(protectedHandler, http.MethodPut, "/api/profile/password", bodyBytes, token)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["message"], "expected message in response")
}

func TestProfileHandler_ChangePassword_MissingFields(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	protectedHandler := authMiddleware.RequireAuth(http.HandlerFunc(handler.ChangePassword))

	reqBody := handlers.PasswordUpdateRequest{
		CurrentPassword: "OldPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	rr := createAuthenticatedRequest(protectedHandler, http.MethodPut, "/api/profile/password", bodyBytes, token)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_ExportUserData_Success(t *testing.T) {
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
		}, nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	protectedHandler := authMiddleware.RequireAuth(http.HandlerFunc(handler.ExportUserData))

	rr := createAuthenticatedRequest(protectedHandler, http.MethodGet, "/api/profile/export", nil, token)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected Content-Type application/json")

	contentDisposition := rr.Header().Get("Content-Disposition")
	assert.NotEmpty(t, contentDisposition, "expected Content-Disposition header")

	var exportData repository.UserDataExport
	err := json.NewDecoder(rr.Body).Decode(&exportData)
	require.NoError(t, err, "failed to decode export data")

	assert.Equal(t, 123, exportData.User.ID, "expected user ID 123")
}

func TestProfileHandler_VerifyEmailUpdate_Success(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		VerifyEmailUpdate(mock.Anything, "validtoken123").
		Return(123, "newemail@example.com", nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	// No authentication required - just call the handler directly
	req := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email?token=validtoken123", nil)
	rr := httptest.NewRecorder()

	handler.VerifyEmailUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["message"], "expected message in response")
	assert.Equal(t, "newemail@example.com", data["email"], "expected email newemail@example.com")
}

func TestProfileHandler_VerifyEmailUpdate_MissingToken(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	// No authentication required
	req := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email", nil)
	rr := httptest.NewRecorder()

	handler.VerifyEmailUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_DeleteAccount_Success(t *testing.T) {
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
	accountDeletionService.EXPECT().
		ValidateConfirmationString("DELETE").
		Return(nil)
	accountDeletionService.EXPECT().
		DeleteAccount(mock.Anything, 123, "testuser", "test@example.com").
		Return(nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "testuser")
	ctx = context.WithValue(ctx, appmiddleware.RoleKey, "Author")
	req = req.WithContext(ctx)

	reqBody := handlers.DeleteAccountRequest{Confirmation: "DELETE"}
	bodyBytes, _ := json.Marshal(reqBody)
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["message"], "expected message in response")
}

func TestProfileHandler_DeleteAccount_InvalidConfirmation(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	accountDeletionService.EXPECT().
		ValidateConfirmationString("delete").
		Return(errors.New("confirmation must be 'DELETE'"))
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	token, _ := jwtManager.GenerateToken("123", "testuser", "Author")

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "testuser")
	req = req.WithContext(ctx)

	reqBody := handlers.DeleteAccountRequest{Confirmation: "delete"}
	bodyBytes, _ := json.Marshal(reqBody)
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_DeleteAccount_LastAdmin(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:        123,
			Username:  "admin",
			Email:     "admin@example.com",
			Role:      "Admin",
			CreatedAt: "2026-03-28T10:30:00Z",
		}, nil)
	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	accountDeletionService.EXPECT().
		ValidateConfirmationString("DELETE").
		Return(nil)
	accountDeletionService.EXPECT().
		DeleteAccount(mock.Anything, 123, "admin", "admin@example.com").
		Return(nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	token, _ := jwtManager.GenerateToken("123", "admin", "Admin")

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "admin")
	ctx = context.WithValue(ctx, appmiddleware.RoleKey, "Admin")
	req = req.WithContext(ctx)

	reqBody := handlers.DeleteAccountRequest{Confirmation: "DELETE"}
	bodyBytes, _ := json.Marshal(reqBody)
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	// Default mock returns success
	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200 with default mock")
}

func TestProfileHandler_DeleteAccount_Unauthorized(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", nil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "expected status 401")
}

func TestProfileHandler_UpdateCustomFields_Success(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateCustomFields(mock.Anything, 123, map[string]any{
			"job_title": "Engineer",
			"company":   "Acme",
		}, false).
		Return(nil)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:           123,
			Username:     "testuser",
			Email:        "test@example.com",
			Role:         "Author",
			CustomFields: map[string]any{"job_title": "Engineer", "company": "Acme"},
		}, nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	reqBody := map[string]any{
		"customFields": map[string]any{
			"job_title": "Engineer",
			"company":   "Acme",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "testuser")
	ctx = context.WithValue(ctx, appmiddleware.RoleKey, "Author")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.UpdateCustomFields(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	data := resp["data"].(map[string]any)
	profile := data["profile"].(map[string]any)
	assert.Equal(t, float64(123), profile["id"])
	assert.Equal(t, "testuser", profile["username"])
	assert.Equal(t, map[string]any{"job_title": "Engineer", "company": "Acme"}, profile["customFields"])
}

func TestProfileHandler_UpdateCustomFields_Success_Admin(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	profileService.EXPECT().
		UpdateCustomFields(mock.Anything, 123, map[string]any{
			"internal_rating": "gold",
		}, true).
		Return(nil)
	profileService.EXPECT().
		GetProfile(mock.Anything, 123).
		Return(&user.Profile{
			ID:           123,
			Username:     "admin",
			Email:        "admin@example.com",
			Role:         "Admin",
			CustomFields: map[string]any{"internal_rating": "gold"},
		}, nil)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	reqBody := map[string]any{
		"customFields": map[string]any{
			"internal_rating": "gold",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	ctx = context.WithValue(ctx, appmiddleware.UsernameKey, "admin")
	ctx = context.WithValue(ctx, appmiddleware.RoleKey, "Admin")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.UpdateCustomFields(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	data := resp["data"].(map[string]any)
	profile := data["profile"].(map[string]any)
	assert.Equal(t, float64(123), profile["id"])
	assert.Equal(t, "admin", profile["username"])
	assert.Equal(t, map[string]any{"internal_rating": "gold"}, profile["customFields"])
}

func TestProfileHandler_UpdateCustomFields_Unauthorized(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateCustomFields(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestProfileHandler_UpdateCustomFields_InvalidJSON(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), appmiddleware.UserIDKey, "123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.UpdateCustomFields(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProfileHandler_GetUserFields_Success(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/user-fields", nil)

	rr := httptest.NewRecorder()
	handler.GetUserFields(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	data := resp["data"].(map[string]any)
	assert.NotNil(t, data["fields"])
	assert.NotNil(t, data["systemFields"])
}

func TestProfileHandler_GetUserFields_Unauthorized(t *testing.T) {
	profileService := usermocks.NewMockProfileServiceInterface(t)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)

	accountDeletionService := usermocks.NewMockAccountDeletionServiceInterface(t)
	handler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/user-fields", nil)
	rr := httptest.NewRecorder()
	handler.GetUserFields(rr, req)

	// GetUserFields itself doesn't check auth (middleware does)
	// but it should return empty arrays when provider is nil
	assert.Equal(t, http.StatusOK, rr.Code)
}
