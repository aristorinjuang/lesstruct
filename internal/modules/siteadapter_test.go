package modules_test

import (
	"context"
	"errors"
	"testing"
	"time"

	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	commentmocks "github.com/aristorinjuang/lesstruct/modules/comments/domain/mocks"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSiteSSRContentReader(t *testing.T) {
	groupID := 3
	updatedAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		setupRepos  func(*contentmocks.MockRepository, *commentmocks.MockCommentRepository)
		call        func(sitessr.ContentReader) (any, error)
		want        any
		wantErr     bool
		wantErrText string
	}{
		{
			name: "success - projects pages with translation groups",
			setupRepos: func(repo *contentmocks.MockRepository, comments *commentmocks.MockCommentRepository) {
				repo.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{
					{
						ID:                 1,
						Slug:               "hello",
						Title:              "Hello",
						PostType:           "page",
						Language:           "en",
						TranslationGroupID: &groupID,
						Author:             "Author",
						CreatedAt:          updatedAt,
						AllowComments:      true,
						CustomFields:       map[string]any{"k": "v"},
					},
				}, nil)
			},
			call: func(reader sitessr.ContentReader) (any, error) {
				return reader.GetPublishedPages(context.Background())
			},
			want: []sitessr.ContentRef{
				{
					ID:                 1,
					Slug:               "hello",
					Title:              "Hello",
					PostType:           "page",
					Language:           "en",
					TranslationGroupID: &groupID,
					Author:             "Author",
					CreatedAt:          updatedAt,
					AllowComments:      true,
					CustomFields:       map[string]any{"k": "v"},
				},
			},
			wantErr: false,
		},
		{
			name: "success - delegates comment reads to the comments service",
			setupRepos: func(repo *contentmocks.MockRepository, comments *commentmocks.MockCommentRepository) {
				comments.On("GetByContentID", mock.Anything, 1).Return([]*commentdomain.Comment{
					{ID: 7, ContentID: 1, Comment: "Nice"},
				}, nil)
			},
			call: func(reader sitessr.ContentReader) (any, error) {
				return reader.GetCommentsForContent(context.Background(), 1)
			},
			want: []*commentdomain.Comment{
				{ID: 7, ContentID: 1, Comment: "Nice"},
			},
			wantErr: false,
		},
		{
			name: "error - content failure propagates",
			setupRepos: func(repo *contentmocks.MockRepository, comments *commentmocks.MockCommentRepository) {
				repo.On("GetPublishedPages", mock.Anything).Return(nil, errors.New("database error"))
			},
			call: func(reader sitessr.ContentReader) (any, error) {
				return reader.GetPublishedPages(context.Background())
			},
			wantErr:     true,
			wantErrText: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			commentRepo := commentmocks.NewMockCommentRepository(t)
			tt.setupRepos(repo, commentRepo)

			content := contentservice.NewService(repo, nil, nil)
			commentsService := commentdomain.NewService(commentRepo)
			reader := modules.NewSiteSSRContentReader(content, commentsService)

			got, err := tt.call(reader)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
