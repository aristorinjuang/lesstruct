package sitessr_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/i18n"
	"github.com/aristorinjuang/lesstruct/kernel/tiptap"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestCatalog(t *testing.T, enExtra, idExtra string) *i18n.Catalog {
	t.Helper()
	dir := t.TempDir()
	enTOML := "[ui]\nbrowse_posts = \"Browse %s.\"\nposts_by = \"Posts by %s.\"\nposts_tagged = \"Posts tagged %q.\"\n" + enExtra
	idTOML := "[ui]\nbrowse_posts = \"Jelajahi %s.\"\nposts_by = \"Postingan oleh %s.\"\nposts_tagged = \"Postingan dengan tag %q.\"\n" + idExtra
	require.NoError(t, os.WriteFile(filepath.Join(dir, "en.toml"), []byte(enTOML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "id.toml"), []byte(idTOML), 0644))
	catalog, err := i18n.NewCatalog(os.DirFS(dir), []string{"en", "id"})
	require.NoError(t, err)
	return catalog
}

func TestAssembler_UICatalog_Listings(t *testing.T) {
	tests := []struct {
		name       string
		languages  []string
		catalog    func(t *testing.T) *i18n.Catalog
		wantBrowse string
		wantBy     string
		wantTagged string
	}{
		{
			name:      "success - Indonesian listings",
			languages: []string{"id", "en"},
			catalog: func(t *testing.T) *i18n.Catalog {
				return newTestCatalog(t, "", "")
			},
			wantBrowse: "Jelajahi Menu Item.",
			wantBy:     "Postingan oleh Admin.",
			wantTagged: "Postingan dengan tag \"tee\".",
		},
		{
			name:      "success - fallback to English when Indonesian key missing",
			languages: []string{"id", "en"},
			catalog: func(t *testing.T) *i18n.Catalog {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "en.toml"), []byte("[ui]\nbrowse_posts = \"Browse %s.\"\nposts_by = \"Posts by %s.\"\nposts_tagged = \"Posts tagged %q.\"\n"), 0644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "id.toml"), []byte("[ui]\n"), 0644))
				c, err := i18n.NewCatalog(os.DirFS(dir), []string{"en", "id"})
				require.NoError(t, err)
				return c
			},
			wantBrowse: "Browse Menu Item.",
			wantBy:     "Posts by Admin.",
			wantTagged: "Posts tagged \"tee\".",
		},
		{
			name:      "success - nil catalog falls back to English",
			languages: []string{"id", "en"},
			catalog: func(t *testing.T) *i18n.Catalog {
				return nil
			},
			wantBrowse: "Browse Menu Item.",
			wantBy:     "Posts by Admin.",
			wantTagged: "Posts tagged \"tee\".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "menu-item", tt.languages, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", tt.languages, 51, 0).Return([]sitessr.ContentRef{
				{Slug: "post-1", Title: "Post", Author: "Admin", Language: "id"},
			}, nil)
			mockService.On("GetPublishedByTag", mock.Anything, "tee", tt.languages, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil).Maybe()
			mockService.On("GetPublishedTags", mock.Anything).Return([]string{}, nil).Maybe()
			mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil).Maybe()
			mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()

			mockResolver := new(MockPostTypeResolver)
			mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)
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
				nil,
				sitessr.SiteConfig{Name: "Test"},
				0,
			)
			if c := tt.catalog(t); c != nil {
				assembler = assembler.WithUICatalog(c)
			}

			indexData, err := assembler.BuildIndexData(context.Background(), "menu-item", 1, 0, 0)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBrowse, indexData.Description)

			authorData, err := assembler.BuildAuthorData(context.Background(), "admin", 1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBy, authorData.Description)

			tagData, err := assembler.BuildTagData(context.Background(), "tee", 1, 0, 0)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTagged, tagData.Description)
		})
	}
}
