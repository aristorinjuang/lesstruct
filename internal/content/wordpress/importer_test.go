package wordpress_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeContentCreator is a test double for the content service. It records every
// create request and can be configured to fail on a specific call index.
type fakeContentCreator struct {
	created  []contentdomain.CreateContentRequest
	userIDs  []int
	failOn   int // index that should return ErrSlugAlreadyExists; -1 = never
	callNext int
}

func (f *fakeContentCreator) Create(_ context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error) {
	idx := f.callNext
	f.callNext++
	f.created = append(f.created, req)
	f.userIDs = append(f.userIDs, userID)
	if f.failOn >= 0 && idx == f.failOn {
		return nil, fmt.Errorf("%w: dup", contentdomain.ErrSlugAlreadyExists)
	}
	return &contentdomain.Content{ID: idx + 1}, nil
}

// fakeUserResolver is a test double for the userResolver interface. It returns a
// sequential ID for each unique call and can be configured to fail for a
// specific login.
type fakeUserResolver struct {
	calls     []string
	counter   int
	failLogin string
}

func (f *fakeUserResolver) ResolveOrCreate(_ context.Context, login, _, _ string) (int, bool, error) {
	f.calls = append(f.calls, login)
	if login == f.failLogin {
		return 0, false, fmt.Errorf("fake resolver: failing for %q", login)
	}
	f.counter++
	return f.counter, true, nil
}

// stubMediaService satisfies the importer's mediaService interface. It never
// succeeds, which is fine because the sample host is unreachable and downloads
// fail at the HTTP layer before reaching GenerateFromBytes.
type stubMediaService struct{}

func (stubMediaService) GenerateFromBytes(_ context.Context, _ []byte, _ int, _, _ string) (*mediadomain.Media, error) {
	return nil, fmt.Errorf("stub: no media service in test")
}

func newTestImporter(creator *fakeContentCreator, resolver *fakeUserResolver) *wordpress.Importer {
	return wordpress.NewImporter(
		creator,
		wordpress.NewMediaDownloader(nil, stubMediaService{}),
		resolver,
		nil,
		"en",
		nil,
	)
}

func newTestImporterWithPostTypes(
	creator *fakeContentCreator,
	resolver *fakeUserResolver,
	postTypes *fakePostTypeLister,
) *wordpress.Importer {
	return wordpress.NewImporter(
		creator,
		wordpress.NewMediaDownloader(nil, stubMediaService{}),
		resolver,
		postTypes,
		"en",
		nil,
	)
}

type fakePostTypeLister struct {
	postTypes map[string][]customfield.FieldSchema
}

func (f *fakePostTypeLister) GetAll() []posttype.PostType {
	result := make([]posttype.PostType, 0, len(f.postTypes))
	for slug := range f.postTypes {
		result = append(result, posttype.PostType{Slug: slug})
	}
	return result
}

func (f *fakePostTypeLister) GetFieldsByPostType(slug string) ([]customfield.FieldSchema, error) {
	fields, ok := f.postTypes[slug]
	if !ok {
		return nil, fmt.Errorf("post type %q not found", slug)
	}
	return fields, nil
}

func TestImporter_RealSample(t *testing.T) {
	tests := []struct {
		name              string
		creator           *fakeContentCreator
		resolver          *fakeUserResolver
		wantImported      int
		wantSkipped       int
		wantUsersImported int
	}{
		{
			name:              "success - all items imported",
			creator:           &fakeContentCreator{failOn: -1},
			resolver:          &fakeUserResolver{},
			wantImported:      4,
			wantSkipped:       0,
			wantUsersImported: 1,
		},
		{
			name:              "success - one slug collision skipped",
			creator:           &fakeContentCreator{failOn: 0},
			resolver:          &fakeUserResolver{},
			wantImported:      3,
			wantSkipped:       1,
			wantUsersImported: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open("../../../samples/wordpress-export.xml")
			require.NoError(t, err)
			defer func() { _ = f.Close() }()

			importer := newTestImporter(tt.creator, tt.resolver)
			result, err := importer.Import(context.Background(), f, 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantImported, result.Imported)
			assert.Equal(t, tt.wantSkipped, result.Skipped)
			assert.Equal(t, tt.wantUsersImported, result.UsersImported)
			require.Len(t, tt.creator.created, 4)
			assert.Equal(t, "post", tt.creator.created[0].PostType)
			assert.Equal(t, "page", tt.creator.created[1].PostType)
		})
	}
}

func TestImporter_SkipMedia(t *testing.T) {
	f, err := os.Open("../../../samples/wordpress-export.xml")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	creator := &fakeContentCreator{failOn: -1}
	importer := newTestImporter(creator, &fakeUserResolver{})
	result, err := importer.Import(context.Background(), f, 1, wordpress.ImportOptions{SkipMedia: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, result.Imported)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 1, result.UsersImported)
	require.Len(t, creator.created, 4)
	// With SkipMedia=true, no download attempts are made, but content still
	// imports with original WordPress image URLs intact (not remapped).
}

func TestImporter_InvalidXML(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "error - malformed input fails fast"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importer := newTestImporter(&fakeContentCreator{failOn: -1}, &fakeUserResolver{})
			result, err := importer.Import(context.Background(), strings.NewReader("<<<broken"), 1, wordpress.ImportOptions{}, nil)
			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}

func TestImporter_MapsStatusAndTags(t *testing.T) {
	tests := []struct {
		name       string
		xml        string
		wantStatus contentdomain.Status
		wantTags   []string
		wantType   string
	}{
		{
			name: "success - published post with tags",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Tagged Post</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>tagged-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<category domain="post_tag" nicename="alpha"><![CDATA[alpha]]></category>
</item>
</channel>
</rss>`,
			wantStatus: contentdomain.StatusPublished,
			wantTags:   []string{"alpha"},
			wantType:   "post",
		},
		{
			name: "success - draft page",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Hidden Page</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Secret</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>hidden-page</wp:post_name>
<wp:status>draft</wp:status>
<wp:post_type>page</wp:post_type>
</item>
</channel>
</rss>`,
			wantStatus: contentdomain.StatusDraft,
			wantTags:   []string{},
			wantType:   "page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &fakeContentCreator{failOn: -1}
			importer := newTestImporter(creator, &fakeUserResolver{})
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Imported)
			require.Len(t, creator.created, 1)
			assert.Equal(t, tt.wantStatus, creator.created[0].Status)
			assert.Equal(t, tt.wantTags, creator.created[0].Tags)
			assert.Equal(t, tt.wantType, creator.created[0].PostType)
		})
	}
}

func TestImporter_TruncatesLongTitlesAndDropsInvalidTags(t *testing.T) {
	longTitle := strings.Repeat("é", 220)
	tests := []struct {
		name      string
		xml       string
		wantTitle string
		wantTags  []string
	}{
		{
			name: "success - title over the 200-rune limit is truncated",
			xml: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title><![CDATA[%s]]></title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>long-title</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`, longTitle),
			wantTitle: strings.Repeat("é", 200),
			wantTags:  []string{},
		},
		{
			name: "success - tags over the 50-rune limit are dropped",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Tagged Post</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>tagged-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<category domain="post_tag" nicename="ok"><![CDATA[ok]]></category>
<category domain="post_tag" nicename="toolong"><![CDATA[` + strings.Repeat("x", 51) + `]]></category>
</item>
</channel>
</rss>`,
			wantTitle: "Tagged Post",
			wantTags:  []string{"ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &fakeContentCreator{failOn: -1}
			importer := newTestImporter(creator, &fakeUserResolver{})
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Imported)
			require.Len(t, creator.created, 1)
			assert.Equal(t, tt.wantTitle, creator.created[0].Title)
			assert.Equal(t, tt.wantTags, creator.created[0].Tags)
		})
	}
}

func TestImporter_AssignsPostsToCreators(t *testing.T) {
	tests := []struct {
		name              string
		xml               string
		resolver          *fakeUserResolver
		creator           *fakeContentCreator
		wantImported      int
		wantUsersImported int
		wantUserIDs       []int
		wantResolverCalls int
	}{
		{
			name: "success - two creators get distinct userIDs, cache prevents duplicate calls",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
	xmlns:wp="http://wordpress.org/export/1.2/"
	xmlns:content="http://purl.org/rss/1.0/modules/content/"
	xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<wp:author><wp:author_id>1</wp:author_id><wp:author_login><![CDATA[alice]]></wp:author_login><wp:author_email><![CDATA[alice@example.com]]></wp:author_email><wp:author_display_name><![CDATA[Alice]]></wp:author_display_name></wp:author>
<wp:author><wp:author_id>2</wp:author_id><wp:author_login><![CDATA[bob]]></wp:author_login><wp:author_email><![CDATA[bob@example.com]]></wp:author_email><wp:author_display_name><![CDATA[Bob]]></wp:author_display_name></wp:author>
<item>
<title>Post A</title>
<dc:creator><![CDATA[alice]]></dc:creator>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>A</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>post-a</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
<item>
<title>Post B</title>
<dc:creator><![CDATA[bob]]></dc:creator>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>B</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>post-b</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
<item>
<title>Post C</title>
<dc:creator><![CDATA[alice]]></dc:creator>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>C</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>post-c</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`,
			resolver:          &fakeUserResolver{},
			creator:           &fakeContentCreator{failOn: -1},
			wantImported:      3,
			wantUsersImported: 2,
			wantUserIDs:       []int{1, 2, 1},
			wantResolverCalls: 2,
		},
		{
			name: "fallback - resolver failure assigns posts to admin",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
	xmlns:wp="http://wordpress.org/export/1.2/"
	xmlns:content="http://purl.org/rss/1.0/modules/content/"
	xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<wp:author><wp:author_id>1</wp:author_id><wp:author_login><![CDATA[ghostwriter]]></wp:author_login><wp:author_email><![CDATA[]]></wp:author_email><wp:author_display_name><![CDATA[Ghost Writer]]></wp:author_display_name></wp:author>
<item>
<title>Ghost Post</title>
<dc:creator><![CDATA[ghostwriter]]></dc:creator>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Ghost</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>ghost-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`,
			resolver:          &fakeUserResolver{failLogin: "ghostwriter"},
			creator:           &fakeContentCreator{failOn: -1},
			wantImported:      1,
			wantUsersImported: 0,
			wantUserIDs:       []int{99},
			wantResolverCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importer := newTestImporter(tt.creator, tt.resolver)
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 99, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantImported, result.Imported)
			assert.Equal(t, tt.wantUsersImported, result.UsersImported)
			require.Len(t, tt.creator.userIDs, len(tt.wantUserIDs))
			assert.Equal(t, tt.wantUserIDs, tt.creator.userIDs)
			assert.Len(t, tt.resolver.calls, tt.wantResolverCalls)
		})
	}
}

func float64Ptr(v float64) *float64 { return &v }

func TestImporter_CustomFields(t *testing.T) {
	eventFields := []customfield.FieldSchema{
		{Slug: "start", Type: customfield.FieldTypeDatetime, Required: true},
		{Slug: "location", Type: customfield.FieldTypeText, Required: true},
		{Slug: "type", Type: customfield.FieldTypeSelect, Required: true, Options: []string{"journalist", "community", "point"}},
		{Slug: "point", Type: customfield.FieldTypeNumber, Required: true, Min: float64Ptr(1)},
		{Slug: "created_at", Type: customfield.FieldTypeDate},
		{Slug: "image_pdf", Type: customfield.FieldTypeUrl},
		{Slug: "link", Type: customfield.FieldTypeUrl},
	}

	tests := []struct {
		name             string
		xml              string
		wantImported     int
		wantSkipped      int
		wantCustomFields map[string]any
	}{
		{
			name: "success - event custom fields converted with correct types",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Annual Ride</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Ride</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>annual-ride</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
<wp:postmeta><wp:meta_key><![CDATA[start]]></wp:meta_key><wp:meta_value><![CDATA[2018-10-27 00:00:00]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[_start]]></wp:meta_key><wp:meta_value><![CDATA[field_abc]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[location]]></wp:meta_key><wp:meta_value><![CDATA[Pontianak]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[type]]></wp:meta_key><wp:meta_value><![CDATA[journalist]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[point]]></wp:meta_key><wp:meta_value><![CDATA[3]]></wp:meta_value></wp:postmeta>
</item>
</channel>
</rss>`,
			wantImported: 1,
			wantSkipped:  0,
			wantCustomFields: map[string]any{
				"start":    "2018-10-27T00:00:00Z",
				"location": "Pontianak",
				"type":     "journalist",
				"point":    float64(3),
			},
		},
		{
			name: "success - ACF date field in Ymd storage format",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Published Article</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Ride</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>published-article</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
<wp:postmeta><wp:meta_key><![CDATA[start]]></wp:meta_key><wp:meta_value><![CDATA[2018-10-27 00:00:00]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[location]]></wp:meta_key><wp:meta_value><![CDATA[Pontianak]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[type]]></wp:meta_key><wp:meta_value><![CDATA[journalist]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[point]]></wp:meta_key><wp:meta_value><![CDATA[3]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[created_at]]></wp:meta_key><wp:meta_value><![CDATA[20250601]]></wp:meta_value></wp:postmeta>
</item>
</channel>
</rss>`,
			wantImported: 1,
			wantSkipped:  0,
			wantCustomFields: map[string]any{
				"start":      "2018-10-27T00:00:00Z",
				"location":   "Pontianak",
				"type":       "journalist",
				"point":      float64(3),
				"created_at": "2025-06-01",
			},
		},
		{
			name: "skip - missing required field",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Missing Point Event</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>No point</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>missing-point-event</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
<wp:postmeta><wp:meta_key><![CDATA[start]]></wp:meta_key><wp:meta_value><![CDATA[2018-10-27 00:00:00]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[location]]></wp:meta_key><wp:meta_value><![CDATA[Pontianak]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[type]]></wp:meta_key><wp:meta_value><![CDATA[journalist]]></wp:meta_value></wp:postmeta>
</item>
</channel>
</rss>`,
			wantImported:     0,
			wantSkipped:      1,
			wantCustomFields: nil,
		},
		{
			name: "success - optional url field with attachment ID value is dropped",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Article With Attachment ID</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Ride</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>article-with-attachment-id</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
<wp:postmeta><wp:meta_key><![CDATA[start]]></wp:meta_key><wp:meta_value><![CDATA[2018-10-27 00:00:00]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[location]]></wp:meta_key><wp:meta_value><![CDATA[Pontianak]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[type]]></wp:meta_key><wp:meta_value><![CDATA[journalist]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[point]]></wp:meta_key><wp:meta_value><![CDATA[3]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[image_pdf]]></wp:meta_key><wp:meta_value><![CDATA[130720]]></wp:meta_value></wp:postmeta>
</item>
</channel>
</rss>`,
			wantImported: 1,
			wantSkipped:  0,
			wantCustomFields: map[string]any{
				"start":    "2018-10-27T00:00:00Z",
				"location": "Pontianak",
				"type":     "journalist",
				"point":    float64(3),
			},
		},
		{
			name: "success - url field value is trimmed",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Article With Messy Link</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Ride</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>article-with-messy-link</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>event</wp:post_type>
<wp:postmeta><wp:meta_key><![CDATA[start]]></wp:meta_key><wp:meta_value><![CDATA[2018-10-27 00:00:00]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[location]]></wp:meta_key><wp:meta_value><![CDATA[Pontianak]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[type]]></wp:meta_key><wp:meta_value><![CDATA[journalist]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[point]]></wp:meta_key><wp:meta_value><![CDATA[3]]></wp:meta_value></wp:postmeta>
<wp:postmeta><wp:meta_key><![CDATA[link]]></wp:meta_key><wp:meta_value><![CDATA[ https://equator.co.id/news/]]></wp:meta_value></wp:postmeta>
</item>
</channel>
</rss>`,
			wantImported: 1,
			wantSkipped:  0,
			wantCustomFields: map[string]any{
				"start":    "2018-10-27T00:00:00Z",
				"location": "Pontianak",
				"type":     "journalist",
				"point":    float64(3),
				"link":     "https://equator.co.id/news/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &fakeContentCreator{failOn: -1}
			postTypes := &fakePostTypeLister{
				postTypes: map[string][]customfield.FieldSchema{
					"event": eventFields,
				},
			}
			importer := newTestImporterWithPostTypes(creator, &fakeUserResolver{}, postTypes)
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantImported, result.Imported)
			assert.Equal(t, tt.wantSkipped, result.Skipped)

			if tt.wantCustomFields != nil {
				require.Len(t, creator.created, 1)
				assert.Equal(t, tt.wantCustomFields, creator.created[0].CustomFields)
			}
		})
	}
}

func TestImporter_FeaturedImage(t *testing.T) {
	t.Run("success - featured image downloaded and local url prepended", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngSignature)
		}))
		defer server.Close()

		mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "http://localhost:8080/uploads/media/featured.webp"}}

		xml := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Post With Thumb</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hello</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>post-with-thumb</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<wp:postmeta>
<wp:meta_key><![CDATA[_thumbnail_id]]></wp:meta_key>
<wp:meta_value><![CDATA[100]]></wp:meta_value>
</wp:postmeta>
</item>
<item>
<title>Thumb</title>
<content:encoded><![CDATA[]]></content:encoded>
<wp:post_id>100</wp:post_id>
<wp:post_name>thumb</wp:post_name>
<wp:status>inherit</wp:status>
<wp:post_type>attachment</wp:post_type>
<wp:attachment_url><![CDATA[` + server.URL + `/featured.png]]></wp:attachment_url>
</item>
</channel>
</rss>`

		creator := &fakeContentCreator{failOn: -1}
		importer := wordpress.NewImporter(
			creator,
			wordpress.NewMediaDownloader(server.Client(), mediaSvc),
			&fakeUserResolver{},
			nil,
			"en",
			nil,
		)
		result, err := importer.Import(context.Background(), strings.NewReader(xml), 1, wordpress.ImportOptions{}, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Imported)
		assert.Equal(t, 0, result.Skipped)
		require.Len(t, creator.created, 1)
		assert.Contains(t, creator.created[0].Content, `"src":"http://localhost:8080/uploads/media/featured.webp"`)
		assert.Equal(t, 1, mediaSvc.called)
	})

	t.Run("fallback - download fails, original url hotlinked", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		mediaSvc := &fakeMediaService{returnMedia: &mediadomain.Media{URL: "http://localhost:8080/uploads/media/featured.webp"}}
		creator := &fakeContentCreator{failOn: -1}
		importer := wordpress.NewImporter(
			creator,
			wordpress.NewMediaDownloader(server.Client(), mediaSvc),
			&fakeUserResolver{},
			nil,
			"en",
			nil,
		)

		xml := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Post Broken Thumb</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>post-broken-thumb</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<wp:postmeta>
<wp:meta_key><![CDATA[_thumbnail_id]]></wp:meta_key>
<wp:meta_value><![CDATA[200]]></wp:meta_value>
</wp:postmeta>
</item>
<item>
<title>Thumb</title>
<content:encoded><![CDATA[]]></content:encoded>
<wp:post_id>200</wp:post_id>
<wp:post_name>thumb</wp:post_name>
<wp:status>inherit</wp:status>
<wp:post_type>attachment</wp:post_type>
<wp:attachment_url><![CDATA[` + server.URL + `/missing.png]]></wp:attachment_url>
</item>
</channel>
</rss>`

		result, err := importer.Import(context.Background(), strings.NewReader(xml), 1, wordpress.ImportOptions{}, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Imported)
		require.Len(t, creator.created, 1)
		assert.Contains(t, creator.created[0].Content, `"src":"`+server.URL+`/missing.png"`)
		assert.NotContains(t, creator.created[0].Content, "localhost:8080")
		assert.Equal(t, 0, mediaSvc.called)
	})

	tests := []struct {
		name           string
		xml            string
		wantImported   int
		wantContentHas string
	}{
		{
			name: "no-op - thumbnail_id references missing attachment",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Post Missing Ref</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>post-missing-ref</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
<wp:postmeta>
<wp:meta_key><![CDATA[_thumbnail_id]]></wp:meta_key>
<wp:meta_value><![CDATA[999]]></wp:meta_value>
</wp:postmeta>
</item>
</channel>
</rss>`,
			wantImported:   1,
			wantContentHas: `"type":"paragraph"`,
		},
		{
			name: "no-op - post without thumbnail_id unchanged",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Plain Post</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Plain</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>plain-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`,
			wantImported:   1,
			wantContentHas: `"type":"paragraph"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &fakeContentCreator{failOn: -1}
			importer := newTestImporter(creator, &fakeUserResolver{})
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantImported, result.Imported)
			require.Len(t, creator.created, tt.wantImported)
			assert.Contains(t, creator.created[0].Content, tt.wantContentHas)
		})
	}
}

func TestImporter_ForwardsLanguage(t *testing.T) {
	tests := []struct {
		name         string
		language     string
		wantLanguage string
	}{
		{
			name:         "success - importer sets configured language on created content",
			language:     "id",
			wantLanguage: "id",
		},
		{
			name:         "success - empty language results in empty field",
			language:     "",
			wantLanguage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Test Post</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>test-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`
			creator := &fakeContentCreator{failOn: -1}
			importer := wordpress.NewImporter(
				creator,
				wordpress.NewMediaDownloader(nil, stubMediaService{}),
				&fakeUserResolver{},
				nil,
				tt.language,
				nil,
			)
			result, err := importer.Import(context.Background(), strings.NewReader(xml), 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Imported)
			require.Len(t, creator.created, 1)
			assert.Equal(t, tt.wantLanguage, creator.created[0].Language)
		})
	}
}

func TestImporter_PreservesPublishDate(t *testing.T) {
	tests := []struct {
		name       string
		xml        string
		wantDate   *time.Time
		wantNilErr bool
	}{
		{
			name: "success - RFC 1123 pubDate parsed into PublishedAt",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>Dated Post</title>
<pubDate>Mon, 08 Aug 2016 00:00:00 +0000</pubDate>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>dated-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`,
			wantDate: func() *time.Time {
				ts := time.Date(2016, 8, 8, 0, 0, 0, 0, time.UTC)
				return &ts
			}(),
		},
		{
			name: "success - missing pubDate leaves PublishedAt nil",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:wp="http://wordpress.org/export/1.2/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>T</title><wp:base_blog_url>http://x.local</wp:base_blog_url>
<item>
<title>No Date Post</title>
<content:encoded><![CDATA[<!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph -->]]></content:encoded>
<wp:post_name>no-date-post</wp:post_name>
<wp:status>publish</wp:status>
<wp:post_type>post</wp:post_type>
</item>
</channel>
</rss>`,
			wantDate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &fakeContentCreator{failOn: -1}
			importer := newTestImporter(creator, &fakeUserResolver{})
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 1, wordpress.ImportOptions{}, nil)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Imported)
			require.Len(t, creator.created, 1)
			if tt.wantDate == nil {
				assert.Nil(t, creator.created[0].PublishedAt)
				return
			}
			require.NotNil(t, creator.created[0].PublishedAt)
			assert.Equal(t, *tt.wantDate, *creator.created[0].PublishedAt)
		})
	}
}

func TestParseWPDate(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		want     string
		wantBool bool
	}{
		{
			name:     "success - RFC 1123 with zone",
			raw:      "Mon, 08 Aug 2016 00:00:00 +0000",
			want:     "2016-08-08T00:00:00Z",
			wantBool: true,
		},
		{
			name:     "success - RFC 3339",
			raw:      "2016-08-08T00:00:00Z",
			want:     "2016-08-08T00:00:00Z",
			wantBool: true,
		},
		{
			name:     "success - WP datetime without zone",
			raw:      "2016-08-08 00:00:00",
			want:     "2016-08-08T00:00:00Z",
			wantBool: true,
		},
		{
			name:     "success - ACF date picker Ymd",
			raw:      "20160808",
			want:     "2016-08-08T00:00:00Z",
			wantBool: true,
		},
		{
			name:     "failure - empty string",
			raw:      "",
			wantBool: false,
		},
		{
			name:     "failure - unparseable value",
			raw:      "not-a-date",
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := wordpress.ParseWPDate(tt.raw)
			assert.Equal(t, tt.wantBool, ok)
			if tt.wantBool {
				assert.Equal(t, tt.want, got.Format(time.RFC3339))
			}
		})
	}
}
