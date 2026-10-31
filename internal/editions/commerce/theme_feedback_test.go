package commerce_test

import (
	"html/template"
	"net/http/httptest"
	"testing"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorefrontThemeBrandFallback(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name           string
		siteConfig     sitessr.SiteConfig
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:       "success - logo renders img plus name together",
			siteConfig: sitessr.SiteConfig{Name: "Lesstruct", Logo: "/uploads/logo.png"},
			wantContains: []string{
				`<img class="brand__logo" src="/uploads/logo.png" alt="">`,
				`<span class="brand__name">Lesstruct</span>`,
			},
			wantNotContain: []string{
				`brand__mark`,
			},
		},
		{
			name:       "success - empty logo renders name as text without mark",
			siteConfig: sitessr.SiteConfig{Name: "Lesstruct"},
			wantContains: []string{
				`<span class="brand__name">Lesstruct</span>`,
			},
			wantNotContain: []string{
				`brand__mark`,
				`brand__logo`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			layout.SiteConfig = tt.siteConfig

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderIndex(w, sitessr.IndexData{
				LayoutData: layout,
				Posts:      []sitessr.PostItem{storefrontPost()},
			}))

			body := w.Body.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContain {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}

func TestStorefrontThemeProductImageRatio(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name          string
		ratio         template.CSS
		wantContains  []string
		wantNotContain []string
	}{
		{
			name:  "success - ratio set emits inline aspect-ratio override",
			ratio: "1 / 1",
			wantContains: []string{
				`aspect-ratio: 1 / 1`,
				`.product-card__media,.product-gallery__main,.cart-line__media`,
			},
			wantNotContain: []string{},
		},
		{
			name:  "success - unset keeps hard-coded default with no override",
			ratio: "",
			wantContains: []string{},
			wantNotContain: []string{
				`aspect-ratio: 1 / 1`,
				`.product-card__media,.product-gallery__main,.cart-line__media{aspect-ratio:`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			layout.SiteConfig.ProductImageRatio = tt.ratio

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderIndex(w, sitessr.IndexData{
				LayoutData: layout,
				Posts:      []sitessr.PostItem{storefrontPost()},
			}))

			body := w.Body.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContain {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}

func TestStorefrontThemeCanonicalGallery(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name       string
		images     any
		wantImages []string
	}{
		{
			name: "success - canonical gallery array renders all urls",
			images: []any{
				map[string]any{"url": "/media/a.jpg", "alt": "A"},
				map[string]any{"url": "/media/b.jpg", "alt": "B"},
			},
			wantImages: []string{"/media/a.jpg", "/media/b.jpg"},
		},
		{
			name:       "success - legacy newline string still renders",
			images:     "/media/a.jpg\n/media/b.jpg",
			wantImages: []string{"/media/a.jpg", "/media/b.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			post := storefrontPost()
			data := sitessr.ContentData{
				LayoutData: layout,
				Slug:       post.Slug,
				PostType:   "product",
				Body:       template.HTML("<p>Details.</p>"),
				CustomFields: map[string]any{
					"images": tt.images,
				},
			}

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderContent(w, data))

			body := w.Body.String()
			for _, want := range tt.wantImages {
				assert.Contains(t, body, want)
			}
		})
	}
}

func TestStorefrontThemeResolvedGallery(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	galleries := map[string][]sitessr.GalleryImage{
		"images": {
			{
				URL: "/uploads/media/a.webp", Alt: "A",
				Thumb: "/uploads/media/a_thumb.webp", Medium: "/uploads/media/a_medium.webp",
				Large: "/uploads/media/a_large.webp", Srcset: "/uploads/media/a_thumb.webp 64w",
				Original: "/uploads/media/a.webp",
			},
			{
				URL: "/uploads/media/b.webp", Alt: "B",
				Thumb: "/uploads/media/b_thumb.webp", Medium: "/uploads/media/b_medium.webp",
				Large: "/uploads/media/b_large.webp", Srcset: "/uploads/media/b_thumb.webp 64w",
				Original: "/uploads/media/b.webp",
			},
		},
	}

	tests := []struct {
		name             string
		galleries        map[string][]sitessr.GalleryImage
		wantContains     []string
		wantNotContain   []string
	}{
		{
			name:      "success - resolved gallery renders large main and thumb thumbs",
			galleries: galleries,
			wantContains: []string{
				`src="/uploads/media/a_large.webp"`,
				`src="/uploads/media/a_thumb.webp"`,
				`data-image="/uploads/media/a_large.webp"`,
			},
			wantNotContain: []string{
				`product-gallery__fallback`,
			},
		},
		{
			name:      "success - missing galleries fall back to legacy lines",
			galleries: nil,
			wantContains: []string{
				`src="/media/tee.jpg"`,
			},
			wantNotContain: []string{
				`data-srcset`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			post := storefrontPost()
			customFields := map[string]any{
				"images": "/media/tee.jpg\n/media/tee-back.jpg",
			}
			data := sitessr.ContentData{
				LayoutData:           layout,
				Slug:                 post.Slug,
				PostType:             "product",
				Body:                 template.HTML("<p>Details.</p>"),
				CustomFields:         customFields,
				CustomFieldGalleries: tt.galleries,
			}

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderContent(w, data))

			body := w.Body.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContain {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}

func TestStorefrontThemeProductMarkup(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name           string
		customFields   map[string]any
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "success - matrix containers and hidden payload SKU row render",
			customFields: map[string]any{
				"material": "Cotton",
				"sku":      "TEE-1",
			},
			wantContains: []string{
				`data-variant-matrix`,
				`data-variant-axes`,
				`id="product-sku-row"`,
				"hidden",
			},
			wantNotContain: []string{
				"TEE-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			post := storefrontPost()

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderContent(w, sitessr.ContentData{
				LayoutData:   layout,
				Slug:         post.Slug,
				PostType:     "product",
				Body:         template.HTML("<p>Details.</p>"),
				CustomFields: tt.customFields,
			}))

			body := w.Body.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContain {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}

func TestStorefrontThemeHomepageHero(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name           string
		siteConfig     sitessr.SiteConfig
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:       "success - hero photo and overrides render",
			siteConfig: sitessr.SiteConfig{Name: "Lesstruct", HeroImage: "/uploads/hero.webp", HeroTitle: "Custom title", HeroSubtitle: "Custom subtitle"},
			wantContains: []string{
				`class="hero__photo"`,
				`src="/uploads/hero.webp"`,
				"Custom title",
				"Custom subtitle",
			},
			wantNotContain: []string{
				`hero__monogram`,
				"Take Lesstruct into the real world",
			},
		},
		{
			name:       "success - empty hero keeps monogram and bilingual default",
			siteConfig: sitessr.SiteConfig{Name: "Lesstruct"},
			wantContains: []string{
				`hero__monogram`,
				"Take Lesstruct into the real world",
			},
			wantNotContain: []string{
				`hero__photo`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			layout.SiteConfig = tt.siteConfig

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderHome(w, sitessr.IndexData{
				LayoutData: layout,
				Posts:      []sitessr.PostItem{storefrontPost()},
			}))

			body := w.Body.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContain {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}
