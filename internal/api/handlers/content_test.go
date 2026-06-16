package handlers_test

import (
	"bytes"
	"fmt"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

func TestContentHandler_CreateContent(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "successful content creation",
			requestBody: `{
				"title": "Test Content",
				"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Test content body\"}]}]}",
				"tags": ["test", "example"],
				"status": "draft"
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(&contentdomain.Content{
					ID:      1,
					UserID:  1,
					Title:   "Test Content",
					Slug:    "test-content",
					Content: "Test content body",
					Tags:    []string{"test", "example"},
					Status:  contentdomain.StatusDraft,
				}, nil)
			},
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				content, ok := data["content"].(map[string]any)
				if !ok {
					t.Errorf("expected content field")
					return
				}
				if content["title"] != "Test Content" {
					t.Errorf("expected title 'Test Content', got %v", content["title"])
				}
			},
		},
		{
			name:           "invalid request body",
			requestBody:    `invalid json`,
			setupService:   func(s *handlersmocks.MockContentServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_request" {
					t.Errorf("expected code 'invalid_request', got %v", err["code"])
				}
			},
		},
		{
			name: "validation error - empty title",
			requestBody: `{
				"title": "",
				"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Test content body\"}]}]}",
				"tags": [],
				"status": "draft"
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(nil, contentdomain.ErrInvalidTitle)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_title" {
					t.Errorf("expected code 'invalid_title', got %v", err["code"])
				}
			},
		},
		{
			name: "slug already exists",
			requestBody: `{
				"title": "Existing Title",
				"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Test content body\"}]}]}",
				"tags": [],
				"status": "draft"
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(nil, contentdomain.ErrSlugAlreadyExists)
			},
			expectedStatus: http.StatusConflict,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "slug_exists" {
					t.Errorf("expected code 'slug_exists', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.CreateContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_GenerateSlug(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:        "successful slug generation",
			requestBody: `{"title": "My Test Title"}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GenerateSlugFromTitle(mock.Anything, "My Test Title").Return("my-test-title", nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				if data["slug"] != "my-test-title" {
					t.Errorf("expected slug 'my-test-title', got %v", data["slug"])
				}
			},
		},
		{
			name:           "invalid request body",
			requestBody:    `invalid json`,
			setupService:   func(s *handlersmocks.MockContentServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_request" {
					t.Errorf("expected code 'invalid_request', got %v", err["code"])
				}
			},
		},
		{
			name:        "validation error - empty title",
			requestBody: `{"title": ""}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GenerateSlugFromTitle(mock.Anything, "").Return("", contentdomain.ErrInvalidTitle)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_title" {
					t.Errorf("expected code 'invalid_title', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			req := httptest.NewRequest(http.MethodPost, "/api/v1/content/slug", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.GenerateSlug(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_UpdateContent(t *testing.T) {
	tests := []struct {
		name           string
		contentID      string
		requestBody    string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful update of draft content",
			contentID: "1",
			requestBody: `{
				"title": "Updated Title",
				"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Updated content\"}]}]}",
				"tags": ["updated"],
				"status": "draft"
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Update(
					mock.Anything,
					1,
					1,
					mock.Anything,
					mock.AnythingOfType("content.UpdateContentRequest"),
				).Return(&contentdomain.Content{
					ID:      1,
					UserID:  1,
					Title:   "Updated Title",
					Slug:    "test-content",
					Content: "Updated content",
					Tags:    []string{"updated"},
					Status:  contentdomain.StatusDraft,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				content, ok := data["content"].(map[string]any)
				if !ok {
					t.Errorf("expected content field")
					return
				}
				if content["title"] != "Updated Title" {
					t.Errorf("expected title 'Updated Title', got %v", content["title"])
				}
				if content["status"] != "draft" {
					t.Errorf("expected status 'draft', got %v", content["status"])
				}
			},
		},
		{
			name:      "successful publish draft to published",
			contentID: "1",
			requestBody: `{
				"title": "Title",
				"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Content\"}]}]}",
				"tags": [],
				"status": "published"
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Update(
					mock.Anything,
					1,
					1,
					mock.Anything,
					mock.AnythingOfType("content.UpdateContentRequest"),
				).Return(&contentdomain.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "test-content",
					Content: "Content",
					Tags:    []string{},
					Status:  contentdomain.StatusPublished,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				content, ok := data["content"].(map[string]any)
				if !ok {
					t.Errorf("expected content field")
					return
				}
				if content["status"] != "published" {
					t.Errorf("expected status 'published', got %v", content["status"])
				}
			},
		},
		{
			name:      "successful unpublish published to draft",
			contentID: "1",
			requestBody: `{
				"title": "Title",
				"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Content\"}]}]}",
				"tags": [],
				"status": "draft"
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Update(
					mock.Anything,
					1,
					1,
					mock.Anything,
					mock.AnythingOfType("content.UpdateContentRequest"),
				).Return(&contentdomain.Content{
					ID:      1,
					UserID:  1,
					Title:   "Title",
					Slug:    "test-content",
					Content: "Content",
					Tags:    []string{},
					Status:  contentdomain.StatusDraft,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				content, ok := data["content"].(map[string]any)
				if !ok {
					t.Errorf("expected content field")
					return
				}
				if content["status"] != "draft" {
					t.Errorf("expected status 'draft', got %v", content["status"])
				}
			},
		},
		{
			name:           "invalid content id",
			contentID:      "invalid",
			requestBody:    `{"title": "Title", "content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Content\"}]}]}", "tags": [], "status": "draft"}`,
			setupService:   func(s *handlersmocks.MockContentServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_content_id" {
					t.Errorf("expected code 'invalid_content_id', got %v", err["code"])
				}
			},
		},
		{
			name:           "invalid request body",
			contentID:      "1",
			requestBody:    `invalid json`,
			setupService:   func(s *handlersmocks.MockContentServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_request" {
					t.Errorf("expected code 'invalid_request', got %v", err["code"])
				}
			},
		},
		{
			name:        "unauthorized - different user",
			contentID:   "1",
			requestBody: `{"title": "Title", "content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Content\"}]}]}", "tags": [], "status": "draft"}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Update(
					mock.Anything,
					1,
					1,
					mock.Anything,
					mock.AnythingOfType("content.UpdateContentRequest"),
				).Return(nil, contentdomain.ErrUnauthorized)
			},
			expectedStatus: http.StatusForbidden,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "forbidden" {
					t.Errorf("expected code 'forbidden', got %v", err["code"])
				}
			},
		},
		{
			name:        "content not found",
			contentID:   "999",
			requestBody: `{"title": "Title", "content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Content\"}]}]}", "tags": [], "status": "draft"}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Update(
					mock.Anything,
					999,
					1,
					mock.Anything,
					mock.AnythingOfType("content.UpdateContentRequest"),
				).Return(nil, contentdomain.ErrContentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "content_not_found" {
					t.Errorf("expected code 'content_not_found', got %v", err["code"])
				}
			},
		},
		{
			name:        "validation error - empty title",
			contentID:   "1",
			requestBody: `{"title": "", "content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Content\"}]}]}", "tags": [], "status": "draft"}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().Update(
					mock.Anything,
					1,
					1,
					mock.Anything,
					mock.AnythingOfType("content.UpdateContentRequest"),
				).Return(nil, contentdomain.ErrInvalidTitle)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_title" {
					t.Errorf("expected code 'invalid_title', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			r := chi.NewRouter()
			r.Put("/api/v1/content_items/{id}", func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), middleware.UserIDKey, "1")
				handler.UpdateContent(w, r.WithContext(ctx))
			})

			req := httptest.NewRequest(http.MethodPut, "/api/v1/content_items/"+tt.contentID, bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_GetPublishedContent(t *testing.T) {
	tests := []struct {
		name           string
		slug           string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "returns ogImage with absolute URL when content has image",
			slug: "test-post",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GetPublishedBySlug(
					mock.Anything,
					"test-post",
					"en",
				).Return(&contentdomain.Content{
					ID:             1,
					Title:          "Test Post",
					Slug:           "test-post",
					Content:        `{"type":"doc","content":[{"type":"paragraph"},{"type":"image","attrs":{"src":"/uploads/media/photo.webp"}}]}`,
					Status:         contentdomain.StatusPublished,
					AllowComments:  true,
					CreatedAt:      "2026-01-01T00:00:00Z",
					UpdatedAt:      "2026-01-01T00:00:00Z",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				ogImage, ok := data["ogImage"].(string)
				if !ok || ogImage == "" {
					t.Errorf("expected ogImage to be set, got %v", data["ogImage"])
					return
				}
				expectedURL := "http://localhost:3000/uploads/media/photo.webp"
				if ogImage != expectedURL {
					t.Errorf("expected ogImage %q, got %q", expectedURL, ogImage)
				}
			},
		},
		{
			name: "no ogImage when content has no images",
			slug: "text-only-post",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GetPublishedBySlug(
					mock.Anything,
					"text-only-post",
					"en",
				).Return(&contentdomain.Content{
					ID:             2,
					Title:          "Text Only Post",
					Slug:           "text-only-post",
					Content:        `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Just text"}]}]}`,
					Status:         contentdomain.StatusPublished,
					AllowComments:  false,
					CreatedAt:      "2026-01-01T00:00:00Z",
					UpdatedAt:      "2026-01-01T00:00:00Z",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				ogImage, ok := data["ogImage"]
				if ok && ogImage != "" {
					t.Errorf("expected no ogImage for text-only content, got %v", ogImage)
				}
			},
		},
		{
			name: "empty slug returns error",
			slug: "",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_slug" {
					t.Errorf("expected code 'invalid_slug', got %v", err["code"])
				}
			},
		},
		{
			name: "content not found returns 404",
			slug: "nonexistent",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GetPublishedBySlug(
					mock.Anything,
					"nonexistent",
					"en",
				).Return(nil, contentdomain.ErrContentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "content_not_found" {
					t.Errorf("expected code 'content_not_found', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(
				mockService,
				util.NewLogger(os.Stdout),
				"http://localhost:3000",
			)

			r := chi.NewRouter()
			r.Get("/api/v1/public/content_items/{slug}", handler.GetPublishedContent)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items/"+tt.slug, nil)

			w := httptest.NewRecorder()

			if tt.slug == "" {
				handler.GetPublishedContent(w, req)
			} else {
				r.ServeHTTP(w, req)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_ListContents(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:        "successful list contents",
			queryParams: "",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GetByUser(mock.Anything, 1, 100, 0).Return([]*contentdomain.Content{
					{
						ID:      1,
						UserID:  1,
						Title:   "First Content",
						Slug:    "first-content",
						Content: "First content body",
						Tags:    []string{"test"},
						Status:  contentdomain.StatusDraft,
					},
					{
						ID:      2,
						UserID:  1,
						Title:   "Second Content",
						Slug:    "second-content",
						Content: "Second content body",
						Tags:    []string{"example"},
						Status:  contentdomain.StatusPublished,
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be an array, got %T", resp["data"])
					return
				}
				if len(data) != 2 {
					t.Errorf("expected 2 contents, got %d", len(data))
				}
			},
		},
		{
			name:        "successful list contents with custom limit and offset",
			queryParams: "?limit=10&offset=5",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GetByUser(mock.Anything, 1, 10, 5).Return([]*contentdomain.Content{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				_, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be an array, got %T", resp["data"])
					return
				}
			},
		},
		{
			name:           "unauthorized - no user context",
			queryParams:    "",
			setupService:   func(s *handlersmocks.MockContentServiceInterface) {},
			expectedStatus: http.StatusUnauthorized,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "unauthorized" {
					t.Errorf("expected code 'unauthorized', got %v", err["code"])
				}
			},
		},
		{
			name:        "empty list",
			queryParams: "",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().GetByUser(mock.Anything, 1, 100, 0).Return([]*contentdomain.Content{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be an array, got %T", resp["data"])
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 contents, got %d", len(data))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items"+tt.queryParams, nil)
			req.Header.Set("Content-Type", "application/json")

			if tt.name != "unauthorized - no user context" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ListContents(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_ListContents_WithSearch(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:        "search routes to ListByFilters for non-admin",
			queryParams: "?search=hello",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().ListByFilters(
					mock.Anything,
					1,
					mock.MatchedBy(func(f contentdomain.ContentFilters) bool { return f.Search == "hello" }),
				).Return([]*contentdomain.Content{
					{
						ID:    1,
						Title: "Hello World",
						Slug:  "hello-world",
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be an array, got %T", resp["data"])
					return
				}
				if len(data) != 1 {
					t.Errorf("expected 1 content, got %d", len(data))
				}
			},
		},
		{
			name:        "search with post_type routes to ListByFilters",
			queryParams: "?search=hello&post_type=post",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().ListByFilters(
					mock.Anything,
					1,
					mock.AnythingOfType("content.ContentFilters"),
				).Return([]*contentdomain.Content{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be an array, got %T", resp["data"])
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 contents, got %d", len(data))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items"+tt.queryParams, nil)
			req.Header.Set("Content-Type", "application/json")

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.ListContents(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_SetSystemFields(t *testing.T) {
	tests := []struct {
		name           string
		contentID      string
		requestBody    string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful system fields update",
			contentID: "1",
			requestBody: `{
				"systemFields": {
					"featured": true,
					"priority": 5
				}
			}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SetSystemFields(
					mock.Anything,
					1,
					map[string]any{"featured": true, "priority": float64(5)},
				).Return(&contentdomain.Content{
					ID:      1,
					Title:   "Test Content",
					Slug:    "test-content",
					Content: "Test body",
					Status:  contentdomain.StatusPublished,
					CustomFields: map[string]any{
						"color":    "blue",
						"featured": true,
						"priority": float64(5),
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				content, ok := data["content"].(map[string]any)
				if !ok {
					t.Errorf("expected content field")
					return
				}
				if content["title"] != "Test Content" {
					t.Errorf("expected title 'Test Content', got %v", content["title"])
				}
			},
		},
		{
			name:        "invalid content id - non-numeric",
			contentID:   "abc",
			requestBody: `{"systemFields": {}}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_content_id" {
					t.Errorf("expected code 'invalid_content_id', got %v", err["code"])
				}
			},
		},
		{
			name:        "invalid request body",
			contentID:   "1",
			requestBody: `not json`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_request" {
					t.Errorf("expected code 'invalid_request', got %v", err["code"])
				}
			},
		},
		{
			name:        "content not found returns 404",
			contentID:   "999",
			requestBody: `{"systemFields": {"featured": true}}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SetSystemFields(
					mock.Anything,
					999,
					map[string]any{"featured": true},
				).Return(nil, contentdomain.ErrContentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "content_not_found" {
					t.Errorf("expected code 'content_not_found', got %v", err["code"])
				}
			},
		},
		{
			name:        "unknown system field key returns 400",
			contentID:   "1",
			requestBody: `{"systemFields": {"nonexistent": "value"}}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SetSystemFields(
					mock.Anything,
					1,
					map[string]any{"nonexistent": "value"},
				).Return(nil, fmt.Errorf("%w: nonexistent", contentdomain.ErrUnknownSystemFieldKey))
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "unknown_system_field_key" {
					t.Errorf("expected code 'unknown_system_field_key', got %v", err["code"])
				}
			},
		},
		{
			name:        "validation error returns 400",
			contentID:   "1",
			requestBody: `{"systemFields": {"priority": "not_a_number"}}`,
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SetSystemFields(
					mock.Anything,
					1,
					map[string]any{"priority": "not_a_number"},
				).Return(nil, fmt.Errorf("%w: Priority: invalid field value", contentdomain.ErrSystemFieldValidation))
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "system_field_validation" {
					t.Errorf("expected code 'system_field_validation', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			r := chi.NewRouter()
			r.Put("/api/admin/content/{id}/system-fields", handler.SetSystemFields)

			req := httptest.NewRequest(http.MethodPut, "/api/admin/content/"+tt.contentID+"/system-fields", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestContentHandler_SearchPublished(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		setupService   func(*handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:        "successful search with results",
			queryString: "?q=golang",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SearchPublished(
					mock.Anything,
					"golang",
					10,
				).Return([]*contentdomain.Content{
					{ID: 1, Title: "Golang Tutorial", Slug: "golang-tut", MetaDescription: "Learn golang", Status: contentdomain.StatusPublished, PostType: "post"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be array")
					return
				}
				if len(data) != 1 {
					t.Errorf("expected 1 result, got %d", len(data))
				}
			},
		},
		{
			name:           "empty query returns empty array",
			queryString:    "",
			setupService:   func(s *handlersmocks.MockContentServiceInterface) {},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be array")
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 results, got %d", len(data))
				}
			},
		},
		{
			name:        "no results returns empty array",
			queryString: "?q=nonexistent",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SearchPublished(
					mock.Anything,
					"nonexistent",
					10,
				).Return([]*contentdomain.Content{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be array")
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 results, got %d", len(data))
				}
			},
		},
		{
			name:        "custom limit parameter",
			queryString: "?q=test&limit=5",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SearchPublished(
					mock.Anything,
					"test",
					5,
				).Return([]*contentdomain.Content{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be array")
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 results, got %d", len(data))
				}
			},
		},
		{
			name:        "invalid limit defaults to 10",
			queryString: "?q=test&limit=abc",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SearchPublished(
					mock.Anything,
					"test",
					10,
				).Return([]*contentdomain.Content{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				_, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be array")
					return
				}
			},
		},
		{
			name:        "search result fields are correct",
			queryString: "?q=golang",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().SearchPublished(
					mock.Anything,
					"golang",
					10,
				).Return([]*contentdomain.Content{
					{Title: "Golang Basics", Slug: "golang-basics", MetaDescription: "Intro to Go"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data to be array")
					return
				}
				if len(data) != 1 {
					t.Errorf("expected 1 result, got %d", len(data))
					return
				}
				item, ok := data[0].(map[string]any)
				if !ok {
					t.Errorf("expected map item")
					return
				}
				if item["slug"] != "golang-basics" {
					t.Errorf("expected slug 'golang-basics', got %v", item["slug"])
				}
				if item["title"] != "Golang Basics" {
					t.Errorf("expected title 'Golang Basics', got %v", item["title"])
				}
				if item["metaDescription"] != "Intro to Go" {
					t.Errorf("expected metaDescription 'Intro to Go', got %v", item["metaDescription"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			r := chi.NewRouter()
			r.Get("/api/v1/public/search", handler.SearchPublished)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/public/search"+tt.queryString, nil)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}
