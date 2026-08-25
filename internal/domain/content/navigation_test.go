package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_GetPublishedPages(t *testing.T) {
	t.Run("returns published pages from repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedPages", mock.Anything).Return([]*content.Content{
			{ID: 1, Title: "About", Slug: "about", PostType: "page"},
			{ID: 2, Title: "Contact", Slug: "contact", PostType: "page"},
		}, nil)

		service := content.NewService(mockRepo, nil, nil)
		pages, err := service.GetPublishedPages(context.Background())

		require.NoError(t, err)
		require.Len(t, pages, 2)
		assert.Equal(t, "About", pages[0].Title)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedPages", mock.Anything).Return(nil, errors.New("database error"))

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedPages(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get published pages")
		mockRepo.AssertExpectations(t)
	})
}

func TestService_GetPublishedCustomPostTypes(t *testing.T) {
	t.Run("returns custom post types from repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"menu-item", "team-member"}, nil)

		service := content.NewService(mockRepo, nil, nil)
		postTypes, err := service.GetPublishedCustomPostTypes(context.Background())

		require.NoError(t, err)
		assert.Len(t, postTypes, 2)
		assert.Contains(t, postTypes, "menu-item")
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedCustomPostTypes", mock.Anything).Return(nil, errors.New("database error"))

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedCustomPostTypes(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get published custom post types")
		mockRepo.AssertExpectations(t)
	})
}

func TestService_GetPublishedByPostType(t *testing.T) {
	t.Run("returns content filtered by post type", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedByPostType", mock.Anything, "menu-item", []string(nil), 0, 0, 50, 0).Return([]*content.Content{
			{ID: 1, Title: "Croissant", PostType: "menu-item"},
		}, nil)

		service := content.NewService(mockRepo, nil, nil)
		items, err := service.GetPublishedByPostType(context.Background(), "menu-item", nil, 0, 0, 50, 0)

		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "Croissant", items[0].Title)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedByPostType", mock.Anything, "menu-item", []string(nil), 0, 0, 50, 0).Return(nil, errors.New("database error"))

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedByPostType(context.Background(), "menu-item", nil, 0, 0, 50, 0)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get published content by post type")
		mockRepo.AssertExpectations(t)
	})
}

func TestService_GetPublishedTags(t *testing.T) {
	t.Run("returns distinct tags from repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedTags", mock.Anything).Return([]string{"golang", "tutorial"}, nil)

		service := content.NewService(mockRepo, nil, nil)
		tags, err := service.GetPublishedTags(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []string{"golang", "tutorial"}, tags)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns empty slice when no tags", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedTags", mock.Anything).Return([]string{}, nil)

		service := content.NewService(mockRepo, nil, nil)
		tags, err := service.GetPublishedTags(context.Background())

		require.NoError(t, err)
		assert.Empty(t, tags)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedTags", mock.Anything).Return(nil, errors.New("database error"))

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedTags(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get published tags")
		mockRepo.AssertExpectations(t)
	})
}

func TestService_GetPublishedAuthors(t *testing.T) {
	t.Run("returns authors ordered by the repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedAuthors", mock.Anything, content.PublishedAuthorFilters{Limit: 100, Offset: 0}).Return([]*content.PublishedAuthor{
			{Username: "jane", DisplayName: "Jane Doe", ContentCount: 5, PostTypes: []string{"article"}},
		}, nil)

		service := content.NewService(mockRepo, nil, nil)
		authors, err := service.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 100, Offset: 0})

		require.NoError(t, err)
		require.Len(t, authors, 1)
		assert.Equal(t, "jane", authors[0].Username)
		assert.Equal(t, 5, authors[0].ContentCount)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns empty slice when no published authors", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedAuthors", mock.Anything, content.PublishedAuthorFilters{Limit: 100, Offset: 0}).Return([]*content.PublishedAuthor{}, nil)

		service := content.NewService(mockRepo, nil, nil)
		authors, err := service.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 100, Offset: 0})

		require.NoError(t, err)
		assert.Empty(t, authors)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("GetPublishedAuthors", mock.Anything, content.PublishedAuthorFilters{Limit: 100, Offset: 0}).Return(nil, errors.New("database error"))

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 100, Offset: 0})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get published authors")
		mockRepo.AssertExpectations(t)
	})

	t.Run("rejects invalid sort order before reaching the repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit:     100,
			Offset:    0,
			SortBy:    "points",
			SortOrder: "sideways",
		})

		require.ErrorIs(t, err, content.ErrInvalidSortOrder)
		mockRepo.AssertNotCalled(t, "GetPublishedAuthors")
	})

	t.Run("rejects invalid custom field filter before reaching the repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit: 100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "points", Operator: content.FilterOperator("bogus"), Value: "5"},
			},
		})

		require.ErrorIs(t, err, content.ErrInvalidFilterOperator)
		mockRepo.AssertNotCalled(t, "GetPublishedAuthors")
	})
}
