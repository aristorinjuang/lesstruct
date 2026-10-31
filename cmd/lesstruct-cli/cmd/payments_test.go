package cmd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentsList(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "success - prints payment rows",
			args:     []string{"payments", "list"},
			status:   http.StatusOK,
			body:     `{"data":[{"id":7,"orderId":42,"method":"manual","status":"verified","reference":"BR-123","amount":522500,"verifiedBy":"admin"}],"error":null}`,
			wantCode: 0,
			wantOut:  "#7 manual verified",
		},
		{
			name:     "error - non-admin key is forbidden",
			args:     []string{"payments", "list"},
			status:   http.StatusForbidden,
			body:     `{"error":{"code":"INSUFFICIENT_PERMISSIONS","message":"You do not have permission to manage payments"}}`,
			wantCode: 5,
			wantErr:  "manage payments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/payments", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut.String())
			if tt.wantOut != "" {
				assert.Contains(t, out.String(), tt.wantOut)
			}
			if tt.wantErr != "" {
				assert.Contains(t, errOut.String(), tt.wantErr)
			}
		})
	}
}

func TestPaymentsGet(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "success - prints the record with its order context",
			args:     []string{"payments", "get", "7"},
			status:   http.StatusOK,
			body:     `{"data":{"id":7,"orderId":42,"method":"manual","status":"verified","reference":"BR-123","amount":522500,"verifiedBy":"admin","order":{"orderNumber":"ORD-000042","status":"paid","totalMinor":522500}},"error":null}`,
			wantCode: 0,
			wantOut:  "ORD-000042",
		},
		{
			name:     "error - unknown payment",
			args:     []string{"payments", "get", "7"},
			status:   http.StatusNotFound,
			body:     `{"error":{"code":"NOT_FOUND","message":"payment not found"}}`,
			wantCode: 4,
			wantErr:  "payment not found",
		},
		{
			name:     "error - bad id never hits the wire",
			args:     []string{"payments", "get", "abc"},
			wantCode: 5,
			wantErr:  "invalid payment id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				assert.Equal(t, "/api/v1/payments/7", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut.String())
			if tt.wantOut != "" {
				assert.Contains(t, out.String(), tt.wantOut)
			}
			if tt.wantErr != "" {
				assert.Contains(t, errOut.String(), tt.wantErr)
			}
			if tt.name == "error - bad id never hits the wire" {
				assert.False(t, called, "validation must precede the request")
			}
		})
	}
}

func TestPaymentsDecide(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		path     string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "success - verify marks the record verified",
			args:     []string{"payments", "verify", "7"},
			path:     "/api/v1/payments/7/verify",
			status:   http.StatusOK,
			body:     `{"data":{"id":7,"orderId":42,"method":"manual","status":"verified","reference":"BR-123","amount":522500,"verifiedBy":"admin"},"error":null}`,
			wantCode: 0,
			wantOut:  "verified",
		},
		{
			name:     "success - reject keeps the order open",
			args:     []string{"payments", "reject", "7"},
			path:     "/api/v1/payments/7/reject",
			status:   http.StatusOK,
			body:     `{"data":{"id":7,"orderId":42,"method":"manual","status":"rejected","reference":"BR-123","amount":522500},"error":null}`,
			wantCode: 0,
			wantOut:  "rejected",
		},
		{
			name:     "error - gateway-paid proof refuses with 409",
			args:     []string{"payments", "verify", "7"},
			path:     "/api/v1/payments/7/verify",
			status:   http.StatusConflict,
			body:     `{"error":{"code":"ORDER_NOT_PAYABLE","message":"The order cannot accept payment proofs"}}`,
			wantCode: 5,
			wantErr:  "cannot accept payment proofs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, tt.path, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut.String())
			if tt.wantOut != "" {
				assert.Contains(t, out.String(), tt.wantOut)
			}
			if tt.wantErr != "" {
				assert.Contains(t, errOut.String(), tt.wantErr)
			}
		})
	}
}
