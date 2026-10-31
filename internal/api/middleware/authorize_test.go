package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/kernel"
	kernelconformance "github.com/aristorinjuang/lesstruct/kernel/conformance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRoleCapabilities struct {
	admin    map[string]bool
	publish  map[string]bool
	media    map[string]bool
	comments map[string]bool
	manage   map[string]map[string]bool
}

func (s *stubRoleCapabilities) IsAdmin(name string) bool {
	return s.admin[name]
}

func (s *stubRoleCapabilities) CanPublish(name string) bool {
	return s.publish[name]
}

func (s *stubRoleCapabilities) CanMedia(name string) bool {
	return s.media[name]
}

func (s *stubRoleCapabilities) CanComment(name string) bool {
	return s.comments[name]
}

func (s *stubRoleCapabilities) CanManageType(name, postTypeSlug string) bool {
	return s.manage[name][postTypeSlug]
}

func newStubRoleCapabilities() *stubRoleCapabilities {
	return &stubRoleCapabilities{
		admin:    map[string]bool{"Admin": true},
		publish:  map[string]bool{"Admin": true, "Contributor": true},
		media:    map[string]bool{"Admin": true, "Contributor": true},
		comments: map[string]bool{"Admin": true, "Contributor": true, "Commentator": true},
		manage: map[string]map[string]bool{
			"Admin":       {"post": true, "page": true},
			"Contributor": {"post": true, "page": true},
		},
	}
}

func TestCapabilityResolver(t *testing.T) {
	resolve := appmiddleware.NewCapabilityResolver(newStubRoleCapabilities())

	tests := []struct {
		name       string
		principal  kernel.Principal
		capability string
		expected   bool
	}{
		{
			name:       "success - admin holds admin",
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			capability: "admin",
			expected:   true,
		},
		{
			name:       "success - contributor publishes",
			principal:  kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Contributor"}},
			capability: "content.publish",
			expected:   true,
		},
		{
			name:       "denied - commentator uploads media",
			principal:  kernel.Principal{ID: "3", Username: "cam", Roles: []string{"Commentator"}},
			capability: "media",
			expected:   false,
		},
		{
			name:       "success - contributor manages post",
			principal:  kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Contributor"}},
			capability: "content.manage:post",
			expected:   true,
		},
		{
			name:       "denied - commentator manages post",
			principal:  kernel.Principal{ID: "3", Username: "cam", Roles: []string{"Commentator"}},
			capability: "content.manage:post",
			expected:   false,
		},
		{
			name:       "denied - contributor holds admin",
			principal:  kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Contributor"}},
			capability: "admin",
			expected:   false,
		},
		{
			name:       "denied - unknown role",
			principal:  kernel.Principal{ID: "9", Username: "ghost", Roles: []string{"Editor"}},
			capability: "media",
			expected:   false,
		},
		{
			name:       "denied - unknown capability fails closed",
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			capability: "site.destroy",
			expected:   false,
		},
		{
			name:       "denied - manage without post type",
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			capability: "content.manage",
			expected:   false,
		},
		{
			name:       "denied - anonymous holds nothing",
			principal:  kernel.Principal{},
			capability: "comments",
			expected:   false,
		},
		{
			name:       "denied - empty role skipped",
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{""}},
			capability: "admin",
			expected:   false,
		},
		{
			name:       "success - any granting role suffices",
			principal:  kernel.Principal{ID: "4", Username: "mix", Roles: []string{"Commentator", "Contributor"}},
			capability: "content.publish",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolve(tt.principal, tt.capability))
		})
	}
}

func TestCapabilityResolverNilRegistry(t *testing.T) {
	resolve := appmiddleware.NewCapabilityResolver(nil)
	require.NotNil(t, resolve)

	assert.False(t, resolve(kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}}, "admin"))
}

func TestRequireCapability(t *testing.T) {
	resolve := appmiddleware.NewCapabilityResolver(newStubRoleCapabilities())

	tests := []struct {
		name       string
		principal  kernel.Principal
		present    bool
		capability string
		resolver   kernel.CapabilityResolver
		wantStatus int
		wantNext   bool
		wantCode   string
	}{
		{
			name:       "success - granted capability reaches next",
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			present:    true,
			capability: "admin",
			resolver:   resolve,
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name:       "denied - contributor lacks admin",
			principal:  kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Contributor"}},
			present:    true,
			capability: "admin",
			resolver:   resolve,
			wantStatus: http.StatusForbidden,
			wantNext:   false,
			wantCode:   "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:       "denied - anonymous without principal",
			present:    false,
			capability: "admin",
			resolver:   resolve,
			wantStatus: http.StatusForbidden,
			wantNext:   false,
			wantCode:   "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:       "denied - nil resolver fails closed",
			principal:  kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
			present:    true,
			capability: "admin",
			resolver:   nil,
			wantStatus: http.StatusForbidden,
			wantNext:   false,
			wantCode:   "INSUFFICIENT_PERMISSIONS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
			if tt.present {
				req = req.WithContext(kernel.ContextWithPrincipal(req.Context(), tt.principal))
			}

			rr := httptest.NewRecorder()
			appmiddleware.RequireCapability(tt.resolver, tt.capability)(next).ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, tt.wantNext, nextCalled)
			if tt.wantCode != "" {
				assert.Contains(t, rr.Body.String(), tt.wantCode)
			}
		})
	}
}

func TestCapabilityResolverConformance(t *testing.T) {
	resolve := appmiddleware.NewCapabilityResolver(role.NewService())

	kernelconformance.CheckAuthorizer(t, resolve, kernelconformance.AuthorizerFixture{
		Admin:     kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}},
		NonAdmin:  kernel.Principal{ID: "2", Username: "ed", Roles: []string{"Contributor"}},
		Anonymous: kernel.Principal{},
	})
}
