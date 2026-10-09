package api

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	kernelseo "github.com/aristorinjuang/lesstruct/kernel/seo"
)

// translationGroupKey returns the key shared by every member of a content item's
// translation group: the declared translation-group id, or the item's own id when
// it is the primary (TranslationGroupID == nil). Used to group published items so
// the sitemap can emit hreflang alternates between translations.
func translationGroupKey(p PageRef) int {
	if p.TranslationGroupID != nil {
		return *p.TranslationGroupID
	}
	return p.ID
}

// sitemapAlternates builds the hreflang alternates for a content item from its
// translation group. members is the full set of published items sharing the item's
// group key (precomputed in buildSitemapEntries). It returns nil when the group
// has fewer than two published language variants — a page with no published
// translations needs no hreflang. Otherwise it returns every member (including the
// page itself, per Google's "list yourself" guidance), one per language, sorted by
// language for deterministic XML.
func sitemapAlternates(baseURL string, members []PageRef) []kernelseo.SitemapAlternate {
	if len(members) <= 1 {
		return nil
	}
	seen := make(map[string]bool, len(members))
	alts := make([]kernelseo.SitemapAlternate, 0, len(members))
	for _, m := range members {
		if m.Language == "" || m.Slug == "" || seen[m.Language] {
			continue
		}
		seen[m.Language] = true
		alts = append(alts, kernelseo.SitemapAlternate{
			Hreflang: m.Language,
			Href:     fmt.Sprintf("%s/%s", baseURL, m.Slug),
		})
	}
	if len(alts) <= 1 {
		return nil
	}
	slices.SortFunc(alts, func(a, b kernelseo.SitemapAlternate) int {
		return strings.Compare(a.Hreflang, b.Hreflang)
	})
	return alts
}

// discardLogger drops log lines. It backs the nil-logger fallback in
// NewSEOHandler so the handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// PageRef is the sitemap-relevant projection of a published content item:
// its identity for translation grouping, its slug and type for filtering,
// and its timestamps for freshness. Core projects content items into it so
// this package never imports the content domain.
type PageRef struct {
	ID                 int
	Slug               string
	PostType           string
	Language           string
	TranslationGroupID *int
	UpdatedAt          time.Time
}

// PublishedPages lists published content for sitemap generation. Core
// satisfies it with a closure over the content service (which cannot be
// imported here); tests stub it with a func literal.
type PublishedPages func(ctx context.Context, limit, offset int) ([]PageRef, error)

// SEOHandler serves sitemap and robots discovery surfaces.
type SEOHandler struct {
	pages    PublishedPages
	baseURL  string
	headless bool
	logger   kernel.Logger
}

// buildSitemapEntries is the single source of truth for what goes into the
// sitemap, shared by the JSON (/api/v1/sitemap) and XML (/sitemap.xml) handlers.
// It always includes the homepage, then every published item that has a public
// /<slug> page. media and comment are attachment/sub-entity post types with no
// public page, so they are excluded; every other published post type — post,
// page, and any custom type (tutorial, showcase, …) — is indexed. Each entry also
// carries its hreflang alternates (the page's other published translations), so a
// multilingual site declares its language variants to search engines.
func (h *SEOHandler) buildSitemapEntries(ctx context.Context) ([]kernelseo.SitemapEntry, error) {
	pages, err := h.pages(ctx, 1000, 0)
	if err != nil {
		return nil, err
	}

	if len(pages) == 1000 {
		h.logger.Info("Sitemap may be incomplete: hit 1000 item limit. Consider pagination.")
	}

	// Group published items by translation group so each entry can declare its
	// language variants (hreflang). Built in memory from the published set — no
	// extra queries, and draft translations are naturally excluded (they are not
	// in the published list, so they never appear as alternates).
	groups := make(map[int][]PageRef, len(pages))
	for _, p := range pages {
		key := translationGroupKey(p)
		groups[key] = append(groups[key], p)
	}

	entries := []kernelseo.SitemapEntry{kernelseo.NewHomepageEntry(h.baseURL)}
	for _, page := range pages {
		if page.Slug == "" || page.UpdatedAt.IsZero() {
			continue
		}
		if page.PostType == "media" || page.PostType == "comment" {
			continue
		}
		entry := kernelseo.ContentToSitemapEntry(h.baseURL, kernelseo.ContentItem{
			Slug:      page.Slug,
			UpdatedAt: page.UpdatedAt,
		}, page.PostType)
		entry.Alternates = sitemapAlternates(h.baseURL, groups[translationGroupKey(page)])
		entries = append(entries, entry)
	}
	return entries, nil
}

func (h *SEOHandler) GetSitemapData(w http.ResponseWriter, r *http.Request) {
	entries, err := h.buildSitemapEntries(r.Context())
	if err != nil {
		h.logger.Error("Failed to get published content: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to get sitemap data", nil)
		return
	}

	type alternate struct {
		Hreflang string `json:"hreflang"`
		Href     string `json:"href"`
	}
	type SitemapDataEntry struct {
		Loc        string      `json:"loc"`
		LastMod    string      `json:"lastmod"`
		ChangeFreq string      `json:"changefreq"`
		Priority   string      `json:"priority"`
		Alternates []alternate `json:"alternates,omitempty"`
	}

	out := make([]SitemapDataEntry, len(entries))
	for i, e := range entries {
		alts := make([]alternate, len(e.Alternates))
		for j, a := range e.Alternates {
			alts[j] = alternate{Hreflang: a.Hreflang, Href: a.Href}
		}
		out[i] = SitemapDataEntry{
			Loc:        e.Loc,
			LastMod:    e.LastMod,
			ChangeFreq: e.ChangeFreq,
			Priority:   e.Priority,
			Alternates: alts,
		}
	}

	sendSuccessResponse(w, http.StatusOK, out)
}

// GetSitemapXML serves the sitemaps.org XML document crawlers expect at
// /sitemap.xml (the path robots.txt advertises). The JSON shape served at
// /api/v1/sitemap is for programmatic callers; crawlers need XML. In headless
// mode there is no server-rendered content site, so every /<slug> URL would
// 404 — the XML sitemap is disabled (404) rather than advertising dead URLs.
func (h *SEOHandler) GetSitemapXML(w http.ResponseWriter, r *http.Request) {
	if h.headless {
		http.NotFound(w, r)
		return
	}

	entries, err := h.buildSitemapEntries(r.Context())
	if err != nil {
		h.logger.Error("Failed to get published content: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to get sitemap data", nil)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(kernelseo.RenderSitemapXML(entries))
}

func (h *SEOHandler) GetRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// In headless mode the content site is not served and sitemap.xml is
	// disabled — block crawling of the CMS itself entirely.
	robotsTxt := "User-agent: *\n"
	if h.headless {
		robotsTxt += "Disallow: /\n"
	} else {
		robotsTxt += "Allow: /\n"
		robotsTxt += "Disallow: /admin\n"
		robotsTxt += "\n"
		robotsTxt += "Sitemap: " + h.baseURL + "/sitemap.xml\n"
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(robotsTxt))
}

// NewSEOHandler constructs an SEOHandler over the given published-pages
// source. A nil logger degrades to a discard sink so the handler is safe to
// construct in any context.
func NewSEOHandler(
	pages PublishedPages,
	baseURL string,
	headless bool,
	logger kernel.Logger,
) *SEOHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &SEOHandler{
		pages:    pages,
		baseURL:  baseURL,
		headless: headless,
		logger:   logger,
	}
}
