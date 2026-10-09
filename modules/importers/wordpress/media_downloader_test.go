package wordpress_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pngSignature is the 8-byte PNG magic sequence; isImageContent only inspects
// magic bytes, so this is enough to pass validation in tests.
var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

// fakeMediaService records GenerateFromBytes calls and returns a canned result.
type fakeMediaService struct {
	called        int
	lastBytes     []byte
	lastAlt       string
	lastFilename  string
	returnMedia   *mediadomain.Media
	returnErr     error
	urlByFilename map[string]string // optional per-image URL override keyed by original filename
}

func (f *fakeMediaService) GenerateFromBytes(_ context.Context, b []byte, _ kernel.Principal, alt, filename string) (*mediadomain.Media, error) {
	f.called++
	f.lastBytes = b
	f.lastAlt = alt
	f.lastFilename = filename
	media := f.returnMedia
	if u, ok := f.urlByFilename[filename]; ok {
		media = &mediadomain.Media{URL: u}
	}
	return media, f.returnErr
}

func TestMediaDownloader_Success(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "success - downloads and re-uploads image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(pngSignature)
			}))
			defer server.Close()

			mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "http://localhost:8080/uploads/media/abc.webp"}}
			dl := wordpress.NewMediaDownloader(server.Client(), mediaSvc, wordpress.MediaDownloaderAllowPrivateHosts())

			url, err := dl.DownloadAndUpload(context.Background(), server.URL+"/image.png", 1)
			require.NoError(t, err)
			assert.Equal(t, "http://localhost:8080/uploads/media/abc.webp", url)
			assert.Equal(t, 1, mediaSvc.called)
			assert.Equal(t, "image.png", mediaSvc.lastFilename)
			assert.Equal(t, pngSignature, mediaSvc.lastBytes)
		})
	}
}

func TestMediaDownloader_CachesByURL(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "success - second call reuses cached result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				_, _ = w.Write(pngSignature)
			}))
			defer server.Close()

			mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "http://localhost:8080/uploads/media/cached.webp"}}
			dl := wordpress.NewMediaDownloader(server.Client(), mediaSvc, wordpress.MediaDownloaderAllowPrivateHosts())

			url1, err := dl.DownloadAndUpload(context.Background(), server.URL+"/image.png", 1)
			require.NoError(t, err)
			url2, err := dl.DownloadAndUpload(context.Background(), server.URL+"/image.png", 1)
			require.NoError(t, err)

			assert.Equal(t, url1, url2)
			assert.Equal(t, 1, hits, "image should be downloaded from the server only once")
			assert.Equal(t, 1, mediaSvc.called, "media service should be invoked only once")
		})
	}
}

func TestMediaDownloader_NonImageSkipped(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "success - mp3 url skipped without download"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "x"}}
			dl := wordpress.NewMediaDownloader(nil, mediaSvc, wordpress.MediaDownloaderAllowPrivateHosts())

			url, err := dl.DownloadAndUpload(context.Background(), "http://wp.local/song.mp3", 1)
			require.NoError(t, err)
			assert.Empty(t, url)
			assert.Equal(t, 0, mediaSvc.called, "non-image URLs must not invoke the media service")
		})
	}
}

func TestMediaDownloader_HTTPFailure(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "error - 404 returns error and records failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "x"}}
			dl := wordpress.NewMediaDownloader(server.Client(), mediaSvc, wordpress.MediaDownloaderAllowPrivateHosts())

			url, err := dl.DownloadAndUpload(context.Background(), server.URL+"/missing.png", 1)
			require.Error(t, err)
			assert.Empty(t, url)
			assert.Equal(t, 0, mediaSvc.called)

			// Second call should hit the failed-cache and not retry.
			url2, err2 := dl.DownloadAndUpload(context.Background(), server.URL+"/missing.png", 1)
			require.NoError(t, err2)
			assert.Empty(t, url2)
		})
	}
}

func TestMediaDownloader_DuplicateReturnsExisting(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "success - duplicate hash reuses existing media URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(pngSignature)
			}))
			defer server.Close()

			mediaSvc := &fakeMediaService{
				returnErr: &mediadomain.DuplicateMediaError{Existing: &mediadomain.Media{URL: "http://localhost:8080/uploads/media/existing.webp"}},
			}
			dl := wordpress.NewMediaDownloader(server.Client(), mediaSvc, wordpress.MediaDownloaderAllowPrivateHosts())

			url, err := dl.DownloadAndUpload(context.Background(), server.URL+"/image.png", 1)
			require.NoError(t, err, "duplicate should not surface as an error")
			assert.Equal(t, "http://localhost:8080/uploads/media/existing.webp", url)
		})
	}
}

func TestMediaDownloader_UnreachableURL(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "error - connection refused returns error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A closed listener yields a connection error on the first request.
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			server.Close()

			mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "x"}}
			dl := wordpress.NewMediaDownloader(server.Client(), mediaSvc, wordpress.MediaDownloaderAllowPrivateHosts())

			url, err := dl.DownloadAndUpload(context.Background(), server.URL+"/image.png", 1)
			require.Error(t, err)
			assert.Empty(t, url)
			assert.False(t, errors.Is(err, mediadomain.ErrDuplicateMedia))
		})
	}
}

func TestMediaDownloader_SSRFGuard(t *testing.T) {
	// The downloader fetches URLs from operator-uploaded WXR files: http(s)
	// only, and hosts must not resolve to loopback, private, or link-local
	// addresses. Without the allow-private hook these are all rejected.
	mediaSvc := &fakeMediaService{}

	tests := []struct {
		name     string
		imageURL string
		contains string
	}{
		{
			name:     "error - ftp scheme",
			imageURL: "ftp://example.com/cover.png",
			contains: "must use http or https",
		},
		{
			name:     "error - loopback IP literal",
			imageURL: "http://127.0.0.1/cover.png",
			contains: "forbidden address",
		},
		{
			name:     "error - loopback hostname",
			imageURL: "http://localhost/cover.png",
			contains: "forbidden address",
		},
		{
			name:     "error - IPv6 loopback",
			imageURL: "http://[::1]/cover.png",
			contains: "forbidden address",
		},
		{
			name:     "error - link-local (cloud metadata)",
			imageURL: "http://169.254.169.254/cover.png",
			contains: "forbidden address",
		},
		{
			name:     "error - private range",
			imageURL: "http://10.0.0.5/cover.png",
			contains: "forbidden address",
		},
		{
			name:     "error - unresolvable host",
			imageURL: "http://this-host-does-not-exist.invalid/cover.png",
			contains: "cannot resolve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dl := wordpress.NewMediaDownloader(nil, mediaSvc)

			url, err := dl.DownloadAndUpload(context.Background(), tt.imageURL, 1)

			require.Error(t, err)
			assert.Empty(t, url)
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestMediaDownloader_SSRFGuard_PrivateAllowed(t *testing.T) {
	// With the test hook the loopback httptest server is reachable — the
	// existing success tests depend on this.
	body := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', // signature
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "http://example.com/uploads/cover.webp"}}
	dl := wordpress.NewMediaDownloader(
		server.Client(),
		mediaSvc,
		wordpress.MediaDownloaderAllowPrivateHosts(),
	)

	url, err := dl.DownloadAndUpload(context.Background(), server.URL+"/cover.png", 1)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/uploads/cover.webp", url)
}
