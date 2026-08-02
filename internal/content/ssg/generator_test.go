package ssg_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/contentpage"
	contentpageMocks "github.com/aristorinjuang/lesstruct/internal/api/contentpage/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/content/ssg"
	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	"github.com/aristorinjuang/lesstruct/internal/config"
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

	mockContentSvc.On("GetPublishedByPostType", mock.Anything, "post", "en", 0, 0, mock.Anything, mock.Anything).
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

	mockContentSvc.On("GetPublishedByAuthorUsername", mock.Anything, "alice", "en", 11, 0).
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

	sitemapContent, ok := contents["sitemap.xml"]
	if assert.True(t, ok, "sitemap.xml should be readable") {
		assert.Contains(t, sitemapContent, "https://example.com/", "sitemap should contain absolute URLs")
		assert.Contains(t, sitemapContent, "https://example.com/hello-world", "sitemap should reference content page")
		assert.NotContains(t, sitemapContent, "<loc>/", "sitemap should not contain relative URLs")
	}

	robotsContent, ok := contents["robots.txt"]
	if assert.True(t, ok, "robots.txt should be readable") {
		assert.Contains(t, robotsContent, "https://example.com/sitemap.xml", "robots.txt should reference absolute sitemap URL")
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
