package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	seoapi "github.com/aristorinjuang/lesstruct/modules/seo/api"
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

func staticPages(pages []seoapi.PageRef, err error) seoapi.PublishedPages {
	return func(context.Context, int, int) ([]seoapi.PageRef, error) {
		return pages, err
	}
}

func newHandler(pages seoapi.PublishedPages, headless bool) *seoapi.SEOHandler {
	return seoapi.NewSEOHandler(pages, "http://localhost:3000", headless, nil)
}

func TestSEOHandler_GetSitemapData(t *testing.T) {
	tests := []struct {
		name           string
		pages          []seoapi.PageRef
		pagesErr       error
		expectedStatus int
		validateBody   func(t *testing.T, body map[string]any)
	}{
		{
			name:           "returns sitemap data with homepage only",
			pages:          []seoapi.PageRef{},
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
			pages: []seoapi.PageRef{
				{Slug: "my-post", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "post"},
				{Slug: "about", UpdatedAt: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC), PostType: "page"},
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
			name:     "returns error when PublishedPages fails",
			pages:    nil,
			pagesErr: errors.New("database connection failed"),
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
			pages: []seoapi.PageRef{
				{Slug: "", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "post"},
				{Slug: "valid-post", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "post"},
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
			pages: []seoapi.PageRef{
				{Slug: "my-post", UpdatedAt: time.Time{}, PostType: "post"},
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
			pages: []seoapi.PageRef{
				{Slug: "custom-type", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "recipe"},
				{Slug: "valid-post", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "post"},
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
			pages: []seoapi.PageRef{
				{Slug: "img-1", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "media"},
				{Slug: "c-1", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "comment"},
				{Slug: "valid-post", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "post"},
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
			handler := newHandler(staticPages(tt.pages, tt.pagesErr), false)

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
		})
	}
}

func TestSEOHandler_GetSitemapXML(t *testing.T) {
	pages := []seoapi.PageRef{
		{Slug: "my-tutorial", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "tutorial"},
		{Slug: "my-post", UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), PostType: "post"},
	}

	handler := newHandler(staticPages(pages, nil), false)

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
}

// TestSEOHandler_GetSitemapXML_HreflangAlternates verifies that translated pages
// declare their language variants via <xhtml:link rel="alternate" hreflang="…">.
// The English "about" (id 1, primary) and Indonesian "tentang" (id 2, translation
// of 1) form one group, so each gets both alternates; the standalone "solo" post
// has no translations and gets none.
func TestSEOHandler_GetSitemapXML_HreflangAlternates(t *testing.T) {
	groupID := 1 // primary "about" (id 1); "tentang" joins its group.
	pages := []seoapi.PageRef{
		{ID: 1, Slug: "about", UpdatedAt: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC), PostType: "page", Language: "en"},
		{ID: 2, Slug: "tentang", UpdatedAt: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC), PostType: "page", Language: "id", TranslationGroupID: &groupID},
		{ID: 3, Slug: "solo", UpdatedAt: time.Date(2026, 4, 16, 8, 0, 0, 0, time.UTC), PostType: "post", Language: "en"},
	}

	handler := newHandler(staticPages(pages, nil), false)

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
	handler := newHandler(staticPages(nil, nil), false)

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
	handler := seoapi.NewSEOHandler(staticPages([]seoapi.PageRef{}, nil), "http://localhost:3000/", false, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap", nil)
	w := httptest.NewRecorder()
	handler.GetSitemapData(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got := sitemapLocs(t, body); len(got) != 1 || got[0] != "http://localhost:3000/" {
		t.Errorf("expected trimmed homepage URL, got %v", got)
	}
}

func TestSEOHandler_HeadlessMode(t *testing.T) {
	t.Run("robots.txt disallows everything in headless mode", func(t *testing.T) {
		handler := newHandler(staticPages(nil, nil), true)

		req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
		w := httptest.NewRecorder()

		handler.GetRobotsTxt(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Disallow: /") {
			t.Errorf("expected 'Disallow: /' in headless robots.txt, got: %q", body)
		}
		if strings.Contains(body, "Sitemap:") {
			t.Errorf("expected no sitemap reference in headless robots.txt, got: %q", body)
		}
	})

	t.Run("sitemap.xml 404s in headless mode", func(t *testing.T) {
		called := false
		pages := seoapi.PublishedPages(func(context.Context, int, int) ([]seoapi.PageRef, error) {
			called = true
			return nil, nil
		})
		handler := newHandler(pages, true)

		req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
		w := httptest.NewRecorder()
		handler.GetSitemapXML(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
		// The published source must not be queried in headless mode.
		if called {
			t.Error("expected PublishedPages not to be called in headless mode")
		}
	})
}
