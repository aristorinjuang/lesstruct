package i18n

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"github.com/BurntSushi/toml"
)

//go:embed translations/*
var translationsRaw embed.FS

// Embedded returns the embedded file system containing translation TOML files,
// rooted at the translations/ directory so callers can read "en.toml" directly.
func Embedded() fs.FS {
	sub, err := fs.Sub(translationsRaw, "translations")
	if err != nil {
		log.Fatalf("i18n: embedded translations/ directory missing: %v", err)
	}
	return sub
}

// Catalog holds UI translation messages for all configured languages.
type Catalog struct {
	messages    map[string]map[string]string // lang -> key -> value
	primaryLang string
}

// NewCatalog loads TOML translation files from fsys for each configured language.
// The primary language is the first entry in languages. Files are expected as
// <lang>.toml (e.g. "en.toml", "fr.toml"). Missing files are silently skipped
// as long as at least the primary language file exists.
func NewCatalog(fsys fs.FS, languages []string) (*Catalog, error) {
	messages := make(map[string]map[string]string)
	primaryLang := "en"
	if len(languages) > 0 {
		primaryLang = languages[0]
	}

	for _, lang := range languages {
		filePath := lang + ".toml"
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && lang != primaryLang {
				continue
			}
			return nil, fmt.Errorf("reading translation file %s: %w", filePath, err)
		}

		var raw map[string]map[string]string
		if err := toml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parsing translation file %s: %w", filePath, err)
		}

		flat := make(map[string]string)
		for section, keys := range raw {
			for k, v := range keys {
				flat[section+"."+k] = v
			}
		}

		messages[lang] = flat
	}

	return &Catalog{
		messages:    messages,
		primaryLang: primaryLang,
	}, nil
}

// T returns the translation for key in the given language.
//
// Fallback chain:
//  1. lang → key
//  2. primary language (first in languages config) → key
//  3. "en" → key
//  4. return key itself
func (c *Catalog) T(lang, key string) string {
	if val := c.lookup(lang, key); val != "" {
		return val
	}
	if val := c.lookup(c.primaryLang, key); val != "" {
		return val
	}
	if val := c.lookup("en", key); val != "" {
		return val
	}
	return key
}

func (c *Catalog) lookup(lang, key string) string {
	if msgs, ok := c.messages[lang]; ok {
		if val, ok := msgs[key]; ok {
			return val
		}
	}
	return ""
}
