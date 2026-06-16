package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/util"
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
	handler := handlers.NewPostTypeHandler(service, logger)

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
	handler := handlers.NewPostTypeHandler(service, logger)

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
	handler := handlers.NewPostTypeHandler(service, logger)

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
	handler := handlers.NewPostTypeHandler(service, logger)

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
	handler := handlers.NewPostTypeHandler(service, logger)

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
	handler := handlers.NewPostTypeHandler(service, logger)

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
	handler := handlers.NewPostTypeHandler(service, logger)

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
