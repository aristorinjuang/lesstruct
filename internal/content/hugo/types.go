package hugo

import "time"

type translationGroup struct {
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
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}
