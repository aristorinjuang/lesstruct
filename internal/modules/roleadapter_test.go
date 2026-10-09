package modules_test

import (
	"context"
	"errors"
	"testing"
	"time"

	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	seoapi "github.com/aristorinjuang/lesstruct/modules/seo/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewRoleCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		service *role.Service
		wantNil bool
	}{
		{
			name:    "success - wraps registry",
			service: role.NewService(),
			wantNil: false,
		},
		{
			name:    "success - nil service yields nil",
			service: nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := modules.NewRoleCapabilities(tt.service)
			if tt.wantNil {
				assert.Nil(t, adapter)
				return
			}
			require.NotNil(t, adapter)
			assert.True(t, adapter.IsAdmin("Admin"))
			assert.False(t, adapter.IsAdmin("Contributor"))
			assert.True(t, adapter.CanPublish("Contributor"))
			assert.True(t, adapter.CanMedia("Commentator"))
			assert.True(t, adapter.CanComment("Commentator"))
			assert.NotEmpty(t, adapter.ManageablePostTypes("Admin", []string{"post"}))
			names := make([]string, 0)
			for _, role := range adapter.GetAll() {
				names = append(names, role.Name)
			}
			assert.Contains(t, names, "Admin")
		})
	}
}

func TestNewCommentRoleCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		service *role.Service
		wantNil bool
	}{
		{
			name:    "success - wraps registry",
			service: role.NewService(),
			wantNil: false,
		},
		{
			name:    "success - nil service yields nil",
			service: nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := modules.NewCommentRoleCapabilities(tt.service)
			if tt.wantNil {
				assert.Nil(t, adapter)
				return
			}
			require.NotNil(t, adapter)
			assert.True(t, adapter.CanComment("Commentator"))
		})
	}
}

func TestNewAgentCommentRoleCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		service *role.Service
		wantNil bool
	}{
		{
			name:    "success - wraps registry",
			service: role.NewService(),
			wantNil: false,
		},
		{
			name:    "success - nil service yields nil",
			service: nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := modules.NewAgentCommentRoleCapabilities(tt.service)
			if tt.wantNil {
				assert.Nil(t, adapter)
				return
			}
			require.NotNil(t, adapter)
			assert.True(t, adapter.CanComment("Commentator"))
		})
	}
}

func TestNewSEOPublishedPages(t *testing.T) {
	groupID := 3
	tests := []struct {
		name      string
		setupRepo func(*contentmocks.MockRepository)
		wantPages []seoapi.PageRef
		wantErr   bool
	}{
		{
			name: "success - projects sitemap facts",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublished", mock.Anything, 1000, 0).Return([]*contentdomain.Content{
					{
						ID:                 1,
						Slug:               "hello",
						PostType:           "post",
						Language:           "en",
						TranslationGroupID: &groupID,
						UpdatedAt:          time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
					},
				}, nil)
			},
			wantPages: []seoapi.PageRef{
				{
					ID:                 1,
					Slug:               "hello",
					PostType:           "post",
					Language:           "en",
					TranslationGroupID: &groupID,
					UpdatedAt:          time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
				},
			},
			wantErr: false,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublished", mock.Anything, 1000, 0).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			pages, err := modules.NewSEOPublishedPages(content)(context.Background(), 1000, 0)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, "database error")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPages, pages)
		})
	}
}
