package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/posttypes-basic/api"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRoles struct {
	admin    map[string]bool
	manage   map[string][]string
	publish  map[string]bool
	media    map[string]bool
	comments map[string]bool
	roles    []api.Role
}

func (s *stubRoles) IsAdmin(name string) bool {
	return s.admin[name]
}

func (s *stubRoles) ManageablePostTypes(name string, allSlugs []string) []string {
	if s.admin[name] {
		result := make([]string, len(allSlugs))
		copy(result, allSlugs)
		return result
	}
	allowed := make(map[string]bool, len(s.manage[name]))
	for _, slug := range s.manage[name] {
		allowed[slug] = true
	}
	result := make([]string, 0, len(allowed))
	for _, slug := range allSlugs {
		if allowed[slug] {
			result = append(result, slug)
		}
	}
	return result
}

func (s *stubRoles) CanPublish(name string) bool {
	return s.admin[name] || s.publish[name]
}

func (s *stubRoles) CanMedia(name string) bool {
	return s.admin[name] || s.media[name]
}

func (s *stubRoles) CanComment(name string) bool {
	return s.admin[name] || s.comments[name]
}

func (s *stubRoles) GetAll() []api.Role {
	return s.roles
}

func newStubRoles() *stubRoles {
	return &stubRoles{
		admin: map[string]bool{"Admin": true},
		manage: map[string][]string{
			"Contributor": {"post", "page", "article", "event"},
			"Journalist":  {"article"},
		},
		publish:  map[string]bool{"Contributor": true, "Journalist": true},
		media:    map[string]bool{"Contributor": true, "Journalist": true},
		comments: map[string]bool{"Contributor": true, "Commentator": true, "Journalist": true},
		roles: []api.Role{
			{Name: "Admin", AllTypes: true, Publish: true, Media: true, Comments: true, IsAdmin: true},
			{Name: "Contributor", AllTypes: true, Publish: true, Media: true, Comments: true},
			{Name: "Commentator", Publish: false, Media: false, Comments: true},
			{Name: "Journalist", PostTypes: []string{"article"}, Publish: true, Media: true, Comments: true},
		},
	}
}

func denyAll(kernel.Principal, string) bool {
	return false
}

func adminOnly(principal kernel.Principal, capability string) bool {
	return capability == "admin" && principal.HasRole("Admin")
}

func newTestService(t *testing.T) *posttype.Service {
	t.Helper()
	service := posttype.NewService()
	require.NoError(t, service.Register(posttype.PostType{
		Name:     "Article",
		Slug:     "article",
		Supports: []string{"title", "content", "tags"},
	}))
	require.NoError(t, service.Register(posttype.PostType{
		Name:     "Event",
		Slug:     "event",
		Supports: []string{"title", "content"},
	}))
	return service
}

func withPrincipal(req *http.Request, principal kernel.Principal) *http.Request {
	return req.WithContext(kernel.ContextWithPrincipal(req.Context(), principal))
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
	return response
}

func assertLegacyEnvelope(t *testing.T, w *httptest.ResponseRecorder, withMeta bool) map[string]any {
	t.Helper()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"error":null`, "legacy envelope must carry explicit error:null")
	if !withMeta {
		return nil
	}
	envelope := decodeEnvelope(t, w)
	meta, ok := envelope["meta"].(map[string]any)
	require.True(t, ok, "legacy envelope must carry meta")
	assert.NotEmpty(t, meta["timestamp"], "legacy envelope meta must carry timestamp")
	return envelope
}

func TestHandler_GetPostTypes(t *testing.T) {
	admin := kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}}
	journalist := kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Journalist"}}

	tests := []struct {
		name      string
		principal kernel.Principal
		present   bool
		resolve   kernel.CapabilityResolver
		comments  bool
		wantSlugs []string
		wantErr   bool
	}{
		{
			name:      "success - no resolver sees all types",
			principal: journalist,
			present:   true,
			resolve:   nil,
			comments:  true,
			wantSlugs: []string{"post", "page", "media", "comment", "article", "event"},
			wantErr:   false,
		},
		{
			name:      "success - admin sees all types",
			principal: admin,
			present:   true,
			resolve:   adminOnly,
			comments:  true,
			wantSlugs: []string{"post", "page", "media", "comment", "article", "event"},
			wantErr:   false,
		},
		{
			name:      "success - restricted role sees manageable types",
			principal: journalist,
			present:   true,
			resolve: func(principal kernel.Principal, capability string) bool {
				if capability == "admin" {
					return principal.HasRole("Admin")
				}
				return capability == "content.manage:article" && principal.HasRole("Journalist")
			},
			comments:  true,
			wantSlugs: []string{"article"},
			wantErr:   false,
		},
		{
			name:      "success - unknown role sees empty list",
			principal: kernel.Principal{ID: "9", Username: "ghost", Roles: []string{"Ghost"}},
			present:   true,
			resolve:   denyAll,
			comments:  true,
			wantSlugs: []string{},
			wantErr:   false,
		},
		{
			name:      "success - anonymous with resolver sees empty list",
			present:   false,
			resolve:   denyAll,
			comments:  true,
			wantSlugs: []string{},
			wantErr:   false,
		},
		{
			name:      "success - comment sentinel dropped when disabled",
			principal: admin,
			present:   true,
			resolve:   adminOnly,
			comments:  false,
			wantSlugs: []string{"post", "page", "media", "article", "event"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := api.New(newTestService(t), tt.comments, tt.resolve, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil)
			if tt.present {
				req = withPrincipal(req, tt.principal)
			}
			w := httptest.NewRecorder()

			handler.GetPostTypes(w, req)

			envelope := assertLegacyEnvelope(t, w, true)
			data, ok := envelope["data"].([]any)
			require.True(t, ok, "data must be an array")
			slugs := make([]string, 0, len(data))
			for _, item := range data {
				slugs = append(slugs, item.(map[string]any)["slug"].(string))
			}
			assert.ElementsMatch(t, tt.wantSlugs, slugs)
		})
	}
}

func TestHandler_GetPostTypes_IncludesHidden(t *testing.T) {
	service := posttype.NewService()
	require.NoError(t, service.Register(posttype.PostType{
		Name:     "Secret",
		Slug:     "secret",
		Supports: []string{"title", "content"},
		Hidden:   true,
	}))
	handler := api.New(service, true, nil, nil)

	w := httptest.NewRecorder()
	handler.GetPostTypes(w, httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil))

	envelope := assertLegacyEnvelope(t, w, true)
	data := envelope["data"].([]any)
	found := false
	for _, item := range data {
		entry := item.(map[string]any)
		if entry["slug"] == "secret" {
			found = true
			assert.Equal(t, true, entry["hidden"], "hidden type must carry hidden=true")
		}
	}
	assert.True(t, found, "admin endpoint must include hidden types")
}

func TestHandler_GetPublicPostTypes(t *testing.T) {
	tests := []struct {
		name      string
		comments  bool
		hideSlug  string
		wantSlugs []string
		wantErr   bool
	}{
		{
			name:      "success - lists visible types",
			comments:  true,
			wantSlugs: []string{"post", "page", "media", "comment", "article", "event"},
			wantErr:   false,
		},
		{
			name:      "success - hidden excluded",
			comments:  true,
			hideSlug:  "hidden-extra",
			wantSlugs: []string{"post", "page", "media", "comment", "article", "event"},
			wantErr:   false,
		},
		{
			name:      "success - comment dropped when disabled",
			comments:  false,
			wantSlugs: []string{"post", "page", "media", "article", "event"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(t)
			if tt.hideSlug != "" {
				require.NoError(t, service.Register(posttype.PostType{
					Name:     "Hidden",
					Slug:     "hidden-extra",
					Supports: []string{"title", "content"},
					Hidden:   true,
				}))
			}
			handler := api.New(service, tt.comments, nil, nil)

			w := httptest.NewRecorder()
			handler.GetPublicPostTypes(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/post_types", nil))

			envelope := assertLegacyEnvelope(t, w, true)
			data, ok := envelope["data"].([]any)
			require.True(t, ok, "data must be an array")
			slugs := make([]string, 0, len(data))
			for _, item := range data {
				slugs = append(slugs, item.(map[string]any)["slug"].(string))
			}
			assert.ElementsMatch(t, tt.wantSlugs, slugs)
			for _, slug := range slugs {
				assert.NotEqual(t, "hidden-extra", slug, "hidden types must not leak to public")
			}
		})
	}
}

func TestHandler_GetRoles(t *testing.T) {
	tests := []struct {
		name          string
		roles         api.RoleCapabilities
		principal     kernel.Principal
		wantStatus    int
		wantRole      string
		wantPostTypes []any
		wantErr       bool
	}{
		{
			name:       "error - no roles fails closed",
			roles:      nil,
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:          "success - roles plus caller capabilities",
			roles:         newStubRoles(),
			principal:     kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Journalist"}},
			wantStatus:    http.StatusOK,
			wantRole:      "Journalist",
			wantPostTypes: []any{"article"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := api.New(newTestService(t), true, nil, tt.roles)

			w := httptest.NewRecorder()
			handler.GetRoles(w, withPrincipal(
				httptest.NewRequest(http.MethodGet, "/api/v1/roles", nil),
				tt.principal,
			))

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr {
				return
			}
			envelope := assertLegacyEnvelope(t, w, true)
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be a map")
			roles, ok := data["roles"].([]any)
			require.True(t, ok, "roles must be an array")
			assert.Len(t, roles, 4)
			me, ok := data["me"].(map[string]any)
			require.True(t, ok, "me must be a map")
			assert.Equal(t, tt.wantRole, me["role"])
			postTypes, ok := me["postTypes"].([]any)
			require.True(t, ok, "me.postTypes must be an array")
			assert.Equal(t, tt.wantPostTypes, postTypes)
			assert.Equal(t, true, me["publish"])
			assert.Equal(t, true, me["media"])
			assert.Equal(t, true, me["comments"])
			assert.Equal(t, false, me["isAdmin"])
		})
	}
}

func TestHandler_GetRoles_Anonymous(t *testing.T) {
	handler := api.New(newTestService(t), true, nil, newStubRoles())

	w := httptest.NewRecorder()
	handler.GetRoles(w, withPrincipal(
		httptest.NewRequest(http.MethodGet, "/api/v1/roles", nil),
		kernel.Principal{},
	))

	require.Equal(t, http.StatusOK, w.Code)
	envelope := decodeEnvelope(t, w)
	me, ok := envelope["data"].(map[string]any)
	require.True(t, ok, "data must be a map")
	me, ok = me["me"].(map[string]any)
	require.True(t, ok, "me must be a map")
	assert.Equal(t, "", me["role"])
	assert.Empty(t, me["postTypes"])
}

func TestHandler_GetRoles_CommentatorGating(t *testing.T) {
	tests := []struct {
		name         string
		comments     bool
		registration *bool
		wantNames    []string
		notWant      []string
	}{
		{
			name:      "success - comments enabled lists Commentator",
			comments:  true,
			wantNames: []string{"Admin", "Contributor", "Commentator", "Journalist"},
		},
		{
			name:      "success - comments disabled hides Commentator",
			comments:  false,
			wantNames: []string{"Admin", "Contributor", "Journalist"},
			notWant:   []string{"Commentator"},
		},
		{
			name:         "success - registration enabled keeps Commentator hidden without comments",
			comments:     false,
			registration: func() *bool { v := true; return &v }(),
			wantNames:    []string{"Admin", "Contributor", "Journalist"},
			notWant:      []string{"Commentator"},
		},
		{
			name:         "success - comments disabled refuses Commentator even with registration",
			comments:     true,
			registration: func() *bool { v := false; return &v }(),
			wantNames:    []string{"Admin", "Contributor", "Journalist"},
			notWant:      []string{"Commentator"},
		},
		{
			name:         "success - comments and registration both enabled lists Commentator",
			comments:     true,
			registration: func() *bool { v := true; return &v }(),
			wantNames:    []string{"Admin", "Contributor", "Commentator", "Journalist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := api.New(newTestService(t), tt.comments, nil, newStubRoles())
			if tt.registration != nil {
				handler.SetRegistrationEnabled(*tt.registration)
			}

			w := httptest.NewRecorder()
			handler.GetRoles(w, withPrincipal(
				httptest.NewRequest(http.MethodGet, "/api/v1/roles", nil),
				kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			))

			require.Equal(t, http.StatusOK, w.Code)
			envelope := decodeEnvelope(t, w)
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be a map")
			roles, ok := data["roles"].([]any)
			require.True(t, ok, "roles must be an array")
			names := make([]string, 0, len(roles))
			for _, r := range roles {
				entry, ok := r.(map[string]any)
				require.True(t, ok, "role must be an object")
				name, _ := entry["name"].(string)
				names = append(names, name)
			}
			assert.ElementsMatch(t, tt.wantNames, names)
			for _, absent := range tt.notWant {
				assert.NotContains(t, names, absent)
			}
		})
	}
}

func TestHandler_GetUserFieldsEndpoint(t *testing.T) {
	admin := kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}}
	editor := kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Editor"}}

	tests := []struct {
		name       string
		principal  kernel.Principal
		present    bool
		resolve    kernel.CapabilityResolver
		wantStatus int
		wantCode   string
		wantErr    bool
	}{
		{
			name:       "success - admin reads schemas",
			principal:  admin,
			present:    true,
			resolve:    adminOnly,
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error - non-admin forbidden with identical message",
			principal:  editor,
			present:    true,
			resolve:    adminOnly,
			wantStatus: http.StatusForbidden,
			wantCode:   "INSUFFICIENT_PERMISSIONS",
			wantErr:    true,
		},
		{
			name:       "error - anonymous forbidden",
			present:    false,
			resolve:    adminOnly,
			wantStatus: http.StatusForbidden,
			wantCode:   "INSUFFICIENT_PERMISSIONS",
			wantErr:    true,
		},
		{
			name:       "error - nil resolver fails closed",
			principal:  admin,
			present:    true,
			resolve:    nil,
			wantStatus: http.StatusForbidden,
			wantCode:   "INSUFFICIENT_PERMISSIONS",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := api.New(posttype.NewService(), true, tt.resolve, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/user_fields", nil)
			if tt.present {
				req = withPrincipal(req, tt.principal)
			}
			w := httptest.NewRecorder()

			handler.GetUserFieldsEndpoint(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr {
				assert.Contains(t, w.Body.String(), tt.wantCode)
				return
			}
			raw := w.Body.String()
			assert.Contains(t, raw, `"error":null`, "endpoint keeps explicit error:null")
			assert.NotContains(t, raw, `"meta"`, "endpoint without meta must omit the key")
			envelope := decodeEnvelope(t, w)
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be a map")
			_, hasFields := data["fields"]
			assert.True(t, hasFields, "data must carry fields")
			_, hasSystem := data["systemFields"]
			assert.True(t, hasSystem, "data must carry systemFields")
		})
	}
}

func TestHandler_LegacyRawBytes(t *testing.T) {
	handler := api.New(posttype.NewService(), true, nil, nil)

	w := httptest.NewRecorder()
	handler.GetPublicPostTypes(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/post_types", nil))

	raw := w.Body.String()
	assert.True(t, strings.HasPrefix(strings.TrimSpace(raw), `{"data":`), "legacy envelope starts with data, got %.60s", raw)
	assert.Contains(t, raw, `"error":null`)
	assert.Contains(t, raw, `"meta":{"timestamp":"`)
}

func TestHandler_CommentSentinel(t *testing.T) {
	handler := api.New(posttype.NewService(), false, nil, nil)

	w := httptest.NewRecorder()
	handler.GetPublicPostTypes(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/post_types", nil))

	envelope := decodeEnvelope(t, w)
	for _, item := range envelope["data"].([]any) {
		assert.NotEqual(t, "comment", item.(map[string]any)["slug"], "comment sentinel must drop when disabled")
	}
}
