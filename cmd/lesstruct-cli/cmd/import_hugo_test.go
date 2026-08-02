package cmd_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHugoImportServer returns a test server that accepts a Hugo import upload
// (202 + jobId), then serves a terminal status when polled. The uploaded
// archive bytes are written to *gotArchive for assertions.
func newHugoImportServer(t *testing.T, gotArchive *[]byte, gotSkipMedia *bool) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/hugo/import":
			mu.Lock()
			defer mu.Unlock()
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			raw, err := io.ReadAll(f)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			*gotArchive = raw
			*gotSkipMedia = r.FormValue("skipMedia") == "true"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":{"jobId":"job-123","state":"running"},"error":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/hugo/import/status/job-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"jobId":"job-123","job":{"state":"done","imported":2,"skipped":1,"total":3,"errors":["skipped \"x\": boom"]}},"error":null}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestImportHugo_FromDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "content/posts"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "static/images"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "content/posts/post.html"),
		[]byte("---\ntitle: Post\n---\n<p>Hello</p>"),
		0o600,
	))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "static/images/foo.jpg"), []byte("img"), 0o600))
	// Junk that must NOT be archived.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "themes/custom"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "themes/custom/layout.html"), []byte("layout"), 0o600))

	var gotArchive []byte
	var gotSkipMedia bool
	srv := newHugoImportServer(t, &gotArchive, &gotSkipMedia)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"import", "hugo", "--source", dir,
			"--base-url", srv.URL, "--api-key", "k",
			"--poll-interval", "10ms",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Contains(t, out.String(), "Import complete: 2 imported, 1 skipped")
	assert.False(t, gotSkipMedia, "skipMedia omitted by default")

	// The archive must contain content/ and static/ but not themes/.
	names := readTarGzNames(t, gotArchive)
	assert.Contains(t, names, "content/posts/post.html")
	assert.Contains(t, names, "static/images/foo.jpg")
	assert.NotContains(t, names, "themes/custom/layout.html")
}

func TestImportHugo_FromDirectory_WithSkipMedia(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "content"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "content/post.md"),
		[]byte("---\ntitle: Post\n---\nBody"),
		0o600,
	))

	var gotArchive []byte
	var gotSkipMedia bool
	srv := newHugoImportServer(t, &gotArchive, &gotSkipMedia)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"import", "hugo", "--source", dir,
			"--base-url", srv.URL, "--api-key", "k",
			"--poll-interval", "10ms",
			"--skip-media",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.True(t, gotSkipMedia, "skipMedia field sent when --skip-media passed")
}

func TestImportHugo_FromArchive(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	content := "---\ntitle: Post\n---\n<p>Hi</p>"
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "content/post.html", Size: int64(len(content)), Mode: 0o600}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	archivePath := filepath.Join(t.TempDir(), "site.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, buf.Bytes(), 0o600))

	var gotArchive []byte
	var gotSkipMedia bool
	srv := newHugoImportServer(t, &gotArchive, &gotSkipMedia)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"import", "hugo", "--source", archivePath,
			"--base-url", srv.URL, "--api-key", "k",
			"--poll-interval", "10ms",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	// The archive is passed through unchanged (not re-archived).
	assert.Equal(t, buf.Bytes(), gotArchive)
}

func TestImportHugo_UploadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"INSUFFICIENT_PERMISSIONS","message":"admin required"}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "content"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "content/post.html"),
		[]byte("---\ntitle: Post\n---\n<p>Hi</p>"),
		0o600,
	))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"import", "hugo", "--source", dir,
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.NotEqual(t, 0, code)
	assert.Contains(t, errOut.String(), "admin required")
}

func TestImportHugo_JobFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/hugo/import":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":{"jobId":"job-fail","state":"running"},"error":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/hugo/import/status/job-fail":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"jobId":"job-fail","job":{"state":"failed","imported":1,"skipped":2,"total":3,"errors":["boom"]}},"error":null}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "content"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "content/post.html"),
		[]byte("---\ntitle: Post\n---\n<p>Hi</p>"),
		0o600,
	))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"import", "hugo", "--source", dir,
			"--base-url", srv.URL, "--api-key", "k",
			"--poll-interval", "10ms",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.NotEqual(t, 0, code)
	assert.Contains(t, errOut.String(), "Import failed after importing 1 items, 2 skipped")
	assert.Contains(t, errOut.String(), "boom")
}

func TestImportHugo_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{
			name:   "missing --source",
			args:   []string{"import", "hugo"},
			wantIn: "source",
		},
		{
			name:   "nonexistent source",
			args:   []string{"import", "hugo", "--source", "/nonexistent/xyz"},
			wantIn: "read --source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(
				append(tt.args, "--base-url", "http://localhost:9999", "--api-key", "k"),
				strings.NewReader(""),
				&out,
				&errOut,
			)
			require.NotEqual(t, 0, code)
			assert.Contains(t, errOut.String(), tt.wantIn)
		})
	}
}

func TestImportHugo_InvalidPollInterval(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "content"), 0755))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"import", "hugo", "--source", dir,
			"--base-url", "http://localhost:9999", "--api-key", "k",
			"--poll-interval", "bogus",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.NotEqual(t, 0, code)
	assert.Contains(t, errOut.String(), "invalid --poll-interval")
}

// readTarGzNames lists the file names inside a tar.gz archive.
func readTarGzNames(t *testing.T, raw []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	require.NoError(t, err)
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, hdr.Name)
	}
	return names
}
