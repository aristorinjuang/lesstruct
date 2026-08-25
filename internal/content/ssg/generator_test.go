package ssg_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/contentpage"
	contentpageMocks "github.com/aristorinjuang/lesstruct/internal/api/contentpage/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/content/ssg"
	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	aliasMocks "github.com/aristorinjuang/lesstruct/internal/domain/alias/mocks"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGenerator_Generate(t *testing.T) {
	mockContentSvc := contentpageMocks.NewMockContentService(t)
	mockResolver := contentpageMocks.NewMockPostTypeResolver(t)

	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	mockContent := &contentdomain.Content{
		ID:        1,
		Slug:      "hello-world",
		Title:     "Hello World",
		Content:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
		Format:    contentdomain.FormatTiptap,
		Status:    contentdomain.StatusPublished,
		PostType:  "post",
		Author:    "Alice",
		Username:  "alice",
		Language:  "en",
		CreatedAt: now,
	}

	mockContentSvc.On("GetPublishedByPostType", mock.Anything, "post", []string{"en"}, 0, 0, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetPublishedBySlugAny", mock.Anything, "hello-world").
		Return(mockContent, nil).Maybe()

	mockContentSvc.On("GetPublishedPages", mock.Anything).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockContentSvc.On("GetPublishedCustomPostTypes", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("GetPublishedTags", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("AuthorExists", mock.Anything, "alice").
		Return(true, nil).Maybe()

	mockContentSvc.On("GetPublishedByAuthorUsername", mock.Anything, "alice", []string{"en"}, 11, 0).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetRelated", mock.Anything, 1, 4).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).
		Return(posttype.PostType{}, assert.AnError).Maybe()

	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	renderer := tiptap.NewRenderer(nil)
	assembler := contentpage.NewDataAssembler(
		mockContentSvc,
		mockResolver,
		nil,
		nil,
		renderer,
		nil,
		[]string{"en"},
		nil,
		config.SiteConfig{Name: "Test Site"},
		10,
	)

	generator := ssg.NewGenerator(assembler, templates, mockContentSvc, "", nil, "https://example.com")

	var buf bytes.Buffer
	err = generator.Generate(context.Background(), &buf)
	require.NoError(t, err)

	assert.Greater(t, buf.Len(), 0, "tar.gz output should not be empty")

	gzr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	files := make(map[string]bool)
	contents := make(map[string]string)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if header.Typeflag == tar.TypeReg {
			files[header.Name] = true
			var contentBuf strings.Builder
			_, _ = io.Copy(&contentBuf, tr)
			contents[header.Name] = contentBuf.String()
		}
	}

	assert.Contains(t, files, "index.html", "homepage should exist")
	assert.Contains(t, files, "hello-world/index.html", "content page should exist")
	assert.Contains(t, files, "hello-world/amp/index.html", "AMP variant should exist")
	assert.Contains(t, files, "sitemap.xml", "sitemap should exist")
	assert.Contains(t, files, "robots.txt", "robots.txt should exist")
	assert.Contains(t, files, "404.html", "404 page should exist")
	assert.Contains(t, files, "index.xml", "RSS feed should exist")

	sitemapContent, ok := contents["sitemap.xml"]
	if assert.True(t, ok, "sitemap.xml should be readable") {
		assert.Contains(t, sitemapContent, "https://example.com/", "sitemap should contain absolute URLs")
		assert.Contains(t, sitemapContent, "https://example.com/hello-world/", "sitemap should reference content page")
		assert.NotContains(t, sitemapContent, "<loc>https://example.com/hello-world</loc>", "sitemap should use trailing-slash directory URLs")
		assert.NotContains(t, sitemapContent, "<loc>/", "sitemap should not contain relative URLs")
	}

	robotsContent, ok := contents["robots.txt"]
	if assert.True(t, ok, "robots.txt should be readable") {
		assert.Contains(t, robotsContent, "https://example.com/sitemap.xml", "robots.txt should reference absolute sitemap URL")
	}

	notFoundContent, ok := contents["404.html"]
	if assert.True(t, ok, "404.html should be readable") {
		assert.Contains(t, notFoundContent, "<title>Not Found - Test Site</title>", "404 page should use the not-found page title")
		assert.Contains(t, notFoundContent, `class="not-found container"`, "404 page should render the not-found body")
		assert.NotContains(t, notFoundContent, "Hello World", "404 page should not leak post content")
	}

	feedContent, ok := contents["index.xml"]
	if assert.True(t, ok, "index.xml should be readable") {
		assert.Contains(t, feedContent, `<rss version="2.0">`, "feed should declare RSS 2.0")
		assert.Contains(t, feedContent, "<title>Test Site</title>", "feed channel should use the site name")
		assert.Contains(t, feedContent, "<link>https://example.com</link>", "feed channel should link the site")
		assert.Contains(t, feedContent, "<title>Hello World</title>", "feed should contain the post title")
		assert.Contains(t, feedContent, "<link>https://example.com/hello-world/</link>", "feed items should link absolute URLs")
	}

	ampContent, ok := contents["hello-world/amp/index.html"]
	if assert.True(t, ok, "hello-world/amp/index.html should be readable") {
		assert.Contains(t, ampContent, `<link rel="canonical" href="https://example.com/hello-world/">`, "AMP page should canonicalize to the trailing-slash directory URL")
		assert.NotContains(t, ampContent, `<link rel="canonical" href="https://example.com/hello-world">`, "AMP canonical should not be slash-less")
	}
}

func TestTransformToAMP(t *testing.T) {
	html := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Test</title><link rel="stylesheet" href="/static/style.css"></head><body><img src="/image.jpg" width="800" height="600" alt="test"><iframe src="https://example.com"></iframe><script>alert('xss')</script></body></html>`

	amp, err := ssg.TransformToAMP(html, nil)
	require.NoError(t, err)

	assert.Contains(t, amp, `<html amp`, "should have amp attribute on html tag")
	assert.Contains(t, amp, `<amp-img`, "should have amp-img")
	assert.Contains(t, amp, `cdn.ampproject.org/v0.js`, "should include AMP JS")
	assert.Contains(t, amp, `amp-boilerplate`, "should include AMP boilerplate")
	assert.NotContains(t, amp, `<script>alert`, "should strip custom JS")
	assert.Contains(t, amp, `amp-iframe`, "should have amp-iframe")

	assert.NotContains(t, amp, `<link rel="stylesheet"`, "should remove external CSS links")
}

func newStaticAssetsTestGenerator(t *testing.T, theme *template.Theme) *ssg.Generator {
	t.Helper()

	mockContentSvc := contentpageMocks.NewMockContentService(t)
	mockResolver := contentpageMocks.NewMockPostTypeResolver(t)

	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	mockContent := &contentdomain.Content{
		ID:        1,
		Slug:      "hello-world",
		Title:     "Hello World",
		Content:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
		Format:    contentdomain.FormatTiptap,
		Status:    contentdomain.StatusPublished,
		PostType:  "post",
		Author:    "Alice",
		Username:  "alice",
		Language:  "en",
		CreatedAt: now,
	}

	mockContentSvc.On("GetPublishedByPostType", mock.Anything, "post", []string{"en"}, 0, 0, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetPublishedBySlugAny", mock.Anything, "hello-world").
		Return(mockContent, nil).Maybe()

	mockContentSvc.On("GetPublishedPages", mock.Anything).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockContentSvc.On("GetPublishedCustomPostTypes", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("GetPublishedTags", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("AuthorExists", mock.Anything, "alice").
		Return(true, nil).Maybe()

	mockContentSvc.On("GetPublishedByAuthorUsername", mock.Anything, "alice", []string{"en"}, 11, 0).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetRelated", mock.Anything, 1, 4).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).
		Return(posttype.PostType{}, assert.AnError).Maybe()

	renderer := tiptap.NewRenderer(nil)
	assembler := contentpage.NewDataAssembler(
		mockContentSvc,
		mockResolver,
		nil,
		nil,
		renderer,
		nil,
		[]string{"en"},
		nil,
		config.SiteConfig{Name: "Test Site"},
		10,
	)

	templates, err := template.NewTemplates(theme, nil)
	require.NoError(t, err)

	return ssg.NewGenerator(assembler, templates, mockContentSvc, "", theme, "https://example.com")
}

func untarArchive(t *testing.T, buf *bytes.Buffer) map[string]string {
	t.Helper()

	gzr, err := gzip.NewReader(buf)
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	contents := make(map[string]string)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if header.Typeflag != tar.TypeReg {
			continue
		}
		var sb strings.Builder
		_, _ = io.Copy(&sb, tr)
		contents[header.Name] = sb.String()
	}

	return contents
}

func writeThemeStaticAssets(t *testing.T) string {
	t.Helper()

	themeDir := t.TempDir()
	staticDir := filepath.Join(themeDir, "static")

	require.NoError(t, os.MkdirAll(filepath.Join(staticDir, "images", "icons"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(staticDir, "js"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "base.css"), []byte("/* theme base override */\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "images", "hero.jpg"), []byte("jpeg-bytes"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "images", "icons", "favicon-32x32.png"), []byte("png-bytes"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "js", "semantic-tabs.min.js"), []byte("tabs();"), 0644))

	return themeDir
}

func TestGenerator_Generate_StaticAssets(t *testing.T) {
	tests := []struct {
		name            string
		theme           func(t *testing.T) *template.Theme
		wantFiles       []string
		wantContents    map[string]string
		wantAbsentFiles []string
		wantErr         bool
	}{
		{
			name: "success - theme static assets merged with embedded fallback",
			theme: func(t *testing.T) *template.Theme {
				t.Helper()
				return &template.Theme{Dir: writeThemeStaticAssets(t)}
			},
			wantFiles: []string{
				"static/base.css",
				"static/style.css",
				"static/images/hero.jpg",
				"static/images/icons/favicon-32x32.png",
				"static/js/semantic-tabs.min.js",
				"static/search.js",
			},
			wantContents: map[string]string{
				"static/base.css":                       "/* theme base override */\n",
				"static/images/hero.jpg":                "jpeg-bytes",
				"static/images/icons/favicon-32x32.png": "png-bytes",
				"static/js/semantic-tabs.min.js":        "tabs();",
			},
			wantAbsentFiles: []string{
				"static/base.src.css",
				"static/style.src.css",
			},
			wantErr: false,
		},
		{
			name: "success - embedded defaults exported without custom theme",
			theme: func(t *testing.T) *template.Theme {
				t.Helper()
				return nil
			},
			wantFiles: []string{
				"static/base.css",
				"static/style.css",
				"static/search.js",
				"static/nav-auth.js",
				"static/math.js",
				"static/highlight.min.js",
			},
			wantContents: map[string]string{},
			wantAbsentFiles: []string{
				// Only referenced by pages the static export never renders
				// (login/register/password/verify), so they are not emitted.
				"static/auth.js",
				"static/reset-password.js",
				"static/verify-email.js",
				// Not referenced because the test content disallows comments.
				"static/comments.js",
				"static/base.src.css",
				"static/style.src.css",
			},
			wantErr: false,
		},
		{
			name: "success - theme without static directory falls back to embedded defaults",
			theme: func(t *testing.T) *template.Theme {
				t.Helper()
				return &template.Theme{Dir: t.TempDir()}
			},
			wantFiles: []string{
				"static/base.css",
				"static/style.css",
				"static/search.js",
			},
			wantContents:    map[string]string{},
			wantAbsentFiles: []string{},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := newStaticAssetsTestGenerator(t, tt.theme(t))

			var buf bytes.Buffer
			err := generator.Generate(context.Background(), &buf)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			contents := untarArchive(t, &buf)

			for _, name := range tt.wantFiles {
				assert.Contains(t, contents, name)
			}

			for name, want := range tt.wantContents {
				assert.Equal(t, want, contents[name])
			}

			for _, name := range tt.wantAbsentFiles {
				assert.NotContains(t, contents, name)
			}
		})
	}
}

func newFeedPost(
	id int,
	slug string,
	title string,
	format contentdomain.Format,
	content string,
	metaDescription string,
	ogDescription string,
) *contentdomain.Content {
	return &contentdomain.Content{
		ID:              id,
		Slug:            slug,
		Title:           title,
		Content:         content,
		Format:          format,
		Status:          contentdomain.StatusPublished,
		PostType:        "post",
		Language:        "en",
		MetaDescription: metaDescription,
		OGDescription:   ogDescription,
		CreatedAt:       time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC),
	}
}

func newFeedTestGenerator(t *testing.T, posts []*contentdomain.Content, listErr error) *ssg.Generator {
	t.Helper()

	mockContentSvc := contentpageMocks.NewMockContentService(t)
	mockResolver := contentpageMocks.NewMockPostTypeResolver(t)

	mockContentSvc.On("GetPublishedByPostType", mock.Anything, "post", []string{"en"}, 0, 0, mock.Anything, mock.Anything).
		Return(posts, listErr).Maybe()

	mockContentSvc.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockContentSvc.On("GetPublishedPages", mock.Anything).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockContentSvc.On("GetPublishedCustomPostTypes", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("GetPublishedTags", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).
		Return(posttype.PostType{}, assert.AnError).Maybe()

	renderer := tiptap.NewRenderer(nil)
	assembler := contentpage.NewDataAssembler(
		mockContentSvc,
		mockResolver,
		nil,
		nil,
		renderer,
		nil,
		[]string{"en"},
		nil,
		config.SiteConfig{Name: "Test Site"},
		10,
	)

	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	return ssg.NewGenerator(assembler, templates, mockContentSvc, "", nil, "https://example.com")
}

func TestGenerator_Generate_Feed(t *testing.T) {
	tiptapBody := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Plain body text."}]}]}`
	longBody := strings.Repeat("a", 350)
	longTiptapBody := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + longBody + `"}]}]}`

	tests := []struct {
		name            string
		posts           []*contentdomain.Content
		listErr         error
		wantContains    []string
		wantNotContains []string
		wantErr         bool
	}{
		{
			name: "success - meta description preferred as item description",
			posts: []*contentdomain.Content{
				newFeedPost(1, "hello-meta", "Hello Meta", contentdomain.FormatTiptap, tiptapBody, "Meta summary", "OG summary"),
			},
			wantContains: []string{
				"<title>Hello Meta</title>",
				"<link>https://example.com/hello-meta/</link>",
				"<guid>https://example.com/hello-meta/</guid>",
				"<description>Meta summary</description>",
				"<pubDate>Sun, 26 Jul 2026 12:00:00 +0000</pubDate>",
			},
			wantErr: false,
		},
		{
			name: "success - og description fallback when meta description empty",
			posts: []*contentdomain.Content{
				newFeedPost(2, "hello-og", "Hello OG", contentdomain.FormatTiptap, tiptapBody, "", "OG summary"),
			},
			wantContains: []string{"<description>OG summary</description>"},
			wantErr:      false,
		},
		{
			name: "success - plain text excerpt fallback for tiptap content",
			posts: []*contentdomain.Content{
				newFeedPost(3, "hello-tiptap", "Hello Tiptap", contentdomain.FormatTiptap, tiptapBody, "", ""),
			},
			wantContains: []string{"<description>Plain body text.</description>"},
			wantErr:      false,
		},
		{
			name: "success - plain text excerpt fallback for html content",
			posts: []*contentdomain.Content{
				newFeedPost(4, "hello-html", "Hello HTML", contentdomain.FormatHTML, "<p>HTML body text.</p>", "", ""),
			},
			wantContains: []string{"<description>HTML body text.</description>"},
			wantErr:      false,
		},
		{
			name: "success - long excerpts truncated with ellipsis",
			posts: []*contentdomain.Content{
				newFeedPost(5, "hello-long", "Hello Long", contentdomain.FormatTiptap, longTiptapBody, "", ""),
			},
			wantContains:    []string{"<description>" + strings.Repeat("a", 297) + "...</description>"},
			wantNotContains: []string{longBody},
			wantErr:         false,
		},
		{
			name: "success - invalid slugs are skipped from items",
			posts: []*contentdomain.Content{
				newFeedPost(6, "..", "Bad Slug", contentdomain.FormatTiptap, tiptapBody, "", ""),
			},
			wantContains:    []string{"</channel>"},
			wantNotContains: []string{"<item>"},
			wantErr:         false,
		},
		{
			name:         "success - empty post list still writes a valid channel",
			posts:        []*contentdomain.Content{},
			wantContains: []string{`<rss version="2.0">`, "<language>en</language>", "</channel>"},
			wantErr:      false,
		},
		{
			name:    "error - post listing failure fails generation",
			listErr: assert.AnError,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := newFeedTestGenerator(t, tt.posts, tt.listErr)

			var buf bytes.Buffer
			err := generator.Generate(context.Background(), &buf)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "write feed")
				return
			}
			require.NoError(t, err)

			contents := untarArchive(t, &buf)
			feedContent, ok := contents["index.xml"]
			require.True(t, ok, "index.xml should exist in the archive")

			for _, want := range tt.wantContains {
				assert.Contains(t, feedContent, want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, feedContent, notWant)
			}
		})
	}
}

func newSinglePostContentService(t *testing.T) *contentpageMocks.MockContentService {
	t.Helper()

	mockContentSvc := contentpageMocks.NewMockContentService(t)

	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	mockContent := &contentdomain.Content{
		ID:        1,
		Slug:      "hello-world",
		Title:     "Hello World",
		Content:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
		Format:    contentdomain.FormatTiptap,
		Status:    contentdomain.StatusPublished,
		PostType:  "post",
		Author:    "Alice",
		Username:  "alice",
		Language:  "en",
		CreatedAt: now,
	}

	mockContentSvc.On("GetPublishedByPostType", mock.Anything, "post", []string{"en"}, 0, 0, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetPublishedBySlugAny", mock.Anything, "hello-world").
		Return(mockContent, nil).Maybe()

	mockContentSvc.On("GetPublishedPages", mock.Anything).
		Return([]*contentdomain.Content{}, nil).Maybe()

	mockContentSvc.On("GetPublishedCustomPostTypes", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("GetPublishedTags", mock.Anything).
		Return([]string{}, nil).Maybe()

	mockContentSvc.On("AuthorExists", mock.Anything, "alice").
		Return(true, nil).Maybe()

	mockContentSvc.On("GetPublishedByAuthorUsername", mock.Anything, "alice", []string{"en"}, 11, 0).
		Return([]*contentdomain.Content{mockContent}, nil).Maybe()

	mockContentSvc.On("GetRelated", mock.Anything, 1, 4).
		Return([]*contentdomain.Content{}, nil).Maybe()

	return mockContentSvc
}

func newAliasRedirectsTestGenerator(
	t *testing.T,
	mockContentSvc *contentpageMocks.MockContentService,
	aliasSvc *aliasdomain.Service,
) *ssg.Generator {
	t.Helper()

	mockResolver := contentpageMocks.NewMockPostTypeResolver(t)
	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).
		Return(posttype.PostType{}, assert.AnError).Maybe()

	renderer := tiptap.NewRenderer(nil)
	assembler := contentpage.NewDataAssembler(
		mockContentSvc,
		mockResolver,
		nil,
		nil,
		renderer,
		nil,
		[]string{"en"},
		nil,
		config.SiteConfig{Name: "Test Site"},
		10,
	)

	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	return ssg.NewGenerator(
		assembler,
		templates,
		mockContentSvc,
		"",
		nil,
		"https://example.com",
	).WithAliases(aliasSvc)
}

func TestGenerator_Generate_AliasRedirects(t *testing.T) {
	publishedTarget := &contentdomain.Content{
		ID:        5,
		Slug:      "new-post",
		Title:     "New Post",
		Status:    contentdomain.StatusPublished,
		PostType:  "post",
		Language:  "en",
		CreatedAt: time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name            string
		setupAliases    func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service)
		wantFiles       []string
		wantContents    map[string]string
		wantAbsentFiles []string
		wantErr         bool
	}{
		{
			name: "success - legacy .html alias emits meta-refresh page",
			setupAliases: func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service) {
				contentSvc := newSinglePostContentService(t)
				contentSvc.On("GetPublishedByID", mock.Anything, 5).
					Return(publishedTarget, nil).Maybe()

				repo := aliasMocks.NewMockRepository(t)
				repo.On("FindAll", mock.Anything).
					Return([]*aliasdomain.Alias{{ID: 1, ContentID: 5, Alias: "posts/2024/new-post.html"}}, nil)

				return contentSvc, aliasdomain.NewService(repo)
			},
			wantFiles: []string{
				"posts/2024/new-post.html",
			},
			wantContents: map[string]string{
				"posts/2024/new-post.html": "<!DOCTYPE html>\n<html>\n<head>\n\t<meta charset=\"UTF-8\">\n\t<meta name=\"robots\" content=\"noindex, follow\">\n\t<title>Redirecting&hellip;</title>\n\t<link rel=\"canonical\" href=\"https://example.com/new-post/\">\n\t<meta http-equiv=\"refresh\" content=\"0; url=https://example.com/new-post/\">\n</head>\n<body>\n\t<p>Page moved. Continue to <a href=\"https://example.com/new-post/\">https://example.com/new-post/</a>.</p>\n</body>\n</html>\n",
			},
			wantAbsentFiles: []string{},
			wantErr:         false,
		},
		{
			name: "success - extensionless alias becomes directory index page",
			setupAliases: func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service) {
				contentSvc := newSinglePostContentService(t)
				contentSvc.On("GetPublishedByID", mock.Anything, 5).
					Return(publishedTarget, nil).Maybe()

				repo := aliasMocks.NewMockRepository(t)
				repo.On("FindAll", mock.Anything).
					Return([]*aliasdomain.Alias{{ID: 2, ContentID: 5, Alias: "legacy-link"}}, nil)

				return contentSvc, aliasdomain.NewService(repo)
			},
			wantFiles: []string{
				"legacy-link/index.html",
			},
			wantContents: map[string]string{},
			wantAbsentFiles: []string{
				"legacy-link.html",
			},
			wantErr: false,
		},
		{
			name: "skip - unpublished target emits nothing",
			setupAliases: func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service) {
				contentSvc := newSinglePostContentService(t)
				contentSvc.On("GetPublishedByID", mock.Anything, 7).
					Return(nil, errors.New("not found")).Maybe()

				repo := aliasMocks.NewMockRepository(t)
				repo.On("FindAll", mock.Anything).
					Return([]*aliasdomain.Alias{{ID: 3, ContentID: 7, Alias: "gone-page"}}, nil)

				return contentSvc, aliasdomain.NewService(repo)
			},
			wantFiles:    []string{},
			wantContents: map[string]string{},
			wantAbsentFiles: []string{
				"gone-page/index.html",
			},
			wantErr: false,
		},
		{
			name: "skip - traversal alias is rejected without target lookup",
			setupAliases: func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service) {
				contentSvc := newSinglePostContentService(t)

				repo := aliasMocks.NewMockRepository(t)
				repo.On("FindAll", mock.Anything).
					Return([]*aliasdomain.Alias{{ID: 4, ContentID: 5, Alias: "../escape"}}, nil)

				return contentSvc, aliasdomain.NewService(repo)
			},
			wantFiles:    []string{},
			wantContents: map[string]string{},
			wantAbsentFiles: []string{
				"escape/index.html",
				"../escape/index.html",
			},
			wantErr: false,
		},
		{
			name: "skip - alias shadowing an emitted page wins nothing",
			setupAliases: func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service) {
				contentSvc := newSinglePostContentService(t)
				// The alias resolves to a different slug than the emitted
				// hello-world page, so it must NOT overwrite the real page.
				contentSvc.On("GetPublishedByID", mock.Anything, 5).
					Return(publishedTarget, nil).Maybe()

				repo := aliasMocks.NewMockRepository(t)
				repo.On("FindAll", mock.Anything).
					Return([]*aliasdomain.Alias{{ID: 6, ContentID: 5, Alias: "hello-world"}}, nil)

				return contentSvc, aliasdomain.NewService(repo)
			},
			wantFiles: []string{
				"hello-world/index.html",
			},
			wantContents: map[string]string{},
			wantErr:      false,
		},
		{
			name: "success - aliases excluded from sitemap",
			setupAliases: func(t *testing.T) (*contentpageMocks.MockContentService, *aliasdomain.Service) {
				contentSvc := newSinglePostContentService(t)
				contentSvc.On("GetPublishedByID", mock.Anything, 5).
					Return(publishedTarget, nil).Maybe()

				repo := aliasMocks.NewMockRepository(t)
				repo.On("FindAll", mock.Anything).
					Return([]*aliasdomain.Alias{{ID: 8, ContentID: 5, Alias: "old-permalink"}}, nil)

				return contentSvc, aliasdomain.NewService(repo)
			},
			wantFiles: []string{
				"old-permalink/index.html",
			},
			wantContents:    map[string]string{},
			wantAbsentFiles: []string{},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentSvc, aliasSvc := tt.setupAliases(t)
			generator := newAliasRedirectsTestGenerator(t, contentSvc, aliasSvc)

			var buf bytes.Buffer
			err := generator.Generate(context.Background(), &buf)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			contents := untarArchive(t, &buf)

			for _, name := range tt.wantFiles {
				assert.Contains(t, contents, name)
			}

			for name, want := range tt.wantContents {
				assert.Equal(t, want, contents[name])
			}

			for _, name := range tt.wantAbsentFiles {
				assert.NotContains(t, contents, name)
			}
		})
	}
}

func TestGenerator_Generate_AliasRedirects_SitemapExclusion(t *testing.T) {
	publishedTarget := &contentdomain.Content{
		ID:        5,
		Slug:      "new-post",
		Title:     "New Post",
		Status:    contentdomain.StatusPublished,
		PostType:  "post",
		Language:  "en",
		CreatedAt: time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC),
	}

	contentSvc := newSinglePostContentService(t)
	contentSvc.On("GetPublishedByID", mock.Anything, 5).
		Return(publishedTarget, nil).Maybe()

	repo := aliasMocks.NewMockRepository(t)
	repo.On("FindAll", mock.Anything).
		Return([]*aliasdomain.Alias{{ID: 9, ContentID: 5, Alias: "old-permalink"}}, nil)

	generator := newAliasRedirectsTestGenerator(t, contentSvc, aliasdomain.NewService(repo))

	var buf bytes.Buffer
	require.NoError(t, generator.Generate(context.Background(), &buf))

	contents := untarArchive(t, &buf)
	sitemap, ok := contents["sitemap.xml"]
	require.True(t, ok, "sitemap.xml should exist in the archive")

	assert.Contains(t, sitemap, "<loc>https://example.com/hello-world/</loc>")
	assert.NotContains(t, sitemap, "<loc>https://example.com/hello-world</loc>")
	assert.NotContains(t, sitemap, "old-permalink")
}

func TestGenerator_Generate_ThemeRootFiles(t *testing.T) {
	themeDir := writeThemeStaticAssets(t)
	rootDir := filepath.Join(themeDir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "webpushr-sw.js"), []byte("console.log('sw')"), 0644))

	generator := newStaticAssetsTestGenerator(t, &template.Theme{Dir: themeDir})

	var buf bytes.Buffer
	require.NoError(t, generator.Generate(context.Background(), &buf))

	contents := untarArchive(t, &buf)

	assert.Equal(t, "console.log('sw')", contents["webpushr-sw.js"])
	assert.NotContains(t, contents, "root/webpushr-sw.js")
}
