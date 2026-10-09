package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

func defaultAdmin() *userdomain.User {
	return &userdomain.User{
		ID:           1,
		Username:     "admin",
		PasswordHash: "hashed:admin",
		Role:         "Admin",
		Status:       "pending",
	}
}

func TestCompleteSetup_Integration(t *testing.T) {
	store := &stubUserStore{admin: defaultAdmin()}
	firstLoginService := authdomain.NewFirstLoginService(stubPolicy{})
	handler := api.NewFirstLoginHandler(firstLoginService, store, stubPolicy{}, nil)

	// Test completing setup with valid data
	reqBody := api.CompleteSetupRequest{
		Password:     "SecurePassword123!",
		Email:        "admin@example.com",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify admin user was updated in the store
	admin := store.admin
	if admin.Email != "admin@example.com" {
		t.Errorf("Expected email 'admin@example.com', got '%s'", admin.Email)
	}

	// Verify first-login is marked complete by checking the derived state
	if !firstLoginService.IsSetupComplete(admin.PasswordHash) {
		t.Error("First-login should be marked complete")
	}

	// Verify password was changed (default password should no longer work)
	policy := stubPolicy{}
	if err := policy.VerifyPassword(admin.PasswordHash, authdomain.DefaultPassword); err == nil {
		t.Error("Default password should no longer work after setup")
	}

	// Verify new password works
	if err := policy.VerifyPassword(admin.PasswordHash, "SecurePassword123!"); err != nil {
		t.Errorf("New password should work, got error: %v", err)
	}

	// Verify admin status is set to 'verified' after completing first login
	if admin.Status != "verified" {
		t.Errorf("Expected admin status 'verified', got '%s'", admin.Status)
	}
}

func TestCompleteSetup_InvalidPassword(t *testing.T) {
	store := &stubUserStore{admin: defaultAdmin()}
	firstLoginService := authdomain.NewFirstLoginService(stubPolicy{})
	handler := api.NewFirstLoginHandler(firstLoginService, store, stubPolicy{passwordErr: errors.New("weak password")}, nil)

	// Test with invalid password (too short)
	reqBody := api.CompleteSetupRequest{
		Password:     "short",
		Email:        "admin@example.com",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify first-login is NOT complete: the stored hash is untouched.
	if firstLoginService.IsSetupComplete(store.admin.PasswordHash) {
		t.Error("First-login should not be marked complete with invalid password")
	}
}

func TestCompleteSetup_InvalidEmail(t *testing.T) {
	store := &stubUserStore{admin: defaultAdmin()}
	firstLoginService := authdomain.NewFirstLoginService(stubPolicy{})
	handler := api.NewFirstLoginHandler(firstLoginService, store, stubPolicy{emailErr: errors.New("bad email")}, nil)

	// Test with invalid email
	reqBody := api.CompleteSetupRequest{
		Password:     "SecurePassword123!",
		Email:        "not-an-email",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify first-login is NOT complete by checking the derived state.
	if firstLoginService.IsSetupComplete(store.admin.PasswordHash) {
		t.Error("First-login should not be marked complete with invalid email")
	}
}

func TestCompleteSetup_AlreadyComplete(t *testing.T) {
	// Simulate already-completed setup: the admin password differs from default.
	store := &stubUserStore{admin: &userdomain.User{
		ID:           1,
		Username:     "admin",
		PasswordHash: "hashed:AlreadyChanged789!",
		Role:         "Admin",
		Status:       "verified",
	}}
	firstLoginService := authdomain.NewFirstLoginService(stubPolicy{})
	handler := api.NewFirstLoginHandler(firstLoginService, store, stubPolicy{}, nil)

	// Try to complete setup again
	reqBody := api.CompleteSetupRequest{
		Password:     "AnotherSecure456!",
		Email:        "another@example.com",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetStatus_AfterSetupComplete(t *testing.T) {
	firstLoginService := authdomain.NewFirstLoginService(stubPolicy{})
	// Simulate setup completion: admin password has been changed from default
	store := &stubUserStore{admin: &userdomain.User{
		ID:           1,
		Username:     "admin",
		PasswordHash: "hashed:AlreadyChanged789!",
		Role:         "Admin",
	}}
	handler := api.NewFirstLoginHandler(firstLoginService, store, stubPolicy{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/first-login", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("Response data should be an object")
	}

	firstLoginComplete, ok := data["firstLoginComplete"].(bool)
	if !ok || !firstLoginComplete {
		t.Error("firstLoginComplete should be true")
	}

	redirect, ok := data["redirect"].(string)
	if !ok || redirect != "/admin/dashboard" {
		t.Errorf("Expected redirect '/admin/dashboard', got '%v'", data["redirect"])
	}
}
