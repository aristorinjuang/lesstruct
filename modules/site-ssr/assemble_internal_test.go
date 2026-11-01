package sitessr

import (
	"context"
	"testing"

	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediamocks "github.com/aristorinjuang/lesstruct/modules/media/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mobileVariants is the default thumbnail-pipeline trio the storefront is
// tuned for: 64px thumb, 280px medium, 616px large.
func mobileVariants() map[string]mediadomain.MediaVariant {
	return map[string]mediadomain.MediaVariant{
		"_thumb":  {URL: "/uploads/media/card_thumb.webp", Width: 64},
		"_medium": {URL: "/uploads/media/card_medium.webp", Width: 280},
		"_large":  {URL: "/uploads/media/card_large.webp", Width: 616},
	}
}

func TestDataAssembler_BuildPostItem_MobileURL(t *testing.T) {
	tests := []struct {
		name         string
		setupMedia   func(*mediamocks.MockRepository)
		featured     string
		wantMobile   string
		wantImageURL string
		wantNotLarge bool
		wantErr      bool
	}{
		{
			name: "success - medium variant is the mobile rendition",
			setupMedia: func(m *mediamocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "card").Return(
					&mediadomain.Media{URL: "/uploads/media/card.webp", Variants: mobileVariants()},
					nil,
				)
			},
			featured:     "/uploads/media/card.webp",
			wantMobile:   "/uploads/media/card_medium.webp",
			wantImageURL: "/uploads/media/card_medium.webp",
			wantNotLarge: true,
			wantErr:      false,
		},
		{
			name: "success - thumb fallback when no medium variant exists",
			setupMedia: func(m *mediamocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "card").Return(
					&mediadomain.Media{
						URL: "/uploads/media/card.webp",
						Variants: map[string]mediadomain.MediaVariant{
							"_thumb": {URL: "/uploads/media/card_thumb.webp", Width: 64},
							"_large": {URL: "/uploads/media/card_large.webp", Width: 616},
						},
					},
					nil,
				)
			},
			featured:     "/uploads/media/card.webp",
			wantMobile:   "/uploads/media/card_thumb.webp",
			wantImageURL: "/uploads/media/card_thumb.webp",
			wantNotLarge: true,
			wantErr:      false,
		},
		{
			name: "success - large fallback when only large variant exists",
			setupMedia: func(m *mediamocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "card").Return(
					&mediadomain.Media{
						URL: "/uploads/media/card.webp",
						Variants: map[string]mediadomain.MediaVariant{
							"_large": {URL: "/uploads/media/card_large.webp", Width: 616},
						},
					},
					nil,
				)
			},
			featured:     "/uploads/media/card.webp",
			wantMobile:   "/uploads/media/card_large.webp",
			wantImageURL: "/uploads/media/card_large.webp",
			wantNotLarge: false,
			wantErr:      false,
		},
		{
			name: "success - missing media falls back to the raw URL",
			setupMedia: func(m *mediamocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "card").Return(
					(*mediadomain.Media)(nil),
					nil,
				)
			},
			featured:     "/uploads/media/card.webp",
			wantMobile:   "/uploads/media/card.webp",
			wantImageURL: "/uploads/media/card.webp",
			wantNotLarge: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mediamocks.NewMockRepository(t)
			tt.setupMedia(mediaRepo)

			assembler := NewDataAssembler(
				nil,
				nil,
				nil,
				nil,
				nil,
				mediaRepo,
				[]string{"en"},
				nil,
				SiteConfig{},
				0,
			)

			item := assembler.buildPostItem(context.Background(), ContentRef{
				Slug:          "card",
				Title:         "Card",
				PostType:      "product",
				Language:      "en",
				FeaturedImage: tt.featured,
			})
			if tt.wantErr {
				require.Empty(t, item.ImageMobileURL)
				return
			}
			assert.Equal(t, tt.wantMobile, item.ImageMobileURL)
			assert.Equal(t, tt.wantImageURL, item.ImageURL)
			if tt.wantNotLarge {
				assert.NotContains(t, item.ImageMobileURL, "_large")
			}
		})
	}
}
