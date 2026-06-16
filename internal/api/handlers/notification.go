package handlers

import (
	"net/http"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// NotificationRef represents a notification in the response
type NotificationRef struct {
	ID        int                         `json:"id"`
	Type      repository.NotificationType `json:"type"`
	Message   string                      `json:"message"`
	CreatedAt string                      `json:"createdAt"`
}

// NotificationsResponse represents the notifications response
type NotificationsResponse struct {
	Notifications []NotificationRef `json:"notifications"`
}

// NotificationHandler handles notification HTTP requests
type NotificationHandler struct {
	logger           *util.Logger
	notificationRepo repository.NotificationRepo
}

// GetNotifications handles GET /api/notifications
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// TODO: Add authentication middleware to ensure only admins can access
	// For now, this endpoint is public but should be protected in production

	notifications, err := h.notificationRepo.GetNotifications(r.Context(), 100)
	if err != nil {
		h.logger.Error("Failed to get notifications: %v", err)
		response.Error(w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to retrieve notifications", nil)
		return
	}

	// Convert to response format
	notificationRefs := make([]NotificationRef, len(notifications))
	for i, notif := range notifications {
		notificationRefs[i] = NotificationRef{
			ID:        notif.ID,
			Type:      notif.Type,
			Message:   notif.Message,
			CreatedAt: notif.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	response.Success(w, NotificationsResponse{
		Notifications: notificationRefs,
	})
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(
	logger *util.Logger,
	notificationRepo repository.NotificationRepo,
) *NotificationHandler {
	return &NotificationHandler{
		logger:           logger,
		notificationRepo: notificationRepo,
	}
}
