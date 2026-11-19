// Package plugingen is the AI plugin generator module's domain layer: the
// brief, project, and validation value types, the generation/store ports, and
// the envelope parser that validates the model's JSON output. Provider
// adapters (the OpenAI-compatible client) live in infrastructure; the
// filesystem store lives in store; neither is imported by the domain.
package plugingen

import (
	"time"
)

const (
	// maxBriefRunes caps the natural-language brief at 5,000 characters.
	maxBriefRunes = 5000
)

// PluginSources is the prompt grounding: the distilled SDK contract plus the
// hook payload example the generator derives from.
type PluginSources struct {
	Contract string
	Examples string
}

// NewPluginSources assembles the prompt grounding material.
func NewPluginSources(contract, examples string) PluginSources {
	return PluginSources{
		Contract: contract,
		Examples: examples,
	}
}

// Brief is the natural-language plugin request from the admin UI.
type Brief struct {
	Description  string
	Language     string
	Hooks        []string
	Capabilities []string
}

// NewBrief assembles a generation brief from its parts.
func NewBrief(
	description, language string,
	hooks []string,
	capabilities []string,
) Brief {
	return Brief{
		Description:  description,
		Language:     language,
		Hooks:        hooks,
		Capabilities: capabilities,
	}
}

// Project is a validated generated plugin project: an installable slug plus
// the relative-path → content file map the store persists. HTTPPatterns and
// DBPermissions carry the capability manifest's URL and database grants; the
// envelope parser fills them, and the build step synthesizes the TOML
// sidecar from them.
type Project struct {
	Name          string
	Language      string
	Hooks         []string
	Capabilities  []string
	HTTPPatterns  []string
	DBPermissions []string
	Files         map[string]string
	Rationale     string
}

// NewProject assembles a project from its validated parts.
func NewProject(
	name, language string,
	hooks []string,
	capabilities []string,
	files map[string]string,
	rationale string,
) Project {
	return Project{
		Name:         name,
		Language:     language,
		Hooks:        hooks,
		Capabilities: capabilities,
		Files:        files,
		Rationale:    rationale,
	}
}

// InstalledProject is the manifest projection the list endpoint serves:
// identity, language, hooks, creation time, and provenance.
type InstalledProject struct {
	Name      string
	Language  string
	Hooks     []string
	CreatedAt time.Time
	Source    string
}

// NewInstalledProject assembles an installed-project projection.
func NewInstalledProject(
	name, language string,
	hooks []string,
	createdAt time.Time,
	source string,
) InstalledProject {
	return InstalledProject{
		Name:      name,
		Language:  language,
		Hooks:     hooks,
		CreatedAt: createdAt,
		Source:    source,
	}
}
