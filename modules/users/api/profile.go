package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// sanitizeFilename removes characters that are unsafe in Content-Disposition headers
func sanitizeFilename(name string) string {
	safe := make([]byte, 0, len(name))
	for i := range len(name) {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			safe = append(safe, c)
		}
	}
	if len(safe) == 0 {
		return "user"
	}
	if len(safe) > 64 {
		return string(safe[:64])
	}
	return string(safe)
}

// exportFilenameTimestamp is the time layout used in data export filenames
const exportFilenameTimestamp = "20060102-150405"

// ProfileResponse represents the response for profile information
type ProfileResponse struct {
	Data *ProfileData         `json:"data"`
	Meta *ProfileResponseMeta `json:"meta"`
}

// ProfileResponseMeta represents metadata in profile response
type ProfileResponseMeta struct {
	Timestamp string `json:"timestamp"`
}

// ProfileData represents profile data in the response
type ProfileData struct {
	Profile *ProfileInfo `json:"profile"`
}

// ProfileInfo represents user profile information
type ProfileInfo struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	Name           string         `json:"name,omitempty"`
	Email          string         `json:"email"`
	Role           string         `json:"role"`
	ProfilePicture string         `json:"profilePicture,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	CustomFields   map[string]any `json:"customFields,omitempty"`
}

// EmailUpdateRequest represents a request to update email
type EmailUpdateRequest struct {
	NewEmail        string `json:"newEmail"`
	CurrentPassword string `json:"currentPassword"`
}

// EmailUpdateResponse represents the response for email update
type EmailUpdateResponse struct {
	Data *EmailUpdateData     `json:"data"`
	Meta *ProfileResponseMeta `json:"meta"`
}

// EmailUpdateData represents email update data in the response
type EmailUpdateData struct {
	Message  string `json:"message"`
	NewEmail string `json:"newEmail"`
}

// PasswordUpdateRequest represents a request to update password
type PasswordUpdateRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// PasswordUpdateResponse represents the response for password update
type PasswordUpdateResponse struct {
	Data *PasswordUpdateData  `json:"data"`
	Meta *ProfileResponseMeta `json:"meta"`
}

// PasswordUpdateData represents password update data in the response
type PasswordUpdateData struct {
	Message string `json:"message"`
}

// VerifyEmailUpdateResponse represents the response for email verification
type VerifyEmailUpdateResponse struct {
	Data *VerifyEmailUpdateData `json:"data"`
	Meta *ProfileResponseMeta   `json:"meta"`
}

// VerifyEmailUpdateData represents email verification data in the response
type VerifyEmailUpdateData struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

// DeleteAccountRequest represents a request to delete an account
type DeleteAccountRequest struct {
	Confirmation string `json:"confirmation"`
}

// DataExportMeta represents metadata in data export response
type DataExportMeta struct {
	Timestamp string `json:"timestamp"`
}

// userFieldsProvider provides user field schema access for self-service profile editing
type userFieldsProvider interface {
	GetUserFields() []customfield.FieldSchema
	GetUserSystemFields() []customfield.FieldSchema
}

// ProfileHandler handles profile management HTTP requests
type ProfileHandler struct {
	profileService           userdomain.ProfileServiceInterface
	accountDeletionService   userdomain.AccountDeletionServiceInterface
	logger                   kernel.Logger
	userFieldsProvider       userFieldsProvider
	profilePictureURLBuilder func(string) string
}

// buildProfilePictureURL converts a filename to a full URL if a builder is set.
func (h *ProfileHandler) buildProfilePictureURL(filename string) string {
	if h.profilePictureURLBuilder != nil && filename != "" {
		return h.profilePictureURLBuilder(filename)
	}
	return filename
}

// GetProfile handles GET /api/profile
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Get profile information
	profile, err := h.profileService.GetProfile(r.Context(), principal)
	if err != nil {
		h.logger.Error("Failed to get profile: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve profile", nil)
		return
	}

	// Send response
	profileInfo := ProfileInfo{
		ID:             profile.ID,
		Username:       profile.Username,
		Name:           profile.Name,
		Email:          profile.Email,
		Role:           profile.Role,
		ProfilePicture: h.buildProfilePictureURL(profile.ProfilePicture),
		CreatedAt:      profile.CreatedAt,
		UpdatedAt:      profile.UpdatedAt,
		CustomFields:   profile.CustomFields,
	}
	httpx.Success(w, map[string]any{
		"profile": profileInfo,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// UpdateEmail handles PUT /api/profile/email
func (h *ProfileHandler) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Parse request body
	var req EmailUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	// Validate request
	if req.NewEmail == "" || req.CurrentPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "New email and current password are required", nil)
		return
	}

	// Update email (initiates verification flow)
	err := h.profileService.UpdateEmail(r.Context(), principal, req.NewEmail, req.CurrentPassword)
	if err != nil {
		h.logger.Error("Failed to update email: %v", err)
		if errors.Is(err, userdomain.ErrInvalidEmail) {
			httpx.Error(w, http.StatusBadRequest, "INVALID_EMAIL", "Invalid email address format", nil)
			return
		}
		if errors.Is(err, userdomain.ErrEmailAlreadyInUse) {
			httpx.Error(w, http.StatusBadRequest, "EMAIL_ALREADY_IN_USE", "Email address is already in use", nil)
			return
		}
		if errors.Is(err, userdomain.ErrInvalidPassword) {
			httpx.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", "Current password is incorrect", nil)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update email", nil)
		return
	}

	// Send response
	httpx.Success(w, map[string]any{
		"message":  "Verification email sent to your new email address. Please verify to complete the update.",
		"newEmail": req.NewEmail,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// ChangePassword handles PUT /api/profile/password
func (h *ProfileHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Parse request body
	var req PasswordUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	// Validate request
	if req.CurrentPassword == "" || req.NewPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Current password and new password are required", nil)
		return
	}

	// Change password
	err := h.profileService.ChangePassword(r.Context(), principal, req.CurrentPassword, req.NewPassword)
	if err != nil {
		h.logger.Error("Failed to change password: %v", err)
		if errors.Is(err, userdomain.ErrInvalidPassword) {
			httpx.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", "Current password is incorrect or new password does not meet requirements", nil)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to change password", nil)
		return
	}

	// Send response
	httpx.Success(w, map[string]any{
		"message": "Password updated successfully",
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// ExportUserData handles GET /api/profile/export
func (h *ProfileHandler) ExportUserData(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Get user data for export
	userData, err := h.profileService.ExportUserData(r.Context(), principal)
	if err != nil {
		h.logger.Error("Failed to export user data: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to export user data", nil)
		return
	}

	// Get username for filename
	username := userData.User.Username
	if username == "" {
		username = "user"
	}

	// Sanitize username for Content-Disposition header
	safeUsername := sanitizeFilename(username)

	// Buffer JSON before writing headers so we can still return error responses on encode failure
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(userData); err != nil {
		h.logger.Error("Failed to encode user data: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode user data", nil)
		return
	}

	// Generate timestamp for the download filename
	now := time.Now()
	timestamp := now.Format(exportFilenameTimestamp)
	filename := "lesstruct-user-data-" + safeUsername + "-" + timestamp + ".json"

	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	// Write buffered JSON
	if _, err := w.Write(buf.Bytes()); err != nil {
		h.logger.Error("Failed to write user data export: %v", err)
		return
	}
}

// VerifyEmailUpdate handles GET /api/profile/verify-email
// This endpoint does NOT require authentication - the verification token is sufficient
func (h *ProfileHandler) VerifyEmailUpdate(w http.ResponseWriter, r *http.Request) {
	// Read token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_TOKEN", "Verification token is required", nil)
		return
	}

	// Verify email update (token contains all necessary information)
	tokenUserID, newEmail, err := h.profileService.VerifyEmailUpdate(r.Context(), token)
	if err != nil {
		h.logger.Error("Failed to verify email update: %v", err)
		httpx.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid or expired verification token", nil)
		return
	}

	// Log the email update
	h.logger.Info("Email updated successfully for user %d to %s", tokenUserID, newEmail)

	// Send response
	httpx.Success(w, map[string]any{
		"message": "Email address updated successfully",
		"email":   newEmail,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// DeleteAccount handles DELETE /api/profile/account
func (h *ProfileHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	userID, err := principalID(principal)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get username from the principal for email notification
	usernameStr := principal.Username
	if usernameStr == "" {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Parse request body
	var req DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	// Validate confirmation string
	if err := h.accountDeletionService.ValidateConfirmationString(req.Confirmation); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_CONFIRMATION", "Invalid confirmation. Please type 'DELETE' to confirm account deletion.", []any{
			map[string]any{
				"field": "confirmation",
				"issue": "must be exactly 'DELETE'",
			},
		})
		return
	}

	// Get user email for notification
	profile, err := h.profileService.GetProfile(r.Context(), kernel.Principal{ID: strconv.Itoa(userID), Username: usernameStr})
	if err != nil {
		h.logger.Error("Failed to get user profile for deletion: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user profile", nil)
		return
	}

	// Delete account
	err = h.accountDeletionService.DeleteAccount(r.Context(), kernel.Principal{ID: strconv.Itoa(userID), Username: usernameStr}, profile.Email)
	if err != nil {
		h.logger.Error("Failed to delete account: %v", err)
		if errors.Is(err, userdomain.ErrLastAdminDeletionForbidden) {
			httpx.Error(w, http.StatusForbidden, "LAST_ADMIN_DELETION_FORBIDDEN", "Cannot delete the last Administrator account. Please promote another user to Administrator first.", nil)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete account", nil)
		return
	}

	// Log the account deletion
	h.logger.Info("Account deleted successfully for user %d (%s)", userID, usernameStr)

	// Send response
	httpx.SendJSON(w, http.StatusOK, httpx.Response{
		Data: map[string]string{
			"message": "Account deleted successfully. All your data has been permanently removed.",
		},
		Meta: map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// UpdateCustomFields handles PUT /api/profile/custom-fields
func (h *ProfileHandler) UpdateCustomFields(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	var req struct {
		CustomFields map[string]any `json:"customFields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	// Admin users can update system fields; non-admin users cannot
	isAdmin := principal.HasRole("Admin")

	if err := h.profileService.UpdateCustomFields(r.Context(), principal, req.CustomFields, isAdmin); err != nil {
		h.logger.Error("Failed to update custom fields: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update profile fields", nil)
		return
	}

	updatedProfile, err := h.profileService.GetProfile(r.Context(), principal)
	if err != nil {
		h.logger.Error("Failed to fetch updated profile: %v", err)
		httpx.Success(w, map[string]any{
			"message": "Profile fields updated successfully",
		})
		return
	}

	profileInfo := ProfileInfo{
		ID:             updatedProfile.ID,
		Username:       updatedProfile.Username,
		Name:           updatedProfile.Name,
		Email:          updatedProfile.Email,
		Role:           updatedProfile.Role,
		ProfilePicture: h.buildProfilePictureURL(updatedProfile.ProfilePicture),
		CreatedAt:      updatedProfile.CreatedAt,
		UpdatedAt:      updatedProfile.UpdatedAt,
		CustomFields:   updatedProfile.CustomFields,
	}
	httpx.Success(w, map[string]any{
		"profile": profileInfo,
	})
}

// UpdateName handles PUT /api/profile/name
func (h *ProfileHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	if err := h.profileService.UpdateName(r.Context(), principal, req.Name); err != nil {
		h.logger.Error("Failed to update name: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update name", nil)
		return
	}

	updatedProfile, err := h.profileService.GetProfile(r.Context(), principal)
	if err != nil {
		h.logger.Error("Failed to fetch updated profile: %v", err)
		httpx.Success(w, map[string]any{
			"message": "Name updated successfully",
		})
		return
	}

	profileInfo := ProfileInfo{
		ID:             updatedProfile.ID,
		Username:       updatedProfile.Username,
		Name:           updatedProfile.Name,
		Email:          updatedProfile.Email,
		Role:           updatedProfile.Role,
		ProfilePicture: h.buildProfilePictureURL(updatedProfile.ProfilePicture),
		CreatedAt:      updatedProfile.CreatedAt,
		UpdatedAt:      updatedProfile.UpdatedAt,
		CustomFields:   updatedProfile.CustomFields,
	}
	httpx.Success(w, map[string]any{
		"profile": profileInfo,
	})
}

// GetUserFields handles GET /api/profile/user-fields
func (h *ProfileHandler) GetUserFields(w http.ResponseWriter, r *http.Request) {
	if h.userFieldsProvider == nil {
		httpx.Success(w, map[string]any{
			"fields":       []customfield.FieldSchema{},
			"systemFields": []customfield.FieldSchema{},
		})
		return
	}

	httpx.Success(w, map[string]any{
		"fields":       h.userFieldsProvider.GetUserFields(),
		"systemFields": h.userFieldsProvider.GetUserSystemFields(),
	})
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(
	profileService userdomain.ProfileServiceInterface,
	accountDeletionService userdomain.AccountDeletionServiceInterface,
	logger kernel.Logger,
	userFieldsProvider userFieldsProvider,
	profilePictureURLBuilder func(string) string,
) *ProfileHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	return &ProfileHandler{
		profileService:           profileService,
		accountDeletionService:   accountDeletionService,
		logger:                   logger,
		userFieldsProvider:       userFieldsProvider,
		profilePictureURLBuilder: profilePictureURLBuilder,
	}
}
