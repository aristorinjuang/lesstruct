package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
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
	userRepo          UserStore
	policy            authdomain.CredentialPolicy
	logger            kernel.Logger
}

// GetStatus handles GET /api/auth/first-login
func (h *FirstLoginHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	admin, err := h.userRepo.GetAdminUser(r.Context())
	if err != nil {
		if err == userdomain.ErrAdminNotFound {
			httpx.Success(w, FirstLoginStatus{FirstLoginComplete: false})
			return
		}
		h.logger.Error("Failed to get admin user: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check setup status", nil)
		return
	}

	status := FirstLoginStatus{
		FirstLoginComplete: h.firstLoginService.IsSetupComplete(admin.PasswordHash),
	}

	if status.FirstLoginComplete {
		status.Redirect = "/admin/dashboard"
	}

	httpx.Success(w, status)
}

// CompleteSetup handles POST /api/auth/first-login
func (h *FirstLoginHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	// Limit request body size to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CompleteSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate database type
	if req.DatabaseType != "sqlite" {
		httpx.Error(w, http.StatusBadRequest, "INVALID_DATABASE_TYPE", "Database type must be 'sqlite'", nil)
		return
	}

	// Validate password
	if err := h.policy.ValidatePassword(req.Password); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", err.Error(), nil)
		return
	}

	// Sanitize and validate email
	req.Email = strings.TrimSpace(req.Email)
	req.Email = strings.ToLower(req.Email)
	if err := h.policy.ValidateEmail(req.Email); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_EMAIL", err.Error(), nil)
		return
	}

	// Check if setup is already complete by querying DB (before expensive bcrypt)
	admin, err := h.userRepo.GetAdminUser(r.Context())
	if err != nil {
		if err == userdomain.ErrAdminNotFound {
			httpx.Error(w, http.StatusInternalServerError, "ADMIN_NOT_FOUND", "Admin user not found. Please check database integrity or restart application.", nil)
			return
		}
		h.logger.Error("Failed to get admin user: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify setup status", nil)
		return
	}

	if h.firstLoginService.IsSetupComplete(admin.PasswordHash) {
		httpx.Error(w, http.StatusBadRequest, "SETUP_ALREADY_COMPLETE", "First-login setup has already been completed", nil)
		return
	}

	// Hash new password (expensive operation — only after confirming setup is needed)
	passwordHash, err := h.policy.HashPassword(req.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "HASH_FAILED", "Failed to hash password", nil)
		return
	}

	// Update admin user in database with optimistic locking
	if err := h.userRepo.UpdateAdminPasswordAndEmail(r.Context(), passwordHash, req.Email, admin.PasswordHash); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			httpx.Error(w, http.StatusRequestTimeout, "REQUEST_TIMEOUT", "Database operation timed out", nil)
			return
		}
		if errors.Is(err, userdomain.ErrSetupAlreadyCompleted) {
			httpx.Error(w, http.StatusBadRequest, "SETUP_ALREADY_COMPLETE", "First-login setup has already been completed", nil)
			return
		}
		h.logger.Error("Failed to update admin user: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update admin user", nil)
		return
	}

	httpx.Success(w, CompleteSetupResponse{
		Message:  "Setup completed successfully",
		Redirect: "/admin/dashboard",
	})
}

// NewFirstLoginHandler creates a new first-login handler. A nil logger
// degrades to a discard sink so the handler is safe to construct in any
// context.
func NewFirstLoginHandler(
	firstLoginService *authdomain.FirstLoginService,
	userRepo UserStore,
	policy authdomain.CredentialPolicy,
	logger kernel.Logger,
) *FirstLoginHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	return &FirstLoginHandler{
		firstLoginService: firstLoginService,
		userRepo:          userRepo,
		policy:            policy,
		logger:            logger,
	}
}
