package blog

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/tiptap"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
)

func TestResolveBodyImageVariants(t *testing.T) {
	tests := []struct {
		name  string
		media *mediadomain.Media
		want  []tiptap.ImageVariant
	}{
		{
			name:  "nil media returns nil",
			media: nil,
			want:  nil,
		},
		{
			name:  "empty variants returns nil",
			media: &mediadomain.Media{Variants: map[string]mediadomain.MediaVariant{}},
			want:  nil,
		},
		{
			name: "sorts variants ascending by width",
			media: &mediadomain.Media{
				Variants: map[string]mediadomain.MediaVariant{
					"_large":  {URL: "/uploads/photo_large.webp", Width: 616, Height: 411},
					"_thumb":  {URL: "/uploads/photo_thumb.webp", Width: 64, Height: 43},
					"_medium": {URL: "/uploads/photo_medium.webp", Width: 280, Height: 187},
				},
			},
			want: []tiptap.ImageVariant{
				{URL: "/uploads/photo_thumb.webp", Width: 64, Height: 43},
				{URL: "/uploads/photo_medium.webp", Width: 280, Height: 187},
				{URL: "/uploads/photo_large.webp", Width: 616, Height: 411},
			},
		},
		{
			name: "original wider than variants stays out of srcset",
			media: &mediadomain.Media{
				URL:    "/uploads/product.webp",
				Width:  1024,
				Height: 683,
				Variants: map[string]mediadomain.MediaVariant{
					"_medium": {URL: "/uploads/product_medium.webp", Width: 280, Height: 187},
					"_large":  {URL: "/uploads/product_large.webp", Width: 616, Height: 411},
					"_thumb":  {URL: "/uploads/product_thumb.webp", Width: 64, Height: 43},
				},
			},
			want: []tiptap.ImageVariant{
				{URL: "/uploads/product_thumb.webp", Width: 64, Height: 43},
				{URL: "/uploads/product_medium.webp", Width: 280, Height: 187},
				{URL: "/uploads/product_large.webp", Width: 616, Height: 411},
			},
		},
		{
			name: "skips variants with empty urls or non-positive widths",
			media: &mediadomain.Media{
				Variants: map[string]mediadomain.MediaVariant{
					"empty": {URL: "", Width: 280, Height: 187},
					"zero":  {URL: "/uploads/zero.webp", Width: 0, Height: 0},
					"valid": {URL: "/uploads/valid.webp", Width: 280, Height: 187},
				},
			},
			want: []tiptap.ImageVariant{
				{URL: "/uploads/valid.webp", Width: 280, Height: 187},
			},
		},
		{
			name: "duplicate widths order deterministically by url",
			media: &mediadomain.Media{
				Variants: map[string]mediadomain.MediaVariant{
					"b": {URL: "/uploads/b.webp", Width: 280, Height: 187},
					"a": {URL: "/uploads/a.webp", Width: 280, Height: 187},
				},
			},
			want: []tiptap.ImageVariant{
				{URL: "/uploads/a.webp", Width: 280, Height: 187},
				{URL: "/uploads/b.webp", Width: 280, Height: 187},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveBodyImageVariants(tt.media))
		})
	}
}
