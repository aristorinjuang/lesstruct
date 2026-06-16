package wordpress

import "encoding/xml"

// rss is the root element of a WordPress eXtended RSS (WXR) export file.
// WXR namespaces (content:, excerpt:, dc:, wp:) are matched in the struct tags
// below via Go's "namespaceURI localname" syntax to avoid local-name collisions
// (e.g. wp:category at channel level vs the non-namespaced category at item level).
type rss struct {
	XMLName xml.Name `xml:"rss"`
	Channel channel  `xml:"channel"`
}

// channel holds the site metadata and all exported items.
type channel struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	BaseSiteURL string       `xml:"base_site_url"`
	BaseBlogURL string       `xml:"base_blog_url"`
	Authors     []author     `xml:"http://wordpress.org/export/1.2/ author"`
	Categories  []wpCategory `xml:"http://wordpress.org/export/1.2/ category"`
	Tags        []wpTag      `xml:"http://wordpress.org/export/1.2/ tag"`
	Items       []item       `xml:"item"`
}

// author represents a WordPress user that created content.
type author struct {
	Login       string `xml:"http://wordpress.org/export/1.2/ author_login"`
	Email       string `xml:"http://wordpress.org/export/1.2/ author_email"`
	DisplayName string `xml:"http://wordpress.org/export/1.2/ author_display_name"`
}

// wpCategory is a WordPress taxonomy category declared at channel level.
type wpCategory struct {
	TermID   int    `xml:"http://wordpress.org/export/1.2/ term_id"`
	NiceName string `xml:"http://wordpress.org/export/1.2/ category_nicename"`
	Parent   string `xml:"http://wordpress.org/export/1.2/ category_parent"`
	Name     string `xml:"http://wordpress.org/export/1.2/ cat_name"`
}

// wpTag is a WordPress taxonomy tag declared at channel level.
type wpTag struct {
	TermID int    `xml:"http://wordpress.org/export/1.2/ term_id"`
	Slug   string `xml:"http://wordpress.org/export/1.2/ tag_slug"`
	Name   string `xml:"http://wordpress.org/export/1.2/ tag_name"`
}

// item is a single WordPress post, page, attachment, or other post-type entry.
type item struct {
	Title          string `xml:"title"`
	Link           string `xml:"link"`
	PubDate        string `xml:"pubDate"`
	Creator        string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	ContentEncoded string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	ExcerptEncoded string `xml:"http://wordpress.org/export/1.2/excerpt/ post_excerpt"`
	PostID         int    `xml:"http://wordpress.org/export/1.2/ post_id"`
	PostDate       string `xml:"http://wordpress.org/export/1.2/ post_date"`
	PostDateGmt    string `xml:"http://wordpress.org/export/1.2/ post_date_gmt"`
	PostName       string `xml:"http://wordpress.org/export/1.2/ post_name"`
	Status         string `xml:"http://wordpress.org/export/1.2/ status"`
	PostParent     int    `xml:"http://wordpress.org/export/1.2/ post_parent"`
	PostType       string `xml:"http://wordpress.org/export/1.2/ post_type"`
	Categories     []itemCategory `xml:"category"`
}

// itemCategory is a category or tag assigned to an item. The Domain attribute
// distinguishes between "category" and "post_tag".
type itemCategory struct {
	Domain   string `xml:"domain,attr"`
	NiceName string `xml:"nicename,attr"`
	Value    string `xml:",chardata"`
}

// WXRDocument is the parsed, normalized representation of a WXR export.
// Only post and page items are retained; other post types (attachment,
// wp_navigation, wp_global_styles, revisions) are filtered out during parsing.
type WXRDocument struct {
	SiteTitle string
	SiteURL   string
	Items     []ParsedItem
}

// ParsedItem is a normalized WordPress item ready for conversion.
type ParsedItem struct {
	Title    string
	Content  string
	Slug     string
	Status   string // mapped to "published" or "draft"
	PostType string // "post" or "page"
	Tags     []string
	PubDate  string
}
