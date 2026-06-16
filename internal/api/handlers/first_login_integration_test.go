package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/constants"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/mock"
)

func setupIntegrationTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	db, err := appdatabase.Open("sqlite", ":memory:", 0)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	if err := db.RunMigrations("sqlite"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create default admin user
	defaultPasswordHash, err := appauth.HashPassword(constants.DefaultPassword)
	if err != nil {
		t.Fatalf("Failed to hash default password: %v", err)
	}

	_, err = db.DB().Exec(`
		INSERT INTO users (username, password_hash, role)
		VALUES (?, ?, ?)
	`, constants.DefaultUsername, defaultPasswordHash, constants.DefaultRole)
	if err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	return db.DB(), defaultPasswordHash
}

func TestCompleteSetup_Integration(t *testing.T) {
	db, defaultPasswordHash := setupIntegrationTestDB(t)

	userRepo := repository.NewUserRepository(db)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(firstLoginService, userRepo, logger)

	// Test completing setup with valid data
	reqBody := handlers.CompleteSetupRequest{
		Password:     "SecurePassword123!",
		Email:        "admin@example.com",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify admin user was updated in database
	user, err := userRepo.GetAdminUser(context.Background())
	if err != nil {
		t.Fatalf("Failed to get admin user: %v", err)
	}

	// Verify first-login is marked complete by checking the DB-derived state
	if !firstLoginService.IsSetupComplete(user.PasswordHash) {
		t.Error("First-login should be marked complete")
	}

	if user.Email != "admin@example.com" {
		t.Errorf("Expected email 'admin@example.com', got '%s'", user.Email)
	}

	// Verify password was changed (default password should no longer work)
	err = appauth.VerifyPassword(user.PasswordHash, constants.DefaultPassword)
	if err == nil {
		t.Error("Default password should no longer work after setup")
	}

	// Verify new password works
	err = appauth.VerifyPassword(user.PasswordHash, "SecurePassword123!")
	if err != nil {
		t.Errorf("New password should work, got error: %v", err)
	}

	// Verify admin status is set to 'verified' after completing first login
	if user.Status != "verified" {
		t.Errorf("Expected admin status 'verified', got '%s'", user.Status)
	}
}

func TestCompleteSetup_InvalidPassword(t *testing.T) {
	db, defaultPasswordHash := setupIntegrationTestDB(t)

	userRepo := repository.NewUserRepository(db)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(firstLoginService, userRepo, logger)

	// Test with invalid password (too short)
	reqBody := handlers.CompleteSetupRequest{
		Password:     "short",
		Email:        "admin@example.com",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify first-login is NOT complete by checking the DB-derived state
	admin, err := userRepo.GetAdminUser(context.Background())
	if err != nil {
		t.Fatalf("Failed to get admin user: %v", err)
	}
	if firstLoginService.IsSetupComplete(admin.PasswordHash) {
		t.Error("First-login should not be marked complete with invalid password")
	}
}

func TestCompleteSetup_InvalidEmail(t *testing.T) {
	db, defaultPasswordHash := setupIntegrationTestDB(t)

	userRepo := repository.NewUserRepository(db)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(firstLoginService, userRepo, logger)

	// Test with invalid email
	reqBody := handlers.CompleteSetupRequest{
		Password:     "SecurePassword123!",
		Email:        "not-an-email",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify first-login is NOT complete by checking the DB-derived state
	admin, err := userRepo.GetAdminUser(context.Background())
	if err != nil {
		t.Fatalf("Failed to get admin user: %v", err)
	}
	if firstLoginService.IsSetupComplete(admin.PasswordHash) {
		t.Error("First-login should not be marked complete with invalid email")
	}
}

func TestCompleteSetup_AlreadyComplete(t *testing.T) {
	db, defaultPasswordHash := setupIntegrationTestDB(t)

	userRepo := repository.NewUserRepository(db)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(firstLoginService, userRepo, logger)

	// Simulate already-completed setup by updating the admin password in the DB
	changedHash, err := appauth.HashPassword("AlreadyChanged789!")
	if err != nil {
		t.Fatalf("Failed to hash changed password: %v", err)
	}
	_, err = db.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, changedHash, constants.DefaultUsername)
	if err != nil {
		t.Fatalf("Failed to update admin password: %v", err)
	}

	// Try to complete setup again
	reqBody := handlers.CompleteSetupRequest{
		Password:     "AnotherSecure456!",
		Email:        "another@example.com",
		DatabaseType: "sqlite",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/auth/first-login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetStatus_AfterSetupComplete(t *testing.T) {
	defaultPasswordHash, err := appauth.HashPassword(constants.DefaultPassword)
	if err != nil {
		t.Fatalf("Failed to hash default password: %v", err)
	}
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	// Simulate setup completion: admin password differs from default
	changedHash := "$2a$12$changedpasswordhash"
	mockRepo := repomocks.NewMockUserRepo(t)
	mockRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: changedHash,
			Role:         "Admin",
		}, nil)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(firstLoginService, mockRepo, logger)

	req := httptest.NewRequest("GET", "/api/auth/first-login", nil)
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
