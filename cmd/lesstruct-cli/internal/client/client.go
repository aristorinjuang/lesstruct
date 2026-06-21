package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// envelope mirrors the server's {data, error, meta} JSON envelope. data and meta
// are kept as RawMessage so callers decode the payload themselves; the client
// never imports the server's response types (the HTTP API is the hard boundary).
type envelope struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error *errorInfo      `json:"error,omitempty"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

// errorInfo is the structured error object the server returns under envelope.error.
type errorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// codeForStatus maps a bare HTTP status to a best-effort UPPER_SNAKE code when
// the server returned no error object.
func codeForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case status == http.StatusNotFound:
		return "NOT_FOUND"
	case status == http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case status >= 500:
		return "INTERNAL_ERROR"
	case status >= 400:
		return "VALIDATION_ERROR"
	default:
		return ""
	}
}

// stripAuthOnCrossHostRedirect is the client's CheckRedirect policy: it drops
// the Authorization header when a redirect crosses to a different host, so the
// API key is never forwarded to another origin (e.g. an http→https upgrade or a
// vanity-domain redirect issued by a proxy).
func stripAuthOnCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 0 && req.URL.Hostname() != via[0].URL.Hostname() {
		req.Header.Del("Authorization")
	}
	return nil
}

// APIError is returned by Client methods when the request fails or the server
// returns an error envelope. StatusCode carries the HTTP status (0 for a
// transport-level failure); Code/Message come from the server's error object.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

// CreateContentRequest is the agent content-create payload. It mirrors the
// {data, error, meta} contract documented in docs/api-reference.md — the client
// hard-codes it (it cannot import the server's DTO types).
//
// All metadata fields (PostType, Tags, Language) use `omitempty` so an unset
// value is identical on the wire to a default-zero request. Tags must already
// be normalized by the caller (the CLI's normalizeTags helper trims, drops
// empties, and dedupes); the server re-validates via contentdomain.ValidateTags.
// Language is forwarded as-given — the server is the single source of truth
// for the configured-languages list (it returns ErrInvalidLanguage for an
// unknown code, mapped to 400 VALIDATION_ERROR by the CLI's exit code map).
type CreateContentRequest struct {
	Title              string         `json:"title"`
	Body               string         `json:"body"`
	Format             string         `json:"format,omitempty"`
	PostType           string         `json:"postType,omitempty"`
	Tags               []string       `json:"tags,omitempty"`
	Language           string         `json:"language,omitempty"`
	IsPublished        bool           `json:"isPublished,omitempty"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
}

// UpdateContentRequest is the agent content-update payload. It carries the same
// fields as CreateContentRequest because the server's PUT /api/v1/content/{id}
// accepts the same ContentRequest shape. The server preserves the
// server-managed fields (SEO metadata — metaDescription / ogTitle / ogDescription,
// allowComments, translationGroupId) from the existing item; the client does
// not surface a flag for translationGroupId on update (the server ignores it).
// CustomFields may be set: when non-nil it replaces the item's custom fields
// (preserving plugin-managed system fields); when nil the existing custom
// fields are preserved. The client cannot import the server's DTO types so the
// subset is re-declared here.
type UpdateContentRequest struct {
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	Format       string         `json:"format,omitempty"`
	PostType     string         `json:"postType,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Language     string         `json:"language,omitempty"`
	IsPublished  bool           `json:"isPublished,omitempty"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// Client is a typed HTTP client over the /api/v1 surface. It depends only on
// the JSON contract (stdlib) — it imports no server internals — which is what
// lets a future MCP server reuse it. Construct it with New.
type Client struct {
	baseURL *url.URL
	apiKey  string
	http    *http.Client
}

// do sends a JSON request and decodes the {data, error, meta} envelope. On any
// failure (transport error, non-2xx, or an error envelope) it returns an
// *APIError so the caller can map it to an exit code; on success it returns the
// raw data and meta payloads for the caller to decode.
//
// An empty query is nil; otherwise it is appended to the path. A 204 No Content,
// a 304 Not Modified, or any 2xx with an empty body returns (nil, nil, nil) so
// the caller can treat it as a success-without-data (used by DeleteContent).
//
// For non-JSON bodies (e.g. multipart/form-data media uploads), use doRaw.
func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body any,
) (json.RawMessage, json.RawMessage, error) {
	var reader io.Reader
	contentType := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	return c.doRequest(ctx, method, path, query, contentType, reader)
}

// doRaw is the non-JSON sibling of do. It sends an arbitrary body with an
// explicit Content-Type (e.g. "multipart/form-data; boundary=…") and decodes
// the {data, error, meta} envelope the same way. Used by UploadMedia for the
// multipart upload. The 204/304/empty-2xx + 200+envelope defenses from
// doRequest apply unchanged.
func (c *Client) doRaw(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	contentType string,
	body io.Reader,
) (json.RawMessage, json.RawMessage, error) {
	return c.doRequest(ctx, method, path, query, contentType, body)
}

// doRequest is the shared core of do (JSON) and doRaw (arbitrary body). It
// builds the URL, builds the request, sends it, and decodes the {data, error,
// meta} envelope — including the 204/304/empty-2xx + 200+envelope defenses
// (which are applied uniformly to every code path).
func (c *Client) doRequest(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	contentType string,
	body io.Reader,
) (json.RawMessage, json.RawMessage, error) {
	reqURL := c.baseURL.JoinPath(path)
	if len(query) > 0 {
		reqURL.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, nil, &APIError{StatusCode: 0, Code: "REQUEST_ERROR", Message: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, &APIError{StatusCode: 0, Code: "NETWORK_ERROR", Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("read response: %s", err)}
	}

	// 204 No Content, 304 Not Modified, or any 2xx with an empty body — success
	// with no envelope. The server's DELETE /api/v1/content/{id} returns 204;
	// 304/empty-2xx are the conventional "no body" responses. Anything 4xx/5xx
	// with an empty body is still an error (the empty body just means the
	// server didn't include an error envelope — the 4xx/5xx is the signal).
	//
	// Belt-and-suspenders: RFC 7230/7232 say 204/304 must NOT have a body. A
	// misbehaving server returning one with an error envelope is a server
	// contract violation; surface it instead of silently reporting success.
	if resp.StatusCode == http.StatusNoContent ||
		resp.StatusCode == http.StatusNotModified ||
		(resp.StatusCode < 300 && len(raw) == 0) {
		if len(raw) > 0 {
			var probe envelope
			if err := json.Unmarshal(raw, &probe); err == nil && probe.Error != nil {
				return nil, nil, &APIError{
					StatusCode: resp.StatusCode,
					Code:       probe.Error.Code,
					Message:    probe.Error.Message,
				}
			}
		}
		return nil, nil, nil
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected non-JSON response (status %d)", resp.StatusCode),
		}
	}

	if env.Error != nil {
		return nil, nil, &APIError{
			StatusCode: resp.StatusCode,
			Code:       env.Error.Code,
			Message:    env.Error.Message,
		}
	}
	if resp.StatusCode >= 400 {
		return nil, nil, &APIError{
			StatusCode: resp.StatusCode,
			Code:       codeForStatus(resp.StatusCode),
			Message:    fmt.Sprintf("request failed (status %d)", resp.StatusCode),
		}
	}

	return env.Data, env.Meta, nil
}

// CreateContent POSTs a content-create request to /api/v1/content and returns
// the decoded data and meta payloads, or an *APIError on failure.
func (c *Client) CreateContent(
	ctx context.Context,
	req CreateContentRequest,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/content", nil, req)
}

// GetContent sends GET /api/v1/content/{id} and returns the decoded data and
// meta payloads, or an *APIError on failure.
func (c *Client) GetContent(
	ctx context.Context,
	id int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/content/%d", id), nil, nil)
}

// ListContentFilters is the cmd-layer projection of contentdomain.ContentFilters
// for the agent List endpoint. The CLI re-declares the shape (it cannot import
// server-internal types) so the wire contract is the source of truth, and only
// the fields used by the agent v1 surface are exposed here. Tags is intentionally
// a slice — the wire format uses repeated ?tag= keys (one per tag, AND-of-tags
// on the server). An empty / zero struct produces a no-filter call (server returns
// the unfiltered list).
type ListContentFilters struct {
	Tags     []string
	Language string
	Status   string
	PostType string
	Author   string
	Search   string
}

// ListContent sends GET /api/v1/content?limit=&cursor=&tag=&language=&status=&post_type=&author=&search=
// and returns the decoded data (a bare array of content projections) and meta
// (with pagination), or an *APIError on failure. A limit of 0 (or negative) is
// passed as "no limit" (the server uses its default); an empty cursor is passed
// as "no cursor" (the first page). Each non-empty filter field emits its query
// key; an empty field is omitted entirely so the wire stays minimal. Tags is
// expanded into repeated ?tag= keys (one per tag) so the server can AND-of-tags.
func (c *Client) ListContent(
	ctx context.Context,
	limit int,
	cursor string,
	filters ListContentFilters,
) (json.RawMessage, json.RawMessage, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	// url.Values supports repeated keys via the slice form; Encode produces
	// `tag=foo&tag=bar` for a 2-element slice — the agent handler reads the
	// whole slice via r.URL.Query()["tag"].
	if len(filters.Tags) > 0 {
		query["tag"] = filters.Tags
	}
	if filters.Language != "" {
		query.Set("language", filters.Language)
	}
	if filters.Status != "" {
		query.Set("status", filters.Status)
	}
	if filters.PostType != "" {
		query.Set("post_type", filters.PostType)
	}
	if filters.Author != "" {
		query.Set("author", filters.Author)
	}
	if filters.Search != "" {
		query.Set("search", filters.Search)
	}
	return c.do(ctx, http.MethodGet, "/api/v1/content", query, nil)
}

// DeleteContent sends DELETE /api/v1/content/{id}. A 204 No Content success
// returns (nil, nil, nil); a 404 or other 4xx/5xx returns an *APIError.
func (c *Client) DeleteContent(
	ctx context.Context,
	id int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/content/%d", id), nil, nil)
}

// PublishContent sends POST /api/v1/content/{id}/publish with an empty body
// and returns the decoded data and meta payloads, or an *APIError on failure.
// The server endpoint is idempotent: publishing an already-published post
// returns 200 with the current projection. Ownership is enforced server-side;
// a non-admin non-owner call returns 404 (not 403, no disclosure).
func (c *Client) PublishContent(
	ctx context.Context,
	id int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/content/%d/publish", id), nil, nil)
}

// UnpublishContent sends POST /api/v1/content/{id}/unpublish with an empty
// body and returns the decoded data and meta payloads, or an *APIError on
// failure. The server endpoint is idempotent: unpublishing an already-draft
// post returns 200 with the current projection. Same ownership contract as
// PublishContent.
func (c *Client) UnpublishContent(
	ctx context.Context,
	id int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/content/%d/unpublish", id), nil, nil)
}

// UpdateContent sends PUT /api/v1/content/{id} with the given request body and
// returns the decoded data and meta payloads, or an *APIError on failure. The
// request body is the same shape as CreateContent. The server's update handler
// preserves the server-managed fields (SEO metadata — metaDescription / ogTitle
// / ogDescription, allowComments, translationGroupId) from the existing item,
// so a partial update that omits them is safe; postType / tags / language are
// now settable and forwarded as-given (the server validates them).
func (c *Client) UpdateContent(
	ctx context.Context,
	id int,
	req UpdateContentRequest,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/content/%d", id), nil, req)
}

// SetSystemFields sends PUT /api/v1/content/{id}/system-fields with the given
// system field key/value pairs and returns the decoded data and meta payloads, or
// an *APIError on failure. System fields are admin-managed metadata (e.g.
// editorial_status, internal_notes), so the server returns 403 FORBIDDEN unless
// the API key belongs to an Admin; an unknown key returns ErrUnknownSystemFieldKey
// and a value that fails the field schema returns ErrSystemFieldValidation, both
// surfaced as a 400 VALIDATION_ERROR.
func (c *Client) SetSystemFields(
	ctx context.Context,
	id int,
	systemFields map[string]any,
) (json.RawMessage, json.RawMessage, error) {
	body := struct {
		SystemFields map[string]any `json:"systemFields"`
	}{SystemFields: systemFields}
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/content/%d/system-fields", id), nil, body)
}

// UploadMediaRequest is the multipart payload sent to POST /api/v1/media. It
// is built in memory by UploadMedia — callers supply the file reader + filename
// and an optional metadata map (the server reads `altText` today; the map
// values are passed through as typed JSON so non-string values such as numbers
// or booleans are accepted, even though the server currently persists only
// `altText`). The client validates that File is non-nil and Filename is
// non-empty so a bad call site fails fast with a clear error instead of a
// runtime panic.
type UploadMediaRequest struct {
	File     io.Reader
	Filename string
	Metadata map[string]any
}

// UploadMedia sends POST /api/v1/media as multipart/form-data with a `file`
// part and an optional `metadata` JSON part, then returns the decoded data
// and meta payloads (the server returns 200 with the media projection in the
// data envelope) or an *APIError on failure.
//
// The multipart body is buffered in memory; the server enforces its own size
// limit at r.ParseMultipartForm. The 204/304+envelope defenses from
// doRequest apply unchanged.
func (c *Client) UploadMedia(
	ctx context.Context,
	req UploadMediaRequest,
) (json.RawMessage, json.RawMessage, error) {
	if req.File == nil {
		return nil, nil, &APIError{
			StatusCode: 0,
			Code:       "VALIDATION_ERROR",
			Message:    "File is required",
		}
	}
	if req.Filename == "" {
		return nil, nil, &APIError{
			StatusCode: 0,
			Code:       "VALIDATION_ERROR",
			Message:    "Filename is required",
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", req.Filename)
	if err != nil {
		return nil, nil, &APIError{
			StatusCode: 0,
			Code:       "MULTIPART_ERROR",
			Message:    fmt.Sprintf("create file part: %s", err),
		}
	}
	if _, err = io.Copy(part, req.File); err != nil {
		return nil, nil, &APIError{
			StatusCode: 0,
			Code:       "MULTIPART_ERROR",
			Message:    fmt.Sprintf("copy file bytes: %s", err),
		}
	}

	if len(req.Metadata) > 0 {
		metaJSON, mErr := json.Marshal(req.Metadata)
		if mErr != nil {
			return nil, nil, &APIError{
				StatusCode: 0,
				Code:       "MULTIPART_ERROR",
				Message:    fmt.Sprintf("marshal metadata: %s", mErr),
			}
		}
		if err = writer.WriteField("metadata", string(metaJSON)); err != nil {
			return nil, nil, &APIError{
				StatusCode: 0,
				Code:       "MULTIPART_ERROR",
				Message:    fmt.Sprintf("write metadata field: %s", err),
			}
		}
	}

	if err = writer.Close(); err != nil {
		return nil, nil, &APIError{
			StatusCode: 0,
			Code:       "MULTIPART_ERROR",
			Message:    fmt.Sprintf("close multipart writer: %s", err),
		}
	}

	return c.doRaw(ctx, http.MethodPost, "/api/v1/media", nil, writer.FormDataContentType(), &body)
}

// GetMedia sends GET /api/v1/media/{id} and returns the decoded data and meta
// payloads, or an *APIError on failure.
func (c *Client) GetMedia(
	ctx context.Context,
	id int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/media/%d", id), nil, nil)
}

// ListMedia sends GET /api/v1/media?limit=&cursor= and returns the decoded
// data (a bare array of media projections) and meta (with pagination), or an
// *APIError on failure. A limit of 0 (or negative) is passed as "no limit"
// (the server uses its default); an empty cursor is passed as "no cursor"
// (the first page).
func (c *Client) ListMedia(
	ctx context.Context,
	limit int,
	cursor string,
) (json.RawMessage, json.RawMessage, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return c.do(ctx, http.MethodGet, "/api/v1/media", query, nil)
}

// New builds a Client targeting baseURL authenticated with apiKey. The base URL
// is normalized (trailing slash trimmed) and must be an http(s) scheme + host
// with no userinfo and no path (the client appends /api/v1/...); rejecting a
// path avoids a silent double-prefix when callers proxy under a path. The
// returned Client follows redirects but strips the API key on cross-host hops.
// A default 30s per-request timeout is applied so a hung server cannot block
// the CLI indefinitely.
func New(baseURL, apiKey string) (*Client, error) {
	trimmed := strings.TrimRight(baseURL, "/")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid base url %q: %w", baseURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid base url %q: scheme must be http or https", baseURL)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid base url %q: must include a host", baseURL)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("invalid base url %q: userinfo is not permitted", baseURL)
	}
	if parsed.Path != "" {
		return nil, fmt.Errorf(
			"invalid base url %q: must be scheme and host only (the client appends /api/v1/...)",
			baseURL,
		)
	}
	return &Client{
		baseURL: parsed,
		apiKey:  apiKey,
		http: &http.Client{
			CheckRedirect: stripAuthOnCrossHostRedirect,
			Timeout:       30 * time.Second,
		},
	}, nil
}
