package export

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockContentService struct {
	mock.Mock
}

func (m *mockContentService) GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*contentdomain.Content), args.Error(1)
}

func (m *mockContentService) GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error) {
	args := m.Called(ctx, translationGroupID, excludeID)
	return args.Get(0).([]*contentdomain.Content), args.Error(1)
}

type mockAliasService struct {
	mock.Mock
}

func (m *mockAliasService) FindByContentID(ctx context.Context, contentID int) ([]*aliasdomain.Alias, error) {
	args := m.Called(ctx, contentID)
	return args.Get(0).([]*aliasdomain.Alias), args.Error(1)
}

func TestExporter_Export(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	contentSvc := new(mockContentService)
	aliasSvc := new(mockAliasService)

	aliasSvc.On("FindByContentID", mock.Anything, 1).Return([]*aliasdomain.Alias{}, nil)
	aliasSvc.On("FindByContentID", mock.Anything, 2).Return([]*aliasdomain.Alias{
		{ID: 1, ContentID: 2, Alias: "old-url.html"},
	}, nil)

	contentSvc.On("GetAll", mock.Anything, 100, 0).Return([]*contentdomain.Content{
		{
			ID:        1,
			Title:     "First Post",
			Slug:      "first-post",
			Content:   `<p>Hello world</p>`,
			Format:    contentdomain.FormatHTML,
			Status:    contentdomain.StatusPublished,
			PostType:  "post",
			Language:  "en",
			Tags:      []string{"go"},
			CreatedAt: now,
		},
		{
			ID:        2,
			Title:     "Second Post",
			Slug:      "second-post",
			Content:   `<p>Second content</p>`,
			Format:    contentdomain.FormatHTML,
			Status:    contentdomain.StatusPublished,
			PostType:  "post",
			Language:  "en",
			CreatedAt: now,
		},
	}, nil)
	contentSvc.On("GetAll", mock.Anything, 100, 100).Return([]*contentdomain.Content{}, nil)

	bodyToHTML := func(s string) (string, error) {
		return s, nil
	}

	exporter := NewExporter(
		contentSvc,
		aliasSvc,
		bodyToHTML,
		"",
		nil,
	)

	var buf bytes.Buffer
	result, err := exporter.Export(context.Background(), &buf)
	require.NoError(t, err)
	assert.Equal(t, 2, result.ContentExported)
	assert.Equal(t, 0, result.MediaBundled)
	assert.Empty(t, result.Errors)

	gzr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	var files []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		files = append(files, header.Name)

		if header.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			require.NoError(t, err)
			content := string(data)
			assert.Contains(t, content, "---\n", "file %s should have frontmatter", header.Name)
			assert.Contains(t, content, "title:", "file %s should have title in frontmatter", header.Name)
		}
	}

	assert.Contains(t, files, "content/post/first-post.en.html")
	assert.Contains(t, files, "content/post/second-post.en.html")
}

func TestExporter_Export_WithTipTapContent(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	contentSvc := new(mockContentService)
	aliasSvc := new(mockAliasService)

	aliasSvc.On("FindByContentID", mock.Anything, 1).Return([]*aliasdomain.Alias{}, nil)

	contentSvc.On("GetAll", mock.Anything, 100, 0).Return([]*contentdomain.Content{
		{
			ID:        1,
			Title:     "TipTap Content",
			Slug:      "tiptap-content",
			Content:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
			Format:    contentdomain.FormatTiptap,
			Status:    contentdomain.StatusPublished,
			PostType:  "post",
			Language:  "en",
			CreatedAt: now,
		},
	}, nil)
	contentSvc.On("GetAll", mock.Anything, 100, 100).Return([]*contentdomain.Content{}, nil)

	bodyToHTML := func(s string) (string, error) {
		return "<p>Hello</p>", nil
	}

	exporter := NewExporter(
		contentSvc,
		aliasSvc,
		bodyToHTML,
		"",
		nil,
	)

	var buf bytes.Buffer
	result, err := exporter.Export(context.Background(), &buf)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ContentExported)

	gzr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	header, err := tr.Next()
	require.NoError(t, err)
	assert.Equal(t, "content/post/tiptap-content.en.html", header.Name)

	data, err := io.ReadAll(tr)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<p>Hello</p>")
}

func TestExporter_Export_WithMedia(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	mediaDir := t.TempDir()
	mediaFile := filepath.Join(mediaDir, "abc123.webp")
	err := os.WriteFile(mediaFile, []byte("fake-image-data"), 0644)
	require.NoError(t, err)

	contentSvc := new(mockContentService)
	aliasSvc := new(mockAliasService)

	aliasSvc.On("FindByContentID", mock.Anything, 1).Return([]*aliasdomain.Alias{}, nil)

	contentSvc.On("GetAll", mock.Anything, 100, 0).Return([]*contentdomain.Content{
		{
			ID:        1,
			Title:     "Post With Image",
			Slug:      "post-with-image",
			Content:   `<p><img src="/uploads/media/abc123.webp" alt="test"></p>`,
			Format:    contentdomain.FormatHTML,
			Status:    contentdomain.StatusPublished,
			PostType:  "post",
			Language:  "en",
			CreatedAt: now,
		},
	}, nil)
	contentSvc.On("GetAll", mock.Anything, 100, 100).Return([]*contentdomain.Content{}, nil)

	bodyToHTML := func(s string) (string, error) {
		return s, nil
	}

	logger := util.NewLogger(io.Discard)
	exporter := NewExporter(
		contentSvc,
		aliasSvc,
		bodyToHTML,
		mediaDir,
		logger,
	)

	var buf bytes.Buffer
	result, err := exporter.Export(context.Background(), &buf)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ContentExported)
	assert.Equal(t, 1, result.MediaBundled)

	gzr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	foundContent := false
	foundMedia := false

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		switch header.Name {
		case "content/post/post-with-image.en.html":
			foundContent = true
		case "static/uploads/media/abc123.webp":
			foundMedia = true
			data, err := io.ReadAll(tr)
			require.NoError(t, err)
			assert.Equal(t, "fake-image-data", string(data))
		}
	}

	assert.True(t, foundContent, "content file should be in tar")
	assert.True(t, foundMedia, "media file should be in tar")
}
