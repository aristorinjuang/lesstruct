package cmd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExport_MediumFixtures(t *testing.T) {
	// Shared fixtures for the bounded-download tests below.
	newFloodServer := func(t *testing.T, size int) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v1/export", r.URL.Path)
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(bytes.Repeat([]byte("x"), size))
		}))
	}

	t.Run("cap trip fails loudly and leaves no archive or part file", func(t *testing.T) {
		srv := newFloodServer(t, 4096)
		defer srv.Close()
		dir := t.TempDir()

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"export", "--output-dir", dir, "--max-download-size", "100", "--base-url", srv.URL, "--api-key", "k"},
			strings.NewReader(""), &out, &errOut,
		)

		require.Equal(t, 1, code, "tripped cap is a generic failure, not silent success")
		assert.Contains(t, errOut.String(), "100")
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries, "no archive and no .part file may remain")
	})

	t.Run("uncapped small download writes the archive with no part file left", func(t *testing.T) {
		srv := newFloodServer(t, 64)
		defer srv.Close()
		dir := t.TempDir()

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"export", "--output-dir", dir, "--max-download-size", "0", "--base-url", srv.URL, "--api-key", "k"},
			strings.NewReader(""), &out, &errOut,
		)

		require.Equal(t, 0, code, "stderr: %s", errOut)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1, "exactly one finished archive")
		assert.True(t, strings.HasSuffix(entries[0].Name(), ".tar.gz"))
		assert.False(t, strings.HasSuffix(entries[0].Name(), ".part"))
	})

	t.Run("invalid timeout is a usage error before any request", func(t *testing.T) {
		srv := newFloodServer(t, 64)
		defer srv.Close()
		dir := t.TempDir()

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"export", "--output-dir", dir, "--timeout", "soon", "--base-url", srv.URL, "--api-key", "k"},
			strings.NewReader(""), &out, &errOut,
		)

		assert.Equal(t, 2, code)
		assert.Contains(t, errOut.String(), "invalid --timeout")
	})

	t.Run("invalid size cap is a usage error before any request", func(t *testing.T) {
		srv := newFloodServer(t, 64)
		defer srv.Close()
		dir := t.TempDir()

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{"export", "--output-dir", dir, "--max-download-size", "huge", "--base-url", srv.URL, "--api-key", "k"},
			strings.NewReader(""), &out, &errOut,
		)

		assert.Equal(t, 2, code)
		assert.Contains(t, errOut.String(), "invalid --max-download-size")
	})

	t.Run("suffixed caps are accepted", func(t *testing.T) {
		for _, tc := range []struct {
			flag string
			want int
		}{
			{"1KB", 1},
			{"2M", 1},
			{"3GiB", 1},
		} {
			srv := newFloodServer(t, 64)
			dir := t.TempDir()

			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(
				[]string{"export", "--output-dir", dir, "--max-download-size", tc.flag, "--base-url", srv.URL, "--api-key", "k"},
				strings.NewReader(""), &out, &errOut,
			)
			require.Equal(t, 0, code, "flag %s must parse (stderr: %s)", tc.flag, errOut)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, tc.want)
			srv.Close()
		}
	})
}
