package cmd_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mediaServer is a media-specific recording server: parses the multipart
// request and exposes the file bytes + metadata JSON + filename for assertions.
type mediaServer struct {
	gotMethod   string
	gotPath     string
	gotCT       string
	gotFile     []byte
	gotFilename string
	gotMetadata string
}

func (m *mediaServer) handler(t *testing.T, status int, body string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.gotMethod = r.Method
		m.gotPath = r.URL.Path
		m.gotCT = r.Header.Get("Content-Type")
		if strings.HasPrefix(m.gotCT, "multipart/form-data") {
			require.NoError(t, r.ParseMultipartForm(32<<20))
			if f, fh, err := r.FormFile("file"); err == nil {
				m.gotFile, _ = readAll(f)
				m.gotFilename = fh.Filename
			}
			m.gotMetadata = r.FormValue("metadata")
		}
		w.WriteHeader(status)
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}
	})
}

// readAll is a tiny helper that just wraps io.ReadAll — defined locally to
// avoid importing "io" here (the package would otherwise need a single
// import line; this keeps the import set minimal).
func readAll(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return buf, nil
			}
			return buf, err
		}
	}
}

const mediaSuccessBody = `{"data":{"media":{"id":5,"filename":"photo.jpg","originalFilename":"photo.jpg","mimeType":"image/jpeg","fileSize":11,"altText":"a view","isWebp":false,"hash":"abc","url":"http://example/uploads/photo.jpg"}}}`

func TestMediaUpload_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(path, []byte("fake-jpeg-bytes"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mediaSuccessBody))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), "#5")
	assert.Contains(t, out.String(), "photo.jpg")
	assert.Contains(t, out.String(), "image/jpeg")
}

func TestMediaUpload_WithAltText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(path, []byte("fake-jpeg-bytes"), 0o644))

	rec := &mediaServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, mediaSuccessBody))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--alt-text", "a scenic view",
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "POST", rec.gotMethod)
	assert.Equal(t, "/api/v1/media", rec.gotPath)
	assert.True(t, strings.HasPrefix(rec.gotCT, "multipart/form-data; boundary="), "got %q", rec.gotCT)
	assert.Equal(t, "photo.jpg", rec.gotFilename)
	assert.Equal(t, []byte("fake-jpeg-bytes"), rec.gotFile)
	assert.Contains(t, rec.gotMetadata, `"altText":"a scenic view"`)
}

func TestMediaUpload_WithMetadataJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	rec := &mediaServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, mediaSuccessBody))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--metadata", `{"altText":"custom","caption":"hello"}`,
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, rec.gotMetadata, `"altText":"custom"`)
	assert.Contains(t, rec.gotMetadata, `"caption":"hello"`)
}

func TestMediaUpload_AltTextAndMetadataMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when both flags set")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--alt-text", "a",
			"--metadata", `{"altText":"b"}`,
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "OR")
}

func TestMediaUpload_MetadataNotJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --metadata is invalid JSON")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--metadata", `not json`,
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "valid JSON")
}

func TestMediaUpload_MissingFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --file is missing")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut.String(), "--file")
}

func TestMediaUpload_UnreadableFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --file is unreadable")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", "/nonexistent/path/to/file.jpg",
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "read --file")
}

func TestMediaUpload_ServerValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"alt text required"}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "alt text required")
}

func TestMediaUpload_DuplicateExitsFive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"CONFLICT","message":"file with the same content hash already exists"}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	// 409 → 4xx family → ExitValidation (5) per codeForStatus + the documented
	// exit scheme. Semantically "duplicate" is not "validation" but the exit
	// code is the same family.
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "content hash")
}

func TestMediaUpload_MissingKeyExitsOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when API key missing")
	}))
	defer srv.Close()
	withNoCredentials(t)

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--file", path, "--base-url", srv.URL},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "no API key found")
}

func TestMediaUpload_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mediaSuccessBody))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--output", "json",
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	// JSON output: the {data,meta} envelope containing the media projection.
	var env struct {
		Data struct {
			Media struct {
				ID int `json:"id"`
			} `json:"media"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &env))
	assert.Equal(t, 5, env.Data.Media.ID)
}

func TestMediaUpload_DirectoryIsRejected(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subdir, 0o755))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --file is a directory")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--file", subdir, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "is a directory")
}

func TestMediaUpload_SymlinkToDirectoryIsRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-dir")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "link-to-dir")
	require.NoError(t, os.Symlink(target, link))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --file is a symlink to a directory")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "upload", "--file", link, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "is a directory")
}

func TestMediaUpload_MetadataAcceptsTypedValues(t *testing.T) {
	// --metadata values may be typed (number/bool/object), not just strings.
	// They are passed through as typed JSON; the CLI must not reject them.
	// (The server persists only altText today, but the wire payload should
	// carry the typed values through unchanged.)
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	rec := &mediaServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, mediaSuccessBody))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--metadata", `{"altText":"a view","priority":42,"featured":true}`,
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, rec.gotMetadata, `"altText":"a view"`)
	assert.Contains(t, rec.gotMetadata, `"priority":42`)
	assert.Contains(t, rec.gotMetadata, `"featured":true`)
}

func TestMediaUpload_AltTextEmptyStillSent(t *testing.T) {
	// --alt-text "" should still send a metadata part with empty altText,
	// distinguishing it from "no --alt-text" (which sends no metadata part).
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jpg")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	rec := &mediaServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, mediaSuccessBody))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"media", "upload",
			"--file", path,
			"--alt-text", "",
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, rec.gotMetadata, `"altText":""`,
		"explicit --alt-text \"\" must still send a metadata part with empty altText")
}
