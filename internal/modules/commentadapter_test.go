package modules_test

import (
	"context"
	"errors"
	"testing"

	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	commentapi "github.com/aristorinjuang/lesstruct/modules/comments/api"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewBrowserContentResolver(t *testing.T) {
	tests := []struct {
		name         string
		slug         string
		setupRepo    func(*contentmocks.MockRepository)
		wantRef      commentapi.ContentRef
		wantErr      error
		wantContains string
	}{
		{
			name: "success - projects id and gate flag",
			slug: "hello",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "hello", "en").Return(&contentdomain.Content{
					ID:            5,
					AllowComments: true,
				}, nil)
			},
			wantRef: commentapi.ContentRef{ID: 5, AllowComments: true},
			wantErr: nil,
		},
		{
			name: "error - missing content translates to the seam sentinel",
			slug: "gone",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "gone", "en").Return(nil, contentdomain.ErrContentNotFound)
			},
			wantErr: commentapi.ErrContentNotFound,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			slug: "boom",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "boom", "en").Return(nil, errors.New("database error"))
			},
			wantContains: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			ref, err := modules.NewBrowserContentResolver(content)(context.Background(), tt.slug, "en")

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRef, ref)
		})
	}
}

func TestNewAgentContentLookup(t *testing.T) {
	tests := []struct {
		name         string
		id           int
		setupRepo    func(*contentmocks.MockRepository)
		want         commentagent.ContentDetail
		wantErr      error
		wantContains string
	}{
		{
			name: "success - projects visibility and gate facts",
			id:   5,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 5).Return(&contentdomain.Content{
					ID:            5,
					UserID:        7,
					Status:        contentdomain.StatusPublished,
					AllowComments: true,
				}, nil)
			},
			want: commentagent.ContentDetail{ID: 5, UserID: 7, Published: true, AllowComments: true},
		},
		{
			name: "success - draft projects unpublished",
			id:   6,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 6).Return(&contentdomain.Content{
					ID:            6,
					UserID:        7,
					Status:        contentdomain.StatusDraft,
					AllowComments: true,
				}, nil)
			},
			want: commentagent.ContentDetail{ID: 6, UserID: 7, Published: false, AllowComments: true},
		},
		{
			name: "error - missing content translates to the seam sentinel",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantErr: commentagent.ErrContentNotFound,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, errors.New("database error"))
			},
			wantContains: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			detail, err := modules.NewAgentContentLookup(content)(context.Background(), tt.id)

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.ID, detail.ID)
			assert.Equal(t, tt.want.UserID, detail.UserID)
			assert.Equal(t, tt.want.Published, detail.Published)
			assert.Equal(t, tt.want.AllowComments, detail.AllowComments)
		})
	}
}

func TestNewAllowCommentsChecker(t *testing.T) {
	tests := []struct {
		name         string
		id           int
		setupRepo    func(*contentmocks.MockRepository)
		wantAllow    bool
		wantErr      error
		wantContains string
	}{
		{
			name: "success - allowed content passes the gate",
			id:   5,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 5).Return(&contentdomain.Content{
					ID:            5,
					AllowComments: true,
				}, nil)
			},
			wantAllow: true,
			wantErr:   nil,
		},
		{
			name: "success - disallowed content fails the gate",
			id:   5,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 5).Return(&contentdomain.Content{
					ID:            5,
					AllowComments: false,
				}, nil)
			},
			wantAllow: false,
			wantErr:   nil,
		},
		{
			name: "error - missing content translates to the domain sentinel",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantAllow: false,
			wantErr:   commentdomain.ErrContentNotFound,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, errors.New("database error"))
			},
			wantAllow:    false,
			wantContains: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			allow, err := modules.NewAllowCommentsChecker(content)(context.Background(), tt.id)

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAllow, allow)
		})
	}
}
