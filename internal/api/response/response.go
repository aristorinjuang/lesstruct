package response

import (
	"encoding/json"
	"net/http"
)

// Response represents a uniform JSON response structure
type Response struct {
	Data any `json:"data,omitempty"`
	Error any `json:"error,omitempty"`
	Meta any `json:"meta,omitempty"`
}

// ErrorInfo represents structured error information
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// SendJSON sends a JSON response with the given status code
func SendJSON(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

// Success sends a successful response with data
func Success(w http.ResponseWriter, data any) {
	SendJSON(w, http.StatusOK, Response{
		Data: data,
	})
}

// Error sends an error response with structured error info
func Error(w http.ResponseWriter, statusCode int, code, message string, details any) {
	SendJSON(w, statusCode, Response{
		Error: ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// listResponse is the dedicated list envelope. Its Data field has NO `omitempty` —
// unlike the shared Response.Data (tagged `json:"data,omitempty"`) — so an empty
// content list serializes as {"data":[],...} rather than omitting the key entirely.
// Go's omitempty drops zero-length slices, which would break the agent v1 list contract
// ({ "data": [...] }). This dedicated type is non-breaking: Success/Error keep using
// Response. meta remains omitempty so it only appears when pagination is present.
type listResponse struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

// Pagination is the cursor-pagination metadata carried in a list response's meta. It is
// generic so the media list (Story 2.3) reuses the same envelope.
type Pagination struct {
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
}

// ListMeta wraps pagination metadata under the canonical "pagination" key.
type ListMeta struct {
	Pagination Pagination `json:"pagination"`
}

// SuccessList sends a list response whose Data is never omitted (so empty lists render
// as "data":[]) and whose optional Meta carries pagination. Use this for the agent v1
// list endpoints instead of Success (see listResponse for the omitempty rationale).
func SuccessList(w http.ResponseWriter, items any, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResponse{
		Data: items,
		Meta: meta,
	})
}
