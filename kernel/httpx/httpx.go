// Package httpx is the kernel's HTTP response envelope: the one wire format
// every module writes. It is byte-identical to internal/api/response by
// contract — the compat suite in the main module diffs both writers — so
// clients cannot tell whether a route is served by core or by a module.
//
// Modules import this package instead of internal code they cannot reach.
// The kernel never imports net/http handlers or routers here; it only
// serializes envelopes.
package httpx

import (
	"encoding/json"
	"net/http"
)

// Response represents a uniform JSON response structure.
type Response struct {
	Data  any `json:"data,omitempty"`
	Error any `json:"error,omitempty"`
	Meta  any `json:"meta,omitempty"`
}

// ErrorInfo represents structured error information.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// SendJSON sends a JSON response with the given status code.
func SendJSON(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

// Success sends a successful response with data.
func Success(w http.ResponseWriter, data any) {
	SendJSON(w, http.StatusOK, Response{
		Data: data,
	})
}

// Error sends an error response with structured error info.
func Error(w http.ResponseWriter, statusCode int, code, message string, details any) {
	SendJSON(w, statusCode, Response{
		Error: ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// listResponse is the dedicated list envelope. Its Data field has NO
// `omitempty` so an empty list serializes as {"data":[],...} rather than
// omitting the key entirely.
type listResponse struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

// Pagination is the list-pagination metadata carried in a list response's
// meta. Cursor endpoints populate NextCursor; offset endpoints populate
// Total/Limit/Offset.
type Pagination struct {
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
	Total      *int   `json:"total,omitempty"`
	Limit      *int   `json:"limit,omitempty"`
	Offset     *int   `json:"offset,omitempty"`
}

// ListMeta wraps pagination metadata under the canonical "pagination" key.
type ListMeta struct {
	Pagination Pagination `json:"pagination"`
}

// SuccessList sends a list response whose Data is never omitted (so empty
// lists render as "data":[]).
func SuccessList(w http.ResponseWriter, items any, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResponse{
		Data: items,
		Meta: meta,
	})
}
