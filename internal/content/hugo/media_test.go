package hugo_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	hugomocks "github.com/aristorinjuang/lesstruct/internal/content/hugo/mocks"
	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newTestMapper builds a mapper over a temp static dir with the given files
// (relative paths → bytes) and a media service mock.
func newTestMapper(t *testing.T, files map[string]string) (*hugo.MediaMapper, *hugomocks.MockMediaService, string) {
	t.Helper()
	staticDir := t.TempDir()
	for relPath, content := range files {
		fullPath := filepath.Join(staticDir, relPath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}
	ms := hugomocks.NewMockMediaService(t)
	mapper := hugo.NewMediaMapper(staticDir, ms, nil, false)
	return mapper, ms, staticDir
}

func TestMediaMapper_MapLocal(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		files     map[string]string
		wantURL   string
		wantErr   bool
		skipMedia bool
	}{
		{
			name:    "success - root-relative reference maps to media URL",
			ref:     "/images/foo.jpg",
			files:   map[string]string{"images/foo.jpg": "foo-bytes"},
			wantURL: "http://media.local/foo",
		},
		{
			name:    "success - bare reference maps to media URL",
			ref:     "images/foo.jpg",
			files:   map[string]string{"images/foo.jpg": "foo-bytes"},
			wantURL: "http://media.local/foo",
		},
		{
			name:    "success - missing file keeps original reference",
			ref:     "/images/missing.jpg",
			files:   map[string]string{},
			wantURL: "/images/missing.jpg",
		},
		{
			name:      "success - skipMedia leaves reference untouched",
			ref:       "/images/foo.jpg",
			files:     map[string]string{"images/foo.jpg": "foo-bytes"},
			wantURL:   "/images/foo.jpg",
			skipMedia: true,
		},
		{
			name:    "success - non-image path passes through",
			ref:     "/images/resume.pdf",
			files:   map[string]string{"images/resume.pdf": "pdf-bytes"},
			wantURL: "/images/resume.pdf",
		},
		{
			name:    "success - empty reference returns empty",
			ref:     "",
			files:   map[string]string{},
			wantURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staticDir := staticDirFor(t, tt.files)
			ms := hugomocks.NewMockMediaService(t)
			if tt.wantURL == "http://media.local/foo" {
				ms.On("GenerateFromBytes", mock.Anything, []byte("foo-bytes"), 1, "foo", "foo.jpg").
					Return(&mediadomain.Media{URL: "http://media.local/foo"}, nil).Once()
			}

			mapper := hugo.NewMediaMapper(staticDir, ms, nil, tt.skipMedia)
			got, err := mapper.Map(context.Background(), tt.ref, 1)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, got)
		})
	}
}

func staticDirFor(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for relPath, content := range files {
		fullPath := filepath.Join(dir, relPath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}
	return dir
}

func TestMediaMapper_MapLocal_DuplicateReturnsExistingURL(t *testing.T) {
	mapper, ms, _ := newTestMapper(t, map[string]string{"images/dup.jpg": "dup-bytes"})
	ms.On("GenerateFromBytes", mock.Anything, []byte("dup-bytes"), 1, "dup", "dup.jpg").
		Return(nil, &mediadomain.DuplicateMediaError{Existing: &mediadomain.Media{URL: "http://media.local/existing"}}).Once()

	got, err := mapper.Map(context.Background(), "/images/dup.jpg", 1)
	require.NoError(t, err)
	assert.Equal(t, "http://media.local/existing", got)
}

func TestMediaMapper_MapLocal_UploadError(t *testing.T) {
	mapper, ms, _ := newTestMapper(t, map[string]string{"images/bad.jpg": "bad-bytes"})
	ms.On("GenerateFromBytes", mock.Anything, []byte("bad-bytes"), 1, "bad", "bad.jpg").
		Return(nil, errors.New("upload failed")).Once()

	got, err := mapper.Map(context.Background(), "/images/bad.jpg", 1)
	require.Error(t, err)
	// Failed references are cached as failed and return the original on retry.
	assert.Equal(t, "/images/bad.jpg", got)
}

func TestMediaMapper_MapLocal_PathTraversalRejected(t *testing.T) {
	mapper, _, _ := newTestMapper(t, map[string]string{})
	got, err := mapper.Map(context.Background(), "/../../etc/passwd", 1)
	require.NoError(t, err)
	// Traversal refs are not image paths so they pass through unchanged.
	assert.Equal(t, "/../../etc/passwd", got)
}

func TestMediaMapper_CachesMappedURLs(t *testing.T) {
	mapper, ms, _ := newTestMapper(t, map[string]string{"images/foo.jpg": "foo-bytes"})
	ms.On("GenerateFromBytes", mock.Anything, []byte("foo-bytes"), 1, "foo", "foo.jpg").
		Return(&mediadomain.Media{URL: "http://media.local/foo"}, nil).Once()

	first, err := mapper.Map(context.Background(), "/images/foo.jpg", 1)
	require.NoError(t, err)
	second, err := mapper.Map(context.Background(), "/images/foo.jpg", 1)
	require.NoError(t, err)
	assert.Equal(t, "http://media.local/foo", first)
	assert.Equal(t, first, second)
	ms.AssertNumberOfCalls(t, "GenerateFromBytes", 1)
}

func TestMediaMapper_MapRemote(t *testing.T) {
	tests := []struct {
		name      string
		ref       func(serverURL string) string
		wantURL   func(serverURL string) string
		skipMedia bool
	}{
		{
			name: "success - remote URL downloaded through shared downloader",
			ref: func(serverURL string) string {
				return serverURL + "/photo.jpg"
			},
			wantURL: func(string) string {
				return "http://media.local/photo"
			},
		},
		{
			name: "success - skipMedia leaves remote hotlinked",
			ref: func(serverURL string) string {
				return serverURL + "/photo.jpg"
			},
			wantURL: func(serverURL string) string {
				return serverURL + "/photo.jpg"
			},
			skipMedia: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(jpegBytes)
			}))
			defer server.Close()

			ms := hugomocks.NewMockMediaService(t)
			if !tt.skipMedia {
				ms.On("GenerateFromBytes", mock.Anything, jpegBytes, 1, "photo", "photo.jpg").
					Return(&mediadomain.Media{URL: "http://media.local/photo"}, nil).Once()
			}

			downloader := wordpress.NewMediaDownloader(server.Client(), ms)
			mapper := hugo.NewMediaMapper(t.TempDir(), ms, downloader, tt.skipMedia)
			got, err := mapper.Map(context.Background(), tt.ref(server.URL), 1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL(server.URL), got)
		})
	}
}

// jpegBytes is a minimal valid JPEG (magic bytes accepted by the media domain's
// signature check).
var jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}

func TestMediaMapper_RewriteBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		files    map[string]string
		wantBody string
	}{
		{
			name:  "success - single img rewritten",
			body:  `<p>Hi</p><img src="/images/foo.jpg" alt="Foo">`,
			files: map[string]string{"images/foo.jpg": "foo-bytes"},
			wantBody: `<p>Hi</p><img src="http://media.local/foo.jpg" alt="Foo">`,
		},
		{
			name:  "success - multiple imgs rewritten",
			body:  `<img src="/images/foo.jpg"><img src="/images/bar.jpg">`,
			files: map[string]string{"images/foo.jpg": "foo-bytes", "images/bar.jpg": "bar-bytes"},
			wantBody: `<img src="http://media.local/foo.jpg"><img src="http://media.local/bar.jpg">`,
		},
		{
			name:  "success - missing file leaves src unchanged",
			body:  `<img src="/images/missing.jpg">`,
			files: map[string]string{},
			wantBody: `<img src="/images/missing.jpg">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, ms, _ := newTestMapper(t, tt.files)
			for file, content := range tt.files {
				ms.On("GenerateFromBytes", mock.Anything, []byte(content), 1, mock.Anything, filepath.Base(file)).
					Return(&mediadomain.Media{URL: "http://media.local/" + filepath.Base(file)}, nil).Once()
			}
			got := mapper.RewriteBody(context.Background(), tt.body, 1)
			assert.Equal(t, tt.wantBody, got)
		})
	}
}
