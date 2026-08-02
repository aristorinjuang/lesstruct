package hugo_test

import (
	"context"
	"errors"
	"io"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	hugomocks "github.com/aristorinjuang/lesstruct/internal/content/hugo/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestImporter(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		items      []*hugo.HugoItem
		userID     int
		setupMocks func(*hugomocks.MockContentCreator, *hugomocks.MockAliasCreator)
		expected   hugo.ImportResult
	}{
		{
			name: "single english item without translation",
			items: []*hugo.HugoItem{
				{
					Title:        "First Post",
					Description:  "A post",
					Tags:         []string{"hello"},
					URL:          "/first-post.html",
					Language:     "en",
					Aliases:      []string{"/old-path", "/another-old-path"},
					OriginalBody: "<p>Hello</p>",
					FilePath:     "content/en/first.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(&contentdomain.Content{ID: 1}, nil).
					Once()
				ac.On("Create", mock.Anything, mock.Anything, "old-path").
					Return(nil).
					Once()
				ac.On("Create", mock.Anything, mock.Anything, "another-old-path").
					Return(nil).
					Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "draft item uses status draft",
			items: []*hugo.HugoItem{
				{
					Title:        "Draft Post",
					Language:     "en",
					URL:          "/draft-post.html",
					OriginalBody: "<p>Draft</p>",
					FilePath:     "content/en/draft.md",
					IsDraft:      true,
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Status == contentdomain.StatusDraft
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "boolean custom fields are set on content",
			items: []*hugo.HugoItem{
				{
					Title:            "Custom Fields Post",
					Language:         "en",
					URL:              "/custom-fields.html",
					OriginalBody:     "<p>Custom</p>",
					FilePath:         "content/en/custom.md",
					HasMath:          true,
					HasChart:         true,
					HasDiagrams:      true,
					HideMobileImages: true,
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.CustomFields["hasMath"] == true &&
							req.CustomFields["hasChart"] == true &&
							req.CustomFields["hasDiagrams"] == true &&
							req.CustomFields["hideMobileImages"] == true
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "slug generated from url with html suffix stripped",
			items: []*hugo.HugoItem{
				{
					Title:        "Post Title",
					Language:     "en",
					URL:          "/post-title.html",
					OriginalBody: "<p>Content</p>",
					FilePath:     "content/en/post.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Slug == "post-title"
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "slug fallback from title when url is empty",
			items: []*hugo.HugoItem{
				{
					Title:        "My Post!",
					Language:     "en",
					URL:          "",
					OriginalBody: "<p>Content</p>",
					FilePath:     "content/en/my-post.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Slug == "my-post"
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "tags are normalized with deduplication and lowercasing",
			items: []*hugo.HugoItem{
				{
					Title:        "Tag Post",
					Language:     "en",
					URL:          "/tag-post.html",
					Tags:         []string{"Go", "go", "Go", "  hugo  ", "HUGO"},
					OriginalBody: "<p>Tags</p>",
					FilePath:     "content/en/tags.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return len(req.Tags) == 2 &&
							req.Tags[0] == "go" &&
							req.Tags[1] == "hugo"
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "translation pair - english and indonesian",
			items: []*hugo.HugoItem{
				{
					Title:        "Post Title",
					Language:     "en",
					URL:          "/post-title.html",
					OriginalBody: "<p>EN content</p>",
					FilePath:     "content/pair/post.md",
				},
				{
					Title:        "Judul Postingan",
					Language:     "id",
					URL:          "/post-title.html",
					OriginalBody: "<p>ID content</p>",
					FilePath:     "content/pair/post.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Language == "en"
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Language == "id" &&
							req.TranslationGroupID != nil &&
							*req.TranslationGroupID == 1
					},
				)).Return(&contentdomain.Content{ID: 2}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 2},
		},
		{
			name: "single indonesian item without matching english",
			items: []*hugo.HugoItem{
				{
					Title:        "Judul Postingan",
					Language:     "id",
					URL:          "/post-title.html",
					OriginalBody: "<p>ID content</p>",
					FilePath:     "content/id/post.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Language == "id" && req.TranslationGroupID == nil
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			},
			expected: hugo.ImportResult{Imported: 1},
		},
		{
			name: "content creation error increments skipped count",
			items: []*hugo.HugoItem{
				{
					Title:        "My Post",
					Language:     "en",
					URL:          "/my-post.html",
					OriginalBody: "<p>Content</p>",
					FilePath:     "content/en/post.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator) {
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.New("creation failed")).
					Once()
			},
			expected: hugo.ImportResult{
				Skipped: 1,
				Errors:  []string{`skipped "My Post": creation failed`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			tt.setupMocks(cc, ac)

			imp := hugo.NewImporter(cc, ac, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{Items: tt.items, SourcePath: "test"}, tt.userID)

			assert.Equal(t, tt.expected.Imported, result.Imported)
			assert.Equal(t, tt.expected.Skipped, result.Skipped)
			assert.Equal(t, tt.expected.Errors, result.Errors)
		})
	}
}
