package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/profilepicture"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// ProfilePictureHandler handles profile picture HTTP requests
type ProfilePictureHandler struct {
	service  *profilepicture.Service
	userRepo repository.UserRepo
	logger   *util.Logger
}

// UploadProfilePicture handles PUT /api/profile/picture
func (h *ProfilePictureHandler) UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get username from context
	usernameStr, ok := middleware.GetUsername(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(profilepicture.ProfilePictureMaxFileSize); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form", nil)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "MISSING_FILE", "Image file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	// Upload profile picture
	pictureURL, err := h.service.Upload(r.Context(), userID, usernameStr, file, header)
	if err != nil {
		h.logger.Error("Failed to upload profile picture: %v", err)
		response.Error(w, http.StatusBadRequest, "UPLOAD_FAILED", "Failed to upload profile picture", nil)
		return
	}

	response.Success(w, map[string]any{
		"profilePicture": pictureURL,
	})
}

// DeleteProfilePicture handles DELETE /api/profile/picture
func (h *ProfilePictureHandler) DeleteProfilePicture(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Delete profile picture
	if err := h.service.Delete(r.Context(), userID); err != nil {
		h.logger.Error("Failed to delete profile picture: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile picture", nil)
		return
	}

	response.Success(w, map[string]string{
		"message": "Profile picture deleted successfully",
	})
}

// AdminUploadUserPicture handles PUT /api/admin/users/{id}/picture
func (h *ProfilePictureHandler) AdminUploadUserPicture(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get target user to get username
	targetUser, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user: %v", err)
		response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(profilepicture.ProfilePictureMaxFileSize); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form", nil)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "MISSING_FILE", "Image file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	// Upload profile picture
	pictureURL, err := h.service.Upload(r.Context(), userID, targetUser.Username, file, header)
	if err != nil {
		h.logger.Error("Failed to upload profile picture: %v", err)
		response.Error(w, http.StatusBadRequest, "UPLOAD_FAILED", "Failed to upload profile picture", nil)
		return
	}

	// Log admin action
	adminIDStr, _ := middleware.GetUserID(r)
	adminID, _ := strconv.Atoi(adminIDStr)
	h.logger.Info("Admin %d uploaded profile picture for user %d", adminID, userID)

	response.Success(w, map[string]any{
		"profilePicture": pictureURL,
		"userId":         fmt.Sprintf("%d", userID),
	})
}

// AdminDeleteUserPicture handles DELETE /api/admin/users/{id}/picture
func (h *ProfilePictureHandler) AdminDeleteUserPicture(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Verify target user exists
	_, err = h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user: %v", err)
		response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}

	// Delete profile picture
	if err := h.service.Delete(r.Context(), userID); err != nil {
		h.logger.Error("Failed to delete profile picture: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile picture", nil)
		return
	}

	// Log admin action
	adminIDStr, _ := middleware.GetUserID(r)
	adminID, _ := strconv.Atoi(adminIDStr)
	h.logger.Info("Admin %d deleted profile picture for user %d", adminID, userID)

	response.Success(w, map[string]string{
		"message": "Profile picture deleted successfully",
	})
}

// NewProfilePictureHandler creates a new profile picture handler
func NewProfilePictureHandler(
	service *profilepicture.Service,
	userRepo repository.UserRepo,
	logger *util.Logger,
) *ProfilePictureHandler {
	return &ProfilePictureHandler{
		service:  service,
		userRepo: userRepo,
		logger:   logger,
	}
}
