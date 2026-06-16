package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// FirstLoginStatus represents the first-login status response
type FirstLoginStatus struct {
	FirstLoginComplete bool   `json:"firstLoginComplete"`
	Redirect           string `json:"redirect,omitempty"`
}

// CompleteSetupRequest represents the request body for completing first-login setup
type CompleteSetupRequest struct {
	Password     string `json:"password"`
	Email        string `json:"email"`
	DatabaseType string `json:"databaseType"`
}

// CompleteSetupResponse represents the response for completing first-login setup
type CompleteSetupResponse struct {
	Message  string `json:"message"`
	Redirect string `json:"redirect"`
}

// FirstLoginHandler handles first-login setup HTTP requests
type FirstLoginHandler struct {
	firstLoginService *authdomain.FirstLoginService
	userRepo          repository.UserRepo
	logger            *util.Logger
}

// GetStatus handles GET /api/auth/first-login
func (h *FirstLoginHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	admin, err := h.userRepo.GetAdminUser(r.Context())
	if err != nil {
		if err == repository.ErrAdminNotFound {
			response.Success(w, FirstLoginStatus{FirstLoginComplete: false})
			return
		}
		h.logger.Error("Failed to get admin user: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check setup status", nil)
		return
	}

	status := FirstLoginStatus{
		FirstLoginComplete: h.firstLoginService.IsSetupComplete(admin.PasswordHash),
	}

	if status.FirstLoginComplete {
		status.Redirect = "/admin/dashboard"
	}

	response.Success(w, status)
}

// CompleteSetup handles POST /api/auth/first-login
func (h *FirstLoginHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	// Limit request body size to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CompleteSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate database type
	if req.DatabaseType != "sqlite" {
		response.Error(w, http.StatusBadRequest, "INVALID_DATABASE_TYPE", "Database type must be 'sqlite'", nil)
		return
	}

	// Validate password
	if err := appauth.ValidatePassword(req.Password); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", err.Error(), nil)
		return
	}

	// Sanitize and validate email
	req.Email = strings.TrimSpace(req.Email)
	req.Email = strings.ToLower(req.Email)
	if err := appauth.ValidateEmail(req.Email); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_EMAIL", err.Error(), nil)
		return
	}

	// Check if setup is already complete by querying DB (before expensive bcrypt)
	admin, err := h.userRepo.GetAdminUser(r.Context())
	if err != nil {
		if err == repository.ErrAdminNotFound {
			response.Error(w, http.StatusInternalServerError, "ADMIN_NOT_FOUND", "Admin user not found. Please check database integrity or restart application.", nil)
			return
		}
		h.logger.Error("Failed to get admin user: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify setup status", nil)
		return
	}

	if h.firstLoginService.IsSetupComplete(admin.PasswordHash) {
		response.Error(w, http.StatusBadRequest, "SETUP_ALREADY_COMPLETE", "First-login setup has already been completed", nil)
		return
	}

	// Hash new password (expensive operation — only after confirming setup is needed)
	passwordHash, err := appauth.HashPassword(req.Password)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "HASH_FAILED", "Failed to hash password", nil)
		return
	}

	// Update admin user in database with optimistic locking
	if err := h.userRepo.UpdateAdminPasswordAndEmail(r.Context(), passwordHash, req.Email, admin.PasswordHash); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			response.Error(w, http.StatusRequestTimeout, "REQUEST_TIMEOUT", "Database operation timed out", nil)
			return
		}
		if errors.Is(err, repository.ErrSetupAlreadyCompleted) {
			response.Error(w, http.StatusBadRequest, "SETUP_ALREADY_COMPLETE", "First-login setup has already been completed", nil)
			return
		}
		h.logger.Error("Failed to update admin user: %v", err)
		response.Error(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update admin user", nil)
		return
	}

	response.Success(w, CompleteSetupResponse{
		Message:  "Setup completed successfully",
		Redirect: "/admin/dashboard",
	})
}

// NewFirstLoginHandler creates a new first-login handler
func NewFirstLoginHandler(
	firstLoginService *authdomain.FirstLoginService,
	userRepo repository.UserRepo,
	logger *util.Logger,
) *FirstLoginHandler {
	return &FirstLoginHandler{
		firstLoginService: firstLoginService,
		userRepo:          userRepo,
		logger:            logger,
	}
}
