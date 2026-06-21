package seo

import (
	"encoding/xml"
	"fmt"
	"time"
)

const (
	changeFrequencyDaily  = "daily"
	changeFrequencyWeekly = "weekly"
	priorityHomepage      = "1.0"
	priorityPost          = "0.8"
	priorityPage          = "0.6"
	postTypePost          = "post"
)

// sitemapAlternate is the XML form of a single <xhtml:link rel="alternate"
// hreflang=".." href=".."/> child of a <url>. It declares another language
// variant of the same page so search engines can serve the right locale
// (https://developers.google.com/search/docs/specialty/international/localized-versions).
type sitemapAlternate struct {
	Rel      string `xml:"rel,attr"`
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

// sitemapURL is the XML representation of a single sitemap <url> element
// (https://www.sitemaps.org/protocol.html). It mirrors SitemapEntry but carries
// the xml tags the protocol requires. Alternates render as <xhtml:link> children
// (the xmlns:xhtml prefix is bound on the urlset root).
type sitemapURL struct {
	Loc        string             `xml:"loc"`
	LastMod    string             `xml:"lastmod,omitempty"`
	ChangeFreq string             `xml:"changefreq,omitempty"`
	Priority   string             `xml:"priority,omitempty"`
	Alternates []sitemapAlternate `xml:"xhtml:link,omitempty"`
}

// sitemapURLSet is the XML <urlset> document root for a sitemap. XHTML binds the
// xhtml: prefix used by the hreflang <xhtml:link> alternates.
type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLns   string       `xml:"xmlns,attr"`
	XHTML   string       `xml:"xmlns:xhtml,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type ContentItem struct {
	Slug      string
	UpdatedAt string
}

// SitemapAlternate declares one language variant of a page: a language code
// (hreflang, e.g. "en", "id") and the absolute URL of that variant (href).
type SitemapAlternate struct {
	Hreflang string
	Href     string
}

type SitemapEntry struct {
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
	// Alternates lists the page's other published language variants (and the page
	// itself) for hreflang. Empty for pages with no translations.
	Alternates []SitemapAlternate
}

// ContentToSitemapEntry builds a single sitemap entry. Every public content item
// is served at the site root by its slug (/<slug>) — see ContentPageHandler.ServeHTTP's
// default case — so the loc is always baseURL/<slug> regardless of post type. postType
// is still used to pick the changefreq/priority (posts update more often than pages).
func ContentToSitemapEntry(baseURL string, content ContentItem, postType string) SitemapEntry {
	return SitemapEntry{
		Loc:        fmt.Sprintf("%s/%s", baseURL, content.Slug),
		LastMod:    content.UpdatedAt,
		ChangeFreq: GetChangeFrequency(postType),
		Priority:   GetPriority(postType),
	}
}

func GetChangeFrequency(postType string) string {
	switch postType {
	case postTypePost:
		return changeFrequencyDaily
	default:
		return changeFrequencyWeekly
	}
}

func GetPriority(postType string) string {
	switch postType {
	case postTypePost:
		return priorityPost
	default:
		return priorityPage
	}
}

func NewHomepageEntry(baseURL string) SitemapEntry {
	return SitemapEntry{
		Loc:        fmt.Sprintf("%s/", baseURL),
		LastMod:    time.Now().Format(time.RFC3339),
		ChangeFreq: changeFrequencyDaily,
		Priority:   priorityHomepage,
	}
}

// RenderSitemapXML renders entries as a sitemaps.org XML <urlset> document. It is
// what GET /sitemap.xml serves to crawlers (the JSON shape from GetSitemapData is
// for programmatic callers; crawlers need XML). Each entry's Alternates render as
// <xhtml:link rel="alternate" hreflang=".." href=".."/> children so a multilingual
// site declares its language variants (hreflang). The XML declaration is prepended
// so the document starts with <?xml ...?>.
func RenderSitemapXML(entries []SitemapEntry) []byte {
	urls := make([]sitemapURL, len(entries))
	for i, e := range entries {
		alts := make([]sitemapAlternate, len(e.Alternates))
		for j, a := range e.Alternates {
			alts[j] = sitemapAlternate{Rel: "alternate", Hreflang: a.Hreflang, Href: a.Href}
		}
		urls[i] = sitemapURL{
			Loc:        e.Loc,
			LastMod:    e.LastMod,
			ChangeFreq: e.ChangeFreq,
			Priority:   e.Priority,
			Alternates: alts,
		}
	}
	doc := sitemapURLSet{
		XMLns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		XHTML: "http://www.w3.org/1999/xhtml",
		URLs:  urls,
	}
	body, _ := xml.MarshalIndent(doc, "", "  ")
	return append([]byte(xml.Header), body...)
}
