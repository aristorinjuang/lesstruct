package hugo_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	hugomocks "github.com/aristorinjuang/lesstruct/internal/content/hugo/mocks"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestImporter(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		items      []*hugo.HugoItem
		userID     int
		setupMocks func(*hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService)
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "draft-post", "en").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "custom-fields", "en").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "post-title", "en").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "my-post", "en").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "tag-post", "en").Return(false, nil).Once()
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
					FilePath:     "content/pair/post.id.md",
				},
			},
			userID: 1,
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "post-title", "en").Return(false, nil).Once()
				sc.On("SlugExists", mock.Anything, "post-title", "id").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "post-title", "id").Return(false, nil).Once()
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
			setupMocks: func(cc *hugomocks.MockContentCreator, ac *hugomocks.MockAliasCreator, sc *hugomocks.MockSlugResolver, _ *hugomocks.MockMediaService) {
				sc.On("SlugExists", mock.Anything, "my-post", "en").Return(false, nil).Once()
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
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)
			tt.setupMocks(cc, ac, sc, ms)

			imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{Items: tt.items, SourcePath: "test"}, tt.userID, hugo.ImportOptions{}, nil)

			assert.Equal(t, tt.expected.Imported, result.Imported)
			assert.Equal(t, tt.expected.Skipped, result.Skipped)
			assert.Equal(t, tt.expected.Errors, result.Errors)
		})
	}
}

func TestImporter_PublishedAt(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2016, 8, 8, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		item     *hugo.HugoItem
		wantDate *time.Time
	}{
		{
			name: "success - frontmatter date becomes PublishedAt",
			item: &hugo.HugoItem{
				Title:        "Dated Post",
				Language:     "en",
				URL:          "/dated-post.html",
				OriginalBody: "<p>Hello</p>",
				FilePath:     "content/en/dated.md",
				Date:         ts,
			},
			wantDate: &ts,
		},
		{
			name: "success - zero date leaves PublishedAt nil",
			item: &hugo.HugoItem{
				Title:        "No Date Post",
				Language:     "en",
				URL:          "/no-date.html",
				OriginalBody: "<p>Hello</p>",
				FilePath:     "content/en/no-date.md",
			},
			wantDate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)
			sc.On("SlugExists", mock.Anything, mock.Anything, "en").Return(false, nil).Once()
			var captured contentdomain.CreateContentRequest
			cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
				func(req contentdomain.CreateContentRequest) bool {
					captured = req
					return true
				},
			)).Return(&contentdomain.Content{ID: 1}, nil).Once()

			imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{Items: []*hugo.HugoItem{tt.item}, SourcePath: "test"}, 1, hugo.ImportOptions{}, nil)

			assert.Equal(t, 1, result.Imported)
			if tt.wantDate == nil {
				assert.Nil(t, captured.PublishedAt)
				return
			}
			require.NotNil(t, captured.PublishedAt)
			assert.Equal(t, *tt.wantDate, *captured.PublishedAt)
		})
	}
}

func TestImporter_IdempotentSkip(t *testing.T) {
	ctx := context.Background()
	item := &hugo.HugoItem{
		Title:        "Existing Post",
		Language:     "en",
		URL:          "/existing-post.html",
		OriginalBody: "<p>Hello</p>",
		FilePath:     "content/en/existing.md",
	}

	tests := []struct {
		name     string
		exists   bool
		checkErr error
		expected hugo.ImportResult
	}{
		{
			name:     "slug exists - item skipped as already imported",
			exists:   true,
			checkErr: nil,
			expected: hugo.ImportResult{
				Skipped: 1,
				Errors:  []string{`skipped "Existing Post": already imported (slug "existing-post" exists)`},
			},
		},
		{
			name:     "slug check error - item skipped",
			exists:   false,
			checkErr: errors.New("db down"),
			expected: hugo.ImportResult{
				Skipped: 1,
				Errors:  []string{`skipped "Existing Post": failed to check slug: db down`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)
			sc.On("SlugExists", mock.Anything, "existing-post", "en").Return(tt.exists, tt.checkErr).Once()
			if tt.exists && tt.checkErr == nil {
				sc.On("GetBySlugAndLanguage", mock.Anything, "existing-post", "en").
					Return(&contentdomain.Content{ID: 7}, nil).Once()
			}

			imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{Items: []*hugo.HugoItem{item}, SourcePath: "test"}, 1, hugo.ImportOptions{}, nil)

			assert.Equal(t, tt.expected, *result)
			cc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestImporter_TranslationLinksWhenEnglishAlreadyImported(t *testing.T) {
	ctx := context.Background()
	en := &hugo.HugoItem{
		Title:        "Post Title",
		Language:     "en",
		URL:          "/post-title.html",
		OriginalBody: "<p>EN</p>",
		FilePath:     "content/pair/post.html",
	}
	id := &hugo.HugoItem{
		Title:        "Judul Postingan",
		Language:     "id",
		URL:          "/post-title.html",
		OriginalBody: "<p>ID</p>",
		FilePath:     "content/pair/post.id.html",
	}

	tests := []struct {
		name         string
		enExists     bool
		wantIDLinked bool
		wantImported int
		wantSkipped  int
	}{
		{
			name:         "success - english already imported, indonesian still imports and links",
			enExists:     true,
			wantIDLinked: true,
			wantImported: 1,
			wantSkipped:  1,
		},
		{
			name:         "success - fresh import links normally",
			enExists:     false,
			wantIDLinked: true,
			wantImported: 2,
			wantSkipped:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)

			sc.On("SlugExists", mock.Anything, "post-title", "en").Return(tt.enExists, nil).Once()
			if tt.enExists {
				sc.On("GetBySlugAndLanguage", mock.Anything, "post-title", "en").
					Return(&contentdomain.Content{ID: 1}, nil).Once()
			}
			sc.On("SlugExists", mock.Anything, "post-title", "id").Return(false, nil).Once()

			if tt.enExists {
				cc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
			} else {
				cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Language == "en" && req.TranslationGroupID == nil
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			}
			cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
				func(req contentdomain.CreateContentRequest) bool {
					return req.Language == "id" && req.TranslationGroupID != nil && *req.TranslationGroupID == 1
				},
			)).Return(&contentdomain.Content{ID: 2}, nil).Once()

			imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{
				Items:      []*hugo.HugoItem{en, id},
				SourcePath: "test",
			}, 1, hugo.ImportOptions{}, nil)

			assert.Equal(t, tt.wantImported, result.Imported)
			assert.Equal(t, tt.wantSkipped, result.Skipped)
		})
	}
}

func TestImporter_MediaRewriteAndFeatured(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		body        string
		images      []string
		skipMedia   bool
		wantBodyHas string
	}{
		{
			name:        "success - body img src rewritten and featured prepended",
			body:        `<p>Hi</p><img src="/images/foo.jpg" alt="Foo">`,
			images:      []string{"/images/cover.jpg"},
			skipMedia:   false,
			wantBodyHas: `<img src="http://media.local/foo" alt="First Post"><p>Hi</p><img src="http://media.local/foo" alt="Foo">`,
		},
		{
			name:        "success - skip media leaves references untouched",
			body:        `<p>Hi</p><img src="/images/foo.jpg" alt="Foo">`,
			images:      []string{"/images/cover.jpg"},
			skipMedia:   true,
			wantBodyHas: `<img src="/images/cover.jpg" alt="First Post"><p>Hi</p><img src="/images/foo.jpg" alt="Foo">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staticDir := t.TempDir()
			err := os.MkdirAll(filepath.Join(staticDir, "images"), 0755)
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(staticDir, "images/foo.jpg"), []byte("foo-bytes"), 0644)
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(staticDir, "images/cover.jpg"), []byte("cover-bytes"), 0644)
			require.NoError(t, err)

			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)
			sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
			if !tt.skipMedia {
				ms.On("GenerateFromBytes", mock.Anything, []byte("foo-bytes"), 1, "foo", "foo.jpg").
					Return(&mediadomain.Media{URL: "http://media.local/foo"}, nil).Once()
				ms.On("GenerateFromBytes", mock.Anything, []byte("cover-bytes"), 1, "cover", "cover.jpg").
					Return(&mediadomain.Media{URL: "http://media.local/foo"}, nil).Once()
			}
			var captured contentdomain.CreateContentRequest
			cc.On("Create", mock.Anything, mock.Anything, mock.MatchedBy(
				func(req contentdomain.CreateContentRequest) bool {
					captured = req
					return true
				},
			)).Return(&contentdomain.Content{ID: 1}, nil).Once()

			imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{
				Items: []*hugo.HugoItem{{
					Title:        "First Post",
					Language:     "en",
					URL:          "/first-post.html",
					OriginalBody: tt.body,
					Images:       tt.images,
					FilePath:     "content/en/first.md",
				}},
				StaticDir: staticDir,
			}, 1, hugo.ImportOptions{SkipMedia: tt.skipMedia}, nil)

			assert.Equal(t, 1, result.Imported)
			assert.Equal(t, tt.wantBodyHas, captured.Content)
		})
	}
}
