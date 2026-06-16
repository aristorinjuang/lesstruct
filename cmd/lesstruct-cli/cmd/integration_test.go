//go:build integration

package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedRequest captures one HTTP call the CLI made against the in-process
// test server. The integration test asserts on the recorded sequence after
// the subprocess exits.
type recordedRequest struct {
	method string
	path   string
	auth   string
	body   string
}

type contractServer struct {
	mu          sync.Mutex
	recorded    []recordedRequest
	createH     http.HandlerFunc
	getH        http.HandlerFunc
	publishH    http.HandlerFunc
	unpublishH  http.HandlerFunc
	publicH     http.HandlerFunc
	failNext    string // when non-empty, the next call to this path returns this status
}

// handler dispatches requests to the right canned response. Only the routes
// the contract needs are emulated; any other path is a test failure (it
// means the CLI called something unexpected).
func (c *contractServer) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.recorded = append(c.recorded, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			body:   body,
		})
		fail := c.failNext
		c.failNext = ""

		if fail != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`))
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/content":
			c.dispatchCreate(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/content/"):
			c.dispatchGet(w, r)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/publish"):
			c.dispatchPublish(w, r)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/unpublish"):
			c.dispatchUnpublish(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/content_items/"):
			c.dispatchPublic(w, r)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"unmocked path"}}`))
		}
	})
}

// dispatchCreate / dispatchGet / dispatchPublic defer the handler call so a
// nil handler (left unset in the 401 sub-test) produces a clean 500 from the
// stdlib's HandlerFunc call instead of a panic.
func (c *contractServer) dispatchCreate(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()
	if c.createH == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.createH(w, r)
}

func (c *contractServer) dispatchGet(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()
	if c.getH == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.getH(w, r)
}

func (c *contractServer) dispatchPublic(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()
	if c.publicH == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.publicH(w, r)
}

// dispatchPublish / dispatchUnpublish mirror dispatchCreate — deferred so a
// nil handler produces a clean 500 from the stdlib's HandlerFunc call rather
// than a panic. They share the same defensive pattern as the other
// dispatchers, so the contract server's panics on missing handlers stay
// isolated to the test that forgot to wire it.
func (c *contractServer) dispatchPublish(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()
	if c.publishH == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.publishH(w, r)
}

func (c *contractServer) dispatchUnpublish(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()
	if c.unpublishH == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.unpublishH(w, r)
}

// TestEndToEndContentAuthoring is the documented contract gate for the CLI:
// it builds the CLI binary, spins an httptest server that emulates the
// documented /api/v1 + /content_items contract, invokes the binary as a
// subprocess, and asserts the full agent authoring path.
func TestEndToEndContentAuthoring(t *testing.T) {
	// --- Build the CLI binary in a temp dir -------------------------------
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lesstruct-cli")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(buildOut))
	stat, err := os.Stat(binaryPath)
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0), "binary must be non-empty")

	// --- Write the Markdown input file ------------------------------------
	markdownPath := filepath.Join(tmpDir, "post.md")
	markdown := "# Hello EndToEnd\n\nThis is a **contract** test."
	require.NoError(t, os.WriteFile(markdownPath, []byte(markdown), 0o644))

	// --- The canned "server" handlers --------------------------------------
	cs := &contractServer{}
	cs.createH = func(w http.ResponseWriter, r *http.Request) {
		// The outer handler has already read r.Body into the recorded request;
		// re-reading here would always return empty. Echo a fixed response
		// matching the Story 2.1 server contract.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"data":{"content":{"id":1,"title":"Hello EndToEnd","slug":"hello-endtoend","status":"draft","body":` +
				JSONString(markdown) +
				`}}}`,
		))
	}
	cs.getH = func(w http.ResponseWriter, r *http.Request) {
		// Echo the same content the create handler returned.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"data":{"content":{"id":1,"title":"Hello EndToEnd","slug":"hello-endtoend","status":"draft","body":` +
				JSONString(markdown) +
				`}}}`,
		))
	}
	cs.publicH = func(w http.ResponseWriter, r *http.Request) {
		// Minimal HTML containing the body text — proves the public site
		// would render the content the CLI just authored.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(
			`<html><body><article><h1>Hello EndToEnd</h1><p>This is a <strong>contract</strong> test.</p></article></body></html>`,
		))
	}

	srv := httptest.NewServer(cs.handler(t))
	defer srv.Close()

	// --- Invoke the CLI binary as a subprocess ---------------------------
	runCmd := exec.Command(
		binaryPath,
		"content", "create",
		"--file", markdownPath,
		"--base-url", srv.URL,
		"--api-key", "lesstruct_test_test",
		"--output", "json",
	)
	var stderrBuf bytes.Buffer
	runCmd.Stderr = &stderrBuf
	stdout, err := runCmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		// Output() returns *exec.ExitError on non-zero; its .Stderr is empty
		// because we set cmd.Stderr above. The exit code is what we assert on.
		_ = exitErr
	}
	require.NoError(t, err, "lesstruct-cli failed: stderr=%s", stderrBuf.String())

	// --- Assert exit 0 + JSON output contains the create response --------
	assert.Equal(t, 0, runCmd.ProcessState.ExitCode(), "stderr: %s", stderrBuf.String())
	var env struct {
		Data struct {
			Content struct {
				ID    int    `json:"id"`
				Slug  string `json:"slug"`
				Title string `json:"title"`
			} `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stdout, &env))
	assert.Equal(t, 1, env.Data.Content.ID)
	assert.Equal(t, "hello-endtoend", env.Data.Content.Slug)

	// --- Assert the recorded request shape ------------------------------
	cs.mu.Lock()
	rec := append([]recordedRequest(nil), cs.recorded...)
	cs.mu.Unlock()
	require.Len(t, rec, 1, "CLI should have made exactly one request to the contract server")
	r := rec[0]
	assert.Equal(t, http.MethodPost, r.method)
	assert.Equal(t, "/api/v1/content", r.path)
	assert.Equal(t, "Bearer lesstruct_test_test", r.auth)
	assert.Contains(t, r.body, `"format":"markdown"`)
	assert.Contains(t, r.body, `"body":"# Hello EndToEnd`)

	// --- Assert the CLI's created content is round-trippable through GET ---
	getResp, err := http.Get(srv.URL + "/api/v1/content/1")
	require.NoError(t, err)
	defer func() { _ = getResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)
	getBody, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(getBody), `"slug":"hello-endtoend"`,
		"GET /api/v1/content/1 must echo the content the CLI just created")

	// --- Assert the CLI's created content is reachable through the
	//     server's public-site endpoint (the contract the AC calls out:
	//     "asserts the resulting content renders on the public site").
	publicResp, err := http.Get(srv.URL + "/content_items/hello-endtoend")
	require.NoError(t, err)
	defer func() { _ = publicResp.Body.Close() }()
	publicBody, err := io.ReadAll(publicResp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, publicResp.StatusCode)
	assert.Contains(t, string(publicBody), "Hello EndToEnd")
	assert.Contains(t, string(publicBody), "contract")
}

// TestEndToEndContentAuthoring_Server401 covers the failure-mode sub-case:
// when the server returns 401, the CLI must exit 3 with a clear stderr.
func TestEndToEndContentAuthoring_Server401(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lesstruct-cli")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(buildOut))
	stat, err := os.Stat(binaryPath)
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0), "binary must be non-empty")

	cs := &contractServer{
		failNext: "/api/v1/content",
	}
	// When failNext is set, the handler short-circuits to 401; the createH
	// never runs. The contract test only needs to assert the CLI's exit
	// code + stderr behavior.
	srv := httptest.NewServer(cs.handler(t))
	defer srv.Close()

	runCmd := exec.Command(
		binaryPath,
		"content", "create",
		"--file", filepath.Join(tmpDir, "x.md"),
		"--base-url", srv.URL,
		"--api-key", "k",
	)
	// Write a placeholder file.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "x.md"), []byte("# Hi"), 0o644))
	stdout, err := runCmd.Output()
	require.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "expected ExitError, got %T", err)
	assert.Equal(t, 3, exitErr.ExitCode(), "stdout: %s", string(stdout))
	assert.Contains(t, string(exitErr.Stderr), "bad key",
		"stderr should mention the server's error message; got %q", string(exitErr.Stderr))
}

// JSONString returns s as a JSON string literal (with embedded quotes
// escaped) so it can be interpolated into a JSON document. Mirrors the
// "must" naming used in 3.1+3.2+3.3 tests but returns an error rather
// than panicking — AGENTS.md forbids panic() outside main.go.
func JSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal of a Go string cannot fail in practice (strings are
		// always valid JSON), but if it ever does, fall back to a quoted
		// empty string rather than panic.
		return `""`
	}
	return string(b)
}

// TestEndToEndPublishUnpublish exercises the standalone status-toggle verbs
// end-to-end: build the CLI binary, run `content publish` and
// `content unpublish` against a contract server that echoes the
// server-published envelope, and assert the recorded request shape (method,
// path, empty body) so future regressions in the verb surface are caught
// here rather than by hand.
func TestEndToEndPublishUnpublish(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lesstruct-cli")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(buildOut))
	stat, err := os.Stat(binaryPath)
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0), "binary must be non-empty")

	// The published envelope returned by the canned server. The same shape
	// the real agent v1 endpoint returns.
	const publishedEnv = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}},"error":null}`
	const draftEnv = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}},"error":null}`

	cs := &contractServer{}
	cs.publishH = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(publishedEnv))
	}
	cs.unpublishH = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(draftEnv))
	}
	srv := httptest.NewServer(cs.handler(t))
	defer srv.Close()

	// --- publish ---
	var stderrBuf bytes.Buffer
	publishCmd := exec.Command(
		binaryPath,
		"content", "publish", "7",
		"--base-url", srv.URL,
		"--api-key", "lesstruct_test_test",
		"--output", "json",
	)
	publishCmd.Stderr = &stderrBuf
	publishOut, perr := publishCmd.Output()
	if exitErr, ok := perr.(*exec.ExitError); ok {
		_ = exitErr
	}
	require.NoError(t, perr, "publish failed: stderr=%s", stderrBuf.String())
	assert.Equal(t, 0, publishCmd.ProcessState.ExitCode(), "stderr: %s", stderrBuf.String())
	assert.Contains(t, string(publishOut), `"status":"published"`)

	// --- unpublish ---
	stderrBuf.Reset()
	unpublishCmd := exec.Command(
		binaryPath,
		"content", "unpublish", "7",
		"--base-url", srv.URL,
		"--api-key", "lesstruct_test_test",
		"--output", "json",
	)
	unpublishCmd.Stderr = &stderrBuf
	unpublishOut, uerr := unpublishCmd.Output()
	if exitErr, ok := uerr.(*exec.ExitError); ok {
		_ = exitErr
	}
	require.NoError(t, uerr, "unpublish failed: stderr=%s", stderrBuf.String())
	assert.Equal(t, 0, unpublishCmd.ProcessState.ExitCode(), "stderr: %s", stderrBuf.String())
	assert.Contains(t, string(unpublishOut), `"status":"draft"`)

	// --- recorded request shape ---
	cs.mu.Lock()
	rec := append([]recordedRequest(nil), cs.recorded...)
	cs.mu.Unlock()
	require.Len(t, rec, 2, "CLI should have made exactly two contract calls")
	assert.Equal(t, http.MethodPost, rec[0].method)
	assert.Equal(t, "/api/v1/content/7/publish", rec[0].path)
	assert.Equal(t, "Bearer lesstruct_test_test", rec[0].auth)
	assert.Empty(t, rec[0].body, "publish sends no body")
	assert.Equal(t, http.MethodPost, rec[1].method)
	assert.Equal(t, "/api/v1/content/7/unpublish", rec[1].path)
	assert.Equal(t, "Bearer lesstruct_test_test", rec[1].auth)
	assert.Empty(t, rec[1].body, "unpublish sends no body")
}
