package dashboard_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	"github.com/aristorinjuang/lesstruct/internal/domain/dashboard/mocks"
	"github.com/aristorinjuang/lesstruct/internal/identity"
	"github.com/aristorinjuang/lesstruct/kernel"
)

func TestService_GetStats(t *testing.T) {
	tests := []struct {
		name    string
		userID  int
		mockFn  func(*mocks.MockRepository)
		want    *dashboard.Stats
		wantErr bool
		errMsg  string
	}{
		{
			name:   "successful stats retrieval",
			userID: 1,
			mockFn: func(m *mocks.MockRepository) {
				expectedStats := &dashboard.Stats{
					PublishedPosts:       10,
					DraftPosts:           5,
					RegisteredUsers:      3,
					PendingRegistrations: 2,
					MediaItems:           25,
					RecentContent: []*dashboard.RecentItem{
						{
							ID:        15,
							Title:     "Latest Post",
							Slug:      "latest-post",
							Status:    "published",
							CreatedAt: time.Date(2026, 4, 10, 10, 30, 0, 0, time.UTC),
						},
					},
				}
				m.On("GetStats", mock.Anything, 1).Return(expectedStats, nil)
			},
			want: &dashboard.Stats{
				PublishedPosts:       10,
				DraftPosts:           5,
				RegisteredUsers:      3,
				PendingRegistrations: 2,
				MediaItems:           25,
				RecentContent: []*dashboard.RecentItem{
					{
						ID:        15,
						Title:     "Latest Post",
						Slug:      "latest-post",
						Status:    "published",
						CreatedAt: time.Date(2026, 4, 10, 10, 30, 0, 0, time.UTC),
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "empty stats",
			userID: 1,
			mockFn: func(m *mocks.MockRepository) {
				expectedStats := &dashboard.Stats{
					PublishedPosts:       0,
					DraftPosts:           0,
					RegisteredUsers:      1,
					PendingRegistrations: 0,
					MediaItems:           0,
					RecentContent:        []*dashboard.RecentItem{},
				}
				m.On("GetStats", mock.Anything, 1).Return(expectedStats, nil)
			},
			want: &dashboard.Stats{
				PublishedPosts:       0,
				DraftPosts:           0,
				RegisteredUsers:      1,
				PendingRegistrations: 0,
				MediaItems:           0,
				RecentContent:        []*dashboard.RecentItem{},
			},
			wantErr: false,
		},
		{
			name:   "repository error",
			userID: 1,
			mockFn: func(m *mocks.MockRepository) {
				m.On("GetStats", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "failed to get dashboard stats: database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockRepository)
			tt.mockFn(mockRepo)

			service := dashboard.NewService(mockRepo)
			got, err := service.GetStats(context.Background(), identity.New(tt.userID, "testuser", ""))

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestNewService(t *testing.T) {
	mockRepo := new(mocks.MockRepository)
	service := dashboard.NewService(mockRepo)

	assert.NotNil(t, service)
}

func TestService_InvalidPrincipal(t *testing.T) {
	service := dashboard.NewService(nil)
	bad := kernel.Principal{ID: "not-a-number", Username: "ghost"}

	tests := []struct {
		name    string
		call    func() error
		wantErr bool
	}{
		{
			name: "get stats rejects non-decimal id",
			call: func() error {
				_, err := service.GetStats(context.Background(), bad)
				return err
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, identity.ErrInvalidUserID)
				return
			}
			assert.NoError(t, err)
		})
	}
}
