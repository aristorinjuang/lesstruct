package wordpress_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeContentCreator is a test double for the content service. It records every
// create request and can be configured to fail on a specific call index.
type fakeContentCreator struct {
	created  []contentdomain.CreateContentRequest
	failOn   int // index that should return ErrSlugAlreadyExists; -1 = never
	callNext int
}

func (f *fakeContentCreator) Create(_ context.Context, _ int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error) {
	idx := f.callNext
	f.callNext++
	f.created = append(f.created, req)
	if f.failOn >= 0 && idx == f.failOn {
		return nil, fmt.Errorf("%w: dup", contentdomain.ErrSlugAlreadyExists)
	}
	return &contentdomain.Content{ID: idx + 1}, nil
}

// stubMediaService satisfies the importer's mediaService interface. It never
// succeeds, which is fine because the sample host is unreachable and downloads
// fail at the HTTP layer before reaching GenerateFromBytes.
type stubMediaService struct{}

func (stubMediaService) GenerateFromBytes(_ context.Context, _ []byte, _ int, _, _ string) (*mediadomain.Media, error) {
	return nil, fmt.Errorf("stub: no media service in test")
}

func newTestImporter(creator *fakeContentCreator) *wordpress.Importer {
	return wordpress.NewImporter(creator, wordpress.NewMediaDownloader(nil, stubMediaService{}), nil)
}

func TestImporter_RealSample(t *testing.T) {
	tests := []struct {
		name         string
		creator      *fakeContentCreator
		wantImported int
		wantSkipped  int
	}{
		{
			name:         "success - all items imported",
			creator:      &fakeContentCreator{failOn: -1},
			wantImported: 4,
			wantSkipped:  0,
		},
		{
			name:         "success - one slug collision skipped",
			creator:      &fakeContentCreator{failOn: 0},
			wantImported: 3,
			wantSkipped:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open("../../../samples/wordpress-export.xml")
			require.NoError(t, err)
			defer func() { _ = f.Close() }()

			importer := newTestImporter(tt.creator)
			result, err := importer.Import(context.Background(), f, 1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantImported, result.Imported)
			assert.Equal(t, tt.wantSkipped, result.Skipped)
			require.Len(t, tt.creator.created, 4)
			assert.Equal(t, "post", tt.creator.created[0].PostType)
			assert.Equal(t, "page", tt.creator.created[1].PostType)
		})
	}
}

func TestImporter_InvalidXML(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "error - malformed input fails fast"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importer := newTestImporter(&fakeContentCreator{failOn: -1})
			result, err := importer.Import(context.Background(), strings.NewReader("<<<broken"), 1)
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
			importer := newTestImporter(creator)
			result, err := importer.Import(context.Background(), strings.NewReader(tt.xml), 1)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Imported)
			require.Len(t, creator.created, 1)
			assert.Equal(t, tt.wantStatus, creator.created[0].Status)
			assert.Equal(t, tt.wantTags, creator.created[0].Tags)
			assert.Equal(t, tt.wantType, creator.created[0].PostType)
		})
	}
}
