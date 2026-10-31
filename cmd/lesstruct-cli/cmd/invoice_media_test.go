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

func TestInvoiceSettingsGet(t *testing.T) {
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
			name:     "success - prints the block",
			args:     []string{"invoice-settings", "get"},
			status:   http.StatusOK,
			body:     `{"data":{"sellerName":"PT Lesstruct Demo","sellerAddress":"Jl. Example 1","sellerTaxId":"01","taxRate":12},"error":null}`,
			wantCode: 0,
			wantOut:  "rate 12.00%",
		},
		{
			name:     "error - non-admin key is forbidden",
			args:     []string{"invoice-settings", "get"},
			status:   http.StatusForbidden,
			body:     `{"error":{"code":"INSUFFICIENT_PERMISSIONS","message":"You do not have permission to manage orders"}}`,
			wantCode: 5,
			wantErr:  "manage orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/orders/invoice-settings", r.URL.Path)
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

func TestInvoiceSettingsSet(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		getBody  string
		wantCode int
		wantOut  string
		wantErr  string
		wantPut  string
	}{
		{
			name:     "success - overlays the rate over the server block",
			args:     []string{"invoice-settings", "set", "--tax-rate", "11"},
			getBody:  `{"data":{"sellerName":"PT Lesstruct Demo","sellerAddress":"Jl. Example 1","sellerTaxId":"01","taxRate":12},"error":null}`,
			wantCode: 0,
			wantOut:  "rate 11.00%",
			wantPut:  `"taxRate":11`,
		},
		{
			name:     "error - nothing to set",
			args:     []string{"invoice-settings", "set"},
			getBody:  `{"data":{"sellerName":"x","sellerAddress":"y","sellerTaxId":"z","taxRate":12},"error":null}`,
			wantCode: 2,
			wantErr:  "nothing to set",
		},
		{
			name:     "error - out-of-range rate surfaces the server refusal",
			args:     []string{"invoice-settings", "set", "--tax-rate", "101"},
			getBody:  `{"data":{"sellerName":"x","sellerAddress":"y","sellerTaxId":"z","taxRate":12},"error":null}`,
			wantCode: 5,
			wantErr:  "bad rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPut string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(tt.getBody))
					return
				}
				require.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/api/v1/orders/invoice-settings", r.URL.Path)
				raw, _ := io.ReadAll(r.Body)
				gotPut = string(raw)
				if strings.Contains(tt.name, "out-of-range") {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"bad rate"}}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":{"sellerName":"x","sellerAddress":"y","sellerTaxId":"z","taxRate":11},"error":null}`))
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
			if tt.wantPut != "" {
				assert.Contains(t, gotPut, tt.wantPut)
				assert.Contains(t, gotPut, `"sellerName":"PT Lesstruct Demo"`, "unchanged fields ride the merged block")
			}
		})
	}
}

func TestMediaDelete(t *testing.T) {
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
			name:     "success - deletes the media",
			args:     []string{"media", "delete", "20"},
			status:   http.StatusNoContent,
			wantCode: 0,
			wantOut:  "Deleted media #20",
		},
		{
			name:     "error - non-owner is forbidden",
			args:     []string{"media", "delete", "20"},
			status:   http.StatusForbidden,
			body:     `{"error":{"code":"FORBIDDEN","message":"Your role cannot manage media"}}`,
			wantCode: 5,
			wantErr:  "cannot manage media",
		},
		{
			name:     "error - ghost id",
			args:     []string{"media", "delete", "999"},
			status:   http.StatusNotFound,
			body:     `{"error":{"code":"NOT_FOUND","message":"Media not found"}}`,
			wantCode: 4,
			wantErr:  "Media not found",
		},
		{
			name:     "error - bad id never hits the wire",
			args:     []string{"media", "delete", "abc"},
			wantCode: 5,
			wantErr:  "invalid media id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				assert.Equal(t, http.MethodDelete, r.Method)
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
