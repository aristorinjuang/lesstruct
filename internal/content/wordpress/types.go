package wordpress

// author represents a WordPress user that created content.
type author struct {
	Login       string `xml:"http://wordpress.org/export/1.2/ author_login"`
	Email       string `xml:"http://wordpress.org/export/1.2/ author_email"`
	DisplayName string `xml:"http://wordpress.org/export/1.2/ author_display_name"`
}

// postMeta is a single WordPress custom field entry (<wp:postmeta>).
type postMeta struct {
	Key   string `xml:"http://wordpress.org/export/1.2/ meta_key"`
	Value string `xml:"http://wordpress.org/export/1.2/ meta_value"`
}

// itemCategory is a category or tag assigned to an item. The Domain attribute
// distinguishes between "category" and "post_tag".
type itemCategory struct {
	Domain   string `xml:"domain,attr"`
	NiceName string `xml:"nicename,attr"`
	Value    string `xml:",chardata"`
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
	AttachmentURL  string `xml:"http://wordpress.org/export/1.2/ attachment_url"`
	PostMeta       []postMeta     `xml:"http://wordpress.org/export/1.2/ postmeta"`
	Categories     []itemCategory `xml:"category"`
}

// ParsedAuthor is a normalized WordPress author ready for user resolution.
type ParsedAuthor struct {
	Login       string
	Email       string
	DisplayName string
}

// WXRDocument is the parsed, normalized representation of a WXR export.
// Only items whose post type is in the caller-supplied allowlist are retained;
// other post types (attachment, wp_navigation, wp_global_styles, revisions) are
// filtered out during parsing. Attachments are captured as a lookup table
// (attachment post ID → media URL) so that featured images can be resolved by
// downstream consumers.
type WXRDocument struct {
	SiteTitle  string
	SiteURL    string
	Authors    []ParsedAuthor
	Items      []ParsedItem
	Attachments map[int]string // attachment post ID → wp:attachment_url
}

// ParsedItem is a normalized WordPress item ready for conversion.
type ParsedItem struct {
	Title    string
	Content  string
	Slug     string
	Status   string // mapped to "published" or "draft"
	PostType string
	Tags     []string
	PubDate  string
	Creator  string // WordPress author login (<dc:creator>)
	Meta     map[string]string
}
