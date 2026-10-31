package sitessr_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/tiptap"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAssembler_HomeSections_Localized(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		section   sitessr.HomepageSection
		resolved  posttype.PostType
		wantTitle string
		wantDesc  string
	}{
		{
			name:      "success - title_id wins in Indonesian",
			languages: []string{"id", "en"},
			section: sitessr.HomepageSection{
				PostType: "article",
				Limit:    4,
				Title:    "Featured",
				TitleID:  "Unggulan",
			},
			resolved: posttype.PostType{
				Name:        "Article",
				Slug:        "article",
				Description: "Base desc",
			},
			wantTitle: "Unggulan",
			wantDesc:  "Base desc",
		},
		{
			name:      "success - falls back to title when title_id unset",
			languages: []string{"id", "en"},
			section: sitessr.HomepageSection{
				PostType: "article",
				Limit:    4,
				Title:    "Featured",
			},
			resolved: posttype.PostType{
				Name:        "Article",
				Slug:        "article",
				Description: "Base desc",
			},
			wantTitle: "Featured",
			wantDesc:  "Base desc",
		},
		{
			name:      "success - falls back to posttype name when title empty",
			languages: []string{"id", "en"},
			section: sitessr.HomepageSection{
				PostType: "article",
				Limit:    4,
			},
			resolved: posttype.PostType{
				Name:        "Article",
				NameID:      "Artikel",
				Slug:        "article",
				Description: "Base desc",
			},
			wantTitle: "Artikel",
			wantDesc:  "Base desc",
		},
		{
			name:      "success - description_id wins in Indonesian",
			languages: []string{"id", "en"},
			section: sitessr.HomepageSection{
				PostType:      "article",
				Limit:         4,
				Title:         "Featured",
				DescriptionID: "Deskripsi ID",
			},
			resolved: posttype.PostType{
				Name:          "Article",
				Slug:          "article",
				Description:   "Base desc",
				DescriptionID: "Deskripsi PT",
			},
			wantTitle: "Featured",
			wantDesc:  "Deskripsi ID",
		},
		{
			name:      "success - English uses base values",
			languages: []string{"en", "id"},
			section: sitessr.HomepageSection{
				PostType:      "article",
				Limit:         4,
				Title:         "Featured",
				TitleID:       "Unggulan",
				DescriptionID: "Deskripsi ID",
			},
			resolved: posttype.PostType{
				Name:          "Article",
				NameID:        "Artikel",
				Slug:          "article",
				Description:   "Base desc",
				DescriptionID: "Deskripsi PT",
			},
			wantTitle: "Featured",
			wantDesc:  "Base desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "post", tt.languages, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedByPostType", mock.Anything, "article", tt.languages, 0, 0, 4, 0).Return([]sitessr.ContentRef{
				{Slug: "a1", Title: "A1", Language: "id"},
			}, nil)
			mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil).Maybe()
			mockService.On("GetPublishedTags", mock.Anything).Return([]string{}, nil).Maybe()
			mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()

			mockResolver := new(MockPostTypeResolver)
			mockResolver.On("GetBySlug", "article").Return(tt.resolved, nil)
			mockResolver.On("GetBySlug", mock.AnythingOfType("string")).Return(posttype.PostType{}, assert.AnError).Maybe()

			renderer := tiptap.NewRenderer(nil)
			assembler := sitessr.NewDataAssembler(
				mockService,
				mockResolver,
				nil,
				nil,
				renderer,
				nil,
				tt.languages,
				[]sitessr.HomepageSection{tt.section},
				sitessr.SiteConfig{Name: "Test"},
				0,
			)

			data, err := assembler.BuildHomeData(context.Background(), 1, 0, 0)
			require.NoError(t, err)
			require.Len(t, data.Sections, 1)
			assert.Equal(t, tt.wantTitle, data.Sections[0].Title)
			assert.Equal(t, tt.wantDesc, data.Sections[0].Description)
		})
	}
}

func TestAssembler_IndexListing_LocalizedPostType(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		resolved  posttype.PostType
		wantTitle string
	}{
		{
			name:      "success - Indonesian name_id wins",
			languages: []string{"id", "en"},
			resolved: posttype.PostType{
				Name:   "Product",
				NameID: "Produk",
				Slug:   "product",
			},
			wantTitle: "Produk",
		},
		{
			name:      "success - falls back to base name",
			languages: []string{"id", "en"},
			resolved: posttype.PostType{
				Name: "Product",
				Slug: "product",
			},
			wantTitle: "Product",
		},
		{
			name:      "success - English uses base name",
			languages: []string{"en", "id"},
			resolved: posttype.PostType{
				Name:   "Product",
				NameID: "Produk",
				Slug:   "product",
			},
			wantTitle: "Product",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "product", tt.languages, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil).Maybe()
			mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()

			mockResolver := new(MockPostTypeResolver)
			mockResolver.On("GetBySlug", "product").Return(tt.resolved, nil)

			renderer := tiptap.NewRenderer(nil)
			assembler := sitessr.NewDataAssembler(
				mockService,
				mockResolver,
				nil,
				nil,
				renderer,
				nil,
				tt.languages,
				nil,
				sitessr.SiteConfig{Name: "Test"},
				0,
			)

			data, err := assembler.BuildIndexData(context.Background(), "product", 1, 0, 0)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTitle, data.Title)
		})
	}
}

func TestAssembler_Navigation_Localized(t *testing.T) {
	tests := []struct {
		name        string
		languages   []string
		withCatalog bool
		wantHome    string
		wantProduct string
	}{
		{
			name:        "success - Indonesian home and post type names",
			languages:   []string{"id", "en"},
			withCatalog: true,
			wantHome:    "Beranda",
			wantProduct: "Produk",
		},
		{
			name:        "success - English uses base names",
			languages:   []string{"en", "id"},
			withCatalog: true,
			wantHome:    "Home",
			wantProduct: "Product",
		},
		{
			name:        "success - nil catalog falls back to English home",
			languages:   []string{"id", "en"},
			withCatalog: false,
			wantHome:    "Home",
			wantProduct: "Produk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "post", tt.languages, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"product"}, nil)
			mockService.On("GetPublishedTags", mock.Anything).Return([]string{}, nil).Maybe()
			mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()

			mockResolver := new(MockPostTypeResolver)
			mockResolver.On("GetBySlug", "product").Return(posttype.PostType{Name: "Product", NameID: "Produk", Slug: "product"}, nil)

			renderer := tiptap.NewRenderer(nil)
			assembler := sitessr.NewDataAssembler(
				mockService,
				mockResolver,
				nil,
				nil,
				renderer,
				nil,
				tt.languages,
				nil,
				sitessr.SiteConfig{Name: "Test"},
				0,
			)
			if tt.withCatalog {
				assembler = assembler.WithUICatalog(newTestCatalog(t, "home = \"Home\"\n", "home = \"Beranda\"\n"))
			}

			data, err := assembler.BuildHomeData(context.Background(), 1, 0, 0)
			require.NoError(t, err)
			require.Len(t, data.NavigationItems, 2)
			assert.Equal(t, tt.wantHome, data.NavigationItems[0].Title)
			assert.True(t, data.NavigationItems[0].IsActive)
			assert.Equal(t, "/", data.NavigationItems[0].URL)
			assert.Equal(t, tt.wantProduct, data.NavigationItems[1].Title)
			assert.Equal(t, "/product", data.NavigationItems[1].URL)
			assert.False(t, data.NavigationItems[1].IsActive)
		})
	}
}
