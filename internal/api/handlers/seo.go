package handlers

import (
	"net/http"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

type SEOHandler struct {
	contentService ContentServiceInterface
	baseURL        string
	logger         *util.Logger
}

func (h *SEOHandler) GetSitemapData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	contents, err := h.contentService.GetPublished(ctx, 1000, 0)
	if err != nil {
		h.logger.Error("Failed to get published content: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to get sitemap data", nil)
		return
	}

	if len(contents) == 1000 {
		h.logger.Info("Sitemap may be incomplete: hit 1000 item limit. Consider pagination.")
	}

	homeEntry := seo.NewHomepageEntry(h.baseURL)

	type SitemapDataEntry struct {
		Loc        string `json:"loc"`
		LastMod    string `json:"lastmod"`
		ChangeFreq string `json:"changefreq"`
		Priority   string `json:"priority"`
	}

	entries := []SitemapDataEntry{
		{
			Loc:        homeEntry.Loc,
			LastMod:    homeEntry.LastMod,
			ChangeFreq: homeEntry.ChangeFreq,
			Priority:   homeEntry.Priority,
		},
	}

	for _, content := range contents {
		if content.Slug == "" || content.UpdatedAt == "" {
			continue
		}
		if content.PostType != "post" && content.PostType != "page" {
			continue
		}

		item := seo.ContentItem{
			Slug:      content.Slug,
			UpdatedAt: content.UpdatedAt,
		}

		entry := seo.ContentToSitemapEntry(h.baseURL, item, content.PostType)
		entries = append(entries, SitemapDataEntry{
			Loc:        entry.Loc,
			LastMod:    entry.LastMod,
			ChangeFreq: entry.ChangeFreq,
			Priority:   entry.Priority,
		})
	}

	sendSuccessResponse(w, http.StatusOK, entries)
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
