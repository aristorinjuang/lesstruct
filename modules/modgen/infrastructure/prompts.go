// Package infrastructure adapts the modgen domain port onto
// OpenAI-compatible chat-completion APIs. Prompt construction grounds the
// model on the distilled module contract; response validation stays in the
// domain's envelope parser.
package infrastructure

import (
	"strings"

	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
)

const (
	// moduleContractPrompt is the distilled module contract injected into every
	// system prompt: the file grammar, the kernel lifecycle, and the layer
	// rules the validator enforces.
	moduleContractPrompt = `You generate Lesstruct Tier 1 Go modules: source compiled into the server binary.

File grammar (exact allowlist, nothing else):
- go.mod, module.go, README.md at the root (all three required)
- domain/types.go (entities + Err* sentinels), domain/repository.go (port interface), domain/service.go (application service), domain/service_test.go
- api/handler.go (HTTP handlers + narrow WidgetStore-style seams), store/store.go (single-source *sql.DB + kernel Dialect adapter), admin/manifest.go (AdminExtensionManifest), testutil/testutil.go (NewModule/Kernel fixture)
- embed.go (migration FS, //go:embed inside the var block) plus migrations/{sqlite,postgresql,mysql}/NNN_name.up.sql and NNN_name.down.sql (bare driver dirs, NNN = 001...)
- File names: lowercase letters, digits, underscore + .go. Module (directory) name: lowercase alphanumerics, hyphen-free, 2-50 runes (it is the Go package name).

Kernel lifecycle (module.go):
- const Name = "<name>" (registry key) and const Version = "0.1.0" (first release always 0.1.0)
- var _ kernel.Module = (*Module)(nil) witness
- type Option func(*Module); adoption options are nil-safe without collaborators (a custom edition wires through these without editing the module): WithLogger(logger kernel.Logger) keeps any existing logger when non-nil; WithDB(db *sql.DB, dialect kerneldatabase.Dialect) adopts the store collaborators. EVERY generated module exposes BOTH options, even leaf modules without a store.
- type Module struct with service/handler/logger fields; Name(), Version(), Requires() (nil for leaves; every entry is a transitive-disable edge), Optional(), Init() (validates presence only — tables do not exist yet, migrations run after InitAll), Shutdown(), AdminManifest() when the admin surface is chosen
- func New(opts ...Option) *Module LAST: applies options, defaults, builds handlers once, returns m. Nothing follows the constructor.
- File layout: const block, then var block, then private helpers, then public declarations (each struct followed by its methods, constructor last). Every exported symbol carries a godoc comment starting with its name. Never use interface{} (use any). HTTP methods use http.MethodGet constants. Modern Go (for i := range n, slices/maps/min/max builtins).

Layer rules (fail validation otherwise):
- domain/ imports stdlib + kernel published language only: NEVER net/http, database/sql, or lesstruct/internal in domain/
- NO file may import lesstruct/internal/... (depguard)
- api/ decodes transport, reads the principal via kernel.PrincipalFromContext, maps domain errors with errors.Is, and writes EVERY response through kernel/httpx (Success/Error envelope) — a module whose api/ never references httpx fails
- store/ implements the repository port on *sql.DB + kernel/database Dialect; no driver types leak upward
- sentinels translate at the seam with errors.Is
- manifest validates centrally; AdminManifest returns kernel.AdminExtensionManifest with menu entries (Label, Path absolute, Icon), no duplicate paths

Migrations (when the migrations surface is chosen):
- implement kernel.Migrator with schema_migrations_<name> tracking; idempotent CREATE TABLE IF NOT EXISTS so pre-module installs upgrade cleanly
- NEVER foreign-key another module's table (no REFERENCES): index the reference column instead
- Down migrations drop in reverse order

Surfaces (declare exactly what the brief needs):
- routes: protected authenticated endpoints (RouteProvider)
- public_routes: anonymous endpoints (PublicRouteProvider)
- migrations: versioned tables (Migrator)
- posttypes: registered content types (ContentTypeProvider)
- roles: capability strings (RoleProvider)
- themes: embedded layouts for the SSR engine (ThemeProvider)
- admin: admin-shell menu/settings/editor entries (AdminExtensionProvider)

Rules:
- go.mod: module github.com/aristorinjuang/lesstruct/modules/<name>; require lesstruct/kernel at the pinned host version PLUS ONLY the testify family (testify, objx, go-spew, go-difflib, yaml.v3) and github.com/go-chi/chi/v5 for route surfaces; NO other requires (unreviewed dependencies execute during verification); NO replace directives (local checkouts resolve through go.work)
- All Go files under 512 KiB, project under 4 MiB, at most 64 files
- Never emit: replace directives, javascript: URLs, go:linkname, cgo (import "C"), os/exec, unsafe, syscall, plugin, or net/* imports (net/http handler constants excepted), lesstruct/internal imports, net/http or database/sql inside domain/
- Root README.md documents what the module owns, its seams, and the [modules] disabled cascade behavior of its Requires()
- Handlers cap request bodies with http.MaxBytesReader on anonymous routes`

	// kernelVersionLine pins the host kernel version in the system prompt.
	kernelVersionLine = `Host kernel version (authoritative — go.mod MUST require exactly this): `

	// maxPromptSourcesRunes bounds injected sources so the system prompt stays
	// within model context.
	maxPromptSourcesRunes = 12000

	// maxRepairFileRunes bounds each file echoed back in a repair prompt.
	maxRepairFileRunes = 16000
	// maxRepairPromptRunes bounds the whole repair prompt: without it a
	// max-file-count project echoes ~1M runes and every repair call fails
	// on model context.
	maxRepairPromptRunes = 200000
)

// truncateRunes shortens s to at most max runes.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// outputFormatSection renders the JSON envelope instructions.
func outputFormatSection() string {
	return `

## Output format

Output ONLY a JSON object with exactly these keys — no markdown fences, no explanation, no commentary:
{"name": "<module-name>", "surfaces": ["routes"], "rationale": "<one-paragraph design explanation>", "files": {"go.mod": "...", "module.go": "...", "README.md": "...", "domain/types.go": "...", "domain/repository.go": "...", "domain/service.go": "...", "api/handler.go": "...", "store/store.go": "...", "admin/manifest.go": "..."}}`
}

// BuildSystemPrompt assembles the module generation system prompt from the
// distilled contract plus the edition's grounding sources.
func BuildSystemPrompt(sources modgendomain.ModuleSources) string {
	prompt := moduleContractPrompt
	if strings.TrimSpace(sources.KernelVersion) != "" {
		prompt += "\n\n" + kernelVersionLine + strings.TrimSpace(sources.KernelVersion)
	}
	if strings.TrimSpace(sources.Contract) != "" {
		contract := truncateRunes(sources.Contract, maxPromptSourcesRunes)
		prompt += "\n\n## Active kernel contract (authoritative)\n\n" + contract
	}
	if strings.TrimSpace(sources.Grammar) != "" {
		grammar := truncateRunes(sources.Grammar, maxPromptSourcesRunes)
		prompt += "\n\n## Module grammar (authoritative)\n\n" + grammar
	}
	if strings.TrimSpace(sources.Examples) != "" {
		examples := truncateRunes(sources.Examples, maxPromptSourcesRunes)
		prompt += "\n\n## Exemplar module sources\n\n" + examples
	}
	prompt += outputFormatSection()
	return prompt
}

// BuildUserPrompt assembles the user prompt from the brief.
func BuildUserPrompt(brief modgendomain.Brief) string {
	trimmed := strings.TrimSpace(brief.Description)
	if len([]rune(trimmed)) > 5000 {
		trimmed = truncateRunes(trimmed, 5000)
	}
	var b strings.Builder
	if len(brief.Surfaces) > 0 {
		b.WriteString("Surfaces: ")
		b.WriteString(strings.Join(brief.Surfaces, ", "))
		b.WriteString("\n\n")
	} else {
		b.WriteString("Surfaces: infer from the brief (choose from routes, public_routes, migrations, posttypes, roles, themes, admin).\n\n")
	}
	b.WriteString("Brief:\n")
	b.WriteString(trimmed)
	return b.String()
}

// BuildRepairPrompt feeds validation findings back to the model.
func BuildRepairPrompt(brief modgendomain.Brief, moduleFiles map[string]string, problems []string) string {
	var b strings.Builder
	if len(brief.Surfaces) > 0 {
		b.WriteString("Surfaces: ")
		b.WriteString(strings.Join(brief.Surfaces, ", "))
		b.WriteString("\n\n")
	}
	b.WriteString("The previous module failed validation. Fix every problem below and return the full corrected JSON envelope.\n\nProblems:\n")
	for _, problem := range problems {
		b.WriteString("- ")
		b.WriteString(problem)
		b.WriteString("\n")
	}
	b.WriteString("\nPrevious files (fix these):\n")
	for filepath, content := range moduleFiles {
		if b.Len() >= maxRepairPromptRunes {
			b.WriteString("--- (remaining files omitted: prompt budget reached) ---\n")
			break
		}
		b.WriteString("--- ")
		b.WriteString(filepath)
		b.WriteString(" ---\n")
		b.WriteString(truncateRunes(content, maxRepairFileRunes))
		b.WriteString("\n")
	}
	return b.String()
}
