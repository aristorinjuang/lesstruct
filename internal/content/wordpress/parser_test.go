package wordpress_test

import (
	"os"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SampleExport(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "success - real WXR sample",
			path: "../../../samples/wordpress-export.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.path)
			require.NoError(t, err)
			defer func() { _ = f.Close() }()

			doc, err := wordpress.Parse(f, map[string]bool{"post": true, "page": true})
			require.NoError(t, err)

			// Site metadata parsed from channel
			assert.Equal(t, "WordPress Test", doc.SiteTitle)
			assert.Equal(t, "http://www.wordpress.local", doc.SiteURL)

			// Only post and page items should remain (4 expected: Hello world!,
			// Sample Page, Privacy Policy, My First Post). Attachments, wp_navigation,
			// and wp_global_styles must be filtered out.
			var posts, pages []wordpress.ParsedItem
			for _, it := range doc.Items {
				switch it.PostType {
				case "post":
					posts = append(posts, it)
				case "page":
					pages = append(pages, it)
				}
			}
			assert.Len(t, posts, 2, "expected 2 posts")
			assert.Len(t, pages, 2, "expected 2 pages")
		})
	}
}

func TestParse_StatusMapping(t *testing.T) {
	tests := []struct {
		name        string
		xml         string
		wantStatus  string
		wantErr     bool
	}{
		{
			name: "success - published status",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>A</title>
<content:encoded><![CDATA[<p>x</p>]]></content:encoded>
<wp:post_name>a</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`,
			wantStatus: "published",
			wantErr:    false,
		},
		{
			name: "success - draft status",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>A</title>
<content:encoded><![CDATA[<p>x</p>]]></content:encoded>
<wp:post_name>a</wp:post_name>
<wp:status>draft</wp:status>
<wp:post_type>page</wp:post_type>
</item>
</channel>
</rss>`,
			wantStatus: "draft",
			wantErr:    false,
		},
		{
			name: "error - invalid XML",
			xml:  `<<<not xml`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.Parse(strings.NewReader(tt.xml), map[string]bool{"post": true, "page": true})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, doc.Items)
			assert.Equal(t, tt.wantStatus, doc.Items[0].Status)
		})
	}
}

func TestParse_TagsCollected(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "success - tags and categories from item",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>A</title>
<content:encoded><![CDATA[<p>x</p>]]></content:encoded>
<wp:post_name>a</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<category domain="post_tag" nicename="first"><![CDATA[first]]></category>
<category domain="post_tag" nicename="test"><![CDATA[test]]></category>
<category domain="category" nicename="test-category"><![CDATA[Test Category]]></category>
</item>
</channel>
</rss>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.Parse(strings.NewReader(tt.xml), map[string]bool{"post": true, "page": true})
			require.NoError(t, err)
			require.NotEmpty(t, doc.Items)
			assert.Equal(t, []string{"first", "test", "Test Category"}, doc.Items[0].Tags)
		})
	}
}

func TestParse_CustomPostType(t *testing.T) {
	tests := []struct {
		name         string
		xml          string
		allowedTypes map[string]bool
		wantItems    int
		wantPostType string
	}{
		{
			name: "success - custom post type imported when in allowlist",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Annual Ride</title>
<content:encoded><![CDATA[<p>Ride</p>]]></content:encoded>
<wp:post_name>annual-ride</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
</item>
</channel>
</rss>`,
			allowedTypes: map[string]bool{"post": true, "page": true, "event": true},
			wantItems:    1,
			wantPostType: "event",
		},
		{
			name: "filtered - custom post type dropped when not in allowlist",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Annual Ride</title>
<content:encoded><![CDATA[<p>Ride</p>]]></content:encoded>
<wp:post_name>annual-ride</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
</item>
</channel>
</rss>`,
			allowedTypes: map[string]bool{"post": true, "page": true},
			wantItems:    0,
			wantPostType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.Parse(strings.NewReader(tt.xml), tt.allowedTypes)
			require.NoError(t, err)
			assert.Len(t, doc.Items, tt.wantItems)
			if tt.wantItems > 0 {
				assert.Equal(t, tt.wantPostType, doc.Items[0].PostType)
			}
		})
	}
}

func TestParse_PostMeta(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		wantMeta map[string]string
	}{
		{
			name: "success - custom fields and ACF internals collected",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Annual Ride</title>
<content:encoded><![CDATA[<p>Ride</p>]]></content:encoded>
<wp:post_name>annual-ride</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
<wp:postmeta>
<wp:meta_key><![CDATA[start]]></wp:meta_key>
<wp:meta_value><![CDATA[2018-10-27 00:00:00]]></wp:meta_value>
</wp:postmeta>
<wp:postmeta>
<wp:meta_key><![CDATA[_start]]></wp:meta_key>
<wp:meta_value><![CDATA[field_5c569cea1f6b3]]></wp:meta_value>
</wp:postmeta>
<wp:postmeta>
<wp:meta_key><![CDATA[location]]></wp:meta_key>
<wp:meta_value><![CDATA[Pontianak]]></wp:meta_value>
</wp:postmeta>
<wp:postmeta>
<wp:meta_key><![CDATA[_thumbnail_id]]></wp:meta_key>
<wp:meta_value><![CDATA[2244]]></wp:meta_value>
</wp:postmeta>
</item>
</channel>
</rss>`,
			wantMeta: map[string]string{
				"start":         "2018-10-27 00:00:00",
				"_start":        "field_5c569cea1f6b3",
				"location":      "Pontianak",
				"_thumbnail_id": "2244",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.Parse(
				strings.NewReader(tt.xml),
				map[string]bool{"event": true},
			)
			require.NoError(t, err)
			require.Len(t, doc.Items, 1)
			assert.Equal(t, tt.wantMeta, doc.Items[0].Meta)
		})
	}
}

func TestParse_AttachmentsCaptured(t *testing.T) {
	tests := []struct {
		name            string
		xml             string
		wantAttachments map[int]string
		wantItems       int
	}{
		{
			name: "success - attachment URLs captured while items filtered",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Post</title>
<content:encoded><![CDATA[<p>Hi</p>]]></content:encoded>
<wp:post_name>post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<wp:postmeta>
<wp:meta_key><![CDATA[_thumbnail_id]]></wp:meta_key>
<wp:meta_value><![CDATA[100]]></wp:meta_value>
</wp:postmeta>
</item>
<item>
<title>Attachment</title>
<content:encoded><![CDATA[]]></content:encoded>
<wp:post_id>100</wp:post_id>
<wp:post_name>attachment</wp:post_name>
<wp:status>inherit</wp:status>
<wp:post_type>attachment</wp:post_type>
<wp:attachment_url><![CDATA[http://wp.local/img.jpg]]></wp:attachment_url>
</item>
</channel>
</rss>`,
			wantAttachments: map[int]string{100: "http://wp.local/img.jpg"},
			wantItems:       1,
		},
		{
			name: "success - attachment without URL is ignored",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
<title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Attachment</title>
<content:encoded><![CDATA[]]></content:encoded>
<wp:post_name>att</wp:post_name>
<wp:status>inherit</wp:status>
<wp:post_type>attachment</wp:post_type>
</item>
</channel>
</rss>`,
			wantAttachments: map[int]string{},
			wantItems:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.Parse(strings.NewReader(tt.xml), map[string]bool{"post": true})
			require.NoError(t, err)
			assert.Len(t, doc.Items, tt.wantItems)
			assert.Equal(t, tt.wantAttachments, doc.Attachments)
		})
	}
}
