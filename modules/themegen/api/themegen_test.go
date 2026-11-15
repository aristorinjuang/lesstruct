package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/themegen/api"
	apimocks "github.com/aristorinjuang/lesstruct/modules/themegen/api/mocks"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func mockAny() any {
	return mock.Anything
}

func testTheme() themegendomain.Theme {
	return themegendomain.NewTheme("midnight-serif", themegendomain.EditionBlog, map[string]string{
		"static/base.css": ":root{}",
	}, "rationale")
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

func TestThemGenHandler_Generate(t *testing.T) {
	tests := []struct {
		name      string
		body      any
		setupMock func(svc *apimocks.MockThemeService)
		wantCode  int
		wantErr   bool
	}{
		{
			name: "success - generates theme",
			body: map[string]any{"description": "A dark editorial theme"},
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Generate(mockAny(), mockAny(), mockAny()).Return(testTheme(), themegendomain.NewValidationReport(true, nil), nil).Once()
			},
			wantCode: http.StatusOK,
			wantErr:  false,
		},
		{
			name: "success - template overrides reach the brief",
			body: map[string]any{"description": "An editorial theme", "templateOverrides": true},
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().
					Generate(
						mockAny(),
						mock.MatchedBy(func(brief themegendomain.Brief) bool {
							return brief.TemplateOverrides
						}),
						mockAny(),
					).
					Return(testTheme(), themegendomain.NewValidationReport(true, nil), nil).
					Once()
			},
			wantCode: http.StatusOK,
			wantErr:  false,
		},
		{
			name:      "error - empty description",
			body:      map[string]any{"description": "  "},
			setupMock: func(svc *apimocks.MockThemeService) {},
			wantCode:  http.StatusBadRequest,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			if tt.setupMock != nil {
				tt.setupMock(svc)
			}
			h := api.NewThemGenHandler(svc, nil, nil, themegendomain.EditionBlog, nil)
			payload, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/themegen/generate", bytes.NewReader(payload))
			req = withPrincipal(req)
			w := httptest.NewRecorder()
			h.Generate(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func TestThemGenHandler_List(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(svc *apimocks.MockThemeService)
		wantCode  int
	}{
		{
			name: "success - lists themes",
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().List(mockAny()).Return([]themegendomain.InstalledTheme{
					themegendomain.NewInstalledTheme("midnight-serif", themegendomain.EditionBlog, time.Now().UTC(), false, "generated"),
				}, nil).Once()
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			tt.setupMock(svc)
			h := api.NewThemGenHandler(svc, nil, nil, themegendomain.EditionBlog, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/themegen/themes", nil)
			req = withPrincipal(req)
			w := httptest.NewRecorder()
			h.List(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func TestThemGenHandler_Delete(t *testing.T) {
	tests := []struct {
		name      string
		themeName string
		setupMock func(svc *apimocks.MockThemeService)
		wantCode  int
	}{
		{
			name:      "success - deletes theme",
			themeName: "midnight-serif",
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Delete(mockAny(), "midnight-serif").Return(nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name:      "error - missing name",
			themeName: "",
			setupMock: func(svc *apimocks.MockThemeService) {},
			wantCode:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			tt.setupMock(svc)
			h := api.NewThemGenHandler(svc, nil, nil, themegendomain.EditionBlog, nil)
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/themegen/themes/"+tt.themeName, nil)
			req = withPrincipal(req)
			req = withChiParam(req, "name", tt.themeName)
			w := httptest.NewRecorder()
			h.Delete(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			require.NotNil(t, w.Body)
		})
	}
}

func decodeRequiresRestart(t *testing.T, w *httptest.ResponseRecorder) bool {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	requiresRestart, _ := envelope.Data["requiresRestart"].(bool)
	return requiresRestart
}

func previewTestKey() []byte {
	return []byte("test-preview-key-32-bytes-long!!")
}

func findPreviewCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == api.PreviewCookieName {
			return cookie
		}
	}
	t.Fatalf("preview cookie %q not set", api.PreviewCookieName)
	return nil
}

func TestThemGenHandler_Preview(t *testing.T) {
	validReport := themegendomain.NewValidationReport(true, nil)

	tests := []struct {
		name       string
		themeName  string
		withKey    bool
		setupMocks func(svc *apimocks.MockThemeService)
		wantCode   int
		wantCookie bool
	}{
		{
			name:      "success - sets signed preview cookie",
			themeName: "midnight-serif",
			withKey:   true,
			setupMocks: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Get(mockAny(), "midnight-serif").Return(testTheme(), nil).Once()
				svc.EXPECT().Validate(mockAny(), mockAny()).Return(validReport).Once()
			},
			wantCode:   http.StatusOK,
			wantCookie: true,
		},
		{
			name:      "error - preview unavailable without key",
			themeName: "midnight-serif",
			withKey:   false,
			setupMocks: func(svc *apimocks.MockThemeService) {
			},
			wantCode:   http.StatusServiceUnavailable,
			wantCookie: false,
		},
		{
			name:      "error - validation failure sets no cookie",
			themeName: "midnight-serif",
			withKey:   true,
			setupMocks: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Get(mockAny(), "midnight-serif").Return(testTheme(), nil).Once()
				svc.EXPECT().Validate(mockAny(), mockAny()).Return(themegendomain.NewValidationReport(false, nil)).Once()
			},
			wantCode:   http.StatusBadRequest,
			wantCookie: false,
		},
		{
			name:      "error - missing name",
			themeName: "",
			withKey:   true,
			setupMocks: func(svc *apimocks.MockThemeService) {
			},
			wantCode:   http.StatusBadRequest,
			wantCookie: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			activations := apimocks.NewMockThemeActivationStore(t)
			tt.setupMocks(svc)
			h := api.NewThemGenHandler(svc, activations, nil, themegendomain.EditionBlog, nil)
			if tt.withKey {
				h.SetPreviewKey(previewTestKey(), false)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/themegen/themes/"+tt.themeName+"/preview", nil)
			req = withPrincipal(req)
			req = withChiParam(req, "name", tt.themeName)
			w := httptest.NewRecorder()
			h.Preview(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			if !tt.wantCookie {
				for _, cookie := range w.Result().Cookies() {
					assert.NotEqual(t, api.PreviewCookieName, cookie.Name)
				}
				return
			}
			cookie := findPreviewCookie(t, w)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, "/", cookie.Path)
			assert.Equal(t, 900, cookie.MaxAge)
			assert.False(t, cookie.Secure)
			name, ok := api.VerifyPreviewCookie(cookie.Value, previewTestKey())
			require.True(t, ok)
			assert.Equal(t, "midnight-serif", name)
		})
	}
}

func TestThemGenHandler_StopPreview(t *testing.T) {
	tests := []struct {
		name     string
		wantCode int
	}{
		{
			name:     "success - clears preview cookie",
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			activations := apimocks.NewMockThemeActivationStore(t)
			h := api.NewThemGenHandler(svc, activations, nil, themegendomain.EditionBlog, nil)
			h.SetPreviewKey(previewTestKey(), false)
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/themegen/preview", nil)
			req = withPrincipal(req)
			w := httptest.NewRecorder()
			h.StopPreview(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			cookie := findPreviewCookie(t, w)
			assert.Equal(t, -1, cookie.MaxAge)
		})
	}
}

func decodePreviewStatus(t *testing.T, w *httptest.ResponseRecorder) (bool, string) {
	t.Helper()
	var envelope struct {
		Data struct {
			Preview bool   `json:"preview"`
			Theme   string `json:"theme"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	return envelope.Data.Preview, envelope.Data.Theme
}

func TestThemGenHandler_PreviewStatus(t *testing.T) {
	tests := []struct {
		name      string
		withKey   bool
		withAuth  bool
		cookie    func() *http.Cookie
		wantCode  int
		wantOn    bool
		wantTheme string
	}{
		{
			name:     "success - returns preview theme for valid cookie",
			withKey:  true,
			withAuth: true,
			cookie: func() *http.Cookie {
				value, _ := api.SignPreviewCookie("midnight-serif", previewTestKey())
				return &http.Cookie{Name: api.PreviewCookieName, Value: value}
			},
			wantCode:  http.StatusOK,
			wantOn:    true,
			wantTheme: "midnight-serif",
		},
		{
			name:     "success - no cookie reports no preview",
			withKey:  true,
			withAuth: true,
			cookie:   func() *http.Cookie { return nil },
			wantCode: http.StatusOK,
			wantOn:   false,
		},
		{
			name:     "success - forged cookie reports no preview",
			withKey:  true,
			withAuth: true,
			cookie: func() *http.Cookie {
				return &http.Cookie{Name: api.PreviewCookieName, Value: "forged.cookie.value"}
			},
			wantCode: http.StatusOK,
			wantOn:   false,
		},
		{
			name:     "success - no key reports no preview even with cookie",
			withKey:  false,
			withAuth: true,
			cookie: func() *http.Cookie {
				value, _ := api.SignPreviewCookie("midnight-serif", previewTestKey())
				return &http.Cookie{Name: api.PreviewCookieName, Value: value}
			},
			wantCode: http.StatusOK,
			wantOn:   false,
		},
		{
			name:     "error - unauthenticated",
			withKey:  true,
			withAuth: false,
			cookie:   func() *http.Cookie { return nil },
			wantCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			activations := apimocks.NewMockThemeActivationStore(t)
			h := api.NewThemGenHandler(svc, activations, nil, themegendomain.EditionBlog, nil)
			if tt.withKey {
				h.SetPreviewKey(previewTestKey(), false)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/themegen/preview", nil)
			if tt.withAuth {
				req = withPrincipal(req)
			}
			if cookie := tt.cookie(); cookie != nil {
				req.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			h.PreviewStatus(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantCode != http.StatusOK {
				return
			}
			preview, theme := decodePreviewStatus(t, w)
			assert.Equal(t, tt.wantOn, preview)
			assert.Equal(t, tt.wantTheme, theme)
		})
	}
}

func TestThemGenHandler_DeleteClearsPreviewCookie(t *testing.T) {
	previewCookieFor := func(name string) *http.Cookie {
		value, _ := api.SignPreviewCookie(name, previewTestKey())
		return &http.Cookie{Name: api.PreviewCookieName, Value: value}
	}

	tests := []struct {
		name        string
		deleted     string
		cookie      func() *http.Cookie
		setupMock   func(svc *apimocks.MockThemeService)
		wantCode    int
		wantCleared bool
	}{
		{
			name:    "success - clears cookie naming the deleted theme",
			deleted: "midnight-serif",
			cookie: func() *http.Cookie {
				return previewCookieFor("midnight-serif")
			},
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Delete(mockAny(), "midnight-serif").Return(nil).Once()
			},
			wantCode:    http.StatusOK,
			wantCleared: true,
		},
		{
			name:    "success - keeps cookie naming another theme",
			deleted: "midnight-serif",
			cookie: func() *http.Cookie {
				return previewCookieFor("paper-light")
			},
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Delete(mockAny(), "midnight-serif").Return(nil).Once()
			},
			wantCode:    http.StatusOK,
			wantCleared: false,
		},
		{
			name:    "success - no cookie sends none",
			deleted: "midnight-serif",
			cookie:  func() *http.Cookie { return nil },
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Delete(mockAny(), "midnight-serif").Return(nil).Once()
			},
			wantCode:    http.StatusOK,
			wantCleared: false,
		},
		{
			name:    "success - forged cookie for deleted theme sends none",
			deleted: "midnight-serif",
			cookie: func() *http.Cookie {
				return &http.Cookie{Name: api.PreviewCookieName, Value: "forged.cookie.value"}
			},
			setupMock: func(svc *apimocks.MockThemeService) {
				svc.EXPECT().Delete(mockAny(), "midnight-serif").Return(nil).Once()
			},
			wantCode:    http.StatusOK,
			wantCleared: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			tt.setupMock(svc)
			h := api.NewThemGenHandler(svc, nil, nil, themegendomain.EditionBlog, nil)
			h.SetPreviewKey(previewTestKey(), false)
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/themegen/themes/"+tt.deleted, nil)
			req = withPrincipal(req)
			req = withChiParam(req, "name", tt.deleted)
			if cookie := tt.cookie(); cookie != nil {
				req.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			h.Delete(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			var cleared *http.Cookie
			for _, cookie := range w.Result().Cookies() {
				if cookie.Name == api.PreviewCookieName {
					cleared = cookie
				}
			}
			if tt.wantCleared {
				require.NotNil(t, cleared, "expected the preview cookie to be cleared")
				assert.Equal(t, -1, cleared.MaxAge)
				return
			}
			assert.Nil(t, cleared, "expected no preview cookie in the response")
		})
	}
}

// stubSnapshotter renders a fixed document or error for snapshot tests.
type stubSnapshotter struct {
	doc string
	err error
}

func (s stubSnapshotter) SnapshotTheme(name string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.doc, nil
}

func TestThemGenHandler_Snapshot(t *testing.T) {
	tests := []struct {
		name        string
		themeName   string
		withAuth    bool
		snapshotter api.ThemeSnapshotter
		wantCode    int
		wantBody    []string
		wantCT      string
		wantCache   string
	}{
		{
			name:        "success - renders the theme document",
			themeName:   "midnight-serif",
			withAuth:    true,
			snapshotter: stubSnapshotter{doc: "<!DOCTYPE html><html><body>Midnight</body></html>"},
			wantCode:    http.StatusOK,
			wantBody:    []string{"Midnight"},
			wantCT:      "text/html; charset=utf-8",
			wantCache:   "private, max-age=300",
		},
		{
			name:        "success - renderer error degrades to placeholder",
			themeName:   "broken",
			withAuth:    true,
			snapshotter: stubSnapshotter{err: assert.AnError},
			wantCode:    http.StatusOK,
			wantBody:    []string{"Snapshot unavailable"},
			wantCT:      "text/html; charset=utf-8",
			wantCache:   "private, max-age=300",
		},
		{
			name:      "error - snapshot unavailable without renderer",
			themeName: "midnight-serif",
			withAuth:  true,
			wantCode:  http.StatusServiceUnavailable,
		},
		{
			name:        "error - missing name",
			themeName:   "",
			withAuth:    true,
			snapshotter: stubSnapshotter{doc: "<html></html>"},
			wantCode:    http.StatusBadRequest,
		},
		{
			name:        "error - unauthenticated",
			themeName:   "midnight-serif",
			withAuth:    false,
			snapshotter: stubSnapshotter{doc: "<html></html>"},
			wantCode:    http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			activations := apimocks.NewMockThemeActivationStore(t)
			h := api.NewThemGenHandler(svc, activations, nil, themegendomain.EditionBlog, nil)
			if tt.snapshotter != nil {
				h.SetSnapshotRenderer(tt.snapshotter)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/themegen/themes/"+tt.themeName+"/snapshot", nil)
			if tt.withAuth {
				req = withPrincipal(req)
			}
			req = withChiParam(req, "name", tt.themeName)
			w := httptest.NewRecorder()
			h.Snapshot(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			for _, want := range tt.wantBody {
				assert.Contains(t, w.Body.String(), want)
			}
			if tt.wantCT != "" {
				assert.Equal(t, tt.wantCT, w.Header().Get("Content-Type"))
			}
			if tt.wantCache != "" {
				assert.Equal(t, tt.wantCache, w.Header().Get("Cache-Control"))
			}
			if tt.wantCode == http.StatusOK {
				assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
				csp := w.Header().Get("Content-Security-Policy")
				assert.Contains(t, csp, "frame-ancestors 'self'")
				assert.Contains(t, csp, "script-src 'none'")
				assert.Empty(t, w.Header().Get("Content-Security-Policy-Report-Only"))
			}
		})
	}
}

func TestThemGenHandler_Activate(t *testing.T) {
	validReport := themegendomain.NewValidationReport(true, nil)

	tests := []struct {
		name        string
		setupMocks  func(svc *apimocks.MockThemeService, activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader)
		useReloader bool
		wantCode    int
		wantRestart bool
		wantErr     bool
	}{
		{
			name: "success - hot reload applies without restart",
			setupMocks: func(svc *apimocks.MockThemeService, activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("").Once()
				svc.EXPECT().Get(mockAny(), "midnight-serif").Return(testTheme(), nil).Once()
				svc.EXPECT().Validate(mockAny(), mockAny()).Return(validReport).Once()
				activations.EXPECT().SaveActive(mockAny(), "midnight-serif").Return(nil).Once()
				reloader.EXPECT().Reload(mockAny()).Return(nil).Once()
			},
			useReloader: true,
			wantCode:    http.StatusOK,
			wantRestart: false,
			wantErr:     false,
		},
		{
			name: "success - nil reloader keeps restart hint",
			setupMocks: func(svc *apimocks.MockThemeService, activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("").Once()
				svc.EXPECT().Get(mockAny(), "midnight-serif").Return(testTheme(), nil).Once()
				svc.EXPECT().Validate(mockAny(), mockAny()).Return(validReport).Once()
				activations.EXPECT().SaveActive(mockAny(), "midnight-serif").Return(nil).Once()
			},
			useReloader: false,
			wantCode:    http.StatusOK,
			wantRestart: true,
			wantErr:     false,
		},
		{
			name: "error - reload failure reverts config",
			setupMocks: func(svc *apimocks.MockThemeService, activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("prev-theme").Once()
				svc.EXPECT().Get(mockAny(), "midnight-serif").Return(testTheme(), nil).Once()
				svc.EXPECT().Validate(mockAny(), mockAny()).Return(validReport).Once()
				activations.EXPECT().SaveActive(mockAny(), "midnight-serif").Return(nil).Once()
				reloader.EXPECT().Reload(mockAny()).Return(assert.AnError).Once()
				activations.EXPECT().SaveActive(mockAny(), "prev-theme").Return(nil).Once()
			},
			useReloader: true,
			wantCode:    http.StatusInternalServerError,
			wantErr:     true,
		},
		{
			name: "error - validation failure blocks save",
			setupMocks: func(svc *apimocks.MockThemeService, activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("").Once()
				svc.EXPECT().Get(mockAny(), "midnight-serif").Return(testTheme(), nil).Once()
				svc.EXPECT().Validate(mockAny(), mockAny()).Return(themegendomain.NewValidationReport(false, nil)).Once()
			},
			useReloader: true,
			wantCode:    http.StatusBadRequest,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			activations := apimocks.NewMockThemeActivationStore(t)
			var reloader *apimocks.MockThemeReloader
			var reloaderIface api.ThemeReloader
			if tt.useReloader {
				reloader = apimocks.NewMockThemeReloader(t)
				reloaderIface = reloader
			}
			tt.setupMocks(svc, activations, reloader)
			h := api.NewThemGenHandler(svc, activations, nil, themegendomain.EditionBlog, reloaderIface)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/themegen/themes/midnight-serif/activate", nil)
			req = withPrincipal(req)
			req = withChiParam(req, "name", "midnight-serif")
			w := httptest.NewRecorder()
			h.Activate(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantErr {
				assert.Contains(t, w.Body.String(), "activation_failed")
				return
			}
			assert.Equal(t, tt.wantRestart, decodeRequiresRestart(t, w))
		})
	}
}

func TestThemGenHandler_Deactivate(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader)
		useReloader bool
		wantCode    int
		wantRestart bool
		wantErr     bool
	}{
		{
			name: "success - hot reload applies without restart",
			setupMocks: func(activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("midnight-serif").Once()
				activations.EXPECT().ClearActive(mockAny()).Return(nil).Once()
				reloader.EXPECT().Reload(mockAny()).Return(nil).Once()
			},
			useReloader: true,
			wantCode:    http.StatusOK,
			wantRestart: false,
			wantErr:     false,
		},
		{
			name: "success - nil reloader keeps restart hint",
			setupMocks: func(activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("midnight-serif").Once()
				activations.EXPECT().ClearActive(mockAny()).Return(nil).Once()
			},
			useReloader: false,
			wantCode:    http.StatusOK,
			wantRestart: true,
			wantErr:     false,
		},
		{
			name: "error - reload failure restores previous theme",
			setupMocks: func(activations *apimocks.MockThemeActivationStore, reloader *apimocks.MockThemeReloader) {
				activations.EXPECT().ActiveTheme(mockAny()).Return("midnight-serif").Once()
				activations.EXPECT().ClearActive(mockAny()).Return(nil).Once()
				reloader.EXPECT().Reload(mockAny()).Return(assert.AnError).Once()
				activations.EXPECT().SaveActive(mockAny(), "midnight-serif").Return(nil).Once()
			},
			useReloader: true,
			wantCode:    http.StatusInternalServerError,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := apimocks.NewMockThemeService(t)
			activations := apimocks.NewMockThemeActivationStore(t)
			var reloader *apimocks.MockThemeReloader
			var reloaderIface api.ThemeReloader
			if tt.useReloader {
				reloader = apimocks.NewMockThemeReloader(t)
				reloaderIface = reloader
			}
			tt.setupMocks(activations, reloader)
			h := api.NewThemGenHandler(svc, activations, nil, themegendomain.EditionBlog, reloaderIface)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/themegen/themes/deactivate", nil)
			req = withPrincipal(req)
			w := httptest.NewRecorder()
			h.Deactivate(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantErr {
				assert.Contains(t, w.Body.String(), "activation_failed")
				return
			}
			assert.Equal(t, tt.wantRestart, decodeRequiresRestart(t, w))
		})
	}
}
