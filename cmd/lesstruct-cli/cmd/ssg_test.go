package cmd_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
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

func buildSiteArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		if strings.HasSuffix(name, "/") {
			require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}))
			continue
		}
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
	return buf.Bytes()
}

func newSSGServer(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/ssg", r.URL.Path)
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSSG_TarballMode(t *testing.T) {
	archive := buildSiteArchive(t, map[string]string{"index.html": "<html></html>"})
	srv := newSSGServer(t, archive)

	outDir := t.TempDir()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"ssg", "--output-dir", outDir, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), "Site generation complete")

	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, strings.HasPrefix(entries[0].Name(), "lesstruct-site-"), "got %q", entries[0].Name())
}

func TestSSG_ExtractDir(t *testing.T) {
	archive := buildSiteArchive(t, map[string]string{
		"index.html":             "<html>home</html>",
		"hello-world/":           "",
		"hello-world/index.html": "<html>post</html>",
		"static/style.css":       "body {}",
	})
	srv := newSSGServer(t, archive)

	dest := filepath.Join(t.TempDir(), "site")

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"ssg", "--extract-dir", dest, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())

	home, err := os.ReadFile(filepath.Join(dest, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<html>home</html>", string(home))

	post, err := os.ReadFile(filepath.Join(dest, "hello-world", "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<html>post</html>", string(post))

	css, err := os.ReadFile(filepath.Join(dest, "static", "style.css"))
	require.NoError(t, err)
	assert.Equal(t, "body {}", string(css))
}

func TestSSG_ExtractDir_JSON(t *testing.T) {
	archive := buildSiteArchive(t, map[string]string{"index.html": "<html></html>"})
	srv := newSSGServer(t, archive)

	dest := filepath.Join(t.TempDir(), "site")

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"ssg", "--extract-dir", dest, "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())

	var env struct {
		Message string `json:"message"`
		Path    string `json:"path"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &env))
	assert.Equal(t, "site extracted", env.Message)
	assert.Equal(t, dest, env.Path)
}

func TestSSG_ExtractDir_RejectsTraversalEntries(t *testing.T) {
	archive := buildSiteArchive(t, map[string]string{
		"index.html":     "<html></html>",
		"../../evil.txt": "pwned",
	})
	srv := newSSGServer(t, archive)

	root := t.TempDir()
	dest := filepath.Join(root, "site")

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"ssg", "--extract-dir", dest, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "escapes the destination directory")

	_, err := os.Stat(filepath.Join(root, "evil.txt"))
	assert.True(t, os.IsNotExist(err), "archive must not write outside the destination directory")
}

func TestSSG_ExtractDir_BadArchive(t *testing.T) {
	srv := newSSGServer(t, []byte("this is not gzip"))
	dest := filepath.Join(t.TempDir(), "site")

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"ssg", "--extract-dir", dest, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "gzip")
}

func TestSSG_OutputDirAndExtractDirAreMutuallyExclusive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called when both --output-dir and --extract-dir are set")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"ssg",
			"--output-dir", t.TempDir(),
			"--extract-dir", filepath.Join(t.TempDir(), "site"),
			"--base-url", srv.URL,
			"--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut.String(), "mutually exclusive")
}
