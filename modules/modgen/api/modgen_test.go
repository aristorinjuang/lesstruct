package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/modgen/api"
	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	project       modgendomain.Project
	result        modgendomain.ApplyResult
	unapply       modgendomain.UnapplyResult
	unapplyErr    error
	err           error
	listErr       error
	zipErr        error
	applyErr      error
	deleteApplied bool
	deleteErr     error
	gotName       string
	gotReplace    bool
	gotConfirm    bool
	blockApply    chan struct{}
	applyStarted  chan struct{}
}

func (f *fakeService) Generate(_ context.Context, _ modgendomain.Brief) (modgendomain.Project, modgendomain.ValidationReport, error) {
	if f.err != nil {
		return modgendomain.Project{}, modgendomain.ValidationReport{}, f.err
	}
	return f.project, modgendomain.NewValidationReport(true, nil), nil
}

func (f *fakeService) Apply(_ context.Context, name string, replace, confirm bool) (modgendomain.Project, modgendomain.ApplyResult, error) {
	f.gotName = name
	f.gotReplace = replace
	f.gotConfirm = confirm
	if f.applyStarted != nil {
		select {
		case f.applyStarted <- struct{}{}:
		default:
		}
	}
	if f.blockApply != nil {
		<-f.blockApply
	}
	if f.applyErr != nil {
		return modgendomain.Project{}, modgendomain.ApplyResult{}, f.applyErr
	}
	return f.project, f.result, nil
}

func (f *fakeService) Unapply(_ context.Context, _ string, _ bool) (modgendomain.UnapplyResult, error) {
	if f.unapplyErr != nil {
		return modgendomain.UnapplyResult{}, f.unapplyErr
	}
	return f.unapply, nil
}

func (f *fakeService) List(_ context.Context) ([]modgendomain.InstalledProject, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []modgendomain.InstalledProject{}, nil
}

func (f *fakeService) Get(_ context.Context, _ string) (modgendomain.Project, error) {
	return f.project, nil
}

func (f *fakeService) Delete(_ context.Context, _ string) (bool, error) {
	if f.deleteErr != nil {
		return false, f.deleteErr
	}
	if f.err != nil {
		return false, f.err
	}
	return f.deleteApplied, nil
}

func (f *fakeService) Zip(_ context.Context, _ string) ([]byte, error) {
	if f.zipErr != nil {
		return nil, f.zipErr
	}
	return []byte("zip"), nil
}

func testModule() modgendomain.Project {
	return modgendomain.NewProject(
		"forum",
		[]string{"routes"},
		map[string]string{"module.go": "package forum"},
		"R.",
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

func TestModuleGenHandlerGenerate(t *testing.T) {
	tests := []struct {
		name      string
		body      any
		service   *fakeService
		principal bool
		wantCode  int
		wantErr   bool
	}{
		{
			name:      "success - generates module",
			body:      map[string]any{"description": "a forum", "surfaces": []string{"routes"}},
			service:   &fakeService{project: testModule()},
			principal: true,
			wantCode:  http.StatusOK,
			wantErr:   false,
		},
		{
			name:      "error - no principal",
			body:      map[string]any{"description": "a forum"},
			service:   &fakeService{project: testModule()},
			principal: false,
			wantCode:  http.StatusUnauthorized,
			wantErr:   false,
		},
		{
			name:      "error - no service",
			body:      map[string]any{"description": "a forum"},
			service:   nil,
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantErr:   false,
		},
		{
			name:      "error - bad body",
			body:      "not-json{{{",
			service:   &fakeService{project: testModule()},
			principal: true,
			wantCode:  http.StatusBadRequest,
			wantErr:   false,
		},
		{
			name:      "error - empty description",
			body:      map[string]any{"description": "  "},
			service:   &fakeService{project: testModule()},
			principal: true,
			wantCode:  http.StatusBadRequest,
			wantErr:   false,
		},
		{
			name:      "error - too long",
			body:      map[string]any{"description": strings.Repeat("a", 5001)},
			service:   &fakeService{project: testModule()},
			principal: true,
			wantCode:  http.StatusBadRequest,
			wantErr:   false,
		},
		{
			name:      "error - service fails",
			body:      map[string]any{"description": "a forum"},
			service:   &fakeService{err: modgendomain.ErrGenerationFailed},
			principal: true,
			wantCode:  http.StatusInternalServerError,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc api.ModuleService
			if tt.service != nil {
				svc = tt.service
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			var payload []byte
			switch body := tt.body.(type) {
			case string:
				payload = []byte(body)
			default:
				payload, _ = json.Marshal(tt.body)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/modgen/generate", bytes.NewReader(payload))
			if tt.principal {
				req = withPrincipal(req)
			}
			rec := httptest.NewRecorder()
			h.Generate(rec, req)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

func TestModuleGenHandlerListDownloadDelete(t *testing.T) {
	tests := []struct {
		name      string
		service   *fakeService
		principal bool
		wantCode  int
		wantErr   bool
	}{
		{
			name:      "success - list",
			service:   &fakeService{project: testModule()},
			principal: true,
			wantCode:  http.StatusOK,
			wantErr:   false,
		},
		{
			name:      "error - list no principal",
			service:   &fakeService{project: testModule()},
			principal: false,
			wantCode:  http.StatusUnauthorized,
			wantErr:   false,
		},
		{
			name:      "error - list no service",
			service:   nil,
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantErr:   false,
		},
		{
			name:      "error - list fails",
			service:   &fakeService{project: testModule(), listErr: modgendomain.ErrProjectNotFound},
			principal: true,
			wantCode:  http.StatusNotFound,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc api.ModuleService
			if tt.service != nil {
				svc = tt.service
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/modgen/projects", nil)
			if tt.principal {
				req = withPrincipal(req)
			}
			rec := httptest.NewRecorder()
			h.List(rec, req)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantCode, rec.Code)

			downloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/modgen/projects/forum/download", nil)
			if tt.principal {
				downloadReq = withPrincipal(downloadReq)
			}
			downloadReq = withChiParam(downloadReq, "name", "forum")
			downloadRec := httptest.NewRecorder()
			h.Download(downloadRec, downloadReq)
			assert.NotEqual(t, 0, downloadRec.Code)

			deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/modgen/projects/forum", nil)
			if tt.principal {
				deleteReq = withPrincipal(deleteReq)
			}
			deleteReq = withChiParam(deleteReq, "name", "forum")
			deleteRec := httptest.NewRecorder()
			h.Delete(deleteRec, deleteReq)
			assert.NotEqual(t, 0, deleteRec.Code)
		})
	}
}

func TestModuleGenHandlerApply(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		service   *fakeService
		principal bool
		wantCode  int
		wantName  string
		wantFlag  bool
		wantErr   bool
	}{
		{
			name:      "success - applies project",
			body:      `{}`,
			service:   &fakeService{project: testModule(), result: modgendomain.NewApplyResult(true, true, nil, []string{"forum"}, true)},
			principal: true,
			wantCode:  http.StatusOK,
			wantName:  "forum",
			wantErr:   false,
		},
		{
			name:      "success - replace and confirm cross the seam",
			body:      `{"replace":true,"confirm":true}`,
			service:   &fakeService{project: testModule(), result: modgendomain.NewApplyResult(true, true, nil, []string{"forum"}, true)},
			principal: true,
			wantCode:  http.StatusOK,
			wantName:  "forum",
			wantFlag:  true,
			wantErr:   false,
		},
		{
			name:      "error - no principal",
			service:   &fakeService{project: testModule()},
			principal: false,
			wantCode:  http.StatusUnauthorized,
			wantErr:   false,
		},
		{
			name:      "error - no service",
			service:   nil,
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantErr:   false,
		},
		{
			name:      "error - apply unavailable",
			service:   &fakeService{project: testModule(), applyErr: modgendomain.ErrApplyUnavailable},
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantErr:   false,
		},
		{
			name:      "error - not a checkout",
			service:   &fakeService{project: testModule(), applyErr: modgendomain.ErrNotCheckout},
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantErr:   false,
		},
		{
			name:      "error - already applied",
			service:   &fakeService{project: testModule(), applyErr: modgendomain.ErrAlreadyApplied},
			principal: true,
			wantCode:  http.StatusConflict,
			wantErr:   false,
		},
		{
			name:      "error - dirty tree",
			service:   &fakeService{project: testModule(), applyErr: modgendomain.ErrTreeDirty},
			principal: true,
			wantCode:  http.StatusConflict,
			wantErr:   false,
		},
		{
			name:      "error - apply failed",
			service:   &fakeService{project: testModule(), applyErr: modgendomain.ErrApplyFailed},
			principal: true,
			wantCode:  http.StatusInternalServerError,
			wantErr:   false,
		},
		{
			name:      "error - project not found",
			service:   &fakeService{project: testModule(), applyErr: modgendomain.ErrProjectNotFound},
			principal: true,
			wantCode:  http.StatusNotFound,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc api.ModuleService
			if tt.service != nil {
				svc = tt.service
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			body := tt.body
			if body == "" {
				body = `{}`
			}
			req := withChiParam(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body)), "name", "forum")
			if tt.principal {
				req = withPrincipal(req)
			}
			rec := httptest.NewRecorder()
			h.Apply(rec, req)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantCode, rec.Code)
			if tt.wantName != "" {
				require.NotNil(t, tt.service)
				assert.Equal(t, tt.wantName, tt.service.gotName)
				assert.Equal(t, tt.wantFlag, tt.service.gotReplace)
				assert.Equal(t, tt.wantFlag, tt.service.gotConfirm)
			}
		})
	}
}

func TestModuleGenHandlerZipDeleteErrors(t *testing.T) {
	tests := []struct {
		name       string
		service    *fakeService
		principal  bool
		wantZip    int
		wantDelete int
		wantErr    bool
	}{
		{
			name:       "error - zip fails",
			service:    &fakeService{project: testModule(), zipErr: modgendomain.ErrProjectNotFound},
			principal:  true,
			wantZip:    http.StatusNotFound,
			wantDelete: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error - delete fails",
			service:    &fakeService{project: testModule(), err: modgendomain.ErrProjectNotFound},
			principal:  true,
			wantZip:    http.StatusOK,
			wantDelete: http.StatusNotFound,
			wantErr:    false,
		},
		{
			name:       "error - no service",
			service:    nil,
			principal:  true,
			wantZip:    http.StatusServiceUnavailable,
			wantDelete: http.StatusServiceUnavailable,
			wantErr:    false,
		},
		{
			name:       "error - no principal",
			service:    &fakeService{project: testModule()},
			principal:  false,
			wantZip:    http.StatusUnauthorized,
			wantDelete: http.StatusUnauthorized,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc api.ModuleService
			if tt.service != nil {
				svc = tt.service
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			zipReq := withChiParam(httptest.NewRequest(http.MethodGet, "/x", nil), "name", "forum")
			if tt.principal {
				zipReq = withPrincipal(zipReq)
			}
			zipRec := httptest.NewRecorder()
			h.Download(zipRec, zipReq)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantZip, zipRec.Code)

			deleteReq := withChiParam(httptest.NewRequest(http.MethodDelete, "/x", nil), "name", "forum")
			if tt.principal {
				deleteReq = withPrincipal(deleteReq)
			}
			deleteRec := httptest.NewRecorder()
			h.Delete(deleteRec, deleteReq)
			assert.Equal(t, tt.wantDelete, deleteRec.Code)
		})
	}
}

func TestModuleGenHandlerUnapply(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		service   *fakeService
		principal bool
		wantCode  int
		wantText  string
		wantErr   bool
	}{
		{
			name:      "success - unapplies with confirm",
			body:      `{"confirm":true}`,
			service:   &fakeService{unapply: modgendomain.NewUnapplyResult(true, true, nil, []string{}, true)},
			principal: true,
			wantCode:  http.StatusOK,
			wantText:  `"removed":true`,
			wantErr:   false,
		},
		{
			name:      "error - no principal",
			body:      `{"confirm":true}`,
			service:   &fakeService{},
			principal: false,
			wantCode:  http.StatusUnauthorized,
			wantErr:   false,
		},
		{
			name:      "error - no service",
			body:      `{"confirm":true}`,
			service:   nil,
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantErr:   false,
		},
		{
			name:      "error - missing confirm",
			body:      `{}`,
			service:   &fakeService{unapplyErr: modgendomain.ErrConfirmationRequired},
			principal: true,
			wantCode:  http.StatusBadRequest,
			wantText:  "confirmation_required",
			wantErr:   false,
		},
		{
			name:      "error - not applied",
			body:      `{"confirm":true}`,
			service:   &fakeService{unapplyErr: modgendomain.ErrNotApplied},
			principal: true,
			wantCode:  http.StatusNotFound,
			wantText:  "module_not_applied",
			wantErr:   false,
		},
		{
			name:      "error - apply unavailable",
			body:      `{"confirm":true}`,
			service:   &fakeService{unapplyErr: modgendomain.ErrApplyUnavailable},
			principal: true,
			wantCode:  http.StatusServiceUnavailable,
			wantText:  "apply_unavailable",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc api.ModuleService
			if tt.service != nil {
				svc = tt.service
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			req := withChiParam(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tt.body)), "name", "forum")
			if tt.principal {
				req = withPrincipal(req)
			}
			rec := httptest.NewRecorder()
			h.Unapply(rec, req)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantCode, rec.Code)
			if tt.wantText != "" {
				assert.Contains(t, rec.Body.String(), tt.wantText)
			}
		})
	}
}

func TestModuleGenHandlerDeleteApplied(t *testing.T) {
	tests := []struct {
		name      string
		service   *fakeService
		principal bool
		wantCode  int
		wantText  string
		wantErr   bool
	}{
		{
			name:      "success - applied delete warns",
			service:   &fakeService{project: testModule(), deleteApplied: true},
			principal: true,
			wantCode:  http.StatusOK,
			wantText:  `"appliedInCheckout":true`,
			wantErr:   false,
		},
		{
			name:      "success - clean delete quiet",
			service:   &fakeService{project: testModule()},
			principal: true,
			wantCode:  http.StatusOK,
			wantText:  `"appliedInCheckout":false`,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc api.ModuleService
			if tt.service != nil {
				svc = tt.service
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			req := withChiParam(httptest.NewRequest(http.MethodDelete, "/x", nil), "name", "forum")
			if tt.principal {
				req = withPrincipal(req)
			}
			rec := httptest.NewRecorder()
			h.Delete(rec, req)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.wantText)
		})
	}
}

func TestModuleGenHandlerApplyBusy(t *testing.T) {
	tests := []struct {
		name     string
		wantCode int
		wantText string
		wantErr  bool
	}{
		{
			name:     "error - concurrent apply answers busy",
			wantCode: http.StatusTooManyRequests,
			wantText: "apply_busy",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{
				project:      testModule(),
				result:       modgendomain.NewApplyResult(false, false, nil, nil, false),
				blockApply:   make(chan struct{}),
				applyStarted: make(chan struct{}, 1),
			}
			h := api.NewModuleGenHandler(svc, nil, 0)
			first := withChiParam(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`)), "name", "forum")
			first = withPrincipal(first)
			firstDone := make(chan struct{})
			go func() {
				defer close(firstDone)
				h.Apply(httptest.NewRecorder(), first)
			}()
			<-svc.applyStarted

			second := withChiParam(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`)), "name", "forum")
			second = withPrincipal(second)
			rec := httptest.NewRecorder()
			h.Apply(rec, second)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.wantText)

			close(svc.blockApply)
			<-firstDone
		})
	}
}
