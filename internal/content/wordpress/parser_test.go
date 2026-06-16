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

			doc, err := wordpress.Parse(f)
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
			doc, err := wordpress.Parse(strings.NewReader(tt.xml))
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
			doc, err := wordpress.Parse(strings.NewReader(tt.xml))
			require.NoError(t, err)
			require.NotEmpty(t, doc.Items)
			assert.Equal(t, []string{"first", "test", "Test Category"}, doc.Items[0].Tags)
		})
	}
}
