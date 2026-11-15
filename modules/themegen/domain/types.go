// Package themegen is the AI theme generator module's domain layer: the
// brief, theme, and validation value types, the generation/store ports, and
// the envelope parser that validates the model's JSON output. Provider
// adapters (the OpenAI-compatible client) live in infrastructure; the
// filesystem store lives in store; neither is imported by the domain.
package themegen

import (
	"time"
)

const (
	// EditionBlog renders the blog content site with theme-default.
	EditionBlog Edition = "blog"
	// EditionStorefront renders the commerce content site with theme-storefront.
	EditionStorefront Edition = "storefront"
)

const (
	// maxBriefRunes caps the natural-language brief at 5,000 characters.
	maxBriefRunes = 5000
)

// Edition selects which embedded theme the generator derives from.
type Edition string

// ThemeSources is the edition's readable prompt grounding: the stylesheet
// sources always, plus the template page sources when the brief asks for
// template overrides.
type ThemeSources struct {
	CSS       string
	Templates string
}

// NewThemeSources assembles the edition's readable prompt grounding.
func NewThemeSources(css, templates string) ThemeSources {
	return ThemeSources{
		CSS:       css,
		Templates: templates,
	}
}

// Brief is the natural-language theme request from the admin UI.
type Brief struct {
	Description       string
	Style             string
	TemplateOverrides bool
}

// NewBrief assembles a generation brief from its parts.
func NewBrief(description, style string, templateOverrides bool) Brief {
	return Brief{
		Description:       description,
		Style:             style,
		TemplateOverrides: templateOverrides,
	}
}

// Theme is a validated generated theme: an installable slug plus the
// relative-path → content file map the store persists.
type Theme struct {
	Name      string
	Edition   Edition
	Files     map[string]string
	Rationale string
}

// NewTheme assembles a theme from its validated parts.
func NewTheme(
	name string,
	edition Edition,
	files map[string]string,
	rationale string,
) Theme {
	return Theme{
		Name:      name,
		Edition:   edition,
		Files:     files,
		Rationale: rationale,
	}
}

// InstalledTheme is the manifest projection the list endpoint serves:
// identity, edition, creation time, active marker, and provenance.
type InstalledTheme struct {
	Name      string
	Edition   Edition
	CreatedAt time.Time
	Active    bool
	Source    string
}

// NewInstalledTheme assembles an installed-theme projection.
func NewInstalledTheme(
	name string,
	edition Edition,
	createdAt time.Time,
	active bool,
	source string,
) InstalledTheme {
	return InstalledTheme{
		Name:      name,
		Edition:   edition,
		CreatedAt: createdAt,
		Active:    active,
		Source:    source,
	}
}
