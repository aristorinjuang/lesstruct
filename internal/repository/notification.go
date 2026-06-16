package repository

import (
	"context"
	"sync"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeUserRegistered NotificationType = "user_registered"
	NotificationTypeUserVerified   NotificationType = "user_verified"
)

// Notification represents an admin notification
type Notification struct {
	ID        int
	Type      NotificationType
	Message   string
	CreatedAt time.Time
}

// NotificationRepo defines the interface for notification operations
type NotificationRepo interface {
	CreateNotification(ctx context.Context, notificationType NotificationType, message string) error
	GetNotifications(ctx context.Context, limit int) ([]Notification, error)
}

// InMemoryNotificationRepository stores notifications in memory for MVP
type InMemoryNotificationRepository struct {
	mu            sync.RWMutex
	notifications []Notification
	nextID        int
}

// CreateNotification creates a new notification
func (r *InMemoryNotificationRepository) CreateNotification(ctx context.Context, notificationType NotificationType, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	notification := Notification{
		ID:        r.nextID,
		Type:      notificationType,
		Message:   message,
		CreatedAt: time.Now(),
	}

	r.notifications = append(r.notifications, notification)
	r.nextID++

	return nil
}

// GetNotifications retrieves notifications in reverse chronological order
func (r *InMemoryNotificationRepository) GetNotifications(ctx context.Context, limit int) ([]Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return notifications in reverse order (newest first)
	result := make([]Notification, 0, len(r.notifications))
	for i := len(r.notifications) - 1; i >= 0; i-- {
		result = append(result, r.notifications[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

// NewInMemoryNotificationRepository creates a new in-memory notification repository
func NewInMemoryNotificationRepository() *InMemoryNotificationRepository {
	return &InMemoryNotificationRepository{
		notifications: make([]Notification, 0),
		nextID:        1,
	}
}
