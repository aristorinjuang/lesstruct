package posttypesbasic_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	posttypesbasic "github.com/aristorinjuang/lesstruct/modules/posttypes-basic"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	"github.com/aristorinjuang/lesstruct/modules/posttypes-basic/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, posttypesbasic.Name, module.Name())
	assert.Equal(t, posttypesbasic.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_PostTypes(t *testing.T) {
	module := testutil.NewModule()
	require.NoError(t, module.Service().Register(posttype.PostType{
		Name:     "Article",
		Slug:     "article",
		Supports: []string{"title", "content"},
	}))

	defs := module.PostTypes()
	bySlug := make(map[string]kernel.PostTypeDef, len(defs))
	for _, def := range defs {
		bySlug[def.Slug] = def
	}

	for _, slug := range []string{"post", "page"} {
		def, ok := bySlug[slug]
		require.True(t, ok, "module must declare %q", slug)
		assert.True(t, def.Protected, "module must reserve %q", slug)
	}
	article, ok := bySlug["article"]
	require.True(t, ok, "module must declare custom types")
	assert.False(t, article.Protected, "custom types are unprotected")
	for _, slug := range []string{"media", "comment"} {
		_, ok := bySlug[slug]
		assert.False(t, ok, "placeholder %q belongs to its future module", slug)
	}
}

func TestModule_Routes(t *testing.T) {
	tests := []struct {
		name       string
		mount      func(chi.Router)
		method     string
		target     string
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "success - protected post types",
			mount:      testutil.NewModule().RegisterRoutes,
			method:     http.MethodGet,
			target:     "/post_types",
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error - roles without registry fails closed",
			mount:      testutil.NewModule().RegisterRoutes,
			method:     http.MethodGet,
			target:     "/roles",
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "error - user fields without resolver fails closed",
			mount:      testutil.NewModule().RegisterRoutes,
			method:     http.MethodGet,
			target:     "/user_fields",
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name:       "success - public post types",
			mount:      testutil.NewModule().RegisterPublicRoutes,
			method:     http.MethodGet,
			target:     "/post_types",
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := chi.NewRouter()
			tt.mount(router)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, nil))

			assert.Equal(t, tt.wantStatus, w.Code)
			if !tt.wantErr {
				var envelope map[string]any
				require.NoError(t, json.NewDecoder(w.Body).Decode(&envelope))
				_, hasData := envelope["data"]
				assert.True(t, hasData)
			}
		})
	}
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
}

func denyAll(kernel.Principal, string) bool {
	return false
}

func TestModule_Options(t *testing.T) {
	module := testutil.NewModule(
		posttypesbasic.WithCommentsEnabled(false),
		posttypesbasic.WithCapabilityResolver(denyAll),
		posttypesbasic.WithRoles(nil),
	)

	require.NoError(t, module.Init(testutil.Kernel()))
	assert.NotEmpty(t, module.PostTypes())
	require.NoError(t, module.Shutdown())
}
