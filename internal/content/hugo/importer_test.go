package hugo_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	hugomocks "github.com/aristorinjuang/lesstruct/internal/content/hugo/mocks"
	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/draw"
)

// testImgSrcRe extracts every img src attribute from a body so tests can
// predict which media uploads the mapper will perform.
var testImgSrcRe = regexp.MustCompile(`<img[^>]*\bsrc="([^"]*)"`)

func imageSrcsInBody(body string) []string {
	matches := testImgSrcRe.FindAllStringSubmatch(body, -1)
	srcs := make([]string, 0, len(matches))
	for _, m := range matches {
		srcs = append(srcs, m[1])
	}
	return srcs
}

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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Language == "en"
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
			cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
				cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
					func(req contentdomain.CreateContentRequest) bool {
						return req.Language == "en" && req.TranslationGroupID == nil
					},
				)).Return(&contentdomain.Content{ID: 1}, nil).Once()
			}
			cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
			wantBodyHas: `<figure><img src="http://media.local/cover" alt="First Post"></figure><p>Hi</p><img src="http://media.local/foo" alt="Foo">`,
		},
		{
			name:        "success - skip media rewrites static files to /static",
			body:        `<p>Hi</p><img src="/images/foo.jpg" alt="Foo">`,
			images:      []string{"/images/cover.jpg"},
			skipMedia:   true,
			wantBodyHas: `<figure><img src="/static/images/cover.jpg" alt="First Post"></figure><p>Hi</p><img src="/static/images/foo.jpg" alt="Foo">`,
		},
		{
			name:        "success - featured skipped when body opens with same image in figure",
			body:        `<figure><a href="https://example.com"><img src="/images/cover.jpg" alt="Cover"></a><figcaption>Cap</figcaption></figure>`,
			images:      []string{"/images/cover.jpg"},
			skipMedia:   false,
			wantBodyHas: `<figure><a href="https://example.com"><img src="http://media.local/cover" alt="Cover"></a><figcaption>Cap</figcaption></figure>`,
		},
		{
			name:        "success - featured skipped when body opens with same image and skip media",
			body:        `<img src="/images/cover.jpg" alt="Cover"><p>Hi</p>`,
			images:      []string{"/images/cover.jpg"},
			skipMedia:   true,
			wantBodyHas: `<img src="/static/images/cover.jpg" alt="Cover"><p>Hi</p>`,
		},
		{
			name:        "success - featured skipped when it appears later in body",
			body:        `<p>Hi</p><img src="/images/foo.jpg" alt="Foo"><img src="/images/cover.jpg" alt="Cover">`,
			images:      []string{"/images/cover.jpg"},
			skipMedia:   false,
			wantBodyHas: `<p>Hi</p><img src="http://media.local/foo" alt="Foo"><img src="http://media.local/cover" alt="Cover">`,
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
				mediaFiles := map[string]struct {
					bytes []byte
					alt   string
					name  string
					url   string
				}{
					"/images/foo.jpg":   {[]byte("foo-bytes"), "foo", "foo.jpg", "http://media.local/foo"},
					"/images/cover.jpg": {[]byte("cover-bytes"), "cover", "cover.jpg", "http://media.local/cover"},
				}
				uploaded := make(map[string]struct{})
				for _, src := range imageSrcsInBody(tt.body) {
					uploaded[src] = struct{}{}
				}
				if len(tt.images) > 0 {
					uploaded[tt.images[0]] = struct{}{}
				}
				for ref := range uploaded {
					file := mediaFiles[ref]
					ms.On("GenerateFromBytes", mock.Anything, file.bytes, 1, file.alt, file.name).
						Return(&mediadomain.Media{URL: file.url}, nil).Once()
				}
			}
			var captured contentdomain.CreateContentRequest
			cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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

// testGradientImage builds a w×h smooth diagonal luma ramp.
func testGradientImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8((x + y) * 255 / (w + h))
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}

func testEncodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestImporter_FeaturedVisualDuplicate(t *testing.T) {
	ctx := context.Background()

	// The same picture as two distinct files — a full-size PNG and a smaller
	// re-encode — plus an unrelated picture.
	photo := testGradientImage(64, 48)
	small := image.NewRGBA(image.Rect(0, 0, 32, 24))
	draw.BiLinear.Scale(small, small.Bounds(), photo, photo.Bounds(), draw.Src, nil)
	other := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			if x >= 32 {
				other.Set(x, y, color.White)
			} else {
				other.Set(x, y, color.Black)
			}
		}
	}

	fileBytes := map[string][]byte{
		"/images/cover.jpg":       testEncodePNG(t, photo),
		"/images/photo-small.jpg": testEncodePNG(t, small),
		"/images/other.jpg":       testEncodePNG(t, other),
	}
	fileURLs := map[string]string{
		"/images/cover.jpg":       "http://media.local/cover",
		"/images/photo-small.jpg": "http://media.local/photo-small",
		"/images/other.jpg":       "http://media.local/other",
	}

	tests := []struct {
		name           string
		body           string
		wantContent    string
		wantDupWarning bool
	}{
		{
			name:           "success - prepend skipped when cover visually duplicates the leading body image",
			body:           `<img src="/images/photo-small.jpg" alt="Photo"><p>Hi</p>`,
			wantContent:    `<img src="http://media.local/photo-small" alt="Photo"><p>Hi</p>`,
			wantDupWarning: true,
		},
		{
			name:           "success - cover prepended when leading body image is a different picture",
			body:           `<img src="/images/other.jpg" alt="Other"><p>Hi</p>`,
			wantContent:    `<figure><img src="http://media.local/cover" alt="First Post"></figure><img src="http://media.local/other" alt="Other"><p>Hi</p>`,
			wantDupWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staticDir := t.TempDir()
			err := os.MkdirAll(filepath.Join(staticDir, "images"), 0755)
			require.NoError(t, err)
			for ref, data := range fileBytes {
				require.NoError(t, os.WriteFile(filepath.Join(staticDir, ref), data, 0644))
			}

			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)

			uploaded := map[string]struct{}{"/images/cover.jpg": {}}
			for _, src := range imageSrcsInBody(tt.body) {
				uploaded[src] = struct{}{}
			}
			for ref := range uploaded {
				ms.On("GenerateFromBytes", mock.Anything, fileBytes[ref], 1, mock.Anything, mock.Anything).
					Return(&mediadomain.Media{URL: fileURLs[ref]}, nil).Once()
			}

			sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
			var captured contentdomain.CreateContentRequest
			cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
					Images:       []string{"/images/cover.jpg"},
					FilePath:     "content/en/first.md",
				}},
				StaticDir: staticDir,
			}, 1, hugo.ImportOptions{}, nil)

			assert.Equal(t, 1, result.Imported)
			assert.Equal(t, tt.wantContent, captured.Content)
			foundWarning := false
			for _, entry := range result.Errors {
				if strings.Contains(entry, "prepend skipped") && strings.Contains(entry, "/images/cover.jpg") {
					foundWarning = true
				}
			}
			assert.Equal(t, tt.wantDupWarning, foundWarning)
		})
	}
}

func TestImporter_SurfacesMediaMigrationFailures(t *testing.T) {
	ctx := context.Background()

	staticDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(staticDir, "images"), 0755)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "images/bad.jpg"), []byte("bad-bytes"), 0644))

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)
	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	ms.On("GenerateFromBytes", mock.Anything, []byte("bad-bytes"), 1, "bad", "bad.jpg").
		Return(nil, errors.New("transcode failed")).Once()

	var captured contentdomain.CreateContentRequest
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
			OriginalBody: `<p>Hi</p><img src="/images/bad.jpg" alt="Bad">`,
			FilePath:     "content/en/first.md",
		}},
		StaticDir: staticDir,
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	// The failure falls back to the /static/ copy and is surfaced as a warning.
	assert.Contains(t, captured.Content, `src="/static/images/bad.jpg"`)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "warning:")
	assert.Contains(t, result.Errors[0], "/images/bad.jpg")
	assert.Contains(t, result.Errors[0], "transcode failed")
}

func TestImporter_WarnsUnresolvedRefsOnceAndSkipsPermalinks(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	// Two items; both reference the same dead static file, one references a
	// known permalink (its own URL), one a bare extension-less path; resolved
	// /static/ and /uploads/ references must stay silent too.
	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	sc.On("SlugExists", mock.Anything, "second-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Twice()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{
			{
				Title:        "First Post",
				Language:     "en",
				URL:          "/first-post.html",
				OriginalBody: `<a href="/bioven/missing.html">A</a><a href="/bioven/missing.html">B</a><a href="/first-post.html">Self</a><a href="/about">Page</a><a href="/static/site.css">Css</a><source src="/uploads/media/x.webp">`,
				FilePath:     "content/en/first.md",
			},
			{
				Title:        "Second Post",
				Language:     "en",
				URL:          "/second-post.html",
				OriginalBody: `<a href="/bioven/missing.html">Dead again</a>`,
				FilePath:     "content/en/second.md",
			},
		},
		StaticDir: t.TempDir(),
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 2, result.Imported)
	// Exactly one warning for the dead ref (deduped within the body AND across
	// items); permalink, extension-less, /static/ and /uploads/ refs stay silent.
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "/bioven/missing.html")
	assert.Contains(t, result.Errors[0], "left unresolved")
}

func TestImporter_FeaturedImageFailureSkipsPrepend(t *testing.T) {
	ctx := context.Background()

	staticDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(staticDir, "images"), 0755)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "images/bad.jpg"), []byte("bad-bytes"), 0644))

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)
	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	ms.On("GenerateFromBytes", mock.Anything, []byte("bad-bytes"), 1, "bad", "bad.jpg").
		Return(nil, errors.New("transcode failed")).Once()

	var captured contentdomain.CreateContentRequest
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
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
			OriginalBody: `<p>Hi</p>`,
			Images:       []string{"/images/bad.jpg"},
			FilePath:     "content/en/first.md",
		}},
		StaticDir: staticDir,
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	// A failed featured image must not be prepended as a broken cover.
	assert.Equal(t, `<p>Hi</p>`, captured.Content)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "/images/bad.jpg")
}

func TestImporter_RepointsDanglingAlias(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Once()
	ac.On("Create", mock.Anything, 1, "old-path").Return(aliasdomain.ErrAliasAlreadyExists).Once()
	ac.On("FindByAlias", mock.Anything, "old-path").Return(
		&aliasdomain.Alias{ID: 5, ContentID: 99, Alias: "old-path"}, nil,
	).Once()
	cc.On("GetByID", mock.Anything, 99).Return(nil, contentdomain.ErrContentNotFound).Once()
	ac.On("Repoint", mock.Anything, "old-path", 99, 1).Return(nil).Once()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{{
			Title:        "First Post",
			Language:     "en",
			URL:          "/first-post.html",
			OriginalBody: "<p>Hello</p>",
			Aliases:      []string{"/old-path"},
			FilePath:     "content/en/first.md",
		}},
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	assert.Empty(t, result.Errors)
}

func TestImporter_RepointConcurrentChange(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Once()
	ac.On("Create", mock.Anything, 1, "old-path").Return(aliasdomain.ErrAliasAlreadyExists).Once()
	ac.On("FindByAlias", mock.Anything, "old-path").Return(
		&aliasdomain.Alias{ID: 5, ContentID: 99, Alias: "old-path"}, nil,
	).Once()
	cc.On("GetByID", mock.Anything, 99).Return(nil, contentdomain.ErrContentNotFound).Once()
	ac.On("Repoint", mock.Anything, "old-path", 99, 1).Return(aliasdomain.ErrAliasNotFound).Once()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{{
			Title:        "First Post",
			Language:     "en",
			URL:          "/first-post.html",
			OriginalBody: "<p>Hello</p>",
			Aliases:      []string{"/old-path"},
			FilePath:     "content/en/first.md",
		}},
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	assert.Empty(t, result.Errors)
}

func TestImporter_RepointFailure(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Once()
	ac.On("Create", mock.Anything, 1, "old-path").Return(aliasdomain.ErrAliasAlreadyExists).Once()
	ac.On("FindByAlias", mock.Anything, "old-path").Return(
		&aliasdomain.Alias{ID: 5, ContentID: 99, Alias: "old-path"}, nil,
	).Once()
	cc.On("GetByID", mock.Anything, 99).Return(nil, contentdomain.ErrContentNotFound).Once()
	ac.On("Repoint", mock.Anything, "old-path", 99, 1).Return(errors.New("db down")).Once()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{{
			Title:        "First Post",
			Language:     "en",
			URL:          "/first-post.html",
			OriginalBody: "<p>Hello</p>",
			Aliases:      []string{"/old-path"},
			FilePath:     "content/en/first.md",
		}},
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	assert.Empty(t, result.Errors)
}

func TestImporter_AliasLookupFailure(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Once()
	ac.On("Create", mock.Anything, 1, "old-path").Return(aliasdomain.ErrAliasAlreadyExists).Once()
	ac.On("FindByAlias", mock.Anything, "old-path").Return(nil, errors.New("db down")).Once()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{{
			Title:        "First Post",
			Language:     "en",
			URL:          "/first-post.html",
			OriginalBody: "<p>Hello</p>",
			Aliases:      []string{"/old-path"},
			FilePath:     "content/en/first.md",
		}},
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	assert.Empty(t, result.Errors)
	ac.AssertNotCalled(t, "Repoint", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestImporter_AliasTargetLookupError(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Once()
	ac.On("Create", mock.Anything, 1, "old-path").Return(aliasdomain.ErrAliasAlreadyExists).Once()
	ac.On("FindByAlias", mock.Anything, "old-path").Return(
		&aliasdomain.Alias{ID: 5, ContentID: 99, Alias: "old-path"}, nil,
	).Once()
	cc.On("GetByID", mock.Anything, 99).Return(nil, errors.New("db down")).Once()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{{
			Title:        "First Post",
			Language:     "en",
			URL:          "/first-post.html",
			OriginalBody: "<p>Hello</p>",
			Aliases:      []string{"/old-path"},
			FilePath:     "content/en/first.md",
		}},
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	assert.Empty(t, result.Errors)
	ac.AssertNotCalled(t, "Repoint", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestImporter_KeepsAliasOfLiveContent(t *testing.T) {
	ctx := context.Background()

	cc := hugomocks.NewMockContentCreator(t)
	ac := hugomocks.NewMockAliasCreator(t)
	sc := hugomocks.NewMockSlugResolver(t)
	ms := hugomocks.NewMockMediaService(t)

	sc.On("SlugExists", mock.Anything, "first-post", "en").Return(false, nil).Once()
	cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&contentdomain.Content{ID: 1}, nil).Once()
	ac.On("Create", mock.Anything, 1, "old-path").Return(aliasdomain.ErrAliasAlreadyExists).Once()
	ac.On("FindByAlias", mock.Anything, "old-path").Return(
		&aliasdomain.Alias{ID: 5, ContentID: 99, Alias: "old-path"}, nil,
	).Once()
	cc.On("GetByID", mock.Anything, 99).Return(&contentdomain.Content{ID: 99}, nil).Once()

	imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	result := imp.Import(ctx, &hugo.HugoSite{
		Items: []*hugo.HugoItem{{
			Title:        "First Post",
			Language:     "en",
			URL:          "/first-post.html",
			OriginalBody: "<p>Hello</p>",
			Aliases:      []string{"/old-path"},
			FilePath:     "content/en/first.md",
		}},
	}, 1, hugo.ImportOptions{}, nil)

	assert.Equal(t, 1, result.Imported)
	ac.AssertNotCalled(t, "Repoint", mock.Anything, mock.Anything, mock.Anything)
}

func TestImporter_HighlightShortcodesRenderChroma(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		originalBody string
		wantInBody   []string
	}{
		{
			name:         "success - whitespace-tolerant closing tag renders chroma block",
			originalBody: "<p>intro</p>{{< highlight go >}}package main{{< / highlight >}}",
			wantInBody: []string{
				"<p>intro</p>",
				`<div class="highlight">`,
				`<code class="language-go nohighlight" data-lang="go">`,
			},
		},
		{
			name:         "success - linenos=table renders line-number table",
			originalBody: "{{< highlight go \"linenos=table\" >}}package main{{< /highlight >}}",
			wantInBody: []string{
				`class="lntable"`,
				`class="lnt"`,
				`<code class="language-go nohighlight" data-lang="go">`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)

			item := &hugo.HugoItem{
				Title:        "Code Post",
				Language:     "en",
				URL:          "/code-post.html",
				OriginalBody: tt.originalBody,
				FilePath:     "content/en/code.md",
			}

			sc.On("SlugExists", mock.Anything, "code-post", "en").Return(false, nil).Once()
			cc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(
				func(req contentdomain.CreateContentRequest) bool {
					for _, want := range tt.wantInBody {
						if !strings.Contains(req.Content, want) {
							return false
						}
					}
					return !strings.Contains(req.Content, "{{<")
				}),
			).Return(&contentdomain.Content{ID: 1}, nil).Once()

			imp := hugo.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
			result := imp.Import(ctx, &hugo.HugoSite{Items: []*hugo.HugoItem{item}, SourcePath: "test"}, 1, hugo.ImportOptions{}, nil)

			require.NotNil(t, result)
			assert.Equal(t, 1, result.Imported)
		})
	}
}
