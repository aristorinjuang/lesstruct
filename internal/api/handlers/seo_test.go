package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/mock"

	. "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
)

func TestSEOHandler_GetSitemapData(t *testing.T) {
	tests := []struct {
		name           string
		contents       []*contentdomain.Content
		contentsErr    error
		expectedStatus int
		validateBody   func(t *testing.T, body map[string]any)
	}{
		{
			name:           "returns sitemap data with homepage only",
			contents:       []*contentdomain.Content{},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				data, ok := body["data"].([]any)
				if !ok {
					t.Fatal("data field is not an array")
				}
				if len(data) != 1 {
					t.Fatalf("expected 1 entry (homepage), got %d", len(data))
				}
				entry := data[0].(map[string]any)
				if entry["loc"] != "http://localhost:3000/" {
					t.Errorf("expected homepage URL, got %v", entry["loc"])
				}
				if entry["priority"] != "1.0" {
					t.Errorf("expected priority 1.0, got %v", entry["priority"])
				}
			},
		},
		{
			name: "returns sitemap data with posts and pages",
			contents: []*contentdomain.Content{
				{Slug: "my-post", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
				{Slug: "about", UpdatedAt: "2026-04-15T08:00:00Z", PostType: "page"},
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				data, ok := body["data"].([]any)
				if !ok {
					t.Fatal("data field is not an array")
				}
				if len(data) != 3 {
					t.Fatalf("expected 3 entries (homepage + post + page), got %d", len(data))
				}
			},
		},
		{
			name:        "returns error when GetPublished fails",
			contents:    nil,
			contentsErr: errors.New("database connection failed"),
			validateBody: func(t *testing.T, body map[string]any) {
				errField, ok := body["error"]
				if !ok || errField == nil {
					t.Fatal("expected error field in response")
				}
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "skips content with empty slug",
			contents: []*contentdomain.Content{
				{Slug: "", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
				{Slug: "valid-post", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				data, ok := body["data"].([]any)
				if !ok {
					t.Fatal("data field is not an array")
				}
				if len(data) != 2 {
					t.Fatalf("expected 2 entries (homepage + 1 valid post), got %d", len(data))
				}
			},
		},
		{
			name: "skips content with empty UpdatedAt",
			contents: []*contentdomain.Content{
				{Slug: "my-post", UpdatedAt: "", PostType: "post"},
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				data, ok := body["data"].([]any)
				if !ok {
					t.Fatal("data field is not an array")
				}
				if len(data) != 1 {
					t.Fatalf("expected 1 entry (homepage only), got %d", len(data))
				}
			},
		},
		{
			name: "skips content with unknown PostType",
			contents: []*contentdomain.Content{
				{Slug: "custom-type", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "recipe"},
				{Slug: "valid-post", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				data, ok := body["data"].([]any)
				if !ok {
					t.Fatal("data field is not an array")
				}
				if len(data) != 2 {
					t.Fatalf("expected 2 entries (homepage + 1 valid post), got %d", len(data))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockContentServiceInterface{}
			mockService.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).Return(tt.contents, tt.contentsErr)

			handler := &SEOHandler{
				contentService: mockService,
				baseURL:        "http://localhost:3000",
				logger:         util.NewLogger(os.Stdout),
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap", nil)
			w := httptest.NewRecorder()

			handler.GetSitemapData(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var body map[string]any
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.validateBody != nil {
				tt.validateBody(t, body)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestSEOHandler_GetRobotsTxt(t *testing.T) {
	handler := &SEOHandler{
		baseURL: "http://localhost:3000",
		logger:  util.NewLogger(os.Stdout),
	}

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()

	handler.GetRobotsTxt(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected content-type to contain text/plain, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Error("expected body to contain 'User-agent: *'")
	}
	if !strings.Contains(body, "Allow: /") {
		t.Error("expected body to contain 'Allow: /'")
	}
	if !strings.Contains(body, "Disallow: /admin") {
		t.Error("expected body to contain 'Disallow: /admin'")
	}
	if !strings.Contains(body, "Sitemap: http://localhost:3000/sitemap.xml") {
		t.Error("expected body to contain sitemap reference")
	}
}

func TestNewSEOHandler_TrimsTrailingSlash(t *testing.T) {
	handler := NewSEOHandler(
		&MockContentServiceInterface{},
		"http://localhost:3000/",
		util.NewLogger(os.Stdout),
	)
	if handler.baseURL != "http://localhost:3000" {
		t.Errorf("expected trailing slash trimmed, got %q", handler.baseURL)
	}
}
