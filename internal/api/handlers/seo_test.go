package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/mock"

	. "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
)

// sitemapLocs extracts the "loc" values from the JSON sitemap envelope's data
// array, asserting the array shape so callers can focus on values/counts.
func sitemapLocs(t *testing.T, body map[string]any) []string {
	t.Helper()
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatal("data field is not an array")
	}
	locs := make([]string, 0, len(data))
	for _, d := range data {
		entry, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if loc, ok := entry["loc"].(string); ok {
			locs = append(locs, loc)
		}
	}
	return locs
}

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
				locs := sitemapLocs(t, body)
				if len(locs) != 3 {
					t.Fatalf("expected 3 entries (homepage + post + page), got %d", len(locs))
				}
				// Content is served at the site root by slug (/<slug>), NOT under
				// /posts/<slug> — that path 404s. Assert both resolve to the root.
				if !slices.Contains(locs, "http://localhost:3000/my-post") {
					t.Errorf("expected post at root URL /my-post, got %v", locs)
				}
				if !slices.Contains(locs, "http://localhost:3000/about") {
					t.Errorf("expected page at root URL /about, got %v", locs)
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
				locs := sitemapLocs(t, body)
				if len(locs) != 2 {
					t.Fatalf("expected 2 entries (homepage + 1 valid post), got %d", len(locs))
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
				locs := sitemapLocs(t, body)
				if len(locs) != 1 {
					t.Fatalf("expected 1 entry (homepage only), got %d", len(locs))
				}
			},
		},
		{
			name: "includes custom post types (tutorial, showcase) at the root URL",
			contents: []*contentdomain.Content{
				{Slug: "custom-type", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "recipe"},
				{Slug: "valid-post", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				locs := sitemapLocs(t, body)
				if len(locs) != 3 {
					t.Fatalf("expected 3 entries (homepage + custom type + post), got %d", len(locs))
				}
				if !slices.Contains(locs, "http://localhost:3000/custom-type") {
					t.Errorf("expected custom post type at root URL, got %v", locs)
				}
			},
		},
		{
			name: "skips media and comment post types (no public page)",
			contents: []*contentdomain.Content{
				{Slug: "img-1", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "media"},
				{Slug: "c-1", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "comment"},
				{Slug: "valid-post", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]any) {
				locs := sitemapLocs(t, body)
				if len(locs) != 2 {
					t.Fatalf("expected 2 entries (homepage + post; media/comment excluded), got %d", len(locs))
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

func TestSEOHandler_GetSitemapXML(t *testing.T) {
	contents := []*contentdomain.Content{
		{Slug: "my-tutorial", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "tutorial"},
		{Slug: "my-post", UpdatedAt: "2026-04-18T10:00:00Z", PostType: "post"},
	}

	mockService := &MockContentServiceInterface{}
	mockService.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).Return(contents, nil)

	handler := &SEOHandler{
		contentService: mockService,
		baseURL:        "http://localhost:3000",
		logger:         util.NewLogger(os.Stdout),
	}

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	handler.GetSitemapXML(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Errorf("expected an xml content-type, got %s", ct)
	}

	body := w.Body.String()
	if !strings.HasPrefix(body, `<?xml`) {
		t.Errorf("expected body to start with the <?xml declaration, got: %q", body)
	}
	if !strings.Contains(body, "<urlset") {
		t.Error("expected an <urlset> root element")
	}
	// Custom post types must be indexed, at the root URL (the real routing), and
	// the sitemap must never emit /posts/<slug> (that path 404s).
	if !strings.Contains(body, "http://localhost:3000/my-tutorial") {
		t.Errorf("expected the tutorial URL at the root, body: %q", body)
	}
	if strings.Contains(body, "/posts/") {
		t.Errorf("sitemap must not emit /posts/ URLs, body: %q", body)
	}

	mockService.AssertExpectations(t)
}

// TestSEOHandler_GetSitemapXML_HreflangAlternates verifies that translated pages
// declare their language variants via <xhtml:link rel="alternate" hreflang="…">.
// The English "about" (id 1, primary) and Indonesian "tentang" (id 2, translation
// of 1) form one group, so each gets both alternates; the standalone "solo" post
// has no translations and gets none.
func TestSEOHandler_GetSitemapXML_HreflangAlternates(t *testing.T) {
	groupID := 1 // primary "about" (id 1); "tentang" joins its group.
	contents := []*contentdomain.Content{
		{ID: 1, Slug: "about", UpdatedAt: "2026-04-15T08:00:00Z", PostType: "page", Language: "en"},
		{ID: 2, Slug: "tentang", UpdatedAt: "2026-04-15T08:00:00Z", PostType: "page", Language: "id", TranslationGroupID: &groupID},
		{ID: 3, Slug: "solo", UpdatedAt: "2026-04-16T08:00:00Z", PostType: "post", Language: "en"},
	}

	mockService := &MockContentServiceInterface{}
	mockService.On("GetPublished", mock.Anything, mock.Anything, mock.Anything).Return(contents, nil)

	handler := &SEOHandler{
		contentService: mockService,
		baseURL:        "http://localhost:3000",
		logger:         util.NewLogger(os.Stdout),
	}

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	handler.GetSitemapXML(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	// The translated group declares both language variants on each member URL.
	if !strings.Contains(body, `<xhtml:link rel="alternate" hreflang="en" href="http://localhost:3000/about"`) {
		t.Errorf("expected an en hreflang alternate for the group, body: %q", body)
	}
	if !strings.Contains(body, `<xhtml:link rel="alternate" hreflang="id" href="http://localhost:3000/tentang"`) {
		t.Errorf("expected an id hreflang alternate for the group, body: %q", body)
	}
	// The standalone post has no translations → it appears but gets no hreflang.
	if !strings.Contains(body, "<loc>http://localhost:3000/solo</loc>") {
		t.Errorf("expected the standalone post in the sitemap, body: %q", body)
	}
	if strings.Contains(body, `hreflang="en" href="http://localhost:3000/solo"`) {
		t.Errorf("standalone post must not get an hreflang alternate, body: %q", body)
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
