package commerce

import (
	"context"
	"testing"

	kernelcontent "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveProductContentID(t *testing.T) {
	rootGroup := 2

	tests := []struct {
		name      string
		slug      string
		contents  map[string]*kernelcontent.Content
		records   map[int]bool
		wantID    int
		wantFound bool
	}{
		{
			name: "success - root with record resolves own ID",
			slug: "tee",
			contents: map[string]*kernelcontent.Content{
				"tee": {ID: 2, Slug: "tee"},
			},
			records: map[int]bool{
				2: true,
			},
			wantID:    2,
			wantFound: true,
		},
		{
			name: "success - translated without own record falls back to group root",
			slug: "kaos",
			contents: map[string]*kernelcontent.Content{
				"kaos": {ID: 11, Slug: "kaos", TranslationGroupID: new(rootGroup)},
			},
			records: map[int]bool{
				2: true,
			},
			wantID:    2,
			wantFound: true,
		},
		{
			name: "success - translated with own record keeps own ID",
			slug: "kaos",
			contents: map[string]*kernelcontent.Content{
				"kaos": {ID: 11, Slug: "kaos", TranslationGroupID: new(rootGroup)},
			},
			records: map[int]bool{
				2:  true,
				11: true,
			},
			wantID:    11,
			wantFound: true,
		},
		{
			name: "success - bare group returns own ID for downstream 404",
			slug: "kaos",
			contents: map[string]*kernelcontent.Content{
				"kaos": {ID: 11, Slug: "kaos", TranslationGroupID: new(rootGroup)},
			},
			records:   map[int]bool{},
			wantID:    11,
			wantFound: true,
		},
		{
			name:      "not found - missing slug",
			slug:      "unknown",
			contents:  map[string]*kernelcontent.Content{},
			records:   map[int]bool{},
			wantID:    0,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(_ context.Context, slug string) (*kernelcontent.Content, error) {
				if content, ok := tt.contents[slug]; ok {
					return content, nil
				}
				return nil, assert.AnError
			}
			hasRecord := func(_ context.Context, contentID int) bool {
				return tt.records[contentID]
			}

			gotID, gotFound := resolveProductContentID(context.Background(), tt.slug, lookup, hasRecord)

			require.Equal(t, tt.wantFound, gotFound)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}
