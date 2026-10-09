package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// discardLogger drops log lines. It is the default when a handler is built
// without a logger so module tests and fixtures need no log wiring.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// sendSuccessResponse reproduces the exact pre-module browser response bytes:
// data plus an explicit error:null plus a timestamp meta. encoding/json
// cannot emit a null for an omitempty field, so this dedicated shape (with a
// literal nil) preserves the legacy envelope. It is byte-identical to the
// core content handler's helper by contract.
func sendSuccessResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  data,
		"error": nil,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, code, message string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}
