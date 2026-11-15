package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GenerateTheme(t *testing.T) {
	tests := []struct {
		name       string
		req        client.GenerateThemeRequest
		status     int
		body       string
		wantKeys   []string
		missingKey []string
		wantErr    bool
		wantCode   string
	}{
		{
			name: "success - full brief round-trips",
			req: client.GenerateThemeRequest{
				Description:       "A dark theme",
				Style:             "dark",
				TemplateOverrides: true,
				Edition:           "blog",
			},
			status:   http.StatusOK,
			body:     `{"data":{"theme":{"name":"midnight-serif","edition":"blog","rationale":"Dark."},"validation":{"valid":true,"findings":[]}}}`,
			wantKeys: []string{"description", "style", "templateOverrides", "edition"},
			wantErr:  false,
		},
		{
			name: "success - empty style and edition omitted from wire",
			req: client.GenerateThemeRequest{
				Description: "A clean theme",
			},
			status:     http.StatusOK,
			body:       `{"data":{"theme":{"name":"paper-light","edition":"blog","rationale":"Light."},"validation":{"valid":true,"findings":[]}}}`,
			wantKeys:   []string{"description"},
			missingKey: []string{"style", "templateOverrides", "edition"},
			wantErr:    false,
		},
		{
			name:     "error - gate failure carries findings in details",
			req:      client.GenerateThemeRequest{Description: "A broken theme"},
			status:   http.StatusInternalServerError,
			body:     `{"error":{"code":"generation_failed","message":"Failed to generate theme","details":{"valid":false,"findings":[{"rule":"css-parse","severity":"error","message":"static/style.css has unbalanced braces"}]}}}`,
			wantErr:  true,
			wantCode: "generation_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
			require.NoError(t, err)

			data, _, err := c.GenerateTheme(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				var apiErr *client.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				require.NotEmpty(t, apiErr.Details, "gate failures must carry findings")
				assert.Contains(t, string(apiErr.Details), "unbalanced braces")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, h.gotMethod)
			assert.Equal(t, "/api/v1/themegen/generate", h.gotPath)
			assert.Equal(t, tt.req.Description, h.gotPayload["description"])
			for _, key := range tt.wantKeys {
				assert.Contains(t, h.gotPayload, key)
			}
			for _, key := range tt.missingKey {
				assert.NotContains(t, h.gotPayload, key)
			}
			var payload struct {
				Theme struct {
					Name string `json:"name"`
				} `json:"theme"`
			}
			require.NoError(t, json.Unmarshal(data, &payload))
			assert.NotEmpty(t, payload.Theme.Name)
		})
	}
}

func TestClient_ThemeLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		call   func(c *client.Client, ctx context.Context) error
	}{
		{
			name:   "success - lists themes",
			method: http.MethodGet,
			path:   "/api/v1/themegen/themes",
			body:   `{"data":{"themes":[]}}`,
			call: func(c *client.Client, ctx context.Context) error {
				_, _, err := c.ThemesList(ctx)
				return err
			},
		},
		{
			name:   "success - activates a theme",
			method: http.MethodPost,
			path:   "/api/v1/themegen/themes/midnight-serif/activate",
			body:   `{"data":{"activated":true,"requiresRestart":false}}`,
			call: func(c *client.Client, ctx context.Context) error {
				_, _, err := c.ThemeActivate(ctx, "midnight-serif")
				return err
			},
		},
		{
			name:   "success - deactivates",
			method: http.MethodPost,
			path:   "/api/v1/themegen/themes/deactivate",
			body:   `{"data":{"activated":false,"requiresRestart":false}}`,
			call: func(c *client.Client, ctx context.Context) error {
				_, _, err := c.ThemeDeactivate(ctx)
				return err
			},
		},
		{
			name:   "success - deletes a theme",
			method: http.MethodDelete,
			path:   "/api/v1/themegen/themes/midnight-serif",
			body:   `{"data":{"deleted":true}}`,
			call: func(c *client.Client, ctx context.Context) error {
				_, _, err := c.ThemeDelete(ctx, "midnight-serif")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: http.StatusOK, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
			require.NoError(t, err)

			require.NoError(t, tt.call(c, context.Background()))
			assert.Equal(t, tt.method, h.gotMethod)
			assert.Equal(t, tt.path, h.gotPath)
			assert.Equal(t, "Bearer lesstruct_a1b2c3d4e5f6_<secret>", h.gotAuth)
		})
	}
}

func TestClient_ThemeDownload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantErr bool
	}{
		{
			name:    "success - streams the zip",
			payload: []byte("PK\x03\x04theme-zip-bytes"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/themegen/themes/midnight-serif/download", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tt.payload)
			}))
			defer srv.Close()

			c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
			require.NoError(t, err)

			var buf bytes.Buffer
			err = c.ThemeDownload(context.Background(), "midnight-serif", &buf, client.DownloadOptions{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.payload, buf.Bytes())
		})
	}
}
