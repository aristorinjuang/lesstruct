package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/require"
)

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func TestPostTypeHandler_GetPostTypes(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().
		GetAll().
		Return([]posttype.PostType{
			{Slug: "post", Name: "Post", Description: "Blog posts", Supports: []string{"title", "content", "tags"}},
			{Slug: "page", Name: "Page", Description: "Static pages", Supports: []string{"title", "content"}},
			{Slug: "recipe", Name: "Recipe", Description: "Recipes", Supports: []string{"title", "content", "tags"}},
			{Slug: "portfolio", Name: "Portfolio", Description: "Portfolio items", Supports: []string{"title", "content", "tags"}},
		})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GetPostTypes() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check response structure
	if _, ok := response["data"]; !ok {
		t.Error("Response missing 'data' field")
	}

	if _, ok := response["meta"]; !ok {
		t.Error("Response missing 'meta' field")
	}

	// Check meta has timestamp
	meta, ok := response["meta"].(map[string]any)
	if !ok {
		t.Fatal("meta is not a map")
	}
	if _, ok := meta["timestamp"]; !ok {
		t.Error("meta missing 'timestamp' field")
	}

	// Check data contains post types
	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}

	if len(data) < 4 {
		t.Errorf("GetPostTypes() returned %d post types, want at least 4", len(data))
	}

	// Check first post type structure
	if len(data) > 0 {
		firstPostType, ok := data[0].(map[string]any)
		if !ok {
			t.Fatal("First post type is not a map")
		}

		requiredFields := []string{"name", "slug", "description", "supports"}
		for _, field := range requiredFields {
			if _, ok := firstPostType[field]; !ok {
				t.Errorf("Post type missing required field: %s", field)
			}
		}
	}
}

func TestPostTypeHandler_GetPostTypes_WithCustomPostTypes(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().
		GetAll().
		Return([]posttype.PostType{
			{
				Name:        "Portfolio",
				Slug:        "portfolio",
				Description: "Portfolio items",
				Supports:    []string{"title", "content", "tags"},
			},
		})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}

	if len(data) != 1 {
		t.Errorf("GetPostTypes() returned %d post types, want 1", len(data))
	}
}

func TestPostTypeHandler_GetPublicPostTypes(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().
		GetAll().
		Return([]posttype.PostType{
			{Slug: "post", Name: "Post", Description: "Blog posts", Supports: []string{"title", "content", "tags"}},
			{Slug: "menu-item", Name: "Menu Item", Description: "Restaurant menu item", Supports: []string{"title", "content"}, Fields: []customfield.FieldSchema{
				{Name: "Price", Slug: "price", Type: customfield.FieldTypeText, Required: true},
				{Name: "Available", Slug: "available", Type: customfield.FieldTypeCheckbox},
			}},
		})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPublicPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GetPublicPostTypes() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", ct)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}
	if len(data) != 2 {
		t.Errorf("GetPublicPostTypes() returned %d post types, want 2", len(data))
	}

	menuItem, ok := data[1].(map[string]any)
	if !ok {
		t.Fatal("second post type is not a map")
	}

	fields, ok := menuItem["fields"].([]any)
	if !ok {
		t.Fatal("fields is not an array")
	}
	if len(fields) != 2 {
		t.Errorf("menu-item has %d fields, want 2", len(fields))
	}

	if _, ok := response["meta"]; !ok {
		t.Error("Response missing 'meta' field")
	}
}

func TestPostTypeHandler_GetPublicPostTypes_Empty(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetAll().Return([]posttype.PostType{})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPublicPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GetPublicPostTypes() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}
	if len(data) != 0 {
		t.Errorf("GetPublicPostTypes() returned %d post types, want 0", len(data))
	}
}

func TestPostTypeHandler_ResponseHeaders(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().
		GetAll().
		Return([]posttype.PostType{
			{Slug: "post", Name: "Post", Description: "Blog posts", Supports: []string{"title", "content"}},
		})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", contentType)
	}
}

// TestPostTypeHandler_GetPostTypes_IncludesHidden verifies the authenticated
// admin endpoint returns hidden post types together with their hidden flag, so
// the admin panel can decide which surfaces to show.
func TestPostTypeHandler_GetPostTypes_IncludesHidden(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().
		GetAll().
		Return([]posttype.PostType{
			{Slug: "post", Name: "Post", Description: "Blog posts", Supports: []string{"title", "content", "tags"}, Hidden: true},
			{Slug: "page", Name: "Page", Description: "Static pages", Supports: []string{"title", "content"}},
		})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GetPostTypes() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}
	if len(data) != 2 {
		t.Fatalf("admin endpoint must include hidden types: got %d, want 2", len(data))
	}

	post, ok := data[0].(map[string]any)
	if !ok {
		t.Fatal("post type is not a map")
	}
	hidden, ok := post["hidden"].(bool)
	if !ok || !hidden {
		t.Error("hidden post type must carry hidden=true in the admin response")
	}
}

// TestPostTypeHandler_GetPublicPostTypes_ExcludesHidden verifies the public
// endpoint drops hidden post types entirely.
func TestPostTypeHandler_GetPublicPostTypes_ExcludesHidden(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().
		GetAll().
		Return([]posttype.PostType{
			{Slug: "post", Name: "Post", Description: "Blog posts", Supports: []string{"title", "content", "tags"}, Hidden: true},
			{Slug: "page", Name: "Page", Description: "Static pages", Supports: []string{"title", "content"}},
		})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/post_types", nil)
	w := httptest.NewRecorder()

	handler.GetPublicPostTypes(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GetPublicPostTypes() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}
	if len(data) != 1 {
		t.Fatalf("public endpoint must exclude hidden types: got %d, want 1", len(data))
	}

	page, ok := data[0].(map[string]any)
	if !ok {
		t.Fatal("post type is not a map")
	}
	if page["slug"] != "page" {
		t.Errorf("public endpoint returned %v, want page (post is hidden)", page["slug"])
	}
}

func TestPostTypeHandler_GetUserFieldsEndpoint(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetUserFields().Return([]customfield.FieldSchema{
		{Name: "Job Title", Slug: "job_title", Type: customfield.FieldTypeText},
		{Name: "Company", Slug: "company", Type: customfield.FieldTypeText},
	})
	service.EXPECT().GetUserSystemFields().Return([]customfield.FieldSchema{
		{Name: "Internal Rating", Slug: "internal_rating", Type: customfield.FieldTypeSelect, Options: []string{"bronze", "silver", "gold", "platinum"}},
	})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user_fields", nil)
	w := httptest.NewRecorder()

	handler.GetUserFieldsEndpoint(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GetUserFieldsEndpoint() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", ct)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["error"] != nil {
		t.Errorf("error = %v, want nil", response["error"])
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatal("data is not a map")
	}

	fields, ok := data["fields"].([]any)
	if !ok {
		t.Fatal("fields is not an array")
	}
	if len(fields) != 2 {
		t.Errorf("fields count = %d, want 2", len(fields))
	}

	systemFields, ok := data["systemFields"].([]any)
	if !ok {
		t.Fatal("systemFields is not an array")
	}
	if len(systemFields) != 1 {
		t.Errorf("systemFields count = %d, want 1", len(systemFields))
	}

	// Check field structure
	firstField, ok := fields[0].(map[string]any)
	if !ok {
		t.Fatal("first field is not a map")
	}
	if firstField["slug"] != "job_title" {
		t.Errorf("first field slug = %v, want job_title", firstField["slug"])
	}
}

func TestPostTypeHandler_GetUserFieldsEndpoint_Empty(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetUserFields().Return([]customfield.FieldSchema{})
	service.EXPECT().GetUserSystemFields().Return([]customfield.FieldSchema{})
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user_fields", nil)
	w := httptest.NewRecorder()

	handler.GetUserFieldsEndpoint(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GetUserFieldsEndpoint() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatal("data is not a map")
	}

	fields, ok := data["fields"].([]any)
	if !ok {
		t.Fatal("fields is not an array")
	}
	if len(fields) != 0 {
		t.Errorf("fields count = %d, want 0", len(fields))
	}

	systemFields, ok := data["systemFields"].([]any)
	if !ok {
		t.Fatal("systemFields is not an array")
	}
	if len(systemFields) != 0 {
		t.Errorf("systemFields count = %d, want 0", len(systemFields))
	}
}

// postTypesForTest returns the shared fixture post types used by the
// role-scoping tests.
func postTypesForTest() []posttype.PostType {
	return []posttype.PostType{
		{Slug: "post", Name: "Post", Description: "Blog posts", Supports: []string{"title", "content", "tags"}},
		{Slug: "page", Name: "Page", Description: "Static pages", Supports: []string{"title", "content"}},
		{Slug: "article", Name: "Article", Description: "Articles", Supports: []string{"title", "content", "tags"}},
		{Slug: "event", Name: "Event", Description: "Events", Supports: []string{"title", "content", "tags"}},
	}
}

func withRole(r *http.Request, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.RoleKey, role))
}

func decodeData(t *testing.T, w *httptest.ResponseRecorder) []any {
	t.Helper()
	var response map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	data, ok := response["data"].([]any)
	if !ok {
		t.Fatal("data is not an array")
	}
	return data
}

// TestPostTypeHandler_GetPostTypes_AdminSeesAllTypes verifies the Admin role is
// not scoped by its manage list.
func TestPostTypeHandler_GetPostTypes_AdminSeesAllTypes(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetAll().Return(postTypesForTest())

	roleService := roledomain.NewService()
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger, handlers.WithPostTypeRoleService(roleService))

	req := withRole(httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil), "Admin")
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	if got := len(decodeData(t, w)); got != 4 {
		t.Fatalf("GetPostTypes() for Admin returned %d types, want 4", got)
	}
}

// TestPostTypeHandler_GetPostTypes_RoleScopedToManageable verifies a restricted
// role only sees the post types in its manage list.
func TestPostTypeHandler_GetPostTypes_RoleScopedToManageable(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetAll().Return(postTypesForTest())

	roleService := roledomain.NewService()
	require.NoError(t, roleService.Register(roledomain.Role{Name: "Journalist", PostTypes: []string{"article"}, Publish: true, Media: true, Comments: true}))

	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger, handlers.WithPostTypeRoleService(roleService))

	req := withRole(httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil), "Journalist")
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	data := decodeData(t, w)
	if len(data) != 1 {
		t.Fatalf("GetPostTypes() for Journalist returned %d types, want 1", len(data))
	}
	first, ok := data[0].(map[string]any)
	if !ok {
		t.Fatal("post type is not a map")
	}
	if first["slug"] != "article" {
		t.Errorf("GetPostTypes() for Journalist returned %v, want article", first["slug"])
	}
}

// TestPostTypeHandler_GetPostTypes_NoRoleServiceScopesNothing verifies that
// without a role service the handler keeps the legacy behavior (all types for
// every caller).
func TestPostTypeHandler_GetPostTypes_NoRoleServiceScopesNothing(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetAll().Return(postTypesForTest())

	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := withRole(httptest.NewRequest(http.MethodGet, "/api/v1/post_types", nil), "Contributor")
	w := httptest.NewRecorder()

	handler.GetPostTypes(w, req)

	if got := len(decodeData(t, w)); got != 4 {
		t.Fatalf("GetPostTypes() without role service returned %d types, want 4", got)
	}
}

// TestPostTypeHandler_GetRoles returns the registered roles plus caller
// capabilities.
func TestPostTypeHandler_GetRoles(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	service.EXPECT().GetAll().Return(postTypesForTest())

	roleService := roledomain.NewService()
	require.NoError(t, roleService.Register(roledomain.Role{Name: "Journalist", PostTypes: []string{"article"}, Publish: true, Media: true, Comments: true}))

	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger, handlers.WithPostTypeRoleService(roleService))

	req := withRole(httptest.NewRequest(http.MethodGet, "/api/v1/roles", nil), "Journalist")
	w := httptest.NewRecorder()

	handler.GetRoles(w, req)

	var response map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatal("data is not a map")
	}

	roles, ok := data["roles"].([]any)
	if !ok {
		t.Fatal("roles is not an array")
	}
	// Admin, Contributor, Commentator, Journalist.
	if len(roles) != 4 {
		t.Fatalf("roles count = %d, want 4", len(roles))
	}

	me, ok := data["me"].(map[string]any)
	if !ok {
		t.Fatal("me is not a map")
	}
	if me["role"] != "Journalist" {
		t.Errorf("me.role = %v, want Journalist", me["role"])
	}
	postTypes, ok := me["postTypes"].([]any)
	if !ok {
		t.Fatal("me.postTypes is not an array")
	}
	if len(postTypes) != 1 || postTypes[0] != "article" {
		t.Errorf("me.postTypes = %v, want [article]", postTypes)
	}
	if me["publish"] != true || me["media"] != true || me["comments"] != true {
		t.Errorf("me capabilities wrong: %v", me)
	}
	if me["isAdmin"] != false {
		t.Errorf("me.isAdmin = %v, want false", me["isAdmin"])
	}
}

// TestPostTypeHandler_GetRoles_WithoutRoleService404 verifies fail-closed when
// the role service was not configured on the handler.
func TestPostTypeHandler_GetRoles_WithoutRoleService404(t *testing.T) {
	service := mocks.NewMockPostTypeServiceInterface(t)
	logger := util.NewLogger(&discardWriter{})
	handler := handlers.NewPostTypeHandler(service, true, logger)

	req := withRole(httptest.NewRequest(http.MethodGet, "/api/v1/roles", nil), "Admin")
	w := httptest.NewRecorder()

	handler.GetRoles(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GetRoles() status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
