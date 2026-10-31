package sitessr_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediaMocks "github.com/aristorinjuang/lesstruct/modules/media/domain/mocks"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// galleryPostType is a product-like type exposing one gallery field.
func galleryPostType() posttype.PostType {
	return posttype.PostType{
		Name: "Product",
		Slug: "product",
		Fields: []customfield.FieldSchema{
			{Name: "Images", Slug: "images", Type: customfield.FieldTypeGallery},
		},
	}
}

func TestBuildContentData_GalleryImages(t *testing.T) {
	mediaURL := "/uploads/media/abc123.webp"
	mediaVariants := map[string]mediadomain.MediaVariant{
		"_thumb":  {URL: "/uploads/media/abc123_thumb.webp", Width: 64},
		"_medium": {URL: "/uploads/media/abc123_medium.webp", Width: 280},
		"_large":  {URL: "/uploads/media/abc123_large.webp", Width: 616},
	}

	tests := []struct {
		name       string
		custom     map[string]any
		setupMedia func(*mediaMocks.MockRepository)
		verify     func(t *testing.T, galleries map[string][]sitessr.GalleryImage)
	}{
		{
			name: "media-library image resolves thumb medium large and srcset",
			custom: map[string]any{
				"images": []any{map[string]any{"url": mediaURL, "alt": "Front"}},
			},
			setupMedia: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					&mediadomain.Media{URL: mediaURL, Variants: mediaVariants},
					nil,
				)
			},
			verify: func(t *testing.T, galleries map[string][]sitessr.GalleryImage) {
				images, ok := galleries["images"]
				require.True(t, ok, "images gallery must resolve")
				require.Len(t, images, 1)
				assert.Equal(t, "Front", images[0].Alt)
				assert.Equal(t, "/uploads/media/abc123_thumb.webp", images[0].Thumb)
				assert.Equal(t, "/uploads/media/abc123_medium.webp", images[0].Medium)
				assert.Equal(t, "/uploads/media/abc123_large.webp", images[0].Large)
				assert.Equal(t, mediaURL, images[0].Original)
				assert.Contains(t, images[0].Srcset, "280w")
				assert.Contains(t, images[0].Srcset, "616w")
			},
		},
		{
			name: "external image renders as-is",
			custom: map[string]any{
				"images": []any{map[string]any{"url": "https://example.com/photo.jpg", "alt": "Remote"}},
			},
			setupMedia: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "photo").Return(
					(*mediadomain.Media)(nil),
					nil,
				)
			},
			verify: func(t *testing.T, galleries map[string][]sitessr.GalleryImage) {
				images, ok := galleries["images"]
				require.True(t, ok, "images gallery must resolve")
				require.Len(t, images, 1)
				assert.Equal(t, "https://example.com/photo.jpg", images[0].Thumb)
				assert.Equal(t, "https://example.com/photo.jpg", images[0].Large)
				assert.Equal(t, "https://example.com/photo.jpg", images[0].Original)
				assert.Empty(t, images[0].Srcset)
			},
		},
		{
			name: "legacy newline-separated string coerces into entries",
			custom: map[string]any{
				"images": "/uploads/media/abc123.webp\n\nhttps://example.com/photo.jpg\n",
			},
			setupMedia: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					&mediadomain.Media{URL: mediaURL, Variants: mediaVariants},
					nil,
				)
				m.On("FindByHashPrefix", mock.Anything, "photo").Return(
					(*mediadomain.Media)(nil),
					nil,
				)
			},
			verify: func(t *testing.T, galleries map[string][]sitessr.GalleryImage) {
				images, ok := galleries["images"]
				require.True(t, ok, "images gallery must resolve")
				require.Len(t, images, 2)
				assert.Equal(t, "/uploads/media/abc123_thumb.webp", images[0].Thumb)
				assert.Equal(t, "https://example.com/photo.jpg", images[1].Large)
			},
		},
		{
			name: "partial variants fall back to original per suffix",
			custom: map[string]any{
				"images": []any{map[string]any{"url": mediaURL, "alt": "Front"}},
			},
			setupMedia: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					&mediadomain.Media{URL: mediaURL, Variants: map[string]mediadomain.MediaVariant{
						"_thumb": {URL: "/uploads/media/abc123_thumb.webp", Width: 64},
					}},
					nil,
				)
			},
			verify: func(t *testing.T, galleries map[string][]sitessr.GalleryImage) {
				images, ok := galleries["images"]
				require.True(t, ok, "images gallery must resolve")
				require.Len(t, images, 1)
				assert.Equal(t, "/uploads/media/abc123_thumb.webp", images[0].Thumb)
				assert.Equal(t, mediaURL, images[0].Medium)
				assert.Equal(t, mediaURL, images[0].Large)
				assert.Equal(t, mediaURL, images[0].Original)
				assert.Contains(t, images[0].Srcset, "64w")
			},
		},
		{
			name: "single map object coerces into one entry",
			custom: map[string]any{
				"images": map[string]any{"url": mediaURL, "alt": "Solo"},
			},
			setupMedia: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					&mediadomain.Media{URL: mediaURL, Variants: mediaVariants},
					nil,
				)
			},
			verify: func(t *testing.T, galleries map[string][]sitessr.GalleryImage) {
				images, ok := galleries["images"]
				require.True(t, ok, "images gallery must resolve")
				require.Len(t, images, 1)
				assert.Equal(t, "Solo", images[0].Alt)
				assert.Equal(t, "/uploads/media/abc123_large.webp", images[0].Large)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			content := sitessr.ContentRef{
				ID:           7,
				Slug:         "tee",
				Title:        "Tee",
				Content:      "<p>Tee</p>",
				Format:       sitessr.FormatHTML,
				PostType:     "product",
				Language:     "en",
				CustomFields: tt.custom,
			}
			mockService.On("GetPublishedBySlugAny", mock.Anything, "tee").Return(content, nil)
			mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)
			mockService.On("GetRelated", mock.Anything, 7, 4).Return([]sitessr.ContentRef{}, nil)

			mockResolver := NewMockPostTypeResolver(t)
			mockResolver.On("GetBySlug", "product").Return(galleryPostType(), nil)

			mediaRepo := mediaMocks.NewMockRepository(t)
			tt.setupMedia(mediaRepo)

			assembler := sitessr.NewDataAssembler(
				mockService,
				mockResolver,
				nil,
				nil,
				nil,
				mediaRepo,
				[]string{"en"},
				nil,
				sitessr.SiteConfig{},
				0,
			)

			data, err := assembler.BuildContentData(context.Background(), "tee")
			require.NoError(t, err)
			tt.verify(t, data.CustomFieldGalleries)
		})
	}
}
