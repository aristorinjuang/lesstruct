package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// sendSuccessResponse reproduces the browser response bytes: data plus an
// explicit error:null plus a timestamp meta.
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
