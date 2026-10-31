package cmd_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrdersList(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
		wantPath string
	}{
		{
			name:     "success - prints order rows",
			args:     []string{"orders", "list"},
			status:   http.StatusOK,
			body:     `{"data":[{"id":42,"orderNumber":"ORD-000042","status":"pending","customerEmail":"a@b.c","currency":"IDR","subtotal":598000,"shippingMinor":0,"totalMinor":598000,"itemCount":1}],"error":null}`,
			wantCode: 0,
			wantOut:  "ORD-000042 pending",
			wantPath: "/api/v1/orders",
		},
		{
			name:     "success - status filter rides the query",
			args:     []string{"orders", "list", "--status", "paid"},
			status:   http.StatusOK,
			body:     `{"data":[],"error":null}`,
			wantCode: 0,
			wantPath: "/api/v1/orders",
		},
		{
			name:     "error - unknown status",
			args:     []string{"orders", "list", "--status", "bogus"},
			status:   http.StatusBadRequest,
			body:     `{"error":{"code":"VALIDATION_ERROR","message":"unknown status"}}`,
			wantCode: 5,
			wantErr:  "unknown status",
			wantPath: "/api/v1/orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, tt.wantPath, r.URL.Path)
				if tt.name == "success - status filter rides the query" {
					assert.Equal(t, "paid", r.URL.Query().Get("status"))
				}
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

func TestOrdersGet(t *testing.T) {
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
			name:     "success - prints the order",
			args:     []string{"orders", "get", "42"},
			status:   http.StatusOK,
			body:     `{"data":{"id":42,"orderNumber":"ORD-000042","status":"paid","customerEmail":"a@b.c","currency":"IDR","subtotal":598000,"shippingMinor":15000,"totalMinor":613000,"itemCount":1},"error":null}`,
			wantCode: 0,
			wantOut:  "Order ORD-000042 paid",
		},
		{
			name:     "error - unknown order",
			args:     []string{"orders", "get", "42"},
			status:   http.StatusNotFound,
			body:     `{"error":{"code":"NOT_FOUND","message":"Order not found"}}`,
			wantCode: 4,
			wantErr:  "Order not found",
		},
		{
			name:     "error - display numbers are rejected client-side",
			args:     []string{"orders", "get", "ORD-000042"},
			wantCode: 5,
			wantErr:  "not ORD-000042",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				assert.Equal(t, "/api/v1/orders/42", r.URL.Path)
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
			if tt.name == "error - display numbers are rejected client-side" {
				assert.False(t, called, "display numbers must never reach the wire")
			}
		})
	}
}

func TestOrdersTransition(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
		wantBody string
	}{
		{
			name:     "success - moves the order",
			args:     []string{"orders", "transition", "42", "packed"},
			status:   http.StatusOK,
			body:     `{"data":{"id":42,"orderNumber":"ORD-000042","status":"packed","customerEmail":"a@b.c","currency":"IDR","subtotal":1,"shippingMinor":0,"totalMinor":1,"itemCount":1},"error":null}`,
			wantCode: 0,
			wantOut:  "Order ORD-000042 packed",
			wantBody: `"status":"packed"`,
		},
		{
			name:     "error - illegal move",
			args:     []string{"orders", "transition", "42", "paid"},
			status:   http.StatusConflict,
			body:     `{"error":{"code":"INVALID_TRANSITION","message":"illegal move"}}`,
			wantCode: 5,
			wantErr:  "illegal move",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/v1/orders/42/status", r.URL.Path)
				raw, _ := io.ReadAll(r.Body)
				gotBody = string(raw)
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
			if tt.wantBody != "" {
				assert.Contains(t, gotBody, tt.wantBody)
			}
		})
	}
}

func TestOrdersShipping(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantErr  string
	}{
		{
			name:     "success - sets the shipping line",
			args:     []string{"orders", "shipping", "42", "--amount", "25000"},
			wantCode: 0,
		},
		{
			name:     "error - missing amount",
			args:     []string{"orders", "shipping", "42"},
			wantCode: 2,
			wantErr:  "--amount is required",
		},
		{
			name:     "error - negative amount",
			args:     []string{"orders", "shipping", "42", "--amount", "-5"},
			wantCode: 5,
			wantErr:  "invalid shipping amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				raw, _ := io.ReadAll(r.Body)
				gotBody = string(raw)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"id":42,"orderNumber":"ORD-000042","status":"pending","customerEmail":"a@b.c","currency":"IDR","subtotal":1,"shippingMinor":25000,"totalMinor":25001,"itemCount":1},"error":null}`))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut.String())
			if tt.wantCode == 0 {
				assert.Equal(t, "/api/v1/orders/42/shipping", gotPath)
				assert.Contains(t, gotBody, `"amount":25000`)
				assert.Contains(t, out.String(), "ORD-000042")
			} else {
				assert.Contains(t, errOut.String(), tt.wantErr)
				assert.Empty(t, gotPath, "validation must precede the request")
			}
		})
	}
}
