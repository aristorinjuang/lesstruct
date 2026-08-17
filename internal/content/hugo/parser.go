package hugo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type yamlFrontmatter struct {
	Date             time.Time `yaml:"date"`
	Description      string    `yaml:"description"`
	Tags             []string  `yaml:"tags"`
	Title            string    `yaml:"title"`
	URL              string    `yaml:"url"`
	Images           []string  `yaml:"images"`
	Aliases          []string  `yaml:"aliases"`
	Language         string    `yaml:"language"`
	HasMath          *bool     `yaml:"hasMath"`
	HasChart         *bool     `yaml:"hasChart"`
	HasDiagrams      *bool     `yaml:"hasDiagrams"`
	HideMobileImages *bool     `yaml:"hideMobileImages"`
	Draft            *bool     `yaml:"draft"`
}

var frontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n(.*?)\n---\s*\n(.*)\z`)

func ParseContentFile(path string) (*HugoItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	matches := frontmatterRe.FindSubmatch(data)
	if matches == nil {
		return nil, fmt.Errorf("no frontmatter found in %s", path)
	}

	var fm yamlFrontmatter
	if err := yaml.Unmarshal(matches[1], &fm); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter in %s: %w", path, err)
	}

	body := string(matches[2])
	body = strings.TrimSpace(body)

	lang := "en"
	if fm.Language != "" {
		lang = fm.Language
	} else if strings.HasSuffix(filepath.Base(path), ".id.html") {
		lang = "id"
	}

	hasMath := false
	if fm.HasMath != nil {
		hasMath = *fm.HasMath
	}
	hasChart := false
	if fm.HasChart != nil {
		hasChart = *fm.HasChart
	}
	hasDiagrams := false
	if fm.HasDiagrams != nil {
		hasDiagrams = *fm.HasDiagrams
	}
	hideMobileImages := false
	if fm.HideMobileImages != nil {
		hideMobileImages = *fm.HideMobileImages
	}
	isDraft := false
	if fm.Draft != nil {
		isDraft = *fm.Draft
	}

	slug := fm.URL
	if slug != "" {
		slug = strings.TrimPrefix(slug, "/")
		slug = strings.TrimSuffix(slug, ".html")
	}

	var aliases []string
	if fm.Aliases != nil {
		aliases = make([]string, len(fm.Aliases))
		for i, a := range fm.Aliases {
			aliases[i] = strings.TrimPrefix(a, "/")
		}
	}
	// Add the old .html URL as an alias
	if fm.URL != "" {
		oldURL := strings.TrimPrefix(fm.URL, "/")
		if oldURL != "" {
			if !slices.Contains(aliases, oldURL) {
				aliases = append(aliases, oldURL)
			}
		}
	}

	return &HugoItem{
		Title:            fm.Title,
		Date:             fm.Date,
		Description:      fm.Description,
		Tags:             fm.Tags,
		URL:              slug,
		Images:           fm.Images,
		Body:             body,
		Language:         lang,
		Aliases:          aliases,
		HasMath:          hasMath,
		HasChart:         hasChart,
		HasDiagrams:      hasDiagrams,
		HideMobileImages: hideMobileImages,
		IsDraft:          isDraft,
		FilePath:         path,
		OriginalBody:     body,
	}, nil
}

// translationKey identifies the logical post a HugoItem belongs to, so
// language variants can be paired without collapsing distinct posts that share
// a directory. The `.id` language suffix (or frontmatter `language: id`) is
// stripped from the basename: `foo.html` and `foo.id.html` in the same
// directory share a key, while `a.html` and `b.html` stay separate.
func translationKey(item *HugoItem) string {
	base := filepath.Base(item.FilePath)
	if item.Language == "id" {
		lower := strings.ToLower(base)
		for _, ext := range []string{".id.html", ".id.md"} {
			if before, ok := strings.CutSuffix(lower, ext); ok {
				// Strip the ".id" language marker, keeping the real extension
				// (".id.html" -> ".html", ".id.md" -> ".md").
				base = before + ext[3:]
				break
			}
		}
	}
	return filepath.Join(filepath.Dir(item.FilePath), base)
}

func GroupTranslations(items []*HugoItem) []any {
	pairMap := make(map[string]*TranslationGroup)

	for _, item := range items {
		key := translationKey(item)

		if item.Language == "id" {
			if pair, ok := pairMap[key]; ok {
				pair.Indonesian = item
			} else {
				pairMap[key] = &TranslationGroup{Indonesian: item}
			}
		} else {
			if pair, ok := pairMap[key]; ok {
				pair.English = item
			} else {
				pairMap[key] = &TranslationGroup{English: item}
			}
		}
	}

	var result []any
	seen := make(map[string]bool)

	for _, item := range items {
		key := translationKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true

		pair := pairMap[key]
		if pair.English != nil && pair.Indonesian != nil {
			result = append(result, *pair)
		} else if pair.English != nil {
			result = append(result, pair.English)
		} else if pair.Indonesian != nil {
			result = append(result, pair.Indonesian)
		}
	}

	return result
}

func WalkContentTree(root string) (*HugoSite, error) {
	site := &HugoSite{
		SourcePath: root,
	}

	var items []*HugoItem

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".html" && ext != ".md" {
			return nil
		}

		item, parseErr := ParseContentFile(path)
		if parseErr != nil {
			return fmt.Errorf("failed to parse %s: %w", path, parseErr)
		}

		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk content tree: %w", err)
	}

	site.Items = removeDups(items)
	return site, nil
}

func removeDups(items []*HugoItem) []*HugoItem {
	seen := make(map[string]bool)
	var result []*HugoItem
	for _, item := range items {
		key := item.FilePath
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

// hugoConfig is the subset of Hugo's site configuration that the importer
// understands. Only top-level scalar keys are decoded; language/custom tables
// are ignored. Tags cover both TOML and YAML config variants.
type hugoConfig struct {
	BaseURL                string `toml:"baseURL" yaml:"baseURL"`
	DefaultContentLanguage string `toml:"defaultContentLanguage" yaml:"defaultContentLanguage"`
}

// configFileCandidates lists Hugo config file names in precedence order:
// modern Hugo (0.110+) prefers hugo.toml over config.toml; both TOML and YAML
// variants are recognized. The first existing file wins.
func configFileCandidates(root string) []string {
	return []string{
		filepath.Join(root, "hugo.toml"),
		filepath.Join(root, "config.toml"),
		filepath.Join(root, "hugo.yaml"),
		filepath.Join(root, "config.yaml"),
	}
}

// LoadSiteConfig reads the Hugo site configuration from the archive root.
// Missing config files are not an error — the importer falls back to defaults
// (empty base URL, "en" default content language).
func LoadSiteConfig(root string) (SiteConfig, error) {
	cfg := SiteConfig{}

	for _, candidate := range configFileCandidates(root) {
		data, err := os.ReadFile(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return cfg, fmt.Errorf("failed to read %s: %w", candidate, err)
		}

		switch strings.ToLower(filepath.Ext(candidate)) {
		case ".toml":
			var parsed hugoConfig
			if err := toml.Unmarshal(data, &parsed); err != nil {
				return cfg, fmt.Errorf("failed to parse %s: %w", candidate, err)
			}
			cfg.BaseURL = strings.TrimSuffix(parsed.BaseURL, "/")
			cfg.DefaultContentLanguage = parsed.DefaultContentLanguage
		case ".yaml", ".yml":
			var parsed hugoConfig
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				return cfg, fmt.Errorf("failed to parse %s: %w", candidate, err)
			}
			cfg.BaseURL = strings.TrimSuffix(parsed.BaseURL, "/")
			cfg.DefaultContentLanguage = parsed.DefaultContentLanguage
		}
		break
	}

	if cfg.DefaultContentLanguage == "" {
		cfg.DefaultContentLanguage = "en"
	}
	return cfg, nil
}
