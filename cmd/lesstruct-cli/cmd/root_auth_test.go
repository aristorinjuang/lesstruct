package cmd_test

import (
	"bytes"
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

// TestAPIKeyStdin covers --api-key-stdin: the key comes from stdin's first
// line (nothing secret appears in argv), the remaining stdin still feeds
// commands that read bodies, and combining it with --api-key is rejected.
func TestAPIKeyStdin(t *testing.T) {
	newKeyServer := func(t *testing.T, wantAuth string) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, wantAuth, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}}}`))
		}))
	}

	t.Run("key from stdin authenticates and body still flows", func(t *testing.T) {
		srv := newKeyServer(t, "Bearer lesstruct_stdin_key_123")
		defer srv.Close()

		var out, errOut bytes.Buffer
		// First stdin line is the key; the rest is the markdown body.
		in := strings.NewReader("lesstruct_stdin_key_123\n# Hello\n")
		code := cmd.ExecuteArgs(
			[]string{"content", "create", "--api-key-stdin", "--base-url", srv.URL},
			in, &out, &errOut,
		)
		require.Equal(t, 0, code, "stderr: %s", errOut)
		assert.Contains(t, out.String(), "Created content #7")
	})

	t.Run("combining with --api-key is a usage error", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"content", "list", "--api-key", "k", "--api-key-stdin", "--base-url", "http://127.0.0.1:1"},
			strings.NewReader("k\n"),
			&out, &errOut,
		)
		assert.Equal(t, 2, code)
		assert.Contains(t, errOut.String(), "cannot be combined")
	})

	t.Run("empty stdin is a usage error", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"content", "list", "--api-key-stdin", "--base-url", "http://127.0.0.1:1"},
			strings.NewReader(""),
			&out, &errOut,
		)
		assert.Equal(t, 2, code)
		assert.Contains(t, errOut.String(), "no key on stdin")
	})
}

// TestInsecureConfigWarning covers the group/world-readable config warning:
// it fires only when the key actually comes from that file.
func TestInsecureConfigWarning(t *testing.T) {
	writeConfig := func(t *testing.T, content string, mode os.FileMode) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		require.NoError(t, os.Chmod(path, mode))
		return path
	}

	t.Run("world-readable key file warns but proceeds", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer lesstruct_filekey", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"nextCursor":"","hasMore":false}}}`))
		}))
		defer srv.Close()

		path := writeConfig(t, "[default]\napi_key = \"lesstruct_filekey\"\n", 0o644)
		t.Setenv("LESSTRUCT_CONFIG", path)
		t.Setenv("LESSTRUCT_API_KEY", "")
		t.Setenv("LESSTRUCT_BASE_URL", srv.URL)

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs([]string{"content", "list"}, strings.NewReader(""), &out, &errOut)

		require.Equal(t, 0, code, "stderr: %s", errOut)
		assert.Contains(t, errOut.String(), "readable by other users")
		assert.Contains(t, errOut.String(), "chmod 600")
	})

	t.Run("locked-down key file stays silent", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"nextCursor":"","hasMore":false}}}`))
		}))
		defer srv.Close()

		path := writeConfig(t, "[default]\napi_key = \"lesstruct_filekey\"\n", 0o600)
		t.Setenv("LESSTRUCT_CONFIG", path)
		t.Setenv("LESSTRUCT_API_KEY", "")
		t.Setenv("LESSTRUCT_BASE_URL", srv.URL)

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs([]string{"content", "list"}, strings.NewReader(""), &out, &errOut)

		require.Equal(t, 0, code, "stderr: %s", errOut)
		assert.NotContains(t, errOut.String(), "readable by other users")
	})

	t.Run("explicit --api-key still warns about the readable file", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer lesstruct_explicit", r.Header.Get("Authorization"), "the flag key still wins the run")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"nextCursor":"","hasMore":false}}}`))
		}))
		defer srv.Close()

		path := writeConfig(t, "[default]\napi_key = \"lesstruct_filekey\"\n", 0o644)
		t.Setenv("LESSTRUCT_CONFIG", path)
		t.Setenv("LESSTRUCT_API_KEY", "")
		t.Setenv("LESSTRUCT_BASE_URL", srv.URL)

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"content", "list", "--api-key", "lesstruct_explicit"},
			strings.NewReader(""), &out, &errOut,
		)

		require.Equal(t, 0, code, "stderr: %s", errOut)
		assert.Contains(t, errOut.String(), "readable by other users", "the file leaks regardless of which source wins this run")
	})

	t.Run("LESSTRUCT_API_KEY set still warns about the readable file", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer lesstruct_envkey", r.Header.Get("Authorization"), "the env key still wins the run")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"nextCursor":"","hasMore":false}}}`))
		}))
		defer srv.Close()

		path := writeConfig(t, "[default]\napi_key = \"lesstruct_filekey\"\n", 0o644)
		t.Setenv("LESSTRUCT_CONFIG", path)
		t.Setenv("LESSTRUCT_API_KEY", "lesstruct_envkey")
		t.Setenv("LESSTRUCT_BASE_URL", srv.URL)

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"content", "list"},
			strings.NewReader(""), &out, &errOut,
		)

		require.Equal(t, 0, code, "stderr: %s", errOut)
		assert.Contains(t, errOut.String(), "readable by other users", "a readable key file warns even when the env key wins")
	})
}
