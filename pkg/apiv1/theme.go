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

// ThemeGenerateTimeout bounds one theme generation run end to end. The run
// includes the repair loop (up to two model retries on validation failure),
// so it cannot share the default 30s client timeout. Five minutes covers the
// server's 180s template-override budget with headroom.
const ThemeGenerateTimeout = 5 * time.Minute

// GenerateThemeRequest is the agent theme-generate payload. It mirrors the
// {description, style, templateOverrides, edition} contract documented in
// docs/api-reference.md — the client hard-codes it (it cannot import the
// server's DTO types). Empty style means auto; edition "" asks for the
// server's default.
type GenerateThemeRequest struct {
	Description       string `json:"description"`
	Style             string `json:"style,omitempty"`
	TemplateOverrides bool   `json:"templateOverrides,omitempty"`
	Edition           string `json:"edition,omitempty"`
}

// GenerateTheme sends POST /api/v1/themegen/generate and returns the decoded
// data ({theme, validation}) and meta payloads, or an *APIError on failure.
// The call uses a dedicated client with ThemeGenerateTimeout instead of the
// shared 30s client. On a failed validation gate the *APIError carries the
// final findings in Details so the caller can report the reason.
func (c *Client) GenerateTheme(
	ctx context.Context,
	req GenerateThemeRequest,
) (json.RawMessage, json.RawMessage, error) {
	generateClient := &http.Client{
		Transport:     c.http.Transport,
		CheckRedirect: redirectPolicy,
		Timeout:       ThemeGenerateTimeout,
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	return c.doRequestWithClient(generateClient, ctx, http.MethodPost, "/api/v1/themegen/generate", nil, "application/json", bytes.NewReader(encoded))
}

// ThemesList sends GET /api/v1/themegen/themes and returns the decoded data
// (installed themes) and meta payloads, or an *APIError on failure.
func (c *Client) ThemesList(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/themegen/themes", nil, nil)
}

// ThemeDownload streams GET /api/v1/themegen/themes/{name}/download into
// output under the caller's download bounds. The server contract caps theme
// zips at 2 MiB, but the policy stays uniform with export/ssg: Timeout caps
// the whole pull and MaxBytes caps its bytes; tripping either fails loudly.
func (c *Client) ThemeDownload(
	ctx context.Context,
	name string,
	output io.Writer,
	opts DownloadOptions,
) error {
	return c.downloadIntoBody(ctx, fmt.Sprintf("/api/v1/themegen/themes/%s/download", name), output, opts)
}

// ThemeActivate sends POST /api/v1/themegen/themes/{name}/activate and
// returns the decoded data ({activated, requiresRestart}) and meta payloads,
// or an *APIError on failure. A reload failure server-side reverts the
// config write and surfaces here as 500 activation_failed.
func (c *Client) ThemeActivate(
	ctx context.Context,
	name string,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/themegen/themes/%s/activate", name), nil, nil)
}

// ThemeDeactivate sends POST /api/v1/themegen/themes/deactivate and returns
// the decoded data and meta payloads, or an *APIError on failure.
func (c *Client) ThemeDeactivate(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/themegen/themes/deactivate", nil, nil)
}

// ThemeDelete sends DELETE /api/v1/themegen/themes/{name} and returns the
// decoded data and meta payloads, or an *APIError on failure. Deleting the
// active theme reads 409 theme_active.
func (c *Client) ThemeDelete(
	ctx context.Context,
	name string,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/themegen/themes/%s", name), nil, nil)
}
