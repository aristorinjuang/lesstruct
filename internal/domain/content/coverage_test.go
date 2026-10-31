package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/identity"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	seo "github.com/aristorinjuang/lesstruct/modules/seo/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_SlugExists(t *testing.T) {
	tests := []struct {
		name     string
		slug     string
		language string
		setup    func(*mocks.MockRepository)
		want     bool
		wantErr  bool
	}{
		{
			name: "empty slug is never reported as existing",
			slug: "   ",
			want: false,
		},
		{
			name: "defaults to english and reports an existing slug",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "hello-world", "en").Return(false, nil)
			},
			want: true,
		},
		{
			name:     "explicit language and reports a free slug",
			slug:     "halo-dunia",
			language: "id",
			setup: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "halo-dunia", "id").Return(true, nil)
			},
			want: false,
		},
		{
			name: "repository failure propagates",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "hello-world", "en").Return(false, errors.New("database unavailable"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			service := content.NewService(mockRepo, nil, nil)

			exists, err := service.SlugExists(context.Background(), tt.slug, tt.language)

			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, exists)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, exists)
		})
	}
}

func TestService_GetBySlugAndLanguage(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		language    string
		setup       func(*mocks.MockRepository)
		want        *contentdomain.Content
		wantErrIs   error
		errContains string
	}{
		{
			name:      "empty slug is not found",
			slug:      "  ",
			wantErrIs: contentdomain.ErrContentNotFound,
		},
		{
			name: "success - defaults to english",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("GetBySlug", mock.Anything, "hello-world", "en").Return(&contentdomain.Content{ID: 1, Slug: "hello-world"}, nil)
			},
			want: &contentdomain.Content{ID: 1, Slug: "hello-world"},
		},
		{
			name:     "success - explicit language",
			slug:     "halo-dunia",
			language: "id",
			setup: func(m *mocks.MockRepository) {
				m.On("GetBySlug", mock.Anything, "halo-dunia", "id").Return(&contentdomain.Content{ID: 2, Slug: "halo-dunia"}, nil)
			},
			want: &contentdomain.Content{ID: 2, Slug: "halo-dunia"},
		},
		{
			name: "error - repository failure propagates",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("GetBySlug", mock.Anything, "hello-world", "en").Return(nil, errors.New("database unavailable"))
			},
			errContains: "database unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetBySlugAndLanguage(context.Background(), tt.slug, tt.language)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				return
			}
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_GetAll(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		want        []*contentdomain.Content
		errContains string
	}{
		{
			name: "success - returns the content page",
			setup: func(m *mocks.MockRepository) {
				m.On("GetAll", mock.Anything, 10, 20).Return([]*contentdomain.Content{{ID: 1}, {ID: 2}}, nil)
			},
			want: []*contentdomain.Content{{ID: 1}, {ID: 2}},
		},
		{
			name: "error - repository failure is wrapped",
			setup: func(m *mocks.MockRepository) {
				m.On("GetAll", mock.Anything, 10, 20).Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get all content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetAll(context.Background(), 10, 20)

			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_GetPublished(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		want        []*contentdomain.Content
		errContains string
	}{
		{
			name: "success - returns published content",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublished", mock.Anything, 5, 0).Return([]*contentdomain.Content{{ID: 7}}, nil)
			},
			want: []*contentdomain.Content{{ID: 7}},
		},
		{
			name: "error - repository failure is wrapped",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublished", mock.Anything, 5, 0).Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get published content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetPublished(context.Background(), 5, 0)

			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_GetTranslations(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		want        []*contentdomain.Content
		errContains string
	}{
		{
			name: "success - passes the group through",
			setup: func(m *mocks.MockRepository) {
				m.On("GetTranslations", mock.Anything, 7, 9).Return([]*contentdomain.Content{{ID: 9}}, nil)
			},
			want: []*contentdomain.Content{{ID: 9}},
		},
		{
			name: "error - repository failure passes through unwrapped",
			setup: func(m *mocks.MockRepository) {
				m.On("GetTranslations", mock.Anything, 7, 9).Return(nil, errors.New("database unavailable"))
			},
			errContains: "database unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetTranslations(context.Background(), 7, 9)

			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_GetPublishedBySlug(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		language    string
		setup       func(*mocks.MockRepository)
		want        *contentdomain.Content
		wantErrIs   error
		errContains string
	}{
		{
			name: "success - defaults to english",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "hello-world", "en").Return(&contentdomain.Content{ID: 1}, nil)
			},
			want: &contentdomain.Content{ID: 1},
		},
		{
			name:     "success - explicit language",
			slug:     "halo-dunia",
			language: "id",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "halo-dunia", "id").Return(&contentdomain.Content{ID: 2}, nil)
			},
			want: &contentdomain.Content{ID: 2},
		},
		{
			name:      "error - invalid slug is rejected before the lookup",
			slug:      "Invalid Slug",
			wantErrIs: contentdomain.ErrInvalidSlug,
		},
		{
			name: "error - repository failure is wrapped",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "hello-world", "en").Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get published content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetPublishedBySlug(context.Background(), tt.slug, tt.language)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				return
			}
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_GetPublishedBySlugAny(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		setup       func(*mocks.MockRepository)
		want        *contentdomain.Content
		wantErrIs   error
		errContains string
	}{
		{
			name: "success - returns the published item",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedBySlugAny", mock.Anything, "hello-world").Return(&contentdomain.Content{ID: 1}, nil)
			},
			want: &contentdomain.Content{ID: 1},
		},
		{
			name:      "error - invalid slug is rejected before the lookup",
			slug:      "Invalid Slug",
			wantErrIs: contentdomain.ErrInvalidSlug,
		},
		{
			name: "error - repository failure is wrapped",
			slug: "hello-world",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedBySlugAny", mock.Anything, "hello-world").Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get published content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetPublishedBySlugAny(context.Background(), tt.slug)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				return
			}
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_GetPublishedByID(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		want        *contentdomain.Content
		wantErrIs   error
		errContains string
	}{
		{
			name: "success - published content is returned",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&contentdomain.Content{ID: 1, Status: contentdomain.StatusPublished}, nil)
			},
			want: &contentdomain.Content{ID: 1, Status: contentdomain.StatusPublished},
		},
		{
			name: "error - draft content is reported as not found",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&contentdomain.Content{ID: 1, Status: contentdomain.StatusDraft}, nil)
			},
			wantErrIs: contentdomain.ErrContentNotFound,
		},
		{
			name: "error - repository failure is wrapped",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get published content by id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetPublishedByID(context.Background(), 1)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				return
			}
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestService_AuthorExists(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		want        bool
		errContains string
	}{
		{
			name: "success - existing author",
			setup: func(m *mocks.MockRepository) {
				m.On("AuthorExists", mock.Anything, "alice").Return(true, nil)
			},
			want: true,
		},
		{
			name: "success - unknown author",
			setup: func(m *mocks.MockRepository) {
				m.On("AuthorExists", mock.Anything, "alice").Return(false, nil)
			},
			want: false,
		},
		{
			name: "error - repository failure passes through unwrapped",
			setup: func(m *mocks.MockRepository) {
				m.On("AuthorExists", mock.Anything, "alice").Return(false, errors.New("database unavailable"))
			},
			errContains: "database unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			exists, err := service.AuthorExists(context.Background(), "alice")

			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.False(t, exists)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, exists)
		})
	}
}

func TestService_GetPublishedAuthor(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		want        *contentdomain.PublishedAuthor
		errContains string
	}{
		{
			name: "success - returns the public profile",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedAuthor", mock.Anything, "alice").Return(&contentdomain.PublishedAuthor{Username: "alice", ContentCount: 3}, nil)
			},
			want: &contentdomain.PublishedAuthor{Username: "alice", ContentCount: 3},
		},
		{
			name: "error - repository failure is wrapped",
			setup: func(m *mocks.MockRepository) {
				m.On("GetPublishedAuthor", mock.Anything, "alice").Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get published author",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.GetPublishedAuthor(context.Background(), "alice")

			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestNewServiceWithHooks_AppliesOptions(t *testing.T) {
	tests := []struct {
		name         string
		options      []content.Option
		wantLanguage string
	}{
		{
			name:         "applies the default language option",
			options:      []content.Option{content.WithDefaultLanguage("id")},
			wantLanguage: "id",
		},
		{
			name:         "keeps english when no option overrides it",
			wantLanguage: "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.On("CheckSlugUnique", mock.Anything, "hello-world", tt.wantLanguage).Return(true, nil)

			service := content.NewServiceWithHooks(mockRepo, nil, nil, nil, tt.options...)

			exists, err := service.SlugExists(context.Background(), "hello-world", "")

			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestService_Create_EdgeCases(t *testing.T) {
	translationGroupID := 5
	longOGTitle := strings.Repeat("x", 61)

	expectCreated := func(m *mocks.MockRepository) {
		m.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
		m.On("Create", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
	}

	tests := []struct {
		name        string
		req         contentdomain.CreateContentRequest
		opts        []content.Option
		setup       func(*mocks.MockRepository)
		seoService  *seo.Service
		wantErrIs   error
		errContains string
		validate    func(*testing.T, *contentdomain.Content)
	}{
		{
			name: "error - invalid format",
			req: contentdomain.CreateContentRequest{
				Title:   "Test Title",
				Content: testTipTapJSON("Body"),
				Tags:    []string{},
				Status:  contentdomain.StatusDraft,
				Format:  "bogus",
			},
			errContains: "format validation failed",
		},
		{
			name: "error - translation group lookup fails",
			req: contentdomain.CreateContentRequest{
				Title:              "Test Title",
				Content:            testTipTapJSON("Body"),
				Tags:               []string{},
				Status:             contentdomain.StatusDraft,
				TranslationGroupID: &translationGroupID,
			},
			setup: func(m *mocks.MockRepository) {
				m.On("TranslationGroupExists", mock.Anything, translationGroupID).Return(false, errors.New("database unavailable"))
			},
			errContains: "failed to check translation group",
		},
		{
			name: "error - translation group does not exist",
			req: contentdomain.CreateContentRequest{
				Title:              "Test Title",
				Content:            testTipTapJSON("Body"),
				Tags:               []string{},
				Status:             contentdomain.StatusDraft,
				TranslationGroupID: &translationGroupID,
			},
			setup: func(m *mocks.MockRepository) {
				m.On("TranslationGroupExists", mock.Anything, translationGroupID).Return(false, nil)
			},
			wantErrIs: contentdomain.ErrTranslationGroupNotFound,
		},
		{
			name: "error - invalid featured image",
			req: contentdomain.CreateContentRequest{
				Title:         "Test Title",
				Content:       testTipTapJSON("Body"),
				Tags:          []string{},
				Status:        contentdomain.StatusDraft,
				FeaturedImage: "javascript:alert(1)",
			},
			setup: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
			},
			wantErrIs: contentdomain.ErrInvalidFeaturedImage,
		},
		{
			name: "error - SEO generation fails when creating as published",
			req: contentdomain.CreateContentRequest{
				Title:   "Test Title",
				Content: testTipTapJSON("Body"),
				Tags:    []string{},
				Status:  contentdomain.StatusPublished,
				OGTitle: longOGTitle,
			},
			setup: func(m *mocks.MockRepository) {
				m.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)
			},
			seoService:  seo.NewService("http://localhost:8080", "Test Site"),
			errContains: "failed to generate SEO metadata",
		},
		{
			name: "success - comments enabled honors the request opt-out",
			req: contentdomain.CreateContentRequest{
				Title:         "Test Title",
				Content:       testTipTapJSON("Body"),
				Tags:          []string{},
				Status:        contentdomain.StatusDraft,
				PostType:      "post",
				AllowComments: new(false),
			},
			opts:  []content.Option{content.WithCommentsEnabled(true)},
			setup: expectCreated,
			validate: func(t *testing.T, c *contentdomain.Content) {
				assert.False(t, c.AllowComments)
			},
		},
		{
			name: "success - comments enabled defaults pages to disabled",
			req: contentdomain.CreateContentRequest{
				Title:    "Test Title",
				Content:  testTipTapJSON("Body"),
				Tags:     []string{},
				Status:   contentdomain.StatusDraft,
				PostType: "page",
			},
			opts:  []content.Option{content.WithCommentsEnabled(true)},
			setup: expectCreated,
			validate: func(t *testing.T, c *contentdomain.Content) {
				assert.False(t, c.AllowComments)
			},
		},
		{
			name: "success - comments enabled honors the request opt-in",
			req: contentdomain.CreateContentRequest{
				Title:         "Test Title",
				Content:       testTipTapJSON("Body"),
				Tags:          []string{},
				Status:        contentdomain.StatusDraft,
				PostType:      "post",
				AllowComments: new(true),
			},
			opts:  []content.Option{content.WithCommentsEnabled(true)},
			setup: expectCreated,
			validate: func(t *testing.T, c *contentdomain.Content) {
				assert.True(t, c.AllowComments)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			service := content.NewService(mockRepo, tt.seoService, nil, tt.opts...)

			result, err := service.Create(
				context.Background(),
				identity.New(1, "admin", contentdomain.RoleAdmin),
				tt.req,
			)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				return
			}
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.validate(t, result)
		})
	}
}

func TestService_Create_FieldValueValidation(t *testing.T) {
	tests := []struct {
		name        string
		fields      []customfield.FieldSchema
		custom      map[string]any
		errContains string
	}{
		{
			name: "error - select field without configured options",
			fields: []customfield.FieldSchema{
				{Name: "Status", Slug: "status", Type: customfield.FieldTypeSelect},
			},
			custom:      map[string]any{"status": "draft"},
			errContains: "Status: has no configured options",
		},
		{
			name: "error - unknown field type",
			fields: []customfield.FieldSchema{
				{Name: "Mystery", Slug: "mystery", Type: "mystery"},
			},
			custom:      map[string]any{"mystery": "value"},
			errContains: "Mystery: unknown field type: mystery",
		},
		{
			name: "error - non-empty boolean value in a required text field",
			fields: []customfield.FieldSchema{
				{Name: "Summary", Slug: "summary", Type: customfield.FieldTypeText, Required: true},
			},
			custom:      map[string]any{"summary": true},
			errContains: "Summary: must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

			mockPostType := mocks.NewMockPostTypeServiceInterface(t)
			mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
			mockPostType.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
			mockPostType.On("GetFieldsByPostType", "product").Return(tt.fields, nil)

			service := content.NewService(mockRepo, nil, mockPostType)

			_, err := service.Create(
				context.Background(),
				identity.New(1, "admin", contentdomain.RoleAdmin),
				contentdomain.CreateContentRequest{
					Title:        "Test Title",
					Content:      testTipTapJSON("Body"),
					Tags:         []string{},
					Status:       contentdomain.StatusDraft,
					PostType:     "product",
					CustomFields: tt.custom,
				},
			)

			require.Error(t, err)
			assert.ErrorIs(t, err, contentdomain.ErrCustomFieldValidation)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestService_Create_PluginSystemFieldValidation(t *testing.T) {
	schemas := []customfield.FieldSchema{
		{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
	}

	tests := []struct {
		name        string
		setup       func(*mocks.MockPostTypeServiceInterface)
		wantErrIs   error
		errContains string
	}{
		{
			name: "error - schema lookup fails during plugin validation",
			setup: func(m *mocks.MockPostTypeServiceInterface) {
				m.On("GetSystemFieldsByPostType", "product").Return(schemas, nil).Once()
				m.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema(nil), errors.New("registry unavailable")).Once()
			},
			errContains: "failed to get system field schemas",
		},
		{
			name: "error - plugin-set key is not a known system field",
			setup: func(m *mocks.MockPostTypeServiceInterface) {
				m.On("GetSystemFieldsByPostType", "product").Return(schemas, nil).Once()
				m.On("GetSystemFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil).Once()
			},
			wantErrIs: contentdomain.ErrUnknownSystemFieldKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.On("CheckSlugUnique", mock.Anything, "test-title", "en").Return(true, nil)

			mockPostType := mocks.NewMockPostTypeServiceInterface(t)
			mockPostType.On("GetBySlug", "product").Return(content.PostType{Slug: "product"}, nil)
			tt.setup(mockPostType)

			hook := &mockHookExecutor{
				transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
					if hookName != plugin.HookBeforeSave {
						return nil, nil
					}
					var payload map[string]any
					if err := json.Unmarshal(data, &payload); err != nil {
						return nil, err
					}
					customFields, _ := payload["customFields"].(map[string]any)
					if customFields == nil {
						customFields = make(map[string]any)
						payload["customFields"] = customFields
					}
					customFields["internal_sku"] = "SKU-001"
					return json.Marshal(payload)
				},
			}

			service := content.NewServiceWithHooks(mockRepo, nil, mockPostType, hook)

			_, err := service.Create(
				context.Background(),
				identity.New(1, "admin", contentdomain.RoleAdmin),
				contentdomain.CreateContentRequest{
					Title:    "Test Title",
					Content:  testTipTapJSON("Body"),
					Tags:     []string{},
					Status:   contentdomain.StatusDraft,
					PostType: "product",
				},
			)

			require.Error(t, err)
			if tt.wantErrIs != nil {
				assert.ErrorIs(t, err, tt.wantErrIs)
			}
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestService_SetSystemFields_Errors(t *testing.T) {
	newExisting := func() *contentdomain.Content {
		return &contentdomain.Content{
			ID:       1,
			UserID:   1,
			Title:    "Test",
			Slug:     "test",
			Content:  testTipTapJSON("Body"),
			PostType: "product",
			Status:   contentdomain.StatusDraft,
		}
	}
	schemas := []customfield.FieldSchema{
		{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
	}

	tests := []struct {
		name          string
		setupRepo     func(*mocks.MockRepository)
		setupPostType func(*mocks.MockPostTypeServiceInterface)
		errContains   string
	}{
		{
			name: "error - content lookup fails",
			setupRepo: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(nil, errors.New("database unavailable"))
			},
			errContains: "failed to get content",
		},
		{
			name: "error - system field schema lookup fails",
			setupRepo: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(newExisting(), nil)
			},
			setupPostType: func(m *mocks.MockPostTypeServiceInterface) {
				m.On("GetSystemFieldsByPostType", "product").Return(nil, errors.New("registry unavailable"))
			},
			errContains: "failed to get system field schemas",
		},
		{
			name: "error - repository update fails",
			setupRepo: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(newExisting(), nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(errors.New("database unavailable"))
			},
			setupPostType: func(m *mocks.MockPostTypeServiceInterface) {
				m.On("GetSystemFieldsByPostType", "product").Return(schemas, nil)
			},
			errContains: "failed to update content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setupRepo(mockRepo)

			mockPostType := mocks.NewMockPostTypeServiceInterface(t)
			if tt.setupPostType != nil {
				tt.setupPostType(mockPostType)
			}

			service := content.NewService(mockRepo, nil, mockPostType)

			result, err := service.SetSystemFields(
				context.Background(),
				1,
				identity.New(1, "admin", contentdomain.RoleAdmin),
				map[string]any{"internal_sku": "SKU-001"},
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, result)
		})
	}
}

func TestService_Update_EdgeCases(t *testing.T) {
	validBody := testTipTapJSON("Body")
	draftContent := func(mutate func(*contentdomain.Content)) *contentdomain.Content {
		c := &contentdomain.Content{
			ID:       1,
			UserID:   1,
			Title:    "Test",
			Slug:     "test",
			Content:  validBody,
			Tags:     []string{},
			Status:   contentdomain.StatusDraft,
			Format:   contentdomain.FormatTiptap,
			PostType: "post",
			Language: "en",
		}
		if mutate != nil {
			mutate(c)
		}
		return c
	}
	schemas := []customfield.FieldSchema{
		{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
		{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"synced", "pending"}},
	}
	updateBody := content.UpdateContentRequest{
		Title:   "Test",
		Content: validBody,
		Tags:    []string{},
		Status:  contentdomain.StatusDraft,
	}

	tests := []struct {
		name        string
		existing    *contentdomain.Content
		req         content.UpdateContentRequest
		opts        []content.Option
		transform   func(plugin.HookName, []byte) ([]byte, error)
		setupRepo   func(*mocks.MockRepository)
		setupPT     func(*mocks.MockPostTypeServiceInterface)
		wantUpdate  bool
		wantErrIs   error
		errContains string
		validate    func(*testing.T, *contentdomain.Content)
	}{
		{
			name:        "error - invalid format",
			existing:    draftContent(nil),
			req:         content.UpdateContentRequest{Title: "Test", Content: validBody, Tags: []string{}, Status: contentdomain.StatusDraft, Format: "bogus"},
			errContains: "format validation failed",
		},
		{
			name:        "error - invalid featured image",
			existing:    draftContent(nil),
			req:         content.UpdateContentRequest{Title: "Test", Content: validBody, Tags: []string{}, Status: contentdomain.StatusDraft, FeaturedImage: new("javascript:alert(1)")},
			wantErrIs:   contentdomain.ErrInvalidFeaturedImage,
			errContains: "featured image must be an http(s) URL or a root-relative path",
		},
		{
			name:     "success - replaces the featured image",
			existing: draftContent(nil),
			req: content.UpdateContentRequest{
				Title:         "Test",
				Content:       validBody,
				Tags:          []string{},
				Status:        contentdomain.StatusDraft,
				FeaturedImage: new("https://cdn.example.com/pic.png"),
			},
			wantUpdate: true,
			validate: func(t *testing.T, c *contentdomain.Content) {
				assert.Equal(t, "https://cdn.example.com/pic.png", c.FeaturedImage)
			},
		},
		{
			name:     "success - comments enabled honors the request override",
			existing: draftContent(func(c *contentdomain.Content) { c.AllowComments = true }),
			req: content.UpdateContentRequest{
				Title:         "Test",
				Content:       validBody,
				Tags:          []string{},
				Status:        contentdomain.StatusDraft,
				AllowComments: new(false),
			},
			opts:       []content.Option{content.WithCommentsEnabled(true)},
			wantUpdate: true,
			validate: func(t *testing.T, c *contentdomain.Content) {
				assert.False(t, c.AllowComments)
			},
		},
		{
			name:     "error - plugin system field validation fails",
			existing: draftContent(func(c *contentdomain.Content) { c.PostType = "product" }),
			req:      updateBody,
			transform: func(hookName plugin.HookName, data []byte) ([]byte, error) {
				if hookName != plugin.HookBeforeSave {
					return nil, nil
				}
				var payload map[string]any
				if err := json.Unmarshal(data, &payload); err != nil {
					return nil, err
				}
				customFields, _ := payload["customFields"].(map[string]any)
				if customFields == nil {
					customFields = make(map[string]any)
					payload["customFields"] = customFields
				}
				customFields["sync_status"] = "bogus"
				return json.Marshal(payload)
			},
			setupPT: func(m *mocks.MockPostTypeServiceInterface) {
				m.On("GetSystemFieldsByPostType", "product").Return(schemas, nil)
			},
			wantErrIs:   contentdomain.ErrSystemFieldValidation,
			errContains: "plugin system field validation failed",
		},
		{
			name: "success - preserves system fields when the request replaces the map",
			existing: draftContent(func(c *contentdomain.Content) {
				c.PostType = "product"
				c.CustomFields = map[string]any{"internal_sku": "SKU-1"}
			}),
			req: content.UpdateContentRequest{
				Title:        "Test",
				Content:      validBody,
				Tags:         []string{},
				Status:       contentdomain.StatusDraft,
				CustomFields: map[string]any{"price": 29.99},
			},
			setupPT: func(m *mocks.MockPostTypeServiceInterface) {
				m.On("GetSystemFieldsByPostType", "product").Return(schemas, nil)
				m.On("GetFieldsByPostType", "product").Return([]customfield.FieldSchema{}, nil)
			},
			wantUpdate: true,
			validate: func(t *testing.T, c *contentdomain.Content) {
				assert.Equal(t, "SKU-1", c.CustomFields["internal_sku"])
				assert.Equal(t, 29.99, c.CustomFields["price"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.On("GetByID", mock.Anything, 1).Return(tt.existing, nil)
			if tt.setupRepo != nil {
				tt.setupRepo(mockRepo)
			}
			if tt.wantUpdate {
				mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*content.Content")).Return(nil)
			}

			var hook content.HookExecutor
			if tt.transform != nil {
				hook = &mockHookExecutor{transform: tt.transform}
			}

			var postType content.PostTypeServiceInterface
			if tt.setupPT != nil {
				mockPostType := mocks.NewMockPostTypeServiceInterface(t)
				tt.setupPT(mockPostType)
				postType = mockPostType
			}

			service := content.NewServiceWithHooks(mockRepo, nil, postType, hook, tt.opts...)

			result, err := service.Update(
				context.Background(),
				1,
				identity.New(1, "admin", contentdomain.RoleAdmin),
				tt.req,
			)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
			}
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			}
			if tt.wantErrIs != nil || tt.errContains != "" {
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.validate(t, result)
		})
	}
}

func TestService_Update_SlugUniquenessError(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		errContains string
	}{
		{
			name: "error - slug uniqueness check fails",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&contentdomain.Content{
					ID:       1,
					UserID:   1,
					Title:    "Test",
					Slug:     "test",
					Content:  testTipTapJSON("Body"),
					Tags:     []string{},
					Status:   contentdomain.StatusDraft,
					Format:   contentdomain.FormatTiptap,
					PostType: "post",
					Language: "en",
				}, nil)
				m.On("CheckSlugUnique", mock.Anything, "new-slug", "en").Return(false, errors.New("database unavailable"))
			},
			errContains: "failed to check slug uniqueness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.Update(
				context.Background(),
				1,
				identity.New(1, "admin", contentdomain.RoleAdmin),
				content.UpdateContentRequest{
					Title:   "Test",
					Slug:    "new-slug",
					Content: testTipTapJSON("Body"),
					Tags:    []string{},
					Status:  contentdomain.StatusDraft,
				},
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, result)
		})
	}
}

func TestService_ListByFilters_InvalidSortOrder(t *testing.T) {
	tests := []struct {
		name    string
		filters contentdomain.ContentFilters
	}{
		{
			name:    "error - unsupported sort order",
			filters: contentdomain.ContentFilters{SortOrder: "sideways"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)

			service := content.NewService(mockRepo, nil, nil)

			result, err := service.ListByFilters(context.Background(), 1, tt.filters)

			require.Error(t, err)
			assert.ErrorIs(t, err, contentdomain.ErrInvalidSortOrder)
			assert.Nil(t, result)
		})
	}
}

func TestService_Publish_SEOGenerationFailure(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		errContains string
	}{
		{
			name: "error - auto generation fails for an untitled draft",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&contentdomain.Content{
					ID:       1,
					UserID:   1,
					Title:    "",
					Slug:     "untitled",
					Content:  "",
					Status:   contentdomain.StatusDraft,
					PostType: "post",
					Language: "en",
				}, nil)
			},
			errContains: "failed to generate SEO metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(
				mockRepo,
				seo.NewService("http://localhost:8080", "Test Site"),
				nil,
			)

			result, err := service.Publish(
				context.Background(),
				1,
				identity.New(1, "admin", contentdomain.RoleAdmin),
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, result)
		})
	}
}

func TestService_DeleteContent_AdminError(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mocks.MockRepository)
		errContains string
	}{
		{
			name: "error - admin delete fails",
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&contentdomain.Content{
					ID:       1,
					UserID:   7,
					Title:    "Test",
					PostType: "post",
				}, nil)
				m.On("DeleteByID", mock.Anything, 1).Return(errors.New("database unavailable"))
			},
			errContains: "failed to delete content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)

			err := service.DeleteContent(
				context.Background(),
				1,
				identity.New(1, "admin", contentdomain.RoleAdmin),
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
