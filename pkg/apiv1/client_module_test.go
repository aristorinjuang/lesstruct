package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GenerateModule(t *testing.T) {
	tests := []struct {
		name       string
		req        client.GenerateModuleRequest
		status     int
		body       string
		wantKeys   []string
		missingKey []string
		wantErr    bool
		wantCode   string
	}{
		{
			name: "success - full brief round-trips",
			req: client.GenerateModuleRequest{
				Description: "A forum module",
				Surfaces:    []string{"routes", "migrations"},
			},
			status:   http.StatusOK,
			body:     `{"data":{"project":{"name":"forum","surfaces":["routes"]},"validation":{"valid":true,"findings":[]}}}`,
			wantKeys: []string{"description", "surfaces"},
			wantErr:  false,
		},
		{
			name: "success - empty surfaces omitted from wire",
			req: client.GenerateModuleRequest{
				Description: "Something",
			},
			status:     http.StatusOK,
			body:       `{"data":{"project":{"name":"mymodule"},"validation":{"valid":true,"findings":[]}}}`,
			wantKeys:   []string{"description"},
			missingKey: []string{"surfaces"},
			wantErr:    false,
		},
		{
			name:     "error - gate failure carries findings in details",
			req:      client.GenerateModuleRequest{Description: "A broken module"},
			status:   http.StatusInternalServerError,
			body:     `{"error":{"code":"generation_failed","message":"Failed to generate module","details":{"valid":false,"findings":[{"rule":"module-go-parse","severity":"error","message":"module.go does not parse"}]}}}`,
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

			data, _, err := c.GenerateModule(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				var apiErr *client.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				require.NotEmpty(t, apiErr.Details, "gate failures must carry findings")
				assert.Contains(t, string(apiErr.Details), "does not parse")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, h.gotMethod)
			assert.Equal(t, "/api/v1/modgen/generate", h.gotPath)
			assert.Equal(t, tt.req.Description, h.gotPayload["description"])
			for _, key := range tt.wantKeys {
				assert.Contains(t, h.gotPayload, key)
			}
			for _, key := range tt.missingKey {
				assert.NotContains(t, h.gotPayload, key)
			}
			var payload struct {
				Project struct {
					Name string `json:"name"`
				} `json:"project"`
			}
			require.NoError(t, json.Unmarshal(data, &payload))
			assert.NotEmpty(t, payload.Project.Name)
		})
	}
}

func TestClient_ModuleLifecycle(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{
			name:    "success - list",
			status:  http.StatusOK,
			body:    `{"data":{"projects":[]}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
			require.NoError(t, err)

			_, _, err = c.ModuleProjectsList(context.Background())
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, err)
			assert.Equal(t, "/api/v1/modgen/projects", h.gotPath)

			_, _, err = c.ModuleDelete(context.Background(), "forum")
			require.NoError(t, err)
			assert.Equal(t, "/api/v1/modgen/projects/forum", h.gotPath)
		})
	}
}

func TestClient_ModuleApply(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantErr  bool
		wantCode string
	}{
		{
			name:    "success - apply stages and verifies",
			status:  http.StatusOK,
			body:    `{"data":{"project":{"name":"forum"},"apply":{"applied":true,"verified":true,"steps":[],"appliedModules":["forum"],"requiresRestart":true}}}`,
			wantErr: false,
		},
		{
			name:     "error - dirty tree",
			status:   http.StatusConflict,
			body:     `{"error":{"code":"tree_dirty","message":"Checkout tree carries uncommitted changes"}}`,
			wantErr:  true,
			wantCode: "tree_dirty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
			require.NoError(t, err)

			data, _, err := c.ModuleApply(context.Background(), "forum", client.ModuleApplyRequest{})
			if tt.wantErr {
				require.Error(t, err)
				var apiErr *client.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, h.gotMethod)
			assert.Equal(t, "/api/v1/modgen/projects/forum/apply", h.gotPath)
			var payload struct {
				Apply struct {
					Applied  bool     `json:"applied"`
					Verified bool     `json:"verified"`
					Steps    []string `json:"steps"`
				} `json:"apply"`
			}
			require.NoError(t, json.Unmarshal(data, &payload))
			assert.True(t, payload.Apply.Applied)
			assert.True(t, payload.Apply.Verified)
		})
	}
}
