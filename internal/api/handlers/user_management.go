package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// GetPendingUsersResponse represents the response for pending users
type GetPendingUsersResponse struct {
	Data []*PendingUser `json:"data"`
	Meta *ResponseMeta  `json:"meta"`
}

// ResponseMeta represents metadata in API responses
type ResponseMeta struct {
	Count     int    `json:"count"`
	Timestamp string `json:"timestamp"`
}

// PendingUser represents a pending user in the response
type PendingUser struct {
	ID             int    `json:"id"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	ProfilePicture string `json:"profilePicture,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

// ApproveUserResponse represents the response for approve user
type ApproveUserResponse struct {
	Message string      `json:"message"`
	User    *UserDetail `json:"user"`
}

// UserDetail represents user details in responses
type UserDetail struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	Email          string         `json:"email"`
	Role           string         `json:"role,omitempty"`
	Status         string         `json:"status"`
	ProfilePicture string         `json:"profilePicture,omitempty"`
	CustomFields   map[string]any `json:"customFields,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
}

// RejectUserResponse represents the response for reject user
type RejectUserResponse struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// MarkUserAsSpamResponse represents the response for mark as spam
type MarkUserAsSpamResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

// GetAllUsersResponse represents the response for get all users
type GetAllUsersResponse struct {
	Data []*UserInList `json:"data"`
	Meta *UsersMeta    `json:"meta"`
}

// UserInList represents a user in the list response
type UserInList struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	Email          string         `json:"email"`
	Status         string         `json:"status"`
	Role           string         `json:"role"`
	ProfilePicture string         `json:"profilePicture,omitempty"`
	CustomFields   map[string]any `json:"customFields,omitempty"`
	CreatedAt      string         `json:"createdAt"`
}

// UsersMeta represents metadata for users list
type UsersMeta struct {
	Count     int    `json:"count"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	Timestamp string `json:"timestamp"`
}

// SuspendUserResponse represents the response for suspend user
type SuspendUserResponse struct {
	Message string      `json:"message"`
	User    *UserDetail `json:"user"`
}

// UnsuspendUserResponse represents the response for unsuspend user
type UnsuspendUserResponse struct {
	Message string      `json:"message"`
	User    *UserDetail `json:"user"`
}

// SoftDeleteUserResponse represents the response for soft delete user
type SoftDeleteUserResponse struct {
	Message            string `json:"message"`
	User               *UserDetail `json:"user"`
	DeletedContentCount int     `json:"deletedContentCount"`
}

// SoftDeleteContentRequest represents the request for soft delete
type SoftDeleteContentRequest struct {
	Confirmed bool   `json:"confirmed"`
	Reason    string `json:"reason"`
}

// GetSoftDeletedContentResponse represents the response for get soft deleted content
type GetSoftDeletedContentResponse struct {
	Data []*DeletedContentItem `json:"data"`
	Meta *ResponseMeta         `json:"meta"`
}

// DeletedContentItem represents a soft deleted content item
type DeletedContentItem struct {
	ID         int    `json:"id"`
	ContentType string `json:"contentType"`
	ContentID  int    `json:"contentId"`
	DeletedAt  string `json:"deletedAt"`
	DeletedBy  string `json:"deletedBy"`
	Reason     string `json:"reason,omitempty"`
}

// RestoreContentResponse represents the response for restore content
type RestoreContentResponse struct {
	Message string             `json:"message"`
	Content *RestoredContent   `json:"content"`
}

// RestoredContent represents restored content details
type RestoredContent struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Username     string         `json:"username"`
	Name         string         `json:"name"`
	Email        string         `json:"email"`
	Role         string         `json:"role"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// CreateUserResponse represents the response for creating a user
type CreateUserResponse struct {
	Message  string      `json:"message"`
	User     *UserDetail `json:"user"`
	Password string      `json:"password"`
}

// UpdateUserProfileRequest represents the request body for updating a user profile
type UpdateUserProfileRequest struct {
	Name         string         `json:"name"`
	Email        string         `json:"email"`
	Role         string         `json:"role"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// UpdateUserProfileResponse represents the response for updating a user profile
type UpdateUserProfileResponse struct {
	Message string      `json:"message"`
	User    *UserDetail `json:"user"`
}

// UserManagementHandler handles user management HTTP requests
type UserManagementHandler struct {
	userService       *user.UserManagementService
	adminCreateService *user.AdminCreateUserService
	userRepo          repository.UserRepo
	softDeleteRepo    repository.SoftDeleteRepo
	jwtManager        *auth.JWTManager
	emailService      email.EmailService
	logger            *util.Logger
	profilePictureURLBuilder func(string) string
}

// getAdminIDFromRequest extracts admin ID from the request context set by auth middleware
func (h *UserManagementHandler) getAdminIDFromRequest(r *http.Request) int {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		return 0
	}
	adminID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0
	}
	return adminID
}

// GetPendingUsers handles GET /api/admin/pending-users
func (h *UserManagementHandler) GetPendingUsers(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters from query string
	// Default: limit=100, offset=0
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 100 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0 // Default offset
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Get pending users from service
	users, err := h.userService.GetPendingUsers(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to get pending users: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve pending users", nil)
		return
	}

	// Convert to response format
	pendingUsers := make([]*PendingUser, len(users))
	for i, u := range users {
		pendingUsers[i] = &PendingUser{
			ID:             u.ID,
			Username:       u.Username,
			Email:          u.Email,
			ProfilePicture: h.buildProfilePictureURL(u.ProfilePicture),
			CreatedAt:      u.CreatedAt,
		}
	}

	// Send response
	resp := GetPendingUsersResponse{
		Data: pendingUsers,
		Meta: &ResponseMeta{
			Count:     len(pendingUsers),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	response.Success(w, resp)
}

// ApproveUser handles POST /api/admin/users/:id/approve
func (h *UserManagementHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get user details before approval for email notification and response
	userDetails, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user details", nil)
		return
	}

	// Approve user
	if err := h.userService.ApproveUser(r.Context(), userID); err != nil {
		h.logger.Error("Failed to approve user: %v", err)
		if errors.Is(err, user.ErrInvalidStatus) {
			response.Error(w, http.StatusBadRequest, "INVALID_USER_STATUS", "User is not in pending status", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to approve user", nil)
		return
	}

	// Send approval email notification
	if err := h.emailService.SendUserApprovedEmail(r.Context(), userDetails.Email, userDetails.Username); err != nil {
		h.logger.Error("Failed to send approval email: %v", err)
		// Continue anyway - email failure shouldn't block approval
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d approved user %d", adminID, userID)

	// Send response using the pre-fetched user details with known updated status
	resp := ApproveUserResponse{
		Message: "User approved successfully",
		User: &UserDetail{
			ID:             userDetails.ID,
			Username:       userDetails.Username,
			Email:          userDetails.Email,
			Status:         "verified",
			ProfilePicture: h.buildProfilePictureURL(userDetails.ProfilePicture),
			CustomFields:   userDetails.CustomFields,
			CreatedAt:      userDetails.CreatedAt,
		},
	}
	response.Success(w, resp)
}

// RejectUser handles POST /api/admin/users/:id/reject
func (h *UserManagementHandler) RejectUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Parse request body for confirmation
	var req struct {
		Confirmed bool `json:"confirmed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty or invalid, require confirmation
		response.Error(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "Confirmation required for destructive action", nil)
		return
	}

	// Require confirmation for destructive action
	if !req.Confirmed {
		response.Error(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "Confirmation required for destructive action", nil)
		return
	}

	// Get user details before deletion for email notification
	userDetails, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}

	// Reject user
	if err := h.userService.RejectUser(r.Context(), userID); err != nil {
		h.logger.Error("Failed to reject user: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reject user", nil)
		return
	}

	// Send rejection email notification
	if err := h.emailService.SendUserRejectedEmail(r.Context(), userDetails.Email, userDetails.Username); err != nil {
		h.logger.Error("Failed to send rejection email: %v", err)
		// Continue anyway - email failure shouldn't block rejection
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d rejected user %d", adminID, userID)

	// Send response
	resp := RejectUserResponse{
		Message:  "User rejected successfully. Rejection email sent.",
		Username: userDetails.Username,
		Email:    userDetails.Email,
	}
	response.Success(w, resp)
}

// MarkUserAsSpam handles POST /api/admin/users/:id/mark-spam
func (h *UserManagementHandler) MarkUserAsSpam(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Parse request body for confirmation
	var req struct {
		Confirmed bool `json:"confirmed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty or invalid, require confirmation
		response.Error(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "Confirmation required for destructive action", nil)
		return
	}

	// Require confirmation for destructive action
	if !req.Confirmed {
		response.Error(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "Confirmation required for destructive action", nil)
		return
	}

	// Get user details before deletion
	userDetails, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}

	// Mark user as spam
	if err := h.userService.MarkUserAsSpam(r.Context(), userID); err != nil {
		h.logger.Error("Failed to mark user as spam: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		if errors.Is(err, user.ErrEmailAlreadyBlocked) {
			response.Error(w, http.StatusBadRequest, "EMAIL_ALREADY_BLOCKED", "This email is already blocked", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark user as spam", nil)
		return
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d marked user %d as spam", adminID, userID)

	// Send response
	resp := MarkUserAsSpamResponse{
		Message: "User marked as spam and email blocked",
		Email:   userDetails.Email,
	}
	response.Success(w, resp)
}

// GetAllUsers handles GET /api/admin/users
func (h *UserManagementHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Set default pagination values
	limit := 100
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			// Enforce max limit of 1000
			if parsedLimit > 1000 {
				limit = 1000
			} else {
				limit = parsedLimit
			}
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Validate status parameter if provided
	if status != "" {
		validStatuses := map[string]bool{
			"suspended":    true,
			"soft_deleted": true,
			"pending":      true,
			"verified":     true,
		}
		if !validStatuses[status] {
			response.Error(w, http.StatusBadRequest, "INVALID_STATUS", "Invalid status parameter", nil)
			return
		}
	}

	// Get users from service
	users, err := h.userService.GetAllUsers(r.Context(), status, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get users: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve users", nil)
		return
	}

	// Convert to response format
	usersList := make([]*UserInList, len(users))
	for i, u := range users {
		usersList[i] = &UserInList{
			ID:             u.ID,
			Username:       u.Username,
			Email:          u.Email,
			Status:         u.Status,
			Role:           u.Role,
			ProfilePicture: h.buildProfilePictureURL(u.ProfilePicture),
			CustomFields:   u.CustomFields,
			CreatedAt:      u.CreatedAt,
		}
	}

	// Send response
	resp := GetAllUsersResponse{
		Data: usersList,
		Meta: &UsersMeta{
			Count:     len(usersList),
			Limit:     limit,
			Offset:    offset,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	response.Success(w, resp)
}

// SuspendUser handles POST /api/admin/users/:id/suspend
func (h *UserManagementHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Parse request body for optional reason
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Get user details before suspension for email notification
	userDetails, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user details", nil)
		return
	}

	// Suspend user
	if err := h.userService.SuspendUser(r.Context(), userID); err != nil {
		h.logger.Error("Failed to suspend user: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		if errors.Is(err, user.ErrInvalidStatus) {
			response.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Cannot suspend user with current status", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to suspend user", nil)
		return
	}

	// Get updated user
	updatedUser, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get updated user: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve updated user", nil)
		return
	}

	// Send suspension email notification
	if err := h.emailService.SendUserSuspendedEmail(r.Context(), userDetails.Email, userDetails.Username, req.Reason); err != nil {
		h.logger.Error("Failed to send suspension email: %v", err)
		// Continue anyway - email failure shouldn't block suspension
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d suspended user %d", adminID, userID)

	// Send response
	resp := SuspendUserResponse{
		Message: "User suspended successfully",
		User: &UserDetail{
			ID:             updatedUser.ID,
			Username:       updatedUser.Username,
			Email:          updatedUser.Email,
			Status:         updatedUser.Status,
			ProfilePicture: h.buildProfilePictureURL(updatedUser.ProfilePicture),
			CustomFields:   updatedUser.CustomFields,
			CreatedAt:      updatedUser.CreatedAt,
		},
	}
	response.Success(w, resp)
}

// UnsuspendUser handles POST /api/admin/users/:id/unsuspend
func (h *UserManagementHandler) UnsuspendUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Parse request body for optional reason
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Get user details before unsuspension for email notification
	userDetails, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user details", nil)
		return
	}

	// Unsuspend user
	if err := h.userService.UnsuspendUser(r.Context(), userID); err != nil {
		h.logger.Error("Failed to unsuspend user: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		if errors.Is(err, user.ErrInvalidStatus) {
			response.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Cannot unsuspend user with current status", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to unsuspend user", nil)
		return
	}

	// Get updated user
	updatedUser, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get updated user: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve updated user", nil)
		return
	}

	// Send unsuspension email notification
	if err := h.emailService.SendUserUnsuspendedEmail(r.Context(), userDetails.Email, userDetails.Username); err != nil {
		h.logger.Error("Failed to send unsuspension email: %v", err)
		// Continue anyway - email failure shouldn't block unsuspension
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d unsuspended user %d", adminID, userID)

	// Send response
	resp := UnsuspendUserResponse{
		Message: "User unsuspended successfully",
		User: &UserDetail{
			ID:             updatedUser.ID,
			Username:       updatedUser.Username,
			Email:          updatedUser.Email,
			Status:         updatedUser.Status,
			ProfilePicture: h.buildProfilePictureURL(updatedUser.ProfilePicture),
			CustomFields:   updatedUser.CustomFields,
			CreatedAt:      updatedUser.CreatedAt,
		},
	}
	response.Success(w, resp)
}

// SoftDeleteUser handles POST /api/admin/users/:id/soft-delete
func (h *UserManagementHandler) SoftDeleteUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Parse request body for confirmation and reason
	var req SoftDeleteContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	// Require confirmation for destructive action
	if !req.Confirmed {
		response.Error(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "Confirmation required for soft delete action", nil)
		return
	}

	// Get user details before deletion for email notification
	userDetails, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user details", nil)
		return
	}

	// Soft delete user (this only updates status, content deletion is handled separately)
	if err := h.userService.SoftDeleteUser(r.Context(), userID); err != nil {
		h.logger.Error("Failed to soft delete user: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		if errors.Is(err, user.ErrInvalidStatus) {
			response.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Cannot soft delete user with current status", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to soft delete user", nil)
		return
	}

	// Mark all user content as deleted
	// For now, we'll mark dummy content - this will be expanded when content types are implemented
	// This is a placeholder for Story 2.x when content management is added
	deletedContentCount := 0

	// Get updated user
	updatedUser, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get updated user: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve updated user", nil)
		return
	}

	// Send soft delete email notification
	if err := h.emailService.SendUserSoftDeletedEmail(r.Context(), userDetails.Email, userDetails.Username, req.Reason); err != nil {
		h.logger.Error("Failed to send soft delete email: %v", err)
		// Continue anyway - email failure shouldn't block deletion
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d soft deleted user %d", adminID, userID)

	// Send response
	resp := SoftDeleteUserResponse{
		Message:             "User account and content soft deleted successfully",
		User: &UserDetail{
			ID:             updatedUser.ID,
			Username:       updatedUser.Username,
			Email:          updatedUser.Email,
			Status:         updatedUser.Status,
			ProfilePicture: h.buildProfilePictureURL(updatedUser.ProfilePicture),
			CustomFields:   updatedUser.CustomFields,
			CreatedAt:      updatedUser.CreatedAt,
		},
		DeletedContentCount: deletedContentCount,
	}
	response.Success(w, resp)
}

// GetSoftDeletedContent handles GET /api/admin/users/:id/deleted-content
func (h *UserManagementHandler) GetSoftDeletedContent(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Validate user ID is positive
	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Verify user exists
	_, err = h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details: %v", err)
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user details", nil)
		return
	}

	// Get soft deleted content for user
	deletedContent, err := h.softDeleteRepo.GetSoftDeletedContentByUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get soft deleted content: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve soft deleted content", nil)
		return
	}

	// Convert to response format
	contentList := make([]*DeletedContentItem, len(deletedContent))
	for i, content := range deletedContent {
		reason := ""
		if content.Reason.Valid {
			reason = content.Reason.String
		}

		// Get admin username who deleted the content
		adminUser, err := h.userRepo.GetUserByID(r.Context(), content.DeletedBy)
		adminUsername := "unknown"
		if err == nil && adminUser != nil {
			adminUsername = adminUser.Username
		}

		contentList[i] = &DeletedContentItem{
			ID:          content.ID,
			ContentType: content.ContentType,
			ContentID:   content.ContentID,
			DeletedAt:   content.DeletedAt,
			DeletedBy:   adminUsername,
			Reason:      reason,
		}
	}

	// Send response
	resp := GetSoftDeletedContentResponse{
		Data: contentList,
		Meta: &ResponseMeta{
			Count:     len(contentList),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	response.Success(w, resp)
}

// RestoreContent handles POST /api/admin/content/:id/restore
func (h *UserManagementHandler) RestoreContent(w http.ResponseWriter, r *http.Request) {
	// Extract soft deleted content ID from URL (this is the soft_deleted_content.id, not the actual content ID)
	softDeletedContentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_CONTENT_ID", "Invalid content ID", nil)
		return
	}

	// Validate content ID is positive
	if softDeletedContentID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_CONTENT_ID", "Invalid content ID", nil)
		return
	}

	// Get the soft deleted content record
	softDeletedContent, err := h.softDeleteRepo.GetSoftDeletedContentByID(r.Context(), softDeletedContentID)
	if err != nil {
		h.logger.Error("Failed to get soft deleted content: %v", err)
		response.Error(w, http.StatusNotFound, "CONTENT_NOT_FOUND", "Soft deleted content not found", nil)
		return
	}

	if softDeletedContent == nil {
		response.Error(w, http.StatusNotFound, "CONTENT_NOT_FOUND", "Soft deleted content not found", nil)
		return
	}

	// Restore the content
	if err := h.softDeleteRepo.RestoreContent(r.Context(), softDeletedContent.ContentType, softDeletedContent.ContentID); err != nil {
		h.logger.Error("Failed to restore content: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to restore content", nil)
		return
	}

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d restored content %d (type: %s)", adminID, softDeletedContent.ContentID, softDeletedContent.ContentType)

	// Send response
	resp := RestoreContentResponse{
		Message: "Content restored successfully",
		Content: &RestoredContent{
			ID:   softDeletedContent.ContentID,
			Type: softDeletedContent.ContentType,
		},
	}
	response.Success(w, resp)
}

// CreateUser handles POST /api/admin/users
func (h *UserManagementHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	if req.Username == "" || req.Email == "" || req.Role == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Username, email, and role are required", nil)
		return
	}

	result, err := h.adminCreateService.CreateUser(r.Context(), user.AdminCreateUserRequest{
		Username: req.Username,
		Name:     req.Name,
		Email:    req.Email,
		Role:         req.Role,
		CustomFields: req.CustomFields,
	})
	if err != nil {
		h.logger.Error("Failed to create user: %v", err)

		if errors.Is(err, user.ErrUsernameInvalid) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		if errors.Is(err, user.ErrEmailInvalid) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid email format", nil)
			return
		}
		if errors.Is(err, user.ErrInvalidRole) {
			response.Error(w, http.StatusBadRequest, "INVALID_ROLE", err.Error(), nil)
			return
		}
		if errors.Is(err, user.ErrUsernameExists) {
			response.Error(w, http.StatusConflict, "USERNAME_EXISTS", err.Error(), nil)
			return
		}
		if errors.Is(err, user.ErrEmailExists) {
			response.Error(w, http.StatusConflict, "EMAIL_EXISTS", err.Error(), nil)
			return
		}
		if errors.Is(err, user.ErrEmailBlocked) {
			response.Error(w, http.StatusForbidden, "EMAIL_BLOCKED", err.Error(), nil)
			return
		}

		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user", nil)
		return
	}

	// Send welcome email (fire and forget — don't block on email failure)
	go func() {
		_ = h.emailService.SendAccountCreatedEmail(
			context.Background(),
			result.User.Email,
			result.User.Username,
			result.PlainPassword,
		)
	}()

	// Log admin action
	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d created user %d", adminID, result.User.ID)

	resp := CreateUserResponse{
		Message: "User created successfully",
		User: &UserDetail{
			ID:             result.User.ID,
			Username:       result.User.Username,
			Email:          result.User.Email,
			Role:           result.User.Role,
			Status:         result.User.Status,
			ProfilePicture: h.buildProfilePictureURL(result.User.ProfilePicture),
			CustomFields:   result.User.CustomFields,
			CreatedAt:      result.User.CreatedAt,
		},
		Password: result.PlainPassword,
	}
	response.Success(w, resp)
}

// UpdateUser handles PUT /api/admin/users/:id
func (h *UserManagementHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	if userID <= 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	var req UpdateUserProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	updatedUser, err := h.userService.UpdateUserProfile(r.Context(), userID, req.Name, req.Email, req.Role, req.CustomFields)
	if err != nil {
		h.logger.Error("Failed to update user profile: %v", err)

		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		if errors.Is(err, user.ErrEmailInvalid) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid email format", nil)
			return
		}
		if errors.Is(err, user.ErrInvalidRole) {
			response.Error(w, http.StatusBadRequest, "INVALID_ROLE", err.Error(), nil)
			return
		}
		if errors.Is(err, user.ErrEmailExists) {
			response.Error(w, http.StatusConflict, "EMAIL_EXISTS", err.Error(), nil)
			return
		}

		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user profile", nil)
		return
	}

	adminID := h.getAdminIDFromRequest(r)
	h.logger.Info("Admin %d updated profile for user %d", adminID, userID)

	resp := UpdateUserProfileResponse{
		Message: "User profile updated successfully",
		User: &UserDetail{
			ID:             updatedUser.ID,
			Username:       updatedUser.Username,
			Email:          updatedUser.Email,
			Role:           updatedUser.Role,
			Status:         updatedUser.Status,
			ProfilePicture: h.buildProfilePictureURL(updatedUser.ProfilePicture),
			CustomFields:   updatedUser.CustomFields,
			CreatedAt:      updatedUser.CreatedAt,
		},
	}
	response.Success(w, resp)
}

// NewUserManagementHandler creates a new user management handler
func NewUserManagementHandler(
	userService *user.UserManagementService,
	adminCreateService *user.AdminCreateUserService,
	userRepo repository.UserRepo,
	softDeleteRepo repository.SoftDeleteRepo,
	jwtManager *auth.JWTManager,
	emailService email.EmailService,
	logger *util.Logger,
	profilePictureURLBuilder func(string) string,
) *UserManagementHandler {
	return &UserManagementHandler{
		userService:       userService,
		adminCreateService: adminCreateService,
		userRepo:          userRepo,
		softDeleteRepo:    softDeleteRepo,
		jwtManager:        jwtManager,
		emailService:      emailService,
		logger:            logger,
		profilePictureURLBuilder: profilePictureURLBuilder,
	}
}

// buildProfilePictureURL converts a filename to a full URL if a builder is set.
func (h *UserManagementHandler) buildProfilePictureURL(filename string) string {
	if h.profilePictureURLBuilder != nil && filename != "" {
		return h.profilePictureURLBuilder(filename)
	}
	return filename
}
