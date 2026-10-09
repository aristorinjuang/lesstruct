package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
)

// ProfilePictureHandler handles profile picture HTTP requests
type ProfilePictureHandler struct {
	service  *profilepicture.Service
	userRepo UserStore
	logger   kernel.Logger
}

// UploadProfilePicture handles PUT /api/profile/picture
func (h *ProfilePictureHandler) UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	if _, err := principalID(principal); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get username from context
	if principal.Username == "" {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(profilepicture.ProfilePictureMaxFileSize); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form", nil)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FILE", "Image file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	// Upload profile picture
	pictureURL, err := h.service.Upload(r.Context(), principal, file, header)
	if err != nil {
		h.logger.Error("Failed to upload profile picture: %v", err)
		httpx.Error(w, http.StatusBadRequest, "UPLOAD_FAILED", "Failed to upload profile picture", nil)
		return
	}

	httpx.Success(w, map[string]any{
		"profilePicture": pictureURL,
	})
}

// DeleteProfilePicture handles DELETE /api/profile/picture
func (h *ProfilePictureHandler) DeleteProfilePicture(w http.ResponseWriter, r *http.Request) {
	// Get identity from context
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Delete profile picture
	if err := h.service.Delete(r.Context(), principal); err != nil {
		h.logger.Error("Failed to delete profile picture: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile picture", nil)
		return
	}

	httpx.Success(w, map[string]string{
		"message": "Profile picture deleted successfully",
	})
}

// AdminUploadUserPicture handles PUT /api/admin/users/{id}/picture
func (h *ProfilePictureHandler) AdminUploadUserPicture(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	if userID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get target user to get username
	targetUser, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user: %v", err)
		httpx.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(profilepicture.ProfilePictureMaxFileSize); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form", nil)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FILE", "Image file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	// Upload profile picture
	pictureURL, err := h.service.Upload(r.Context(), kernel.Principal{ID: strconv.Itoa(userID), Username: targetUser.Username}, file, header)
	if err != nil {
		h.logger.Error("Failed to upload profile picture: %v", err)
		httpx.Error(w, http.StatusBadRequest, "UPLOAD_FAILED", "Failed to upload profile picture", nil)
		return
	}

	// Log admin action
	adminID := 0
	if admin, ok := kernel.PrincipalFromContext(r.Context()); ok {
		adminID, _ = principalID(admin)
	}
	h.logger.Info("Admin %d uploaded profile picture for user %d", adminID, userID)

	httpx.Success(w, map[string]any{
		"profilePicture": pictureURL,
		"userId":         fmt.Sprintf("%d", userID),
	})
}

// AdminDeleteUserPicture handles DELETE /api/admin/users/{id}/picture
func (h *ProfilePictureHandler) AdminDeleteUserPicture(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	if userID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Verify target user exists
	_, err = h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user: %v", err)
		httpx.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}

	// Delete profile picture
	if err := h.service.Delete(r.Context(), kernel.Principal{ID: strconv.Itoa(userID)}); err != nil {
		h.logger.Error("Failed to delete profile picture: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile picture", nil)
		return
	}

	// Log admin action
	adminID := 0
	if admin, ok := kernel.PrincipalFromContext(r.Context()); ok {
		adminID, _ = principalID(admin)
	}
	h.logger.Info("Admin %d deleted profile picture for user %d", adminID, userID)

	httpx.Success(w, map[string]string{
		"message": "Profile picture deleted successfully",
	})
}

// NewProfilePictureHandler creates a new profile picture handler
func NewProfilePictureHandler(
	service *profilepicture.Service,
	userRepo UserStore,
	logger kernel.Logger,
) *ProfilePictureHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	return &ProfilePictureHandler{
		service:  service,
		userRepo: userRepo,
		logger:   logger,
	}
}
