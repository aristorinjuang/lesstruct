package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/plugingen/api"
	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	project       plugingendomain.Project
	err           error
	buildResult   plugingendomain.BuildResult
	buildErr      error
	blockGenerate chan struct{}
	blockBuild    chan struct{}
	generateCalls atomic.Int32
	buildCalls    atomic.Int32
}

func (f *fakeService) Generate(_ context.Context, _ plugingendomain.Brief) (plugingendomain.Project, plugingendomain.ValidationReport, error) {
	f.generateCalls.Add(1)
	if f.blockGenerate != nil {
		<-f.blockGenerate
	}
	if f.err != nil {
		return plugingendomain.Project{}, plugingendomain.ValidationReport{}, f.err
	}
	return f.project, plugingendomain.NewValidationReport(true, nil), nil
}

func (f *fakeService) Build(_ context.Context, _ string) (plugingendomain.BuildResult, error) {
	f.buildCalls.Add(1)
	if f.blockBuild != nil {
		<-f.blockBuild
	}
	if f.buildErr != nil {
		return plugingendomain.BuildResult{}, f.buildErr
	}
	return f.buildResult, nil
}

func (f *fakeService) List(_ context.Context) ([]plugingendomain.InstalledProject, error) {
	return []plugingendomain.InstalledProject{}, nil
}

func (f *fakeService) Get(_ context.Context, _ string) (plugingendomain.Project, error) {
	return f.project, nil
}

func (f *fakeService) Delete(_ context.Context, _ string) error {
	return nil
}

func (f *fakeService) Zip(_ context.Context, _ string) ([]byte, error) {
	return []byte("zip"), nil
}

func testProject() plugingendomain.Project {
	return plugingendomain.NewProject(
		"my-plugin",
		"go",
		[]string{"before_save"},
		nil,
		map[string]string{"main.go": "package main", "go.mod": "module x"},
		"",
	)
}

func withPrincipal(req *http.Request) *http.Request {
	ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{ID: "1", Username: "admin"})
	return req.WithContext(ctx)
}

func withChiParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func TestPluginGenHandlerGenerate(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		wantCode int
		wantErr  bool
	}{
		{
			name:     "success - generates project",
			body:     map[string]any{"description": "uppercase titles", "hooks": []string{"before_save"}},
			wantCode: http.StatusOK,
			wantErr:  false,
		},
		{
			name:     "error - empty description",
			body:     map[string]any{"description": "  "},
			wantCode: http.StatusBadRequest,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{project: testProject()}
			h := api.NewPluginGenHandler(svc, nil, nil, 0)
			payload, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/generate", bytes.NewReader(payload))
			req = withPrincipal(req)
			w := httptest.NewRecorder()
			h.Generate(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func TestPluginGenHandlerBuild(t *testing.T) {
	tests := []struct {
		name        string
		buildResult plugingendomain.BuildResult
		buildErr    error
		wantCode    int
		wantBody    []string
		wantErr     bool
	}{
		{
			name:        "success - installed without restart",
			buildResult: plugingendomain.NewBuildResult(true, false),
			wantCode:    http.StatusOK,
			wantBody:    []string{`"installed":true`, `"requiresRestart":false`},
			wantErr:     false,
		},
		{
			name:        "success - installed with restart",
			buildResult: plugingendomain.NewBuildResult(true, true),
			wantCode:    http.StatusOK,
			wantBody:    []string{`"installed":true`, `"requiresRestart":true`},
			wantErr:     false,
		},
		{
			name:     "error - build unavailable",
			buildErr: plugingendomain.ErrBuildUnavailable,
			wantCode: http.StatusServiceUnavailable,
			wantBody: []string{"build_unavailable"},
			wantErr:  false,
		},
		{
			name:     "error - project not found",
			buildErr: plugingendomain.ErrProjectNotFound,
			wantCode: http.StatusNotFound,
			wantBody: []string{"project_not_found"},
			wantErr:  false,
		},
		{
			name:     "error - already installed",
			buildErr: plugingendomain.ErrPluginInstalled,
			wantCode: http.StatusConflict,
			wantBody: []string{"plugin_exists"},
			wantErr:  false,
		},
		{
			name:     "error - build failed",
			buildErr: plugingendomain.ErrBuildFailed,
			wantCode: http.StatusInternalServerError,
			wantBody: []string{"build_failed"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{project: testProject(), buildResult: tt.buildResult, buildErr: tt.buildErr}
			h := api.NewPluginGenHandler(svc, nil, nil, 0)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/projects/my-plugin/build", nil)
			req = withPrincipal(req)
			req = withChiParam(req, "name", "my-plugin")
			w := httptest.NewRecorder()
			h.Build(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			for _, want := range tt.wantBody {
				assert.Contains(t, w.Body.String(), want)
			}
		})
	}
}

type fakeStatusReader struct {
	plugins []api.PluginStatus
}

func (f fakeStatusReader) LoadedPlugins(_ context.Context) []api.PluginStatus {
	return f.plugins
}

func TestPluginGenHandlerStatusKeys(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - status uses lowercase json keys",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{project: testProject()}
			status := fakeStatusReader{plugins: []api.PluginStatus{{Name: "hello-world", Status: "loaded", Hooks: []string{"BeforeSaveContent"}}}}
			h := api.NewPluginGenHandler(svc, status, nil, 0)
			req := withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil))
			w := httptest.NewRecorder()
			h.Status(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), `"name":"hello-world"`)
			assert.Contains(t, w.Body.String(), `"status":"loaded"`)
			assert.Contains(t, w.Body.String(), `"hooks":["BeforeSaveContent"]`)
			require.Equal(t, false, tt.wantErr)
		})
	}
}

func TestPluginGenHandlerListDownloadDeleteStatus(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - list download delete status",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{project: testProject()}
			h := api.NewPluginGenHandler(svc, nil, nil, 0)
			req := withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/plugingen/projects", nil))
			w := httptest.NewRecorder()
			h.List(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			req = withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/plugingen/projects/my-plugin/download", nil))
			req = withChiParam(req, "name", "my-plugin")
			w = httptest.NewRecorder()
			h.Download(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			req = withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/plugingen/projects/my-plugin", nil))
			req = withChiParam(req, "name", "my-plugin")
			w = httptest.NewRecorder()
			h.Delete(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			req = withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil))
			w = httptest.NewRecorder()
			h.Status(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, false, tt.wantErr)
		})
	}
}

func TestPluginGenHandlerConcurrencyGuards(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - busy semaphores refuse with 429",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := make(chan struct{})
			svc := &fakeService{project: testProject(), blockGenerate: release, blockBuild: release}
			h := api.NewPluginGenHandler(svc, nil, nil, 0)

			var wg sync.WaitGroup
			for range 2 {
				wg.Go(func() {
					req := withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/generate", strings.NewReader(`{"description":"x","hooks":["before_save"]}`)))
					w := httptest.NewRecorder()
					h.Generate(w, req)
				})
			}
			require.Eventually(t, func() bool {
				return svc.generateCalls.Load() == 2
			}, 5*time.Second, 10*time.Millisecond)

			fullReq := withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/generate", strings.NewReader(`{"description":"y","hooks":["before_save"]}`)))
			fullW := httptest.NewRecorder()
			h.Generate(fullW, fullReq)
			assert.Equal(t, http.StatusTooManyRequests, fullW.Code)
			assert.Contains(t, fullW.Body.String(), "generation_busy")

			buildRelease := make(chan struct{})
			buildSvc := &fakeService{project: testProject(), buildResult: plugingendomain.NewBuildResult(true, false), blockBuild: buildRelease}
			buildH := api.NewPluginGenHandler(buildSvc, nil, nil, 0)
			wg.Go(func() {
				req := withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/projects/my-plugin/build", nil))
				req = withChiParam(req, "name", "my-plugin")
				w := httptest.NewRecorder()
				buildH.Build(w, req)
			})
			require.Eventually(t, func() bool {
				return buildSvc.buildCalls.Load() == 1
			}, 5*time.Second, 10*time.Millisecond)

			busyReq := withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/projects/my-plugin/build", nil))
			busyReq = withChiParam(busyReq, "name", "my-plugin")
			busyW := httptest.NewRecorder()
			buildH.Build(busyW, busyReq)
			assert.Equal(t, http.StatusTooManyRequests, busyW.Code)
			assert.Contains(t, busyW.Body.String(), "build_busy")

			close(release)
			close(buildRelease)
			wg.Wait()
			require.Equal(t, false, tt.wantErr)
		})
	}
}

func TestPluginGenHandlerGenerateBodyCap(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - oversized body rejected",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{project: testProject()}
			h := api.NewPluginGenHandler(svc, nil, nil, 0)
			huge := `{"description":"` + strings.Repeat("a", 40<<10) + `","hooks":["before_save"]}`
			req := withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/plugingen/generate", strings.NewReader(huge)))
			w := httptest.NewRecorder()
			h.Generate(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "invalid_request")
			require.Equal(t, false, tt.wantErr)
		})
	}
}
