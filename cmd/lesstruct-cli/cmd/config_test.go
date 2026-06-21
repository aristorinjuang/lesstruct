package cmd_test

import (
	"bytes"
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

// newAuthRecorder returns an httptest server that records the Authorization
// header of the request it serves (the content create).
func newAuthRecorder(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"content":{"id":1,"title":"x","slug":"x","status":"draft"}}}`))
	}))
	return srv, &gotAuth
}

// writeConfig writes a TOML config file under a temp dir and returns its path.
func writeConfig(t *testing.T, apiKey, baseURL string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := "[default]\n"
	if apiKey != "" {
		content += "api_key = \"" + apiKey + "\"\n"
	}
	if baseURL != "" {
		content += "base_url = \"" + baseURL + "\"\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// runCreate invokes `content create` against srvURL with the given extra args
// (caller supplies the body source — positional or via stdin in `in`).
func runCreate(t *testing.T, srvURL string, in io.Reader, extraArgs ...string) (int, string, string) {
	t.Helper()
	args := append([]string{"content", "create", "--base-url", srvURL, "--output", "json"}, extraArgs...)
	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(args, in, &out, &errOut)
	return code, out.String(), errOut.String()
}

// withNoCredentials neutralizes every credential source so a test can assert the
// "no API key found" error deterministically, regardless of the shell it runs
// in. A developer's shell often has LESSTRUCT_API_KEY exported (a real key);
// without this, the "missing key" tests would find it and pass the auth check.
// It also points the config lookup at a non-existent path so no config-file key
// is loaded (LESSTRUCT_CONFIG takes precedence over the XDG/HOME locations).
func withNoCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("LESSTRUCT_API_KEY", "")
	t.Setenv("LESSTRUCT_CONFIG", filepath.Join(t.TempDir(), "absent-config.toml"))
}

func TestCredentials_FlagPrecedence(t *testing.T) {
	srv, gotAuth := newAuthRecorder(t)
	defer srv.Close()

	t.Setenv("LESSTRUCT_API_KEY", "envkey")
	t.Setenv("LESSTRUCT_CONFIG", writeConfig(t, "cfgkey", ""))

	code, _, errOut := runCreate(t, srv.URL, strings.NewReader(""), "# Hi", "--api-key", "flagkey")
	assert.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "Bearer flagkey", *gotAuth)
}

func TestCredentials_EnvPrecedence(t *testing.T) {
	srv, gotAuth := newAuthRecorder(t)
	defer srv.Close()

	t.Setenv("LESSTRUCT_API_KEY", "envkey")
	t.Setenv("LESSTRUCT_CONFIG", writeConfig(t, "cfgkey", ""))

	code, _, errOut := runCreate(t, srv.URL, strings.NewReader(""), "# Hi")
	assert.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "Bearer envkey", *gotAuth)
}

func TestCredentials_ConfigFilePrecedence(t *testing.T) {
	srv, gotAuth := newAuthRecorder(t)
	defer srv.Close()

	// Neutralize the env key so the config file's key wins (env otherwise beats
	// the config file in the precedence chain).
	t.Setenv("LESSTRUCT_API_KEY", "")
	t.Setenv("LESSTRUCT_CONFIG", writeConfig(t, "cfgkey", ""))

	code, _, errOut := runCreate(t, srv.URL, strings.NewReader(""), "# Hi")
	assert.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "Bearer cfgkey", *gotAuth)
}

func TestCredentials_MissingKeyExitsOne(t *testing.T) {
	srv, _ := newAuthRecorder(t)
	defer srv.Close()

	// No flag, no env, no config.
	withNoCredentials(t)

	code, _, errOut := runCreate(t, srv.URL, strings.NewReader(""), "# Hi")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "no API key found")
}

func TestCredentials_ConfigFileBaseURL(t *testing.T) {
	// The config file's base_url is used when --base-url/env are absent.
	gotPath := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"content":{"id":1}}}`))
	}))
	defer srv.Close()

	t.Setenv("LESSTRUCT_CONFIG", writeConfig(t, "cfgkey", srv.URL))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hi", "--output", "json"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "/api/v1/content", gotPath)
}
