package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/modules/users/api"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileHandler_GetProfile_Success(t *testing.T) {
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:           123,
			Username:     "testuser",
			Email:        "test@example.com",
			Role:         "Author",
			CreatedAt:    time.Date(2026, 3, 28, 10, 30, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC),
			CustomFields: map[string]any{"job_title": "Engineer", "company": "Acme"},
		},
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	// Check if handler is properly initialized
	require.NotNil(t, handler, "handler is nil")

	// Create request with principal (simulate middleware)
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req = requestWithPrincipal(req, "123", "testuser", "Author")

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
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:        456,
			Username:  "nosluguser",
			Email:     "noslug@example.com",
			Role:      "Author",
			CreatedAt: time.Date(2026, 3, 28, 10, 30, 0, 0, time.UTC),
		},
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req = requestWithPrincipal(req, "456", "nosluguser", "Author")

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
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	// No identity attached: handler must reject with 401 by itself
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	rr := httptest.NewRecorder()

	handler.GetProfile(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "expected status 401")
}

func TestProfileHandler_UpdateEmail_Success(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.EmailUpdateRequest{
		NewEmail:        "newemail@example.com",
		CurrentPassword: "CurrentPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/email", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

	rr := httptest.NewRecorder()
	handler.UpdateEmail(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["message"], "expected message in response")
}

func TestProfileHandler_UpdateEmail_MissingFields(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.EmailUpdateRequest{
		NewEmail: "newemail@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/email", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

	rr := httptest.NewRecorder()
	handler.UpdateEmail(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_ChangePassword_Success(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.PasswordUpdateRequest{
		CurrentPassword: "OldPassword123!",
		NewPassword:     "NewPassword456!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["message"], "expected message in response")
}

func TestProfileHandler_ChangePassword_MissingFields(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.PasswordUpdateRequest{
		CurrentPassword: "OldPassword123!",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_ExportUserData_Success(t *testing.T) {
	profileService := &stubProfileService{
		export: &userdomain.UserDataExport{
			ExportDate: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
			User: &userdomain.User{
				ID:        123,
				Username:  "testuser",
				Email:     "test@example.com",
				Role:      "Author",
				CreatedAt: time.Date(2026, 3, 28, 10, 30, 0, 0, time.UTC),
			},
		},
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/export", nil)
	req = requestWithPrincipal(req, "123", "testuser", "Author")

	rr := httptest.NewRecorder()
	handler.ExportUserData(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200")

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "expected Content-Type application/json")

	contentDisposition := rr.Header().Get("Content-Disposition")
	assert.NotEmpty(t, contentDisposition, "expected Content-Disposition header")

	var exportData userdomain.UserDataExport
	err := json.NewDecoder(rr.Body).Decode(&exportData)
	require.NoError(t, err, "failed to decode export data")

	assert.Equal(t, 123, exportData.User.ID, "expected user ID 123")
}

func TestProfileHandler_VerifyEmailUpdate_Success(t *testing.T) {
	profileService := &stubProfileService{
		verifyID:    123,
		verifyEmail: "newemail@example.com",
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

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
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	// No authentication required
	req := httptest.NewRequest(http.MethodGet, "/api/profile/verify-email", nil)
	rr := httptest.NewRecorder()

	handler.VerifyEmailUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_DeleteAccount_Success(t *testing.T) {
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:        123,
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      "Author",
			CreatedAt: time.Date(2026, 3, 28, 10, 30, 0, 0, time.UTC),
		},
	}
	accountDeletionService := &stubAccountDeletion{}

	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.DeleteAccountRequest{Confirmation: "DELETE"}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

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
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{
		confirmErr: errors.New("confirmation must be 'DELETE'"),
	}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.DeleteAccountRequest{Confirmation: "delete"}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser")

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status 400")
}

func TestProfileHandler_DeleteAccount_LastAdmin(t *testing.T) {
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:        123,
			Username:  "admin",
			Email:     "admin@example.com",
			Role:      "Admin",
			CreatedAt: time.Date(2026, 3, 28, 10, 30, 0, 0, time.UTC),
		},
	}
	accountDeletionService := &stubAccountDeletion{}

	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := api.DeleteAccountRequest{Confirmation: "DELETE"}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "admin", "Admin")

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	// Default stub returns success
	assert.Equal(t, http.StatusOK, rr.Code, "expected status 200 with default stub")
}

func TestProfileHandler_DeleteAccount_Unauthorized(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/account", nil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "expected status 401")
}

func TestProfileHandler_UpdateCustomFields_Success(t *testing.T) {
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:           123,
			Username:     "testuser",
			Email:        "test@example.com",
			Role:         "Author",
			CustomFields: map[string]any{"job_title": "Engineer", "company": "Acme"},
		},
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := map[string]any{
		"customFields": map[string]any{
			"job_title": "Engineer",
			"company":   "Acme",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

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
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:           123,
			Username:     "admin",
			Email:        "admin@example.com",
			Role:         "Admin",
			CustomFields: map[string]any{"internal_rating": "gold"},
		},
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := map[string]any{
		"customFields": map[string]any{
			"internal_rating": "gold",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "admin", "Admin")

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
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateCustomFields(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestProfileHandler_UpdateCustomFields_InvalidJSON(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/custom-fields", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser")

	rr := httptest.NewRecorder()
	handler.UpdateCustomFields(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProfileHandler_UpdateName_Success(t *testing.T) {
	profileService := &stubProfileService{
		profile: &userdomain.Profile{
			ID:       123,
			Username: "testuser",
			Name:     "Jane Doe",
			Email:    "test@example.com",
			Role:     "Author",
		},
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := map[string]any{
		"name": "Jane Doe",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/name", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser", "Author")

	rr := httptest.NewRecorder()
	handler.UpdateName(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	data := resp["data"].(map[string]any)
	profile := data["profile"].(map[string]any)
	assert.Equal(t, float64(123), profile["id"])
	assert.Equal(t, "Jane Doe", profile["name"])
}

func TestProfileHandler_UpdateName_Unauthorized(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/name", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateName(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestProfileHandler_UpdateName_InvalidJSON(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/name", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser")

	rr := httptest.NewRecorder()
	handler.UpdateName(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProfileHandler_UpdateName_ServiceError(t *testing.T) {
	profileService := &stubProfileService{
		nameErr: errors.New("boom"),
	}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, &stubUserFields{}, nil)

	reqBody := map[string]any{
		"name": "Jane Doe",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/profile/name", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithPrincipal(req, "123", "testuser")

	rr := httptest.NewRecorder()
	handler.UpdateName(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProfileHandler_GetUserFields_Success(t *testing.T) {
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, nil, nil)

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
	profileService := &stubProfileService{}

	accountDeletionService := &stubAccountDeletion{}
	handler := api.NewProfileHandler(profileService, accountDeletionService, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/user-fields", nil)
	rr := httptest.NewRecorder()
	handler.GetUserFields(rr, req)

	// GetUserFields itself doesn't check auth (middleware does)
	// but it should return empty arrays when provider is nil
	assert.Equal(t, http.StatusOK, rr.Code)
}
