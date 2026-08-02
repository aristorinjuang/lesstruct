package hugo

import "time"

// TranslationGroup pairs an English Hugo item with its Indonesian variant
// when both exist for the same logical post (page bundle or leaf file).
type TranslationGroup struct {
	English    *HugoItem
	Indonesian *HugoItem
}

type HugoItem struct {
	Title            string
	Date             time.Time
	Description      string
	Tags             []string
	URL              string
	Images           []string
	Body             string
	Language         string
	Aliases          []string
	HasMath          bool
	HasChart         bool
	HasDiagrams      bool
	HideMobileImages bool
	IsDraft          bool
	FilePath         string
	OriginalBody     string
}

type HugoSite struct {
	Items      []*HugoItem
	StaticDir  string
	SourcePath string
	Config     SiteConfig
}

// SiteConfig carries the minimal Hugo site configuration the importer acts on.
type SiteConfig struct {
	BaseURL                string
	DefaultContentLanguage string
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}
