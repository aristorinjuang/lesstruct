package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_DeleteContent(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		userID  int
		setup   func(*mocks.MockRepository)
		wantErr error
	}{
		{
			name:   "successful deletion",
			id:     1,
			userID: 1,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:     1,
					UserID: 1,
					Title:  "Test Content",
				}, nil)
				m.On("Delete", mock.Anything, 1, 1).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:   "content not found",
			id:     999,
			userID: 1,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return((*content.Content)(nil), content.ErrContentNotFound)
			},
			wantErr: content.ErrContentNotFound,
		},
		{
			name:   "unauthorized - wrong owner",
			id:     1,
			userID: 2,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:     1,
					UserID: 1,
					Title:  "Test Content",
				}, nil)
			},
			wantErr: content.ErrUnauthorized,
		},
		{
			name:   "repository delete fails",
			id:     1,
			userID: 1,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:     1,
					UserID: 1,
					Title:  "Test Content",
				}, nil)
				m.On("Delete", mock.Anything, 1, 1).Return(fmt.Errorf("database error"))
			},
			wantErr: fmt.Errorf("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			err := service.DeleteContent(context.Background(), tt.id, tt.userID, "")

			if tt.wantErr != nil {
				require.Error(t, err)
				t.Logf("Error: %v", err)
			} else {
				require.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

var errPluginVeto = errors.New("plugin veto")

func TestService_DeleteContent_BeforeDeleteHook(t *testing.T) {
	tests := []struct {
		name      string
		id        int
		userID    int
		role      string
		setup     func(*mocks.MockRepository)
		transform func(plugin.HookName, []byte) ([]byte, error)
		wantErr   bool
	}{
		{
			name:   "hook fires with full payload before owner delete",
			id:     1,
			userID: 1,
			role:   "",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					UserID:   1,
					Title:    "Test Content",
					Content:  testTipTapJSON("Body"),
					Tags:     []string{"tag1"},
					Status:   content.StatusPublished,
					PostType: "article",
					CustomFields: map[string]any{
						"point": 42.0,
					},
				}, nil)
				m.On("Delete", mock.Anything, 1, 1).Return(nil)
			},
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				assert.Equal(t, plugin.HookBeforeDelete, hookName)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(data, &payload))
				assert.Equal(t, float64(1), payload["contentId"])
				assert.Equal(t, float64(1), payload["userId"], "userId must be the content author")
				assert.Equal(t, "Test Content", payload["title"])
				assert.Equal(t, "published", payload["status"])
				assert.Equal(t, "article", payload["postType"])
				cf, ok := payload["customFields"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, 42.0, cf["point"], "plugin-managed system fields included")
				return nil, nil
			},
			wantErr: false,
		},
		{
			name:   "hook fires before admin delete via DeleteByID",
			id:     1,
			userID: 1,
			role:   content.RoleAdmin,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					UserID:   7,
					Title:    "Test Content",
					PostType: "article",
				}, nil)
				m.On("DeleteByID", mock.Anything, 1).Return(nil)
			},
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				assert.Equal(t, plugin.HookBeforeDelete, hookName)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(data, &payload))
				assert.Equal(t, float64(1), payload["contentId"])
				assert.Equal(t, float64(7), payload["userId"], "userId must be the author, not the deleting admin")
				return nil, nil
			},
			wantErr: false,
		},
		{
			name:   "hook error aborts the delete",
			id:     1,
			userID: 1,
			role:   "",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					UserID:   1,
					Title:    "Test Content",
					PostType: "article",
				}, nil)
			},
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				return nil, errPluginVeto
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			mockHook := &mockHookExecutor{
				transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
					if hookName == plugin.HookBeforeDelete {
						return tt.transform(hookName, data)
					}
					return nil, nil
				},
			}

			service := content.NewServiceWithHooks(mockRepo, nil, nil, nil, mockHook)
			err := service.DeleteContent(context.Background(), tt.id, tt.userID, tt.role)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, "before_delete hook failed")
				assert.ErrorIs(t, err, errPluginVeto)
				return
			}
			require.NoError(t, err)

			mockRepo.AssertExpectations(t)
		})
	}
}
