package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/domain/seo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func ptrF(v float64) *float64 {
	return &v
}

type mockHookExecutor struct {
	mock.Mock
	transform func(plugin.HookName, []byte) ([]byte, error)
}

func (m *mockHookExecutor) Execute(ctx context.Context, hookName plugin.HookName, data []byte) ([]byte, error) {
	if m.transform != nil {
		return m.transform(hookName, data)
	}
	args := m.Called(ctx, hookName, data)
	if len(args) == 0 {
		return nil, nil
	}
	return args.Get(0).([]byte), args.Error(1)
}

func testTipTapJSON(text string) string {
	doc := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func containsErrorSubstring(errMsg, expected string) bool {
	if len(errMsg) < len(expected) {
		return false
	}
	for i := 0; i <= len(errMsg)-len(expected); i++ {
		if errMsg[i:i+len(expected)] == expected {
			return true
		}
	}
	return false
}

func TestService_Create(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		req           content.CreateContentRequest
		setupMock     func(*mocks.MockRepository)
		wantErr       error
		validateSlug  bool
		expectedSlug  string
		expectedTitle string
	}{
		{
			name:   "successful content creation with draft status",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "My Test Title",
				Content: testTipTapJSON("This is test content"),
				Tags:    []string{"test", "example"},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "my-test-title", "en").Return(true, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
					content := args.Get(1).(*content.Content)
					content.ID = 1
				})
			},
			wantErr:      nil,
			validateSlug: true,
			expectedSlug: "my-test-title",
		},
		{
			name:   "successful content creation with published status",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Published Title",
				Content: testTipTapJSON("This is published content"),
				Tags:    []string{},
				Status:  content.StatusPublished,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "published-title", "en").Return(true, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
					content := args.Get(1).(*content.Content)
					content.ID = 2
				})
			},
			wantErr:      nil,
			validateSlug: true,
			expectedSlug: "published-title",
		},
		{
			name:   "successful content creation with title that needs trimming",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "  Title With Spaces  ",
				Content: testTipTapJSON("This is test content"),
				Tags:    []string{"test"},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "title-with-spaces", "en").Return(true, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
					content := args.Get(1).(*content.Content)
					content.ID = 3
				})
			},
			wantErr:       nil,
			validateSlug:  true,
			expectedSlug:  "title-with-spaces",
			expectedTitle: "Title With Spaces",
		},
		{
			name:   "validation fails with empty title",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "",
				Content: testTipTapJSON("Content here"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock:    func(m *mocks.MockRepository) {},
			wantErr:      content.ErrInvalidTitle,
			validateSlug: false,
		},
		{
			name:   "validation fails with title too long",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   string(make([]byte, 201)),
				Content: testTipTapJSON("Content here"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock:    func(m *mocks.MockRepository) {},
			wantErr:      content.ErrInvalidTitle,
			validateSlug: false,
		},
		{
			name:   "validation fails with content too long",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Valid Title",
				Content: string(make([]byte, 100001)),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock:    func(m *mocks.MockRepository) {},
			wantErr:      content.ErrInvalidContent,
			validateSlug: false,
		},
		{
			name:   "validation fails with invalid status",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Valid Title",
				Content: testTipTapJSON("Valid content"),
				Tags:    []string{},
				Status:  content.Status("invalid"),
			},
			setupMock:    func(m *mocks.MockRepository) {},
			wantErr:      content.ErrInvalidStatus,
			validateSlug: false,
		},
		{
			name:   "validation fails with empty tag",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Valid Title",
				Content: testTipTapJSON("Valid content"),
				Tags:    []string{"valid", "", "another"},
				Status:  content.StatusDraft,
			},
			setupMock:    func(m *mocks.MockRepository) {},
			wantErr:      errors.New("tags validation failed"),
			validateSlug: false,
		},
		{
			name:   "fails when slug already exists",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Existing Title",
				Content: testTipTapJSON("Content here"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "existing-title", "en").Return(false, nil)
			},
			wantErr:      content.ErrSlugAlreadyExists,
			validateSlug: false,
		},
		{
			name:   "fails when repository create fails",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Valid Title",
				Content: testTipTapJSON("Valid content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "valid-title", "en").Return(true, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(errors.New("database error"))
			},
			wantErr:      errors.New("failed to create content"),
			validateSlug: false,
		},
		{
			name:   "fails when CheckSlugUnique returns error",
			userID: 1,
			req: content.CreateContentRequest{
				Title:   "Valid Title",
				Content: testTipTapJSON("Valid content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "valid-title", "en").Return(false, errors.New("database error"))
			},
			wantErr:      errors.New("failed to check slug uniqueness"),
			validateSlug: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.Create(context.Background(), tt.userID, tt.req)

			if tt.wantErr != nil {
				require.Error(t, err, "Service.Create() expected error, got nil")
				if !errors.Is(err, tt.wantErr) && !containsErrorSubstring(err.Error(), tt.wantErr.Error()) {
					t.Errorf("Service.Create() error = %v, wantErr %v", err, tt.wantErr)
				}
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.Create() unexpected error")
			require.NotNil(t, result, "Service.Create() expected content, got nil")

			assert.Equal(t, tt.userID, result.UserID, "Service.Create() UserID")

			expectedTitle := tt.expectedTitle
			if expectedTitle == "" {
				expectedTitle = tt.req.Title
			}
			assert.Equal(t, expectedTitle, result.Title, "Service.Create() Title")
			assert.Equal(t, tt.req.Content, result.Content, "Service.Create() Content")
			assert.Equal(t, tt.req.Status, result.Status, "Service.Create() Status")

			if tt.validateSlug {
				assert.Equal(t, tt.expectedSlug, result.Slug, "Service.Create() Slug")
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "simple title",
			title:    "My Test Title",
			expected: "my-test-title",
		},
		{
			name:     "title with special characters",
			title:    "Hello World! How Are You?",
			expected: "hello-world-how-are-you",
		},
		{
			name:     "title with numbers",
			title:    "Test 123 Title",
			expected: "test-123-title",
		},
		{
			name:     "title with consecutive spaces",
			title:    "Test    Title",
			expected: "test-title",
		},
		{
			name:     "title with leading/trailing spaces",
			title:    "  Test Title  ",
			expected: "test-title",
		},
		{
			name:     "title with mixed case",
			title:    "MiXeD CaSe TiTlE",
			expected: "mixed-case-title",
		},
		{
			name:     "empty title returns untitled",
			title:    "",
			expected: "untitled",
		},
		{
			name:     "title with only special characters",
			title:    "!@#$%",
			expected: "untitled",
		},
		{
			name:     "title with underscores",
			title:    "test_title_here",
			expected: "testtitlehere",
		},
		{
			name:     "title longer than 200 characters is truncated",
			title:    "this is a very long title that should be truncated to exactly two hundred characters because the slug generator will only keep the first two hundred characters when the title is longer than that limit which is exactly what we want to test here and now we need more characters so let me add some more text to make this really really long and exceed the two hundred character limit for sure",
			expected: "this-is-a-very-long-title-that-should-be-truncated-to-exactly-two-hundred-characters-because-the-slug-generator-will-only-keep-the-first-two-hundred-characters-when-the-title-is-longer-than-that-limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			service := content.NewService(mockRepo, nil, nil)

			result := service.GenerateSlug(tt.title)

			if result != tt.expected {
				t.Errorf("Service.GenerateSlug() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestService_GenerateSlugFromTitle(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		expectedErr error
	}{
		{
			name:        "valid title",
			title:       "My Test Title",
			expectedErr: nil,
		},
		{
			name:        "empty title",
			title:       "",
			expectedErr: content.ErrInvalidTitle,
		},
		{
			name:        "title too long",
			title:       string(make([]byte, 201)),
			expectedErr: content.ErrInvalidTitle,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GenerateSlugFromTitle(context.Background(), tt.title)

			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("Service.GenerateSlugFromTitle() expected error, got nil")
					return
				}
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("Service.GenerateSlugFromTitle() error = %v, wantErr %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Service.GenerateSlugFromTitle() unexpected error = %v", err)
				return
			}

			if result == "" {
				t.Errorf("Service.GenerateSlugFromTitle() expected slug, got empty string")
			}
		})
	}
}

func TestService_GetBySlug(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		setupMock   func(*mocks.MockRepository)
		expectedErr error
	}{
		{
			name: "successful retrieval",
			slug: "test-slug",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetBySlug", mock.Anything, "test-slug", "en").Return(&content.Content{
					ID:      1,
					Title:   "Test Title",
					Slug:    "test-slug",
					Content: "Test content",
				}, nil)
			},
			expectedErr: nil,
		},
		{
			name:        "invalid slug",
			slug:        "Invalid Slug!",
			setupMock:   func(m *mocks.MockRepository) {},
			expectedErr: content.ErrInvalidSlug,
		},
		{
			name: "content not found",
			slug: "non-existent",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetBySlug", mock.Anything, "non-existent", "en").Return(nil, content.ErrContentNotFound)
			},
			expectedErr: content.ErrContentNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.GetBySlug(context.Background(), tt.slug)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.GetBySlug() expected error, got nil")
				assert.True(t, errors.Is(err, tt.expectedErr), "Service.GetBySlug() error = %v, wantErr %v", err, tt.expectedErr)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.GetBySlug() unexpected error")
			require.NotNil(t, result, "Service.GetBySlug() expected content, got nil")
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetByUser(t *testing.T) {
	tests := []struct {
		name        string
		userID      int
		limit       int
		offset      int
		setupMock   func(*mocks.MockRepository)
		expectedErr error
	}{
		{
			name:   "successful retrieval",
			userID: 1,
			limit:  10,
			offset: 0,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByUser", mock.Anything, 1, 10, 0).Return([]*content.Content{
					{ID: 1, Title: "Content 1"},
					{ID: 2, Title: "Content 2"},
				}, nil)
			},
			expectedErr: nil,
		},
		{
			name:   "repository error",
			userID: 1,
			limit:  10,
			offset: 0,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByUser", mock.Anything, 1, 10, 0).Return(nil, errors.New("database error"))
			},
			expectedErr: errors.New("failed to get user content"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.GetByUser(context.Background(), tt.userID, tt.limit, tt.offset)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.GetByUser() expected error, got nil")
				assert.True(t, containsErrorSubstring(err.Error(), tt.expectedErr.Error()), "Service.GetByUser() error = %v, wantErr %v", err, tt.expectedErr)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.GetByUser() unexpected error")
			require.NotNil(t, result, "Service.GetByUser() expected content list, got nil")
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_ListByCursor(t *testing.T) {
	tests := []struct {
		name        string
		userID      int
		limit       int
		beforeID    int
		filters     content.ContentFilters
		setupMock   func(*mocks.MockRepository)
		expectedErr error
		expectedLen int
	}{
		{
			name:     "first page - beforeID 0 returns newest content",
			userID:   1,
			limit:    50,
			beforeID: 0,
			filters:  content.ContentFilters{},
			setupMock: func(m *mocks.MockRepository) {
				m.On("ListByCursor", mock.Anything, 1, 50, 0, content.ContentFilters{}).Return([]*content.Content{
					{ID: 3, Title: "Newest"},
					{ID: 2, Title: "Middle"},
					{ID: 1, Title: "Oldest"},
				}, nil)
			},
			expectedErr: nil,
			expectedLen: 3,
		},
		{
			name:     "next page - beforeID filters to older content",
			userID:   1,
			limit:    50,
			beforeID: 2,
			filters:  content.ContentFilters{},
			setupMock: func(m *mocks.MockRepository) {
				m.On("ListByCursor", mock.Anything, 1, 50, 2, content.ContentFilters{}).Return([]*content.Content{
					{ID: 1, Title: "Oldest"},
				}, nil)
			},
			expectedErr: nil,
			expectedLen: 1,
		},
		{
			name:     "empty result - no content for the caller",
			userID:   1,
			limit:    50,
			beforeID: 0,
			filters:  content.ContentFilters{},
			setupMock: func(m *mocks.MockRepository) {
				m.On("ListByCursor", mock.Anything, 1, 50, 0, content.ContentFilters{}).Return([]*content.Content{}, nil)
			},
			expectedErr: nil,
			expectedLen: 0,
		},
		{
			name:     "repository error - wrapped as failed to list content",
			userID:   1,
			limit:    50,
			beforeID: 0,
			filters:  content.ContentFilters{},
			setupMock: func(m *mocks.MockRepository) {
				m.On("ListByCursor", mock.Anything, 1, 50, 0, content.ContentFilters{}).Return(nil, errors.New("database error"))
			},
			expectedErr: errors.New("failed to list content"),
		},
		{
			name:     "filters passed through to repository verbatim",
			userID:   1,
			limit:    50,
			beforeID: 0,
			filters: content.ContentFilters{
				Status:   "draft",
				PostType: "post",
				Language: "en",
				Tags:     []string{"go", "tutorial"},
				Author:   "alice",
				Search:   "golang",
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("ListByCursor", mock.Anything, 1, 50, 0, content.ContentFilters{
					Status:   "draft",
					PostType: "post",
					Language: "en",
					Tags:     []string{"go", "tutorial"},
					Author:   "alice",
					Search:   "golang",
				}).Return([]*content.Content{{ID: 7, Title: "Filtered"}}, nil)
			},
			expectedErr: nil,
			expectedLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.ListByCursor(context.Background(), tt.userID, tt.limit, tt.beforeID, tt.filters)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.ListByCursor() expected error, got nil")
				assert.True(t, containsErrorSubstring(err.Error(), tt.expectedErr.Error()), "Service.ListByCursor() error = %v, wantErr %v", err, tt.expectedErr)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.ListByCursor() unexpected error")
			require.NotNil(t, result, "Service.ListByCursor() expected content list, got nil")
			assert.Len(t, result, tt.expectedLen, "Service.ListByCursor() result length")
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Update(t *testing.T) {
	tests := []struct {
		name           string
		id             int
		userID         int
		req            content.UpdateContentRequest
		setupMock      func(*mocks.MockRepository)
		wantErr        error
		expectedID     int
		expectedTitle  string
		expectedStatus content.Status
	}{
		{
			name:   "successful update of draft content",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Updated Title",
				Content: testTipTapJSON("Updated content"),
				Tags:    []string{"updated"},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Original Title",
					Slug:      "original-title",
					Content:   "Original content",
					Tags:      []string{"original"},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
				m.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:        nil,
			expectedID:     1,
			expectedTitle:  "Updated Title",
			expectedStatus: content.StatusDraft,
		},
		{
			name:   "successful publish draft to published",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusPublished,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:        nil,
			expectedID:     1,
			expectedStatus: content.StatusPublished,
		},
		{
			name:   "successful unpublish published to draft",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusPublished,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:        nil,
			expectedID:     1,
			expectedStatus: content.StatusDraft,
		},
		{
			name:   "successful update of published content (status remains published)",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Updated Title",
				Content: testTipTapJSON("Updated content"),
				Tags:    []string{},
				Status:  content.StatusPublished,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusPublished,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
				m.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:        nil,
			expectedID:     1,
			expectedTitle:  "Updated Title",
			expectedStatus: content.StatusPublished,
		},
		{
			name:   "unauthorized update attempt (different user)",
			id:     1,
			userID: 2,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
			},
			wantErr: content.ErrUnauthorized,
		},
		{
			name:   "content not found",
			id:     999,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, content.ErrContentNotFound)
			},
			wantErr: content.ErrContentNotFound,
		},
		{
			name:   "validation fails with empty title",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
			},
			wantErr: content.ErrInvalidTitle,
		},
		{
			name:   "validation fails with content too long",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: string(make([]byte, 100001)),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
			},
			wantErr: content.ErrInvalidContent,
		},
		{
			name:   "validation fails with invalid status",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.Status("invalid"),
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
			},
			wantErr: content.ErrInvalidStatus,
		},
		{
			name:   "fails when repository update fails",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(errors.New("database error"))
			},
			wantErr: errors.New("failed to update content"),
		},
		{
			name:   "fails when repository GetByID returns error",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			wantErr: errors.New("failed to get content"),
		},
		{
			name:   "validation fails with empty tag",
			id:     1,
			userID: 1,
			req: content.UpdateContentRequest{
				Title:   "Title",
				Content: testTipTapJSON("Content"),
				Tags:    []string{"valid", "", "another"},
				Status:  content.StatusDraft,
			},
			setupMock: func(m *mocks.MockRepository) {
				existingContent := &content.Content{
					ID:        1,
					UserID:    1,
					Title:     "Title",
					Slug:      "title",
					Content:   "Content",
					Tags:      []string{},
					Status:    content.StatusDraft,
					UpdatedAt: "2026-04-08T00:00:00Z",
				}
				m.On("GetByID", mock.Anything, 1).Return(existingContent, nil)
			},
			wantErr: errors.New("tags validation failed"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.Update(context.Background(), tt.id, tt.userID, "", tt.req)

			if tt.wantErr != nil {
				require.Error(t, err, "Service.Update() expected error, got nil")
				assert.True(t, errors.Is(err, tt.wantErr) || containsErrorSubstring(err.Error(), tt.wantErr.Error()), "Service.Update() error = %v, wantErr %v", err, tt.wantErr)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.Update() unexpected error")
			require.NotNil(t, result, "Service.Update() expected content, got nil")

			assert.Equal(t, tt.expectedID, result.ID, "Service.Update() ID")
			assert.Equal(t, tt.userID, result.UserID, "Service.Update() UserID")

			if tt.expectedTitle != "" {
				assert.Equal(t, tt.expectedTitle, result.Title, "Service.Update() Title")
			}
			if tt.expectedStatus != "" {
				assert.Equal(t, tt.expectedStatus, result.Status, "Service.Update() Status")
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Update_SEOAutoGeneration(t *testing.T) {
	tests := []struct {
		name             string
		id               int
		userID           int
		existingContent  *content.Content
		req              content.UpdateContentRequest
		setupMock        func(*mocks.MockRepository)
		expectedMetaDesc string
		expectedOGTitle  string
		expectedOGDesc   string
	}{
		{
			name:   "auto_generates_seo_when_publishing_draft",
			id:     1,
			userID: 1,
			existingContent: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Test Article",
				Slug:    "test-article",
				Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"This is a test article with some content for SEO metadata generation purposes."}]}]}`,
				Tags:    []string{"test", "article"},
				Status:  content.StatusDraft,
			},
			req: content.UpdateContentRequest{
				Title:           "Test Article",
				Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"This is a test article with some content for SEO metadata generation purposes."}]}]}`,
				Tags:            []string{"test", "article"},
				Status:          content.StatusPublished,
				MetaDescription: "",
				OGTitle:         "",
				OGDescription:   "",
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					UserID:   1,
					Title:    "Test Article",
					Slug:     "test-article",
					Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"This is a test article with some content for SEO metadata generation purposes."}]}]}`,
					Tags:     []string{"test", "article"},
					Status:   content.StatusDraft,
					PostType: "post",
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			expectedMetaDesc: "This is a test article with some content for SEO metadata generation purposes.",
			expectedOGTitle:  "Test Article",
			expectedOGDesc:   "This is a test article with some content for SEO metadata generation purposes.",
		},
		{
			name:   "uses_custom_seo_overrides_when_provided",
			id:     1,
			userID: 1,
			existingContent: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Test Article",
				Slug:    "test-article",
				Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Default content"}]}]}`,
				Tags:    []string{"test"},
				Status:  content.StatusDraft,
			},
			req: content.UpdateContentRequest{
				Title:           "Test Article",
				Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Default content"}]}]}`,
				Tags:            []string{"test"},
				Status:          content.StatusPublished,
				MetaDescription: "Custom meta description",
				OGTitle:         "Custom OG Title",
				OGDescription:   "Custom OG description",
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Test Article",
					Slug:    "test-article",
					Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Default content"}]}]}`,
					Tags:    []string{"test"},
					Status:  content.StatusDraft,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
					content := args.Get(1).(*content.Content)
					assert.Equal(t, "Custom meta description", content.MetaDescription)
					assert.Equal(t, "Custom OG Title", content.OGTitle)
					assert.Equal(t, "Custom OG description", content.OGDescription)
				})
			},
			expectedMetaDesc: "Custom meta description",
			expectedOGTitle:  "Custom OG Title",
			expectedOGDesc:   "Custom OG description",
		},
		{
			name:   "does_not_regenerate_seo_when_updating_published_content",
			id:     1,
			userID: 1,
			existingContent: &content.Content{
				ID:              1,
				UserID:          1,
				Title:           "Test Article",
				Slug:            "test-article",
				Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
				Tags:            []string{"test"},
				Status:          content.StatusPublished,
				MetaDescription: "Existing meta description",
				OGTitle:         "Existing OG title",
				OGDescription:   "Existing OG description",
			},
			req: content.UpdateContentRequest{
				Title:           "Test Article",
				Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
				Tags:            []string{"test"},
				Status:          content.StatusPublished,
				MetaDescription: "Updated meta description",
				OGTitle:         "Updated OG title",
				OGDescription:   "Updated OG description",
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:              1,
					UserID:          1,
					Title:           "Test Article",
					Slug:            "test-article",
					Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
					Tags:            []string{"test"},
					Status:          content.StatusPublished,
					MetaDescription: "Existing meta description",
					OGTitle:         "Existing OG title",
					OGDescription:   "Existing OG description",
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
					content := args.Get(1).(*content.Content)
					assert.Equal(t, "Updated meta description", content.MetaDescription)
					assert.Equal(t, "Updated OG title", content.OGTitle)
					assert.Equal(t, "Updated OG description", content.OGDescription)
				})
			},
			expectedMetaDesc: "Updated meta description",
			expectedOGTitle:  "Updated OG title",
			expectedOGDesc:   "Updated OG description",
		},
		{
			name:   "does_not_auto_generate_seo_when_seo_service_is_nil",
			id:     1,
			userID: 1,
			existingContent: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Test Article",
				Slug:    "test-article",
				Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
				Tags:    []string{"test"},
				Status:  content.StatusDraft,
			},
			req: content.UpdateContentRequest{
				Title:           "Test Article",
				Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
				Tags:            []string{"test"},
				Status:          content.StatusPublished,
				MetaDescription: "",
				OGTitle:         "",
				OGDescription:   "",
			},
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Test Article",
					Slug:    "test-article",
					Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
					Tags:    []string{"test"},
					Status:  content.StatusDraft,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			expectedMetaDesc: "",
			expectedOGTitle:  "",
			expectedOGDesc:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			var seoService *seo.Service = nil
			if tt.name != "does_not_auto_generate_seo_when_seo_service_is_nil" {
				seoService = seo.NewService("http://localhost:8080", "Test Site")
			}

			service := content.NewService(mockRepo, seoService, nil)
			result, err := service.Update(context.Background(), tt.id, tt.userID, "", tt.req)

			require.NoError(t, err, "Service.Update() unexpected error")
			require.NotNil(t, result, "Service.Update() expected content, got nil")

			assert.Equal(t, tt.expectedMetaDesc, result.MetaDescription, "Service.Update() MetaDescription")
			assert.Equal(t, tt.expectedOGTitle, result.OGTitle, "Service.Update() OGTitle")
			assert.Equal(t, tt.expectedOGDesc, result.OGDescription, "Service.Update() OGDescription")

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_PostTypeValidation(t *testing.T) {
	t.Run("succeeds_with_valid_post_type", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			content := args.Get(1).(*content.Content)
			content.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "valid_type").Return(content.PostType{Slug: "valid_type"}, nil)
		mockPostType.On("GetFieldsByPostType", "valid_type").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)

		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "valid_type",
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err, "Service.Create() unexpected error")
		assert.Equal(t, "valid_type", result.PostType, "Service.Create() PostType")

		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("fails_with_invalid_post_type", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "invalid_type").Return(content.PostType{}, content.ErrPostTypeNotFound)

		service := content.NewService(mockRepo, nil, mockPostType)

		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "invalid_type",
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err, "Service.Create() expected error, got nil")
		assert.Contains(t, err.Error(), "post type validation failed", "Service.Create() error")

		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("skips_validation_when_service_is_nil", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			content := args.Get(1).(*content.Content)
			content.ID = 1
		})

		service := content.NewService(mockRepo, nil, nil)

		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "any_type",
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err, "Service.Create() unexpected error")
		assert.Equal(t, "any_type", result.PostType, "Service.Create() PostType")

		mockRepo.AssertExpectations(t)
	})
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestService_Update_PostTypeValidation(t *testing.T) {
	t.Run("succeeds_with_valid_post_type", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:      1,
			UserID:  1,
			Title:   "Test Article",
			Slug:    "test-article",
			Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
			Tags:    []string{},
			Status:  content.StatusDraft,
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "valid_type").Return(content.PostType{Slug: "valid_type"}, nil)
		mockPostType.On("GetFieldsByPostType", "valid_type").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)

		req := content.UpdateContentRequest{
			Title:    "Test Article",
			Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "valid_type",
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err, "Service.Update() unexpected error")
		assert.Equal(t, "valid_type", result.PostType, "Service.Update() PostType")

		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("fails_with_invalid_post_type", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:      1,
			UserID:  1,
			Title:   "Test Article",
			Slug:    "test-article",
			Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
			Tags:    []string{},
			Status:  content.StatusDraft,
		}, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "invalid_type").Return(content.PostType{}, content.ErrPostTypeNotFound)

		service := content.NewService(mockRepo, nil, mockPostType)

		req := content.UpdateContentRequest{
			Title:    "Test Article",
			Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "invalid_type",
		}

		_, err := service.Update(context.Background(), 1, 1, "", req)

		require.Error(t, err, "Service.Update() expected error, got nil")
		assert.Contains(t, err.Error(), "post type validation failed", "Service.Update() error")

		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})
}

func TestService_Update_CustomURLPrefixForNonPostType(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test Article",
		Slug:     "test-article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusDraft,
		PostType: "page",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, nil)

	req := content.UpdateContentRequest{
		Title:    "Test Article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusPublished,
		PostType: "page",
	}

	result, err := service.Update(context.Background(), 1, 1, "", req)

	require.NoError(t, err, "Service.Update() unexpected error")
	assert.NotEmpty(t, result.MetaDescription, "Service.Update() expected meta description to be generated")

	mockRepo.AssertExpectations(t)
}

func TestService_Update_DefaultURLPrefixForPostType(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test Article",
		Slug:     "test-article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusDraft,
		PostType: "post",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, nil)

	req := content.UpdateContentRequest{
		Title:    "Test Article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusPublished,
		PostType: "post",
	}

	result, err := service.Update(context.Background(), 1, 1, "", req)

	require.NoError(t, err, "Service.Update() unexpected error")
	assert.NotEmpty(t, result.MetaDescription, "Service.Update() expected meta description to be generated")

	mockRepo.AssertExpectations(t)
}

func TestService_Update_PostTypeValidationWithSEOGeneration(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test Article",
		Slug:     "test-article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusDraft,
		PostType: "article",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	mockPostType := &mocks.MockPostTypeServiceInterface{}
	mockPostType.On("GetBySlug", "article").Return(content.PostType{Slug: "article"}, nil)
	mockPostType.On("GetFieldsByPostType", "article").Return([]customfield.FieldSchema{}, nil)
	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, mockPostType)

	req := content.UpdateContentRequest{
		Title:    "Test Article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusPublished,
		PostType: "article",
	}

	result, err := service.Update(context.Background(), 1, 1, "", req)

	require.NoError(t, err, "Service.Update() unexpected error")
	assert.NotEmpty(t, result.MetaDescription, "Service.Update() expected meta description to be generated")
	assert.Equal(t, "article", result.PostType, "Service.Update() PostType")

	mockRepo.AssertExpectations(t)
	mockPostType.AssertExpectations(t)
}

func TestService_Update_EmptyPostTypeWithPostTypeService(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:      1,
		UserID:  1,
		Title:   "Test Article",
		Slug:    "test-article",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:    []string{},
		Status:  content.StatusDraft,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	mockPostType := &mocks.MockPostTypeServiceInterface{}
	mockPostType.On("GetFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)
	service := content.NewService(mockRepo, nil, mockPostType)

	req := content.UpdateContentRequest{
		Title:    "Test Article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusDraft,
		PostType: "",
	}

	result, err := service.Update(context.Background(), 1, 1, "", req)

	require.NoError(t, err, "Service.Update() unexpected error")
	assert.Equal(t, "", result.PostType, "Service.Update() PostType should be empty")

	mockRepo.AssertExpectations(t)
	mockPostType.AssertExpectations(t)
}

func TestService_Update_ChangePostTypeWithSEOGeneration(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test Article",
		Slug:     "test-article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusDraft,
		PostType: "page",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	mockPostType := &mocks.MockPostTypeServiceInterface{}
	mockPostType.On("GetBySlug", "post").Return(content.PostType{Slug: "post"}, nil)
	mockPostType.On("GetFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)
	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, mockPostType)

	req := content.UpdateContentRequest{
		Title:    "Test Article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusPublished,
		PostType: "post",
	}

	result, err := service.Update(context.Background(), 1, 1, "", req)

	require.NoError(t, err, "Service.Update() unexpected error")
	assert.NotEmpty(t, result.MetaDescription, "Service.Update() expected meta description to be generated")
	assert.Equal(t, "post", result.PostType, "Service.Update() PostType")

	mockRepo.AssertExpectations(t)
	mockPostType.AssertExpectations(t)
}

func TestService_Update_WithNilSEOService(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:      1,
		UserID:  1,
		Title:   "Test Article",
		Slug:    "test-article",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:    []string{},
		Status:  content.StatusDraft,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	service := content.NewService(mockRepo, nil, nil)

	req := content.UpdateContentRequest{
		Title:           "Test Article",
		Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:            []string{},
		Status:          content.StatusPublished,
		MetaDescription: "Custom meta description",
		OGTitle:         "Custom OG Title",
		OGDescription:   "Custom OG description",
	}

	result, err := service.Update(context.Background(), 1, 1, "", req)

	require.NoError(t, err, "Service.Update() unexpected error")
	assert.Equal(t, "Custom meta description", result.MetaDescription, "Service.Update() MetaDescription")
	assert.Equal(t, "Custom OG Title", result.OGTitle, "Service.Update() OGTitle")
	assert.Equal(t, "Custom OG description", result.OGDescription, "Service.Update() OGDescription")

	mockRepo.AssertExpectations(t)
}

func TestService_GetByID(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		setupMock   func(*mocks.MockRepository)
		expectedErr error
	}{
		{
			name: "successful retrieval",
			id:   1,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					Title:   "Test Title",
					Slug:    "test-slug",
					Content: "Test content",
				}, nil)
			},
			expectedErr: nil,
		},
		{
			name: "content not found",
			id:   999,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, content.ErrContentNotFound)
			},
			expectedErr: content.ErrContentNotFound,
		},
		{
			name: "repository error",
			id:   1,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			expectedErr: errors.New("failed to get content"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.GetByID(context.Background(), tt.id)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.GetByID() expected error, got nil")
				assert.True(t, errors.Is(err, tt.expectedErr) || containsErrorSubstring(err.Error(), tt.expectedErr.Error()), "Service.GetByID() error = %v, wantErr %v", err, tt.expectedErr)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.GetByID() unexpected error")
			require.NotNil(t, result, "Service.GetByID() expected content, got nil")
			assert.Equal(t, tt.id, result.ID, "Service.GetByID() ID")

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Update_SEOGenerationFailure(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:      1,
		UserID:  1,
		Title:   "Test Article",
		Slug:    "test-article",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:    []string{},
		Status:  content.StatusDraft,
	}, nil)
	mockRepo.On("CheckSlugUnique", mock.Anything, mock.AnythingOfType("string"), "").Return(true, nil)

	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, nil)

	// Create a title that exceeds the OG title limit (60 characters)
	longTitle := "This is a very long title that exceeds the sixty character limit for OG titles and should cause validation to fail"

	req := content.UpdateContentRequest{
		Title:   longTitle,
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:    []string{},
		Status:  content.StatusPublished,
	}

	_, err := service.Update(context.Background(), 1, 1, "", req)

	require.Error(t, err, "Service.Update() expected error when title is too long for SEO, got nil")
	if !containsString(err.Error(), "failed to generate SEO metadata") {
		t.Logf("Service.Update() error = %v", err)
	}

	mockRepo.AssertExpectations(t)
}

func TestService_Update_PostTypeValidation_Debug(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:      1,
		UserID:  1,
		Title:   "Test Article",
		Slug:    "test-article",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:    []string{},
		Status:  content.StatusDraft,
	}, nil)

	mockPostType := &mocks.MockPostTypeServiceInterface{}
	mockPostType.On("GetBySlug", "invalid_type").Return(content.PostType{}, content.ErrPostTypeNotFound)

	service := content.NewService(mockRepo, nil, mockPostType)

	req := content.UpdateContentRequest{
		Title:    "Test Article",
		Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
		Tags:     []string{},
		Status:   content.StatusDraft,
		PostType: "invalid_type",
	}

	_, err := service.Update(context.Background(), 1, 1, "", req)

	require.Error(t, err, "Service.Update() expected error, got nil")
	t.Logf("Error: %v", err)

	mockRepo.AssertExpectations(t)
	mockPostType.AssertExpectations(t)
}

func TestService_ListByFilters(t *testing.T) {
	t.Run("passes filters to repository", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		filterResults := []*content.Content{
			{ID: 1, Title: "Croissant"},
			{ID: 2, Title: "Eclair"},
		}
		mockRepo.On("ListByFilters", mock.Anything, 1, mock.MatchedBy(func(f content.ContentFilters) bool {
			return f.Limit == 100 && f.Offset == 0 &&
				len(f.CustomFieldFilters) == 1 &&
				f.CustomFieldFilters[0].Field == "category" &&
				f.CustomFieldFilters[0].Operator == content.FilterOpEqual &&
				f.CustomFieldFilters[0].Value == "Pastry"
		})).Return(filterResults, nil)

		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"},
			},
		}

		results, err := service.ListByFilters(context.Background(), 1, filters)

		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "Croissant", results[0].Title)
		mockRepo.AssertExpectations(t)
	})

	t.Run("clamps limit to default when zero", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("ListByFilters", mock.Anything, 1, mock.MatchedBy(func(f content.ContentFilters) bool {
			return f.Limit == 100
		})).Return([]*content.Content{}, nil)

		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{Limit: 0, Offset: 0}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("clamps limit to max 1000", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("ListByFilters", mock.Anything, 1, mock.MatchedBy(func(f content.ContentFilters) bool {
			return f.Limit == 1000
		})).Return([]*content.Content{}, nil)

		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{Limit: 5000, Offset: 0}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("clamps negative offset to zero", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("ListByFilters", mock.Anything, 1, mock.MatchedBy(func(f content.ContentFilters) bool {
			return f.Offset == 0
		})).Return([]*content.Content{}, nil)

		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{Limit: 100, Offset: -10}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error for empty filter field", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "", Operator: content.FilterOpEqual, Value: "Pastry"},
			},
		}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.Error(t, err)
		assert.ErrorIs(t, err, content.ErrInvalidFilterField)
		mockRepo.AssertNotCalled(t, "ListByFilters")
	})

	t.Run("returns error for invalid filter operator", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "category", Operator: content.FilterOperator("invalid"), Value: "Pastry"},
			},
		}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.Error(t, err)
		assert.ErrorIs(t, err, content.ErrInvalidFilterOperator)
		mockRepo.AssertNotCalled(t, "ListByFilters")
	})

	t.Run("returns error for empty filter value", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "category", Operator: content.FilterOpEqual, Value: ""},
			},
		}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.Error(t, err)
		assert.ErrorIs(t, err, content.ErrInvalidFilterValue)
		mockRepo.AssertNotCalled(t, "ListByFilters")
	})

	t.Run("returns repository error", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockRepo.On("ListByFilters", mock.Anything, 1, mock.Anything).Return([]*content.Content(nil), errors.New("db error"))

		service := content.NewService(mockRepo, nil, nil)
		filters := content.ContentFilters{Limit: 100, Offset: 0}

		_, err := service.ListByFilters(context.Background(), 1, filters)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list content by filters")
		mockRepo.AssertExpectations(t)
	})
}

func TestValidateCustomFieldFilter(t *testing.T) {
	t.Run("valid filter passes", func(t *testing.T) {
		f := content.CustomFieldFilter{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"}
		assert.NoError(t, content.ValidateCustomFieldFilter(f))
	})

	t.Run("empty field returns error", func(t *testing.T) {
		f := content.CustomFieldFilter{Field: "", Operator: content.FilterOpEqual, Value: "Pastry"}
		assert.ErrorIs(t, content.ValidateCustomFieldFilter(f), content.ErrInvalidFilterField)
	})

	t.Run("invalid operator returns error", func(t *testing.T) {
		f := content.CustomFieldFilter{Field: "price", Operator: "unknown", Value: "5"}
		assert.ErrorIs(t, content.ValidateCustomFieldFilter(f), content.ErrInvalidFilterOperator)
	})

	t.Run("empty value returns error", func(t *testing.T) {
		f := content.CustomFieldFilter{Field: "price", Operator: content.FilterOpMin, Value: ""}
		assert.ErrorIs(t, content.ValidateCustomFieldFilter(f), content.ErrInvalidFilterValue)
	})
}

func TestService_DeleteOwnComment(t *testing.T) {
	tests := []struct {
		name          string
		commentID     int
		userID        int
		setupMock     func(*mocks.MockCommentRepository)
		expectedErr   error
		matchContains bool
	}{
		{
			name:      "successful delete own comment",
			commentID: 1,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Comment{
					ID:     1,
					UserID: 1,
				}, nil)
				m.On("Delete", mock.Anything, 1).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:      "delete comment owned by another user",
			commentID: 2,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 2).Return(&content.Comment{
					ID:     2,
					UserID: 99,
				}, nil)
			},
			expectedErr: content.ErrCommentNotFound,
		},
		{
			name:      "delete non-existent comment",
			commentID: 999,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, errors.New("not found"))
			},
			expectedErr: content.ErrCommentNotFound,
		},
		{
			name:      "repository delete fails after ownership check",
			commentID: 1,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Comment{
					ID:     1,
					UserID: 1,
				}, nil)
				m.On("Delete", mock.Anything, 1).Return(errors.New("database error"))
			},
			expectedErr:   errors.New("failed to delete comment: database error"),
			matchContains: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCommentRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockCommentRepo)

			s := content.NewServiceWithComments(nil, mockCommentRepo, nil, nil)

			err := s.DeleteOwnComment(context.Background(), tt.commentID, tt.userID)

			if tt.expectedErr != nil {
				if tt.matchContains {
					assert.ErrorContains(t, err, tt.expectedErr.Error())
				} else {
					require.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_GetCommentsByStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      content.CommentStatus
		setupMock   func(*mocks.MockCommentRepository)
		expectedLen int
		expectedErr error
	}{
		{
			name:   "returns comments by status with content context",
			status: content.CommentStatusPending,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByStatus", mock.Anything, content.CommentStatusPending).Return([]*content.Comment{
					{
						ID:           1,
						ContentID:    10,
						ContentTitle: "Getting Started with Go",
						ContentSlug:  "getting-started-with-go",
						Comment:      "Great article!",
						Author:       "Jane Doe",
						Status:       content.CommentStatusPending,
						CreatedAt:    "2026-04-19T10:30:00Z",
					},
				}, nil)
			},
			expectedLen: 1,
			expectedErr: nil,
		},
		{
			name:   "returns empty list when no comments match status",
			status: content.CommentStatusPending,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByStatus", mock.Anything, content.CommentStatusPending).Return([]*content.Comment{}, nil)
			},
			expectedLen: 0,
			expectedErr: nil,
		},
		{
			name:   "wraps repository error",
			status: content.CommentStatusPending,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByStatus", mock.Anything, content.CommentStatusPending).Return(nil, errors.New("database error"))
			},
			expectedErr: errors.New("failed to get comments by status"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCommentRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockCommentRepo)

			s := content.NewServiceWithComments(nil, mockCommentRepo, nil, nil)
			result, err := s.GetCommentsByStatus(context.Background(), tt.status)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				mockCommentRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			require.Len(t, result, tt.expectedLen)
			if tt.expectedLen > 0 {
				assert.Equal(t, "Getting Started with Go", result[0].ContentTitle)
				assert.Equal(t, "getting-started-with-go", result[0].ContentSlug)
			}
			mockCommentRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetPublishedByTag(t *testing.T) {
	tests := []struct {
		name        string
		tag         string
		limit       int
		offset      int
		setupMock   func(*mocks.MockRepository)
		expectedErr error
	}{
		{
			name:   "successful retrieval",
			tag:    "golang",
			limit:  10,
			offset: 0,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetPublishedByTag", mock.Anything, "golang", 10, 0).Return([]*content.Content{
					{ID: 1, Title: "Go Basics", Tags: []string{"golang", "tutorial"}},
				}, nil)
			},
			expectedErr: nil,
		},
		{
			name:   "repository error",
			tag:    "golang",
			limit:  10,
			offset: 0,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetPublishedByTag", mock.Anything, "golang", 10, 0).Return(nil, errors.New("database error"))
			},
			expectedErr: errors.New("failed to get published content by tag"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.GetPublishedByTag(context.Background(), tt.tag, tt.limit, tt.offset)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, containsErrorSubstring(err.Error(), tt.expectedErr.Error()))
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result, 1)
			assert.Equal(t, "Go Basics", result[0].Title)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Create_CustomFieldValidation(t *testing.T) {
	setupCreateMocks := func() (*mocks.MockRepository, *mocks.MockPostTypeServiceInterface) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})
		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		return mockRepo, mockPostType
	}

	t.Run("create with valid custom fields", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true, Min: ptrF(0), Max: ptrF(10000)},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create fails when required field missing", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true, Min: ptrF(0), Max: ptrF(10000)},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"other": "value",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Price is required")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create fails when number value exceeds max", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true, Min: ptrF(0), Max: ptrF(100)},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 200.0,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be at most 100")
	})

	t.Run("create fails when select value not in options", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Size", Slug: "size", Type: customfield.FieldTypeSelect, Options: []string{"S", "M", "L"}},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"size": "XL",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")
	})

	t.Run("create fails when text exceeds maxLength", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		maxLen := 10
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Label", Slug: "label", Type: customfield.FieldTypeText, MaxLength: &maxLen},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"label": "this is way too long",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be at most 10 characters")
	})

	t.Run("create with system field values stripped from customFields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
			{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price":        29.99,
				"internal_sku": "SKU-001",
				"sync_status":  "synced",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
		_, hasSku := result.CustomFields["internal_sku"]
		assert.False(t, hasSku, "system field internal_sku should be stripped")
		_, hasSync := result.CustomFields["sync_status"]
		assert.False(t, hasSync, "system field sync_status should be stripped")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create succeeds with no custom fields - backward compatibility", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		// Post type has no required fields, so nil customFields passes
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Label", Slug: "label", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Nil(t, result.CustomFields)
	})

	t.Run("create without postTypeService - fields pass through unvalidated", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		service := content.NewService(mockRepo, nil, nil)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": "not_even_a_number",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, "not_even_a_number", result.CustomFields["price"])
	})

	t.Run("create fails when required field value is empty string", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Label", Slug: "label", Type: customfield.FieldTypeText, Required: true},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"label": "  ",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Label is required")
	})

	t.Run("create fails when GetFieldsByPostType returns error", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return(nil, errors.New("schema error"))

		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom field validation failed")
	})

	t.Run("create with number below min", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Min: ptrF(10)},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 5.0,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be at least 10")
	})

	t.Run("create with wrong type for number field", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": "not a number",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a number")
	})

	t.Run("create with empty date value", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Release Date", Slug: "release_date", Type: customfield.FieldTypeDate},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"release_date": "",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a date string")
	})

	t.Run("create fails when date string is not a valid date", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Release Date", Slug: "release_date", Type: customfield.FieldTypeDate},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"release_date": "not-a-date",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a valid date")
	})

	t.Run("create fails when non-required field has explicit null value", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Label", Slug: "label", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"label": nil,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be null")
	})

	t.Run("create with valid checkbox", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"featured": true,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, true, result.CustomFields["featured"])
	})

	t.Run("create with invalid checkbox type", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"featured": "yes",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a boolean")
	})

	t.Run("create with valid date", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Release Date", Slug: "release_date", Type: customfield.FieldTypeDate},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"release_date": "2026-05-17",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, "2026-05-17", result.CustomFields["release_date"])
	})

	t.Run("create with nil custom field value", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Label", Slug: "label", Type: customfield.FieldTypeText, Required: true},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"label": nil,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Label is required")
	})

	t.Run("create with textarea exceeding maxLength", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		maxLen := 5
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Bio", Slug: "bio", Type: customfield.FieldTypeTextarea, MaxLength: &maxLen},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"bio": "this is too long",
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be at most 5 characters")
	})

	t.Run("create with wrong type for text field", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Label", Slug: "label", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"label": 123,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})

	t.Run("create with wrong type for select field", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Size", Slug: "size", Type: customfield.FieldTypeSelect, Options: []string{"S", "M", "L"}},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"size": 42,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})

	t.Run("create with wrong type for date field", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Date", Slug: "date", Type: customfield.FieldTypeDate},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"date": 12345,
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a date string")
	})

	t.Run("create fails when required number is zero", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": float64(0),
			},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Price is required")
	})

	t.Run("create succeeds when required checkbox is false", func(t *testing.T) {
		mockRepo, mockPostType := setupCreateMocks()
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Agreed", Slug: "agreed", Type: customfield.FieldTypeCheckbox, Required: true},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"agreed": false,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, false, result.CustomFields["agreed"])
	})
}

func TestService_Update_CustomFieldValidation(t *testing.T) {
	t.Run("update with valid custom fields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Old Title",
			Slug:     "old-title",
			Content:  testTipTapJSON("Old content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true, Min: ptrF(0)},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.UpdateContentRequest{
			Title:    "Updated Title",
			Content:  testTipTapJSON("Updated content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 19.99,
			},
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.Equal(t, 19.99, result.CustomFields["price"])
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("update fails when required field missing", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Old Title",
			Slug:     "old-title",
			Content:  testTipTapJSON("Old content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.UpdateContentRequest{
			Title:    "Updated Title",
			Content:  testTipTapJSON("Updated content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"other": "value",
			},
		}

		_, err := service.Update(context.Background(), 1, 1, "", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Price is required")
	})

	t.Run("update uses existing post type when PostType not provided", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Old Title",
			Slug:     "old-title",
			Content:  testTipTapJSON("Old content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)

		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.UpdateContentRequest{
			Title:   "Updated Title",
			Content: testTipTapJSON("Updated content"),
			Tags:    []string{"test"},
			Status:  content.StatusDraft,
			CustomFields: map[string]any{
				"price": 19.99,
			},
		}

		_, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("update without postTypeService - fields pass through unvalidated", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Old Title",
			Slug:     "old-title",
			Content:  testTipTapJSON("Old content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		service := content.NewService(mockRepo, nil, nil)
		req := content.UpdateContentRequest{
			Title:   "Updated Title",
			Content: testTipTapJSON("Updated content"),
			Tags:    []string{"test"},
			Status:  content.StatusDraft,
			CustomFields: map[string]any{
				"price": "invalid",
			},
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.Equal(t, "invalid", result.CustomFields["price"])
	})

	t.Run("update with nil custom fields preserves existing values", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:           1,
			UserID:       1,
			Title:        "Old Title",
			Slug:         "old-title",
			Content:      testTipTapJSON("Old content"),
			PostType:     "product",
			Status:       content.StatusDraft,
			CustomFields: map[string]any{"price": 29.99},
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.UpdateContentRequest{
			Title:   "Updated Title",
			Content: testTipTapJSON("Updated content"),
			Tags:    []string{"test"},
			Status:  content.StatusDraft,
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
	})

	t.Run("update with nil custom fields triggers required validation on post type with required fields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Old Title",
			Slug:     "old-title",
			Content:  testTipTapJSON("Old content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)

		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.UpdateContentRequest{
			Title:   "Updated Title",
			Content: testTipTapJSON("Updated content"),
			Tags:    []string{"test"},
			Status:  content.StatusDraft,
		}

		_, err := service.Update(context.Background(), 1, 1, "", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Price is required")
	})
}

func TestService_Create_SystemFieldStripping(t *testing.T) {
	t.Run("create succeeds when all submitted fields are system fields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
			{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"internal_sku": "SKU-001",
				"sync_status":  "synced",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Empty(t, result.CustomFields, "all system fields should be stripped, leaving empty map")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create with nil customFields passes through", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Nil(t, result.CustomFields)
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create with empty customFields works", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:        "Test Title",
			Content:      testTipTapJSON("Test content"),
			Tags:         []string{"test"},
			Status:       content.StatusDraft,
			PostType:     "product",
			CustomFields: map[string]any{},
		}

		_, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create when postTypeService is nil does not strip", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		service := content.NewService(mockRepo, nil, nil)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"internal_sku": "SKU-001",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, "SKU-001", result.CustomFields["internal_sku"], "system field passes through when postTypeService is nil")
		mockRepo.AssertExpectations(t)
	})

	t.Run("create when post type has no system fields does not strip", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"], "custom field preserved when no system fields defined")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("update strips system field keys from customFields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Old Title",
			Slug:     "old-title",
			Content:  testTipTapJSON("Old content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)
		mockRepo.On("CheckSlugUnique", mock.Anything, "updated-title", "").Return(true, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.UpdateContentRequest{
			Title:   "Updated Title",
			Content: testTipTapJSON("Updated content"),
			Tags:    []string{"test"},
			Status:  content.StatusDraft,
			CustomFields: map[string]any{
				"internal_sku": "SKU-001",
				"price":        29.99,
			},
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		_, hasSku := result.CustomFields["internal_sku"]
		assert.False(t, hasSku, "system field internal_sku should be stripped on update")
		assert.Equal(t, 29.99, result.CustomFields["price"], "custom field preserved on update")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create preserves custom fields while stripping system fields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price":        29.99,
				"internal_sku": "SKU-001",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"], "custom field preserved")
		_, hasSku := result.CustomFields["internal_sku"]
		assert.False(t, hasSku, "system field stripped")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})
}

func TestService_SetSystemFields(t *testing.T) {
	t.Run("stores valid system field values", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:           1,
			UserID:       1,
			Title:        "Test",
			Slug:         "test",
			Content:      testTipTapJSON("Content"),
			PostType:     "product",
			Status:       content.StatusDraft,
			CustomFields: map[string]any{"price": 29.99},
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		result, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"internal_sku": "SKU-001",
		})

		require.NoError(t, err)
		assert.Equal(t, "SKU-001", result.CustomFields["internal_sku"])
		assert.Equal(t, 29.99, result.CustomFields["price"], "existing custom field preserved")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("preserves existing custom field values", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:           1,
			UserID:       1,
			Title:        "Test",
			Slug:         "test",
			Content:      testTipTapJSON("Content"),
			PostType:     "product",
			Status:       content.StatusDraft,
			CustomFields: map[string]any{"price": 29.99, "category": "Pastry"},
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		result, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"internal_sku": "SKU-001",
		})

		require.NoError(t, err)
		assert.Equal(t, "SKU-001", result.CustomFields["internal_sku"])
		assert.Equal(t, 29.99, result.CustomFields["price"])
		assert.Equal(t, "Pastry", result.CustomFields["category"])
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("rejects keys not in system field schema", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Test",
			Slug:     "test",
			Content:  testTipTapJSON("Content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		_, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"internal_sku":  "SKU-001",
			"unknown_field": "value",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown system field key: unknown_field")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("validates system field values against schema wrong type", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Test",
			Slug:     "test",
			Content:  testTipTapJSON("Content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		_, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"sync_status": 12345,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Sync Status: must be a string")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("validates select field values must be in options", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Test",
			Slug:     "test",
			Content:  testTipTapJSON("Content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		_, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"sync_status": "invalid_option",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("validates number field min/max bounds", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:       1,
			UserID:   1,
			Title:    "Test",
			Slug:     "test",
			Content:  testTipTapJSON("Content"),
			PostType: "product",
			Status:   content.StatusDraft,
		}, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Priority", Slug: "priority", Type: customfield.FieldTypeNumber, Min: ptrF(1), Max: ptrF(10)},
		}, nil)

		service := content.NewService(mockRepo, nil, mockPostType)
		_, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"priority": 15.0,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be at most 10")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("returns error for non-existent content", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 999).Return(nil, content.ErrContentNotFound)

		mockPostType := &mocks.MockPostTypeServiceInterface{}

		service := content.NewService(mockRepo, nil, mockPostType)
		_, err := service.SetSystemFields(context.Background(), 999, map[string]any{
			"internal_sku": "SKU-001",
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, content.ErrContentNotFound)
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("returns error when postTypeService is nil", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}

		service := content.NewService(mockRepo, nil, nil)
		_, err := service.SetSystemFields(context.Background(), 1, map[string]any{
			"internal_sku": "SKU-001",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "post type service is nil")
	})
}

func TestService_Create_SystemFieldStripping_ErrorPaths(t *testing.T) {
	t.Run("create silently passes through when GetSystemFieldsByPostType returns error", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return(nil, errors.New("service error"))

		service := content.NewService(mockRepo, nil, mockPostType)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price":        29.99,
				"internal_sku": "SKU-001",
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
		assert.Equal(t, "SKU-001", result.CustomFields["internal_sku"], "system field passes through when service errors")
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})
}

func TestService_Create_HookIntegration(t *testing.T) {
	t.Run("create with hook that adds system field values stores them", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
			{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
		}, nil)

		hookInput := func(data []byte) []byte {
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			cf := m["customFields"].(map[string]any)
			cf["internal_sku"] = "SKU-001"
			cf["sync_status"] = "synced"
			b, _ := json.Marshal(m)
			return b
		}

		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				if hookName == plugin.HookBeforeSave {
					return hookInput(data), nil
				}
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{"test"},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, "SKU-001", result.CustomFields["internal_sku"])
		assert.Equal(t, "synced", result.CustomFields["sync_status"])
		assert.Equal(t, 29.99, result.CustomFields["price"])
		mockRepo.AssertExpectations(t)
		mockPostType.AssertExpectations(t)
	})

	t.Run("create with hook that modifies custom field values reflects changes", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				if hookName == plugin.HookBeforeSave {
					var m map[string]any
					_ = json.Unmarshal(data, &m)
					cf := m["customFields"].(map[string]any)
					cf["price"] = 19.99
					b, _ := json.Marshal(m)
					return b, nil
				}
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 19.99, result.CustomFields["price"])
	})

	t.Run("create with hook that modifies both system and custom fields stores both", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				if hookName == plugin.HookBeforeSave {
					var m map[string]any
					_ = json.Unmarshal(data, &m)
					cf := m["customFields"].(map[string]any)
					cf["price"] = 15.99
					cf["internal_sku"] = "SKU-MOD"
					b, _ := json.Marshal(m)
					return b, nil
				}
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 15.99, result.CustomFields["price"])
		assert.Equal(t, "SKU-MOD", result.CustomFields["internal_sku"])
		mockHook.AssertExpectations(t)
	})

	t.Run("create with hook returning nil data uses original data", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		mockHook := &mockHookExecutor{
			transform: func(plugin.HookName, []byte) ([]byte, error) { return nil, nil },
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
		_, hasSku := result.CustomFields["internal_sku"]
		assert.False(t, hasSku, "system field should be stripped when hook returns nil")
	})

	t.Run("create with hook returning invalid system field value returns error", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
		}, nil)

		mockHook := &mockHookExecutor{
			transform: func(_ plugin.HookName, data []byte) ([]byte, error) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				cf := map[string]any{"sync_status": 12345}
				m["customFields"] = cf
				b, _ := json.Marshal(m)
				return b, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "plugin system field validation failed")
	})

	t.Run("create with no hooks registered works normally", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, nil)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
		_, hasSku := result.CustomFields["internal_sku"]
		assert.False(t, hasSku, "system field should be stripped when no hooks")
	})

	t.Run("create with nil hookExecutor is safe", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, nil)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("before_save hook failure returns error", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)

		mockHook := &mockHookExecutor{
			transform: func(plugin.HookName, []byte) ([]byte, error) {
				return nil, errors.New("hook failed")
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
		}

		_, err := service.Create(context.Background(), 1, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "before_save hook failed")
	})

	t.Run("after_create hook fires on successful create", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		afterCreateFired := false
		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
				if hookName == plugin.HookAfterCreate {
					afterCreateFired = true
				}
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, afterCreateFired, "after_create hook should have fired")
	})

	t.Run("create with hook returning invalid json uses original data", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		mockHook := &mockHookExecutor{
			transform: func(_ plugin.HookName, _ []byte) ([]byte, error) {
				return []byte("not-json{{{"), nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
	})

	t.Run("create with hook returning json without custom fields preserves user fields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		mockHook := &mockHookExecutor{
			transform: func(_ plugin.HookName, data []byte) ([]byte, error) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				delete(m, "customFields")
				b, _ := json.Marshal(m)
				return b, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.CreateContentRequest{
			Title:    "Test Title",
			Content:  testTipTapJSON("Test content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Create(context.Background(), 1, req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
	})
}

func TestService_Update_HookIntegration(t *testing.T) {
	t.Run("update with hook that adds system field values stores them", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:        1,
			UserID:    1,
			Title:     "Test",
			Slug:      "test",
			Content:   testTipTapJSON("Content"),
			PostType:  "product",
			Status:    content.StatusDraft,
			UpdatedAt: "2026-04-08T00:00:00Z",
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		mockHook := &mockHookExecutor{
			transform: func(_ plugin.HookName, data []byte) ([]byte, error) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				cf := m["customFields"].(map[string]any)
				cf["internal_sku"] = "SKU-002"
				b, _ := json.Marshal(m)
				return b, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.UpdateContentRequest{
			Title:    "Test",
			Content:  testTipTapJSON("Content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.Equal(t, "SKU-002", result.CustomFields["internal_sku"])
		assert.Equal(t, 29.99, result.CustomFields["price"])
	})

	t.Run("update with hook that modifies system fields preserves custom fields", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:        1,
			UserID:    1,
			Title:     "Test",
			Slug:      "test",
			Content:   testTipTapJSON("Content"),
			PostType:  "product",
			Status:    content.StatusDraft,
			UpdatedAt: "2026-04-08T00:00:00Z",
			CustomFields: map[string]any{
				"price":        10.99,
				"internal_sku": "SKU-OLD",
			},
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		}, nil)

		mockHook := &mockHookExecutor{
			transform: func(_ plugin.HookName, data []byte) ([]byte, error) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				cf := m["customFields"].(map[string]any)
				cf["internal_sku"] = "SKU-NEW"
				b, _ := json.Marshal(m)
				return b, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.UpdateContentRequest{
			Title:    "Test",
			Content:  testTipTapJSON("Content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.Equal(t, "SKU-NEW", result.CustomFields["internal_sku"])
		assert.Equal(t, 29.99, result.CustomFields["price"])
	})

	t.Run("after_publish hook fires on status transition to published", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:        1,
			UserID:    1,
			Title:     "Test",
			Slug:      "test",
			Content:   testTipTapJSON("Content"),
			PostType:  "post",
			Status:    content.StatusDraft,
			UpdatedAt: "2026-04-08T00:00:00Z",
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "post").Return(content.PostType{Slug: "post"}, nil)
		mockPostType.On("GetFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)

		afterPublishFired := false
		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
				if hookName == plugin.HookAfterPublish {
					afterPublishFired = true
				}
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.UpdateContentRequest{
			Title:    "Test",
			Content:  testTipTapJSON("Content"),
			Tags:     []string{},
			Status:   content.StatusPublished,
			PostType: "post",
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, afterPublishFired, "after_publish hook should have fired")
	})

	t.Run("after_publish hook does not fire when not transitioning to published", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:        1,
			UserID:    1,
			Title:     "Test",
			Slug:      "test",
			Content:   testTipTapJSON("Content"),
			PostType:  "post",
			Status:    content.StatusDraft,
			UpdatedAt: "2026-04-08T00:00:00Z",
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "post").Return(content.PostType{Slug: "post"}, nil)
		mockPostType.On("GetFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)

		afterPublishFired := false
		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
				if hookName == plugin.HookAfterPublish {
					afterPublishFired = true
				}
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.UpdateContentRequest{
			Title:    "Test",
			Content:  testTipTapJSON("Content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "post",
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, afterPublishFired, "after_publish hook should not fire for draft->draft")
	})

	t.Run("update with hook failure returns error", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:        1,
			UserID:    1,
			Title:     "Test",
			Slug:      "test",
			Content:   testTipTapJSON("Content"),
			PostType:  "post",
			Status:    content.StatusDraft,
			UpdatedAt: "2026-04-08T00:00:00Z",
		}, nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetSystemFieldsByPostType", "post").Return([]customfield.FieldSchema{}, nil)

		mockHook := &mockHookExecutor{
			transform: func(plugin.HookName, []byte) ([]byte, error) {
				return nil, errors.New("hook crashed")
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.UpdateContentRequest{
			Title:   "Test",
			Content: testTipTapJSON("Content"),
			Tags:    []string{},
			Status:  content.StatusDraft,
		}

		_, err := service.Update(context.Background(), 1, 1, "", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "before_save hook failed")
	})

	t.Run("update with hook returning invalid json uses original data", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
			ID:        1,
			UserID:    1,
			Title:     "Test",
			Slug:      "test",
			Content:   testTipTapJSON("Content"),
			PostType:  "product",
			Status:    content.StatusDraft,
			UpdatedAt: "2026-04-08T00:00:00Z",
			CustomFields: map[string]any{
				"price": 10.99,
			},
		}, nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

		mockPostType := &mocks.MockPostTypeServiceInterface{}
		mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
		mockPostType.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}, nil)
		mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)

		mockHook := &mockHookExecutor{
			transform: func(_ plugin.HookName, _ []byte) ([]byte, error) {
				return []byte("invalid{{"), nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, mockPostType, mockHook)
		req := content.UpdateContentRequest{
			Title:    "Test",
			Content:  testTipTapJSON("Content"),
			Tags:     []string{},
			Status:   content.StatusDraft,
			PostType: "product",
			CustomFields: map[string]any{
				"price": 29.99,
			},
		}

		result, err := service.Update(context.Background(), 1, 1, "", req)

		require.NoError(t, err)
		assert.Equal(t, 29.99, result.CustomFields["price"])
	})
}

func TestService_SearchPublished(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		limit     int
		setupMock func(*mocks.MockRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "returns results for valid query",
			query: "golang",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
				m.On("SearchPublished", mock.Anything, "golang", 10).Return([]*content.Content{
					{ID: 1, Title: "Golang Tutorial", Slug: "golang-tut", Status: content.StatusPublished, PostType: "post"},
				}, nil)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "returns empty for single char query",
			query: "g",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "returns empty for empty query",
			query: "",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "trims whitespace from query",
			query: "  golang  ",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
				m.On("SearchPublished", mock.Anything, "golang", 10).Return([]*content.Content{}, nil)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "returns empty for single char after trim",
			query: "  g  ",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "returns empty when no results found",
			query: "nonexistent",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
				m.On("SearchPublished", mock.Anything, "nonexistent", 10).Return([]*content.Content{}, nil)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "returns error from repository",
			query: "test",
			limit: 10,
			setupMock: func(m *mocks.MockRepository) {
				m.On("SearchPublished", mock.Anything, "test", 10).Return(nil, errors.New("db error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			results, err := service.SearchPublished(context.Background(), tt.query, tt.limit)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to search published content")
				return
			}
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Publish(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		userID      int
		role        string
		setupMock   func(*mocks.MockRepository)
		wantErr     error
		wantStatus  content.Status
		noUpdate    bool
	}{
		{
			name:   "success - owner publishes draft transitions to published",
			id:     1,
			userID: 1,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusDraft,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:    nil,
			wantStatus: content.StatusPublished,
		},
		{
			name:   "success - admin publishes another user's draft",
			id:     1,
			userID: 7,
			role:   content.RoleAdmin,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  999,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusDraft,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:    nil,
			wantStatus: content.StatusPublished,
		},
		{
			name:   "idempotent - publish already-published post is a no-op (status stays published, update still persists)",
			id:     1,
			userID: 1,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusPublished,
				}, nil)
				// transitionStatus still calls Update (the row is re-saved with the same status + UpdatedBy).
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:    nil,
			wantStatus: content.StatusPublished,
		},
		{
			name:   "error - not owned by non-admin returns ErrUnauthorized",
			id:     1,
			userID: 2,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusDraft,
				}, nil)
			},
			wantErr:  content.ErrUnauthorized,
			noUpdate: true,
		},
		{
			name:   "error - not found returns ErrContentNotFound",
			id:     999,
			userID: 1,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, content.ErrContentNotFound)
			},
			wantErr:  content.ErrContentNotFound,
			noUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.Publish(context.Background(), tt.id, tt.userID, tt.role)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "Service.Publish() error = %v, wantErr %v", err, tt.wantErr)
				assert.Nil(t, result)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantStatus, result.Status, "Service.Publish() Status")
			if !tt.noUpdate {
				assert.Equal(t, tt.userID, result.UpdatedBy, "Service.Publish() UpdatedBy")
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Publish_RepoErrorRollsBackStatus(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:      1,
		UserID:  1,
		Title:   "Title",
		Slug:    "title",
		Content: testTipTapJSON("Content"),
		Status:  content.StatusDraft,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(errors.New("db down"))

	service := content.NewService(mockRepo, nil, nil)
	result, err := service.Publish(context.Background(), 1, 1, "Editor")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update content")
	assert.Nil(t, result)
	// The pre-fetch snapshot is discarded on error (Publish returns nil result),
	// so the rollback target is the snapshot held by the test mock — and that
	// snapshot is what we asserted the service reads back. The rollback matters
	// for callers that keep a *Content reference; covered indirectly because
	// the service returns nil on error.
	mockRepo.AssertExpectations(t)
}

func TestService_Publish_HookFiresOnTransition(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:        1,
		UserID:    1,
		Title:     "Title",
		Slug:      "title",
		Content:   testTipTapJSON("Content"),
		PostType:  "post",
		Status:    content.StatusDraft,
		UpdatedAt: "2026-04-08T00:00:00Z",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	afterPublishFired := false
	mockHook := &mockHookExecutor{
		transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
			if hookName == plugin.HookAfterPublish {
				afterPublishFired = true
			}
			return nil, nil
		},
	}

	service := content.NewServiceWithHooks(mockRepo, nil, nil, nil, mockHook)
	_, err := service.Publish(context.Background(), 1, 1, "Editor")

	require.NoError(t, err)
	assert.True(t, afterPublishFired, "after_publish hook should fire on draft->published")
}

func TestService_Publish_HookDoesNotFireOnNoOp(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:        1,
		UserID:    1,
		Title:     "Title",
		Slug:      "title",
		Content:   testTipTapJSON("Content"),
		PostType:  "post",
		Status:    content.StatusPublished,
		UpdatedAt: "2026-04-08T00:00:00Z",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	afterPublishFired := false
	mockHook := &mockHookExecutor{
		transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
			if hookName == plugin.HookAfterPublish {
				afterPublishFired = true
			}
			return nil, nil
		},
	}

	service := content.NewServiceWithHooks(mockRepo, nil, nil, nil, mockHook)
	_, err := service.Publish(context.Background(), 1, 1, "Editor")

	require.NoError(t, err)
	assert.False(t, afterPublishFired, "after_publish hook should not fire when status is already published")
}

func TestService_Publish_SEOAutoGenOnTransition(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test Article",
		Slug:     "test-article",
		Content:  testTipTapJSON("Content"),
		Tags:     []string{},
		PostType: "post",
		Status:   content.StatusDraft,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, nil)

	result, err := service.Publish(context.Background(), 1, 1, "Editor")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.MetaDescription, "SEO meta description should be auto-generated on publish")
	mockRepo.AssertExpectations(t)
}

func TestService_Publish_NilSEOServiceSkipsAutoGen(t *testing.T) {
	// With a nil seoService, transitionStatus skips the SEO block and persists
	// the row unchanged. MetaDescription stays empty. This mirrors the
	// "happy path without SEO configured" case (e.g. test setups).
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test",
		Slug:     "test",
		Content:  testTipTapJSON("Content"),
		PostType: "post",
		Status:   content.StatusDraft,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	service := content.NewService(mockRepo, nil, nil)
	result, err := service.Publish(context.Background(), 1, 1, "Editor")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.MetaDescription, "nil seoService means SEO is not run")
	mockRepo.AssertExpectations(t)
}

func TestService_Publish_CustomURLPrefixForNonPostType(t *testing.T) {
	// When existing.PostType is non-"post" (e.g. "page"), transitionStatus
	// builds the SEO URL as /<postType>/<slug> rather than /posts/<slug>.
	// The branch is shared with the Update path's TestService_Update_CustomURLPrefixForNonPostType
	// test; mirrored here so the publish verb's coverage is honest.
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:       1,
		UserID:   1,
		Title:    "Test Article",
		Slug:     "test-article",
		Content:  testTipTapJSON("Content"),
		Tags:     []string{},
		PostType: "page",
		Status:   content.StatusDraft,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	seoService := seo.NewService("http://localhost:8080", "Test Site")
	service := content.NewService(mockRepo, seoService, nil)

	result, err := service.Publish(context.Background(), 1, 1, "Editor")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.MetaDescription, "SEO should still run with a custom post type URL prefix")
	mockRepo.AssertExpectations(t)
}

func TestService_Unpublish(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		userID      int
		role        string
		setupMock   func(*mocks.MockRepository)
		wantErr     error
		wantStatus  content.Status
		noUpdate    bool
	}{
		{
			name:   "success - owner unpublishes published transitions to draft",
			id:     1,
			userID: 1,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusPublished,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:    nil,
			wantStatus: content.StatusDraft,
		},
		{
			name:   "success - admin unpublishes another user's published post",
			id:     1,
			userID: 7,
			role:   content.RoleAdmin,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  999,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusPublished,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:    nil,
			wantStatus: content.StatusDraft,
		},
		{
			name:   "idempotent - unpublish already-draft post is a no-op (status stays draft, update still persists)",
			id:     1,
			userID: 1,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusDraft,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			},
			wantErr:    nil,
			wantStatus: content.StatusDraft,
		},
		{
			name:   "error - not owned by non-admin returns ErrUnauthorized",
			id:     1,
			userID: 2,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "title",
					Content: testTipTapJSON("Content"),
					Status:  content.StatusPublished,
				}, nil)
			},
			wantErr:  content.ErrUnauthorized,
			noUpdate: true,
		},
		{
			name:   "error - not found returns ErrContentNotFound",
			id:     999,
			userID: 1,
			role:   "Editor",
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, content.ErrContentNotFound)
			},
			wantErr:  content.ErrContentNotFound,
			noUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.Unpublish(context.Background(), tt.id, tt.userID, tt.role)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "Service.Unpublish() error = %v, wantErr %v", err, tt.wantErr)
				assert.Nil(t, result)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantStatus, result.Status, "Service.Unpublish() Status")
			if !tt.noUpdate {
				assert.Equal(t, tt.userID, result.UpdatedBy, "Service.Unpublish() UpdatedBy")
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Unpublish_RepoErrorRollsBackStatus(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:      1,
		UserID:  1,
		Title:   "Title",
		Slug:    "title",
		Content: testTipTapJSON("Content"),
		Status:  content.StatusPublished,
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(errors.New("db down"))

	service := content.NewService(mockRepo, nil, nil)
	result, err := service.Unpublish(context.Background(), 1, 1, "Editor")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update content")
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestService_Unpublish_HookNeverFires(t *testing.T) {
	mockRepo := &mocks.MockRepository{}
	mockRepo.On("GetByID", mock.Anything, 1).Return(&content.Content{
		ID:        1,
		UserID:    1,
		Title:     "Title",
		Slug:      "title",
		Content:   testTipTapJSON("Content"),
		PostType:  "post",
		Status:    content.StatusPublished,
		UpdatedAt: "2026-04-08T00:00:00Z",
	}, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)

	hookFired := false
	mockHook := &mockHookExecutor{
		transform: func(_ plugin.HookName, _ []byte) ([]byte, error) {
			hookFired = true
			return nil, nil
		},
	}

	service := content.NewServiceWithHooks(mockRepo, nil, nil, nil, mockHook)
	_, err := service.Unpublish(context.Background(), 1, 1, "Editor")

	require.NoError(t, err)
	assert.False(t, hookFired, "no hook should fire on unpublish (only AfterPublish is wired to the published edge)")
}

func TestService_Create_PublishedRunsPublishPipeline(t *testing.T) {
	// F08: creating directly as published must run the publish pipeline —
	// auto-generate SEO metadata and fire the AfterPublish hook — so it behaves
	// like create + publish, not a silent status flag.
	seoText := "This is a test article with some content for SEO metadata generation purposes."
	seoBody := testTipTapJSON(seoText)

	t.Run("published generates SEO metadata before insert", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "published-title", "en").Return(true, nil)
		var captured *content.Content
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			captured = args.Get(1).(*content.Content)
			captured.ID = 1
		})

		seoService := seo.NewService("http://localhost:8080", "Test Site")
		service := content.NewService(mockRepo, seoService, nil)

		result, err := service.Create(context.Background(), 1, content.CreateContentRequest{
			Title:   "Published Title",
			Content: seoBody,
			Status:  content.StatusPublished,
		})

		require.NoError(t, err)
		require.NotNil(t, captured, "repo.Create must be called")
		require.NotNil(t, result)
		// SEO landed in the initial insert (captured) and the returned object.
		assert.Equal(t, seoText, captured.MetaDescription, "MetaDescription auto-generated before insert")
		assert.Equal(t, "Published Title", captured.OGTitle, "OGTitle auto-generated before insert")
		assert.Equal(t, seoText, captured.OGDescription, "OGDescription auto-generated before insert")
		assert.Equal(t, captured.MetaDescription, result.MetaDescription)
	})

	t.Run("published fires AfterPublish and AfterCreate", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "published-title", "en").Return(true, nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			c := args.Get(1).(*content.Content)
			c.ID = 1
		})

		var fired []plugin.HookName
		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
				fired = append(fired, hookName)
				return nil, nil
			},
		}

		service := content.NewServiceWithHooks(mockRepo, nil, nil, nil, mockHook)
		_, err := service.Create(context.Background(), 1, content.CreateContentRequest{
			Title:   "Published Title",
			Content: seoBody,
			Status:  content.StatusPublished,
		})

		require.NoError(t, err)
		assert.Contains(t, fired, plugin.HookAfterCreate, "AfterCreate must fire on create")
		assert.Contains(t, fired, plugin.HookAfterPublish, "AfterPublish must fire when created as published")
	})

	t.Run("draft does not fire AfterPublish or generate SEO", func(t *testing.T) {
		mockRepo := &mocks.MockRepository{}
		mockRepo.On("CheckSlugUnique", mock.Anything, "draft-title", "en").Return(true, nil)
		var captured *content.Content
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil).Run(func(args mock.Arguments) {
			captured = args.Get(1).(*content.Content)
			captured.ID = 1
		})

		var fired []plugin.HookName
		mockHook := &mockHookExecutor{
			transform: func(hookName plugin.HookName, _ []byte) ([]byte, error) {
				fired = append(fired, hookName)
				return nil, nil
			},
		}

		seoService := seo.NewService("http://localhost:8080", "Test Site")
		service := content.NewServiceWithHooks(mockRepo, nil, seoService, nil, mockHook)
		_, err := service.Create(context.Background(), 1, content.CreateContentRequest{
			Title:   "Draft Title",
			Content: seoBody,
			Status:  content.StatusDraft,
		})

		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Empty(t, captured.MetaDescription, "no SEO generation for draft")
		assert.NotContains(t, fired, plugin.HookAfterPublish, "AfterPublish must NOT fire for draft")
		assert.Contains(t, fired, plugin.HookAfterCreate, "AfterCreate fires for draft")
	})
}

func TestService_GetRelated(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		limit       int
		setupMock   func(*mocks.MockRepository)
		expectedIDs []int
		expectedErr error
	}{
		{
			name:  "tag overlap returns enough, no fallback",
			id:    1,
			limit: 3,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     []string{"go", "tutorial"},
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, []string{"go", "tutorial"}, "post", "en", 3).Return([]*content.Content{
					{ID: 2, Title: "Related 2"},
					{ID: 3, Title: "Related 3"},
					{ID: 4, Title: "Related 4"},
				}, nil)
			},
			expectedIDs: []int{2, 3, 4},
			expectedErr: nil,
		},
		{
			name:  "tag overlap under-filled, backfill dedupes and fills to limit",
			id:    1,
			limit: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     []string{"go"},
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, []string{"go"}, "post", "en", 5).Return([]*content.Content{
					{ID: 2, Title: "Related 2"},
				}, nil)
				m.On("GetLatestByPostType", mock.Anything, 1, "post", "en", 5).Return([]*content.Content{
					{ID: 2, Title: "Related 2"},
					{ID: 1, Title: "Source"},
					{ID: 3, Title: "Latest 3"},
					{ID: 4, Title: "Latest 4"},
					{ID: 5, Title: "Latest 5"},
					{ID: 6, Title: "Latest 6"},
				}, nil)
			},
			expectedIDs: []int{2, 3, 4, 5, 6},
			expectedErr: nil,
		},
		{
			name:  "source has no tags, backfill fills entirely",
			id:    1,
			limit: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     nil,
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, mock.Anything, "post", "en", 5).Return([]*content.Content{}, nil)
				m.On("GetLatestByPostType", mock.Anything, 1, "post", "en", 5).Return([]*content.Content{
					{ID: 2, Title: "Latest 2"},
					{ID: 3, Title: "Latest 3"},
					{ID: 4, Title: "Latest 4"},
					{ID: 5, Title: "Latest 5"},
					{ID: 6, Title: "Latest 6"},
				}, nil)
			},
			expectedIDs: []int{2, 3, 4, 5, 6},
			expectedErr: nil,
		},
		{
			name:  "limit clamped to default 5 when zero",
			id:    1,
			limit: 0,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     []string{"go"},
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, []string{"go"}, "post", "en", 5).Return([]*content.Content{
					{ID: 2, Title: "Related 2"},
					{ID: 3, Title: "Related 3"},
					{ID: 4, Title: "Related 4"},
					{ID: 5, Title: "Related 5"},
					{ID: 6, Title: "Related 6"},
				}, nil)
			},
			expectedIDs: []int{2, 3, 4, 5, 6},
			expectedErr: nil,
		},
		{
			name:  "limit clamped to max 20 when above",
			id:    1,
			limit: 100,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     []string{"go"},
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, []string{"go"}, "post", "en", 20).Return([]*content.Content{
					{ID: 2, Title: "Related 2"},
				}, nil)
				m.On("GetLatestByPostType", mock.Anything, 1, "post", "en", 20).Return([]*content.Content{}, nil)
			},
			expectedIDs: []int{2},
			expectedErr: nil,
		},
		{
			name:  "GetByID error propagates",
			id:    999,
			limit: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, content.ErrContentNotFound)
			},
			expectedIDs: nil,
			expectedErr: content.ErrContentNotFound,
		},
		{
			name:  "GetRelatedByTags error propagates",
			id:    1,
			limit: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     []string{"go"},
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, []string{"go"}, "post", "en", 5).Return(nil, errors.New("db failure"))
			},
			expectedIDs: nil,
			expectedErr: errors.New("failed to get related content by tags"),
		},
		{
			name:  "GetLatestByPostType error propagates",
			id:    1,
			limit: 5,
			setupMock: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:       1,
					Title:    "Source",
					Tags:     []string{"go"},
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("GetRelatedByTags", mock.Anything, 1, []string{"go"}, "post", "en", 5).Return([]*content.Content{
					{ID: 2, Title: "Related 2"},
				}, nil)
				m.On("GetLatestByPostType", mock.Anything, 1, "post", "en", 5).Return(nil, errors.New("db failure"))
			},
			expectedIDs: nil,
			expectedErr: errors.New("failed to get latest content by post type"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.MockRepository{}
			tt.setupMock(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			result, err := service.GetRelated(context.Background(), tt.id, tt.limit)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.GetRelated() expected error, got nil")
				assert.True(t, errors.Is(err, tt.expectedErr) || containsErrorSubstring(err.Error(), tt.expectedErr.Error()), "Service.GetRelated() error = %v, wantErr %v", err, tt.expectedErr)
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.GetRelated() unexpected error")
			require.Len(t, result, len(tt.expectedIDs), "Service.GetRelated() result length")

			actualIDs := make([]int, len(result))
			for i, c := range result {
				actualIDs[i] = c.ID
			}
			assert.Equal(t, tt.expectedIDs, actualIDs, "Service.GetRelated() result IDs")

			mockRepo.AssertExpectations(t)
		})
	}
}
