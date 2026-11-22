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

// ModuleGenerateTimeout bounds one module generation run end to end. The run
// includes the repair loop (up to two model retries on validation failure),
// so it cannot share the default 30s client timeout.
const ModuleGenerateTimeout = 5 * time.Minute

// GenerateModuleRequest is the agent module-generate payload. It mirrors the
// {description, surfaces} contract documented in docs/api-reference.md — the
// client hard-codes it (it cannot import the server's DTO types). An empty
// surfaces list lets the model infer them from the description.
type GenerateModuleRequest struct {
	Description string   `json:"description"`
	Surfaces    []string `json:"surfaces,omitempty"`
}

// ModuleApplyRequest is the agent module-apply payload: replace with
// confirm restages over an applied tree the registry owns. Both default
// to false for plain first-time applies.
type ModuleApplyRequest struct {
	Replace bool `json:"replace,omitempty"`
	Confirm bool `json:"confirm,omitempty"`
}

// GenerateModule sends POST /api/v1/modgen/generate and returns the decoded
// data ({project, validation}) and meta payloads, or an *APIError on failure.
func (c *Client) GenerateModule(
	ctx context.Context,
	req GenerateModuleRequest,
) (json.RawMessage, json.RawMessage, error) {
	generateClient := &http.Client{
		Transport:     c.http.Transport,
		CheckRedirect: redirectPolicy,
		Timeout:       ModuleGenerateTimeout,
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	return c.doRequestWithClient(generateClient, ctx, http.MethodPost, "/api/v1/modgen/generate", nil, "application/json", bytes.NewReader(encoded))
}

// ModuleProjectsList sends GET /api/v1/modgen/projects and returns the
// decoded data (installed projects) and meta payloads.
func (c *Client) ModuleProjectsList(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/modgen/projects", nil, nil)
}

// ModuleDownload streams GET /api/v1/modgen/projects/{name}/download into
// output under the caller's download bounds.
func (c *Client) ModuleDownload(
	ctx context.Context,
	name string,
	output io.Writer,
	opts DownloadOptions,
) error {
	return c.downloadIntoBody(ctx, fmt.Sprintf("/api/v1/modgen/projects/%s/download", name), output, opts)
}

// ModuleDelete sends DELETE /api/v1/modgen/projects/{name} and returns the
// decoded data and meta payloads.
func (c *Client) ModuleDelete(
	ctx context.Context,
	name string,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/modgen/projects/%s", name), nil, nil)
}

// ModuleApplyTimeout bounds one module apply end to end: module tests in
// both workspaces plus the custom-binary build share it, so it cannot use
// the default 30s client timeout.
const ModuleApplyTimeout = 15 * time.Minute

// ModuleApply sends POST /api/v1/modgen/projects/{name}/apply and returns
// the decoded data ({project, apply}) and meta payloads, or an *APIError on
// failure.
func (c *Client) ModuleApply(
	ctx context.Context,
	name string,
	req ModuleApplyRequest,
) (json.RawMessage, json.RawMessage, error) {
	applyClient := &http.Client{
		Transport:     c.http.Transport,
		CheckRedirect: redirectPolicy,
		Timeout:       ModuleApplyTimeout,
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	return c.doRequestWithClient(applyClient, ctx, http.MethodPost, fmt.Sprintf("/api/v1/modgen/projects/%s/apply", name), nil, "application/json", bytes.NewReader(encoded))
}

// ModuleUnapply sends POST /api/v1/modgen/projects/{name}/unapply and returns
// the decoded data ({unapply}) and meta payloads, or an *APIError on failure.
func (c *Client) ModuleUnapply(
	ctx context.Context,
	name string,
	confirm bool,
) (json.RawMessage, json.RawMessage, error) {
	applyClient := &http.Client{
		Transport:     c.http.Transport,
		CheckRedirect: redirectPolicy,
		Timeout:       ModuleApplyTimeout,
	}
	encoded, err := json.Marshal(map[string]bool{"confirm": confirm})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	return c.doRequestWithClient(applyClient, ctx, http.MethodPost, fmt.Sprintf("/api/v1/modgen/projects/%s/unapply", name), nil, "application/json", bytes.NewReader(encoded))
}
