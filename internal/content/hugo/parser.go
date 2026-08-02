package hugo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

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

func groupTranslations(items []*HugoItem) []any {
	dirMap := make(map[string]*translationGroup)

	for _, item := range items {
		dir := filepath.Dir(item.FilePath)

		if item.Language == "id" {
			if pair, ok := dirMap[dir]; ok {
				pair.Indonesian = item
			} else {
				dirMap[dir] = &translationGroup{Indonesian: item}
			}
		} else {
			if pair, ok := dirMap[dir]; ok {
				pair.English = item
			} else {
				dirMap[dir] = &translationGroup{English: item}
			}
		}
	}

	var result []interface{}
	seen := make(map[string]bool)

	for _, item := range items {
		dir := filepath.Dir(item.FilePath)
		if seen[dir] {
			continue
		}
		seen[dir] = true

		pair := dirMap[dir]
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
