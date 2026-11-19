package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PluginGenerateTimeout bounds one plugin generation run end to end. The run
// includes the repair loop (up to two model retries on validation failure),
// so it cannot share the default 30s client timeout.
const PluginGenerateTimeout = 5 * time.Minute

// GeneratePluginRequest is the agent plugin-generate payload. It mirrors the
// {description, language, hooks, capabilities} contract documented in
// docs/api-reference.md — the client hard-codes it (it cannot import the
// server's DTO types). Empty language means go.
type GeneratePluginRequest struct {
	Description  string   `json:"description"`
	Language     string   `json:"language,omitempty"`
	Hooks        []string `json:"hooks,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// GeneratePlugin sends POST /api/v1/plugingen/generate and returns the decoded
// data ({project, validation}) and meta payloads, or an *APIError on failure.
func (c *Client) GeneratePlugin(
	ctx context.Context,
	req GeneratePluginRequest,
) (json.RawMessage, json.RawMessage, error) {
	generateClient := &http.Client{
		Transport:     c.http.Transport,
		CheckRedirect: redirectPolicy,
		Timeout:       PluginGenerateTimeout,
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	return c.doRequestWithClient(generateClient, ctx, http.MethodPost, "/api/v1/plugingen/generate", nil, "application/json", bytes.NewReader(encoded))
}

// PluginProjectsList sends GET /api/v1/plugingen/projects and returns the
// decoded data (installed projects) and meta payloads.
func (c *Client) PluginProjectsList(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/plugingen/projects", nil, nil)
}

// PluginDownload streams GET /api/v1/plugingen/projects/{name}/download into
// output under the caller's download bounds.
func (c *Client) PluginDownload(
	ctx context.Context,
	name string,
	output io.Writer,
	opts DownloadOptions,
) error {
	return c.downloadIntoBody(ctx, fmt.Sprintf("/api/v1/plugingen/projects/%s/download", name), output, opts)
}

// PluginBuild sends POST /api/v1/plugingen/projects/{name}/build and returns
// the decoded data ({installed, requiresRestart}) and meta payloads, or an
// *APIError on failure. The call uses a dedicated client with
// PluginGenerateTimeout instead of the shared 30s client: server-side TinyGo
// compiles can run past a minute.
func (c *Client) PluginBuild(
	ctx context.Context,
	name string,
) (json.RawMessage, json.RawMessage, error) {
	buildClient := &http.Client{
		Transport:     c.http.Transport,
		CheckRedirect: redirectPolicy,
		Timeout:       PluginGenerateTimeout,
	}
	return c.doRequestWithClient(buildClient, ctx, http.MethodPost, fmt.Sprintf("/api/v1/plugingen/projects/%s/build", name), nil, "", nil)
}

// PluginDelete sends DELETE /api/v1/plugingen/projects/{name} and returns the
// decoded data and meta payloads.
func (c *Client) PluginDelete(
	ctx context.Context,
	name string,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/plugingen/projects/%s", name), nil, nil)
}

// PluginsStatus sends GET /api/v1/plugins and returns the decoded data
// (loaded plugins) and meta payloads.
func (c *Client) PluginsStatus(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/plugins", nil, nil)
}
