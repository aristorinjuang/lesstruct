package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// UsersList sends GET /api/v1/users with optional status, limit, and
// offset filters and returns the decoded data (user summaries) and meta
// payloads, or an *APIError on failure. Empty filters are omitted.
func (c *Client) UsersList(
	ctx context.Context,
	status string,
	limit int,
	offset int,
) (json.RawMessage, json.RawMessage, error) {
	query := url.Values{}
	if status != "" {
		query.Set("status", status)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	return c.do(ctx, http.MethodGet, "/api/v1/users", query, nil)
}

// UsersPending sends GET /api/v1/users/pending and returns the decoded
// registration queue and meta payloads, or an *APIError on failure.
func (c *Client) UsersPending(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/users/pending", nil, nil)
}

// userDecisionBody is the confirmation payload destructive user decisions
// require. reason rides suspend and soft-delete only.
func userDecisionBody(confirmed bool, reason string) map[string]any {
	body := map[string]any{"confirmed": confirmed}
	if reason != "" {
		body["reason"] = reason
	}
	return body
}

// UsersApprove sends POST /api/v1/users/{id}/approve with an empty body
// and returns the decoded decision and meta payloads, or an *APIError on
// failure. Unverified emails and non-pending users read 409/400.
func (c *Client) UsersApprove(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/approve", id), nil, nil)
}

// UsersReject sends POST /api/v1/users/{id}/reject with the confirmation
// body and returns the decoded decision and meta payloads, or an *APIError
// on failure. Unconfirmed calls read 400.
func (c *Client) UsersReject(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/reject", id), nil, userDecisionBody(true, ""))
}

// UsersMarkSpam sends POST /api/v1/users/{id}/mark-spam with the
// confirmation body and returns the decoded decision and meta payloads, or
// an *APIError on failure.
func (c *Client) UsersMarkSpam(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/mark-spam", id), nil, userDecisionBody(true, ""))
}

// UsersSuspend sends POST /api/v1/users/{id}/suspend with an optional
// reason and returns the decoded decision and meta payloads, or an
// *APIError on failure. Settled states read 409.
func (c *Client) UsersSuspend(
	ctx context.Context,
	id int64,
	reason string,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/suspend", id), nil, userDecisionBody(true, reason))
}

// UsersUnsuspend sends POST /api/v1/users/{id}/unsuspend with an empty
// body and returns the decoded decision and meta payloads, or an *APIError
// on failure.
func (c *Client) UsersUnsuspend(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/unsuspend", id), nil, nil)
}

// UsersSoftDelete sends POST /api/v1/users/{id}/soft-delete with the
// confirmation body and returns the decoded decision and meta payloads, or
// an *APIError on failure. Unconfirmed calls read 400.
func (c *Client) UsersSoftDelete(
	ctx context.Context,
	id int64,
	reason string,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/soft-delete", id), nil, userDecisionBody(true, reason))
}
