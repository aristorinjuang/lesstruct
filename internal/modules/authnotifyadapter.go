package modules

import (
	"context"
	"fmt"
	"html"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	authapi "github.com/aristorinjuang/lesstruct/modules/auth/api"
)

// AuthNotificationRecorder adapts the notification repository to the auth
// module's NotificationRecorder seam. Message formatting (HTML-escaped,
// matching the pre-module handler text byte-for-byte) lives here so the
// module never imports repository notification types. It lives in core —
// the only layer that may import both sides.
type AuthNotificationRecorder struct {
	repo repository.NotificationRepo
}

// RecordUserRegistered records a new-registration admin notification.
func (r *AuthNotificationRecorder) RecordUserRegistered(ctx context.Context, username, email string) error {
	return r.repo.CreateNotification(ctx, repository.NotificationTypeUserRegistered,
		fmt.Sprintf("New user registration: %s (%s)", html.EscapeString(username), html.EscapeString(email)))
}

// RecordUserVerified records an email-verified admin notification.
func (r *AuthNotificationRecorder) RecordUserVerified(ctx context.Context, username, email string) error {
	return r.repo.CreateNotification(ctx, repository.NotificationTypeUserVerified,
		fmt.Sprintf("User %s verified their email (%s)", html.EscapeString(username), html.EscapeString(email)))
}

// NewAuthNotificationRecorder creates an AuthNotificationRecorder.
func NewAuthNotificationRecorder(repo repository.NotificationRepo) *AuthNotificationRecorder {
	return &AuthNotificationRecorder{
		repo: repo,
	}
}

var _ authapi.NotificationRecorder = (*AuthNotificationRecorder)(nil)
