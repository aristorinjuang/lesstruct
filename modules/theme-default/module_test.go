package themedefault_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	"github.com/aristorinjuang/lesstruct/modules/theme-default/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, themedefault.Name, module.Name())
	assert.Equal(t, themedefault.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_ThemeFiles(t *testing.T) {
	module := testutil.NewModule()

	files := module.ThemeFiles()
	require.NotNil(t, files)

	// Every layout and asset the engine resolves must be present under the
	// historical paths.
	for _, path := range []string{
		"pages/layout.gohtml",
		"pages/post.gohtml",
		"pages/index.gohtml",
		"pages/homepage.gohtml",
		"pages/author.gohtml",
		"pages/tag.gohtml",
		"pages/not_found.gohtml",
		"pages/login.gohtml",
		"pages/register.gohtml",
		"pages/forgot_password.gohtml",
		"pages/verify_email.gohtml",
		"pages/reset_password.gohtml",
		"static/base.css",
		"static/style.css",
	} {
		_, err := fs.Stat(files, path)
		assert.NoError(t, err, "theme file %q must exist", path)
	}
}

func TestModule_ThemeFuncs(t *testing.T) {
	module := testutil.NewModule()

	assert.NotNil(t, module.ThemeFuncs())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestModule_ImplementsThemeProvider(t *testing.T) {
	var _ kernel.ThemeProvider = testutil.NewModule()
}
