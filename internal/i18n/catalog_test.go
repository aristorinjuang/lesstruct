package i18n_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCatalog(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "en.toml"),
		[]byte("[ui]\nhello = \"Hello\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(dir, "id.toml"),
		[]byte("[ui]\nhello = \"Halo\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	c, err := i18n.NewCatalog(os.DirFS(dir), []string{"en", "id"})
	require.NoError(t, err)
	assert.Equal(t, "Hello", c.T("en", "ui.hello"))
	assert.Equal(t, "Halo", c.T("id", "ui.hello"))
}

func TestNewCatalog_MissingNonPrimaryLanguage(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "en.toml"),
		[]byte("[ui]\nhello = \"Hello\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	// "id" file doesn't exist — should be skipped
	c, err := i18n.NewCatalog(os.DirFS(dir), []string{"en", "id"})
	require.NoError(t, err)
	assert.Equal(t, "Hello", c.T("en", "ui.hello"))
}

func TestNewCatalog_MissingPrimaryLanguage(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "id.toml"),
		[]byte("[ui]\nhello = \"Halo\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	_, err := i18n.NewCatalog(os.DirFS(dir), []string{"en", "id"})
	require.Error(t, err)
}

func TestNewCatalog_MissingDir(t *testing.T) {
	_, err := i18n.NewCatalog(os.DirFS("/nonexistent/path/translations"), []string{"en"})
	require.Error(t, err)
}

func TestNewCatalog_InvalidTOML(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "en.toml"),
		[]byte(`this is not valid toml {{{`),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	_, err := i18n.NewCatalog(os.DirFS(dir), []string{"en"})
	require.Error(t, err)
}

func TestT_FallbackChain(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "en.toml"),
		[]byte("[ui]\nhello = \"Hello\"\nonly_en = \"Only English\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(dir, "fr.toml"),
		[]byte("[ui]\nhello = \"Bonjour\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	c, err := i18n.NewCatalog(os.DirFS(dir), []string{"en", "fr"})
	require.NoError(t, err)

	tests := []struct {
		name     string
		lang     string
		key      string
		expected string
	}{
		{
			name:     "direct lookup in French",
			lang:     "fr",
			key:      "ui.hello",
			expected: "Bonjour",
		},
		{
			name:     "direct lookup in English",
			lang:     "en",
			key:      "ui.hello",
			expected: "Hello",
		},
		{
			name:     "fallback to primary (en) when missing in fr",
			lang:     "fr",
			key:      "ui.only_en",
			expected: "Only English",
		},
		{
			name:     "fallback to key itself when missing everywhere",
			lang:     "fr",
			key:      "ui.missing",
			expected: "ui.missing",
		},
		{
			name:     "fallback to key when lang unknown",
			lang:     "zz",
			key:      "ui.hello",
			expected: "Hello",
		},
		{
			name:     "empty lang falls back to primary",
			lang:     "",
			key:      "ui.hello",
			expected: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.T(tt.lang, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestT_EmptyCatalog(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "en.toml"),
		[]byte{},
		0644,
	); err != nil {
		t.Fatal(err)
	}

	c, err := i18n.NewCatalog(os.DirFS(dir), []string{"en"})
	require.NoError(t, err)

	// Empty file — everything falls through to key itself
	assert.Equal(t, "ui.anykey", c.T("en", "ui.anykey"))
}

func TestNewCatalog_EmbeddedFS(t *testing.T) {
	c, err := i18n.NewCatalog(i18n.Embedded(), []string{"en", "id"})
	require.NoError(t, err)

	// Verify real translations load from embedded FS
	assert.Equal(t, "Login", c.T("en", "ui.login"))
	assert.Equal(t, "Masuk", c.T("id", "ui.login"))

	// Fallback: fr missing → falls back to primary en
	assert.Equal(t, "Login", c.T("fr", "ui.login"))
}
