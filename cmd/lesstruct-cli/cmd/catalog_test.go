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

func TestCatalogGet(t *testing.T) {
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
			name:     "success - prints the product",
			args:     []string{"catalog", "get", "9"},
			status:   http.StatusOK,
			body:     `{"data":{"contentId":9,"price":299000,"sku":"TSHIRT","trackInventory":true,"stock":7,"updatedAt":"2026-10-18T10:00:00Z"},"error":null}`,
			wantCode: 0,
			wantOut:  "Catalog product #9",
		},
		{
			name:     "error - unknown product",
			args:     []string{"catalog", "get", "9"},
			status:   http.StatusNotFound,
			body:     `{"error":{"code":"NOT_FOUND","message":"no catalog data"}}`,
			wantCode: 4,
			wantErr:  "no catalog data",
		},
		{
			name:     "error - non-admin key is forbidden",
			args:     []string{"catalog", "get", "9"},
			status:   http.StatusForbidden,
			body:     `{"error":{"code":"INSUFFICIENT_PERMISSIONS","message":"your role cannot manage the catalog"}}`,
			wantCode: 5,
			wantErr:  "cannot manage the catalog",
		},
		{
			name:     "error - bad id never hits the wire",
			args:     []string{"catalog", "get", "abc"},
			wantCode: 5,
			wantErr:  "invalid content id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/catalog/products/9", r.URL.Path)
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

func TestCatalogUpsert(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		getBody  string
		getCode  int
		putBody  string
		wantCode int
		wantOut  string
		wantErr  string
		wantPut  string
	}{
		{
			name:     "success - merges flags over the server record",
			args:     []string{"catalog", "upsert", "9", "--price", "319000"},
			getBody:  `{"data":{"contentId":9,"price":299000,"sku":"TSHIRT","trackInventory":true,"stock":7},"error":null}`,
			getCode:  http.StatusOK,
			putBody:  `{"data":{"contentId":9,"price":319000,"sku":"TSHIRT","trackInventory":true,"stock":7},"error":null}`,
			wantCode: 0,
			wantOut:  "price 319000",
			wantPut:  `"price":319000`,
		},
		{
			name:     "success - creates from flags when no record exists",
			args:     []string{"catalog", "upsert", "9", "--price", "299000", "--sku", "TSHIRT"},
			getBody:  `{"error":{"code":"NOT_FOUND","message":"none"}}`,
			getCode:  http.StatusNotFound,
			putBody:  `{"data":{"contentId":9,"price":299000,"sku":"TSHIRT"},"error":null}`,
			wantCode: 0,
			wantOut:  "Catalog product #9",
		},
		{
			name:     "error - create without price is rejected client-side",
			args:     []string{"catalog", "upsert", "9", "--sku", "TSHIRT"},
			getBody:  `{"error":{"code":"NOT_FOUND","message":"none"}}`,
			getCode:  http.StatusNotFound,
			wantCode: 2,
			wantErr:  "--price is required",
		},
		{
			name:     "error - nothing to set",
			args:     []string{"catalog", "upsert", "9"},
			getBody:  `{"data":{"contentId":9,"price":299000},"error":null}`,
			getCode:  http.StatusOK,
			wantCode: 2,
			wantErr:  "nothing to set",
		},
		{
			name:     "error - bad variants json",
			args:     []string{"catalog", "upsert", "9", "--variants-json", "{oops"},
			getBody:  `{"data":{"contentId":9,"price":299000},"error":null}`,
			getCode:  http.StatusOK,
			wantCode: 5,
			wantErr:  "--variants-json is not valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPut string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					assert.Equal(t, "/api/v1/catalog/products/9", r.URL.Path)
					w.WriteHeader(tt.getCode)
					_, _ = w.Write([]byte(tt.getBody))
				case http.MethodPut:
					assert.Equal(t, "/api/v1/catalog/products/9", r.URL.Path)
					raw, _ := io.ReadAll(r.Body)
					gotPut = string(raw)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(tt.putBody))
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				}
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
			}
		})
	}
}
