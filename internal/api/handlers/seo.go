package handlers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

type SEOHandler struct {
	contentService ContentServiceInterface
	baseURL        string
	logger         *util.Logger
}

// buildSitemapEntries is the single source of truth for what goes into the
// sitemap, shared by the JSON (/api/v1/sitemap) and XML (/sitemap.xml) handlers.
// It always includes the homepage, then every published item that has a public
// /<slug> page. media and comment are attachment/sub-entity post types with no
// public page, so they are excluded; every other published post type — post,
// page, and any custom type (tutorial, showcase, …) — is indexed.
// buildSitemapEntries is the single source of truth for what goes into the
// sitemap, shared by the JSON (/api/v1/sitemap) and XML (/sitemap.xml) handlers.
// It always includes the homepage, then every published item that has a public
// /<slug> page. media and comment are attachment/sub-entity post types with no
// public page, so they are excluded; every other published post type — post,
// page, and any custom type (tutorial, showcase, …) — is indexed. Each entry also
// carries its hreflang alternates (the page's other published translations), so a
// multilingual site declares its language variants to search engines.
func (h *SEOHandler) buildSitemapEntries(ctx context.Context) ([]seo.SitemapEntry, error) {
	contents, err := h.contentService.GetPublished(ctx, 1000, 0)
	if err != nil {
		return nil, err
	}

	if len(contents) == 1000 {
		h.logger.Info("Sitemap may be incomplete: hit 1000 item limit. Consider pagination.")
	}

	// Group published items by translation group so each entry can declare its
	// language variants (hreflang). Built in memory from the published set — no
	// extra queries, and draft translations are naturally excluded (they are not
	// in the published list, so they never appear as alternates).
	groups := make(map[int][]*contentdomain.Content, len(contents))
	for _, c := range contents {
		key := translationGroupKey(c)
		groups[key] = append(groups[key], c)
	}

	entries := []seo.SitemapEntry{seo.NewHomepageEntry(h.baseURL)}
	for _, content := range contents {
		if content.Slug == "" || content.UpdatedAt == "" {
			continue
		}
		if content.PostType == "media" || content.PostType == "comment" {
			continue
		}
		entry := seo.ContentToSitemapEntry(h.baseURL, seo.ContentItem{
			Slug:      content.Slug,
			UpdatedAt: content.UpdatedAt,
		}, content.PostType)
		entry.Alternates = sitemapAlternates(h.baseURL, groups[translationGroupKey(content)])
		entries = append(entries, entry)
	}
	return entries, nil
}

// translationGroupKey returns the key shared by every member of a content item's
// translation group: the declared translation-group id, or the item's own id when
// it is the primary (TranslationGroupID == nil). Used to group published items so
// the sitemap can emit hreflang alternates between translations.
func translationGroupKey(c *contentdomain.Content) int {
	if c.TranslationGroupID != nil {
		return *c.TranslationGroupID
	}
	return c.ID
}

// sitemapAlternates builds the hreflang alternates for a content item from its
// translation group. members is the full set of published items sharing the item's
// group key (precomputed in buildSitemapEntries). It returns nil when the group
// has fewer than two published language variants — a page with no published
// translations needs no hreflang. Otherwise it returns every member (including the
// page itself, per Google's "list yourself" guidance), one per language, sorted by
// language for deterministic XML.
func sitemapAlternates(baseURL string, members []*contentdomain.Content) []seo.SitemapAlternate {
	if len(members) <= 1 {
		return nil
	}
	seen := make(map[string]bool, len(members))
	alts := make([]seo.SitemapAlternate, 0, len(members))
	for _, m := range members {
		if m.Language == "" || m.Slug == "" || seen[m.Language] {
			continue
		}
		seen[m.Language] = true
		alts = append(alts, seo.SitemapAlternate{
			Hreflang: m.Language,
			Href:     fmt.Sprintf("%s/%s", baseURL, m.Slug),
		})
	}
	if len(alts) <= 1 {
		return nil
	}
	slices.SortFunc(alts, func(a, b seo.SitemapAlternate) int {
		return strings.Compare(a.Hreflang, b.Hreflang)
	})
	return alts
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
// /api/v1/sitemap is for programmatic callers; crawlers need XML.
func (h *SEOHandler) GetSitemapXML(w http.ResponseWriter, r *http.Request) {
	entries, err := h.buildSitemapEntries(r.Context())
	if err != nil {
		h.logger.Error("Failed to get published content: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to get sitemap data", nil)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(seo.RenderSitemapXML(entries))
}

func (h *SEOHandler) GetRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	robotsTxt := "User-agent: *\n"
	robotsTxt += "Allow: /\n"
	robotsTxt += "Disallow: /admin\n"
	robotsTxt += "\n"
	robotsTxt += "Sitemap: " + h.baseURL + "/sitemap.xml\n"

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(robotsTxt))
}

func NewSEOHandler(
	contentService ContentServiceInterface,
	baseURL string,
	logger *util.Logger,
) *SEOHandler {
	baseURL = strings.TrimRight(baseURL, "/")
	return &SEOHandler{
		contentService: contentService,
		baseURL:        baseURL,
		logger:         logger,
	}
}
