package modules_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubNotificationRepo struct {
	notifications []stubNotification
	err           error
}

type stubNotification struct {
	notificationType repository.NotificationType
	message          string
}

func (s *stubNotificationRepo) CreateNotification(_ context.Context, notificationType repository.NotificationType, message string) error {
	if s.err != nil {
		return s.err
	}
	s.notifications = append(s.notifications, stubNotification{
		notificationType: notificationType,
		message:          message,
	})
	return nil
}

func (s *stubNotificationRepo) GetNotifications(_ context.Context, _ int) ([]repository.Notification, error) {
	return nil, nil
}

func TestAuthNotificationRecorder(t *testing.T) {
	tests := []struct {
		name         string
		record       func(recorder *modules.AuthNotificationRecorder) error
		wantType     repository.NotificationType
		wantMessage  string
		wantErr      bool
	}{
		{
			name: "success - user registered records typed escaped notification",
			record: func(recorder *modules.AuthNotificationRecorder) error {
				return recorder.RecordUserRegistered(context.Background(), "newuser", "new@example.com")
			},
			wantType:    repository.NotificationTypeUserRegistered,
			wantMessage: "New user registration: newuser (new@example.com)",
			wantErr:     false,
		},
		{
			name: "success - user verified records typed escaped notification",
			record: func(recorder *modules.AuthNotificationRecorder) error {
				return recorder.RecordUserVerified(context.Background(), "newuser", "new@example.com")
			},
			wantType:    repository.NotificationTypeUserVerified,
			wantMessage: "User newuser verified their email (new@example.com)",
			wantErr:     false,
		},
		{
			name: "success - usernames are HTML-escaped",
			record: func(recorder *modules.AuthNotificationRecorder) error {
				return recorder.RecordUserRegistered(context.Background(), "<img>", "x@y.z")
			},
			wantType:    repository.NotificationTypeUserRegistered,
			wantMessage: "New user registration: &lt;img&gt; (x@y.z)",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubNotificationRepo{}
			recorder := modules.NewAuthNotificationRecorder(repo)

			err := tt.record(recorder)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, repo.notifications, 1)
			assert.Equal(t, tt.wantType, repo.notifications[0].notificationType)
			assert.Equal(t, tt.wantMessage, repo.notifications[0].message)
		})
	}
}
