package sitessr_test

import (
	"testing"
	"time"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/aristorinjuang/lesstruct/modules/site-ssr/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, sitessr.Name, module.Name())
	assert.Equal(t, sitessr.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Equal(t, []string{"theme-default"}, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestContentRef_ProjectionShape(t *testing.T) {
	groupID := 3
	ref := sitessr.ContentRef{
		ID:                 1,
		Slug:               "hello",
		Title:              "Hello",
		Content:            `{"type":"doc"}`,
		Tags:               []string{"go"},
		Format:             sitessr.FormatHTML,
		PostType:           "post",
		Language:           "en",
		TranslationGroupID: &groupID,
		Author:             "Author",
		Username:           "author",
		CreatedAt:          time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
		AllowComments:      true,
		CustomFields:       map[string]any{"k": "v"},
		MetaDescription:    "desc",
		OGTitle:            "og",
		OGDescription:      "og desc",
	}

	assert.Equal(t, 1, ref.ID)
	assert.Equal(t, "hello", ref.Slug)
	assert.Equal(t, "html", ref.Format)
	assert.Equal(t, &groupID, ref.TranslationGroupID)
	assert.True(t, ref.AllowComments)
}
