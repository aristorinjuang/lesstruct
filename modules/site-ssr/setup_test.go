package sitessr_test

import (
	"os"
	"testing"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
)

// TestMain installs the default theme artifact once for the whole package.
// Page rendering resolves embedded layouts through the engine, whose files
// moved to the theme-default module in Phase 3.5a.
func TestMain(m *testing.M) {
	sitessr.SetEmbeddedFS(themedefault.Files())
	os.Exit(m.Run())
}
