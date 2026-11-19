// Package infrastructure adapts the plugingen domain port onto
// OpenAI-compatible chat-completion APIs. Prompt construction grounds the
// model on the distilled plugin contract; response validation stays in the
// domain's envelope parser.
package infrastructure

import (
	"strings"

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
)

const (
	// pluginContractPrompt is the distilled plugin contract injected into every
	// system prompt: the hook table, the memory protocol, and the host-function
	// vocabulary.
	pluginContractPrompt = `You generate Lesstruct WASM plugins written in Go (TinyGo, WASI target).
Every plugin is one main.go plus go.mod plus a TOML manifest sibling.

Hook contract (WASM export suffix -> canonical hook):
- before_save (BeforeSaveContent): transform titles, validate fields, fill system fields. Return JSON with customFields.
- after_create (AfterCreateContent): notification-style, errors only log.
- after_publish (AfterPublishContent): notification-style, errors only log.
- before_delete (BeforeDeleteContent): FailFast, error aborts the delete.
- after_unpublish (AfterUnpublishContent): notification-style, errors only log.
- on_plugin_loaded (OnPluginLoaded): reserved, fires never in production.

Memory protocol (copy verbatim structure):
- var resultBuf [4096]byte
- //export hook_<suffix>
- func hookXxx(offset uint32, length uint32) uint32 { input := unsafe.Slice(...); copy into resultBuf; return offset of resultBuf }
- //export __hook_result_len
- func hookResultLen(inputLen uint32) uint32 { return resultLen }
- func main() {} // required for WASI

Host functions (module "lesstruct", via //go:wasmimport):
- http_get, http_post (need capabilities.http URL patterns, trailing * only)
- db_query (needs read:<table>), db_exec (needs write:<table>) for tables content, media, users
- log_info, log_error (always available when any manifest exists)

Manifest vocabulary (<name>.manifest TOML):
- name (required), version (required), max_memory_mb (non-negative, advisory)
- [capabilities] http = ["https://api.example.com/*"], database = ["read:content"]
- Valid DB perms: read:content, read:media, read:users, write:content, write:media, write:users
- Never promise rate_limit or max_memory enforcement: they are parsed but not enforced.

Rules:
- Language is always "go" in v1. Ship main.go, go.mod, README.md, build.sh, project.json.
- go.mod must be dependency-free: no require or replace directives (offline build).
- Minimal imports: unsafe only. No os.Exit, no syscall, no cgo.
- Keep JSON handling byte-level (indexOf pattern), no encoding/json in TinyGo unless needed.
- All files under 512 KiB, total under 2 MiB (enforced).
- Never emit javascript: URLs, go:linkname, or extra wasmexports.
- Hooks list must match the exported hook_ functions exactly.
- Declare every grant the plugin needs: httpPatterns lists each http_get/http_post call target (exact URL or trailing-* prefix); dbPermissions lists each db_query/db_exec table (read: for queries, write: for execs). Calls without grants are denied at runtime and fail validation.`

	// hookPayloadExample is the before_save JSON shape the host sends.
	hookPayloadExample = `{"contentId":0,"userId":42,"title":"My Product","content":"<p>Lorem ipsum.</p>","tags":["featured"],"status":"draft","postType":"product","customFields":{"price":29.99}}`

	// maxPromptSourcesRunes bounds injected sources so the system prompt stays
	// within model context.
	maxPromptSourcesRunes = 12000

	// maxRepairFileRunes bounds each file echoed back in a repair prompt.
	maxRepairFileRunes = 16000
)

// languageHint maps the UI language to a directive. Only go ships in v1.
func languageHint(language string) string {
	trimmed := strings.ToLower(strings.TrimSpace(language))
	if trimmed == "" || trimmed == "go" {
		return "Language: go (TinyGo, WASI target). Build with: tinygo build -o <name>.wasm -target=wasi main.go"
	}
	return "Language: go (TinyGo, WASI target). Build with: tinygo build -o <name>.wasm -target=wasi main.go"
}

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
{"name": "<plugin-slug>", "language": "go", "hooks": ["before_save"], "capabilities": ["log_info"], "httpPatterns": ["https://api.example.com/*"], "dbPermissions": ["read:content"], "rationale": "<one-paragraph design explanation>", "files": {"main.go": "...", "go.mod": "...", "README.md": "...", "build.sh": "...", "project.json": "..."}}`
}

// BuildSystemPrompt assembles the plugin generation system prompt from the
// distilled contract plus the edition's grounding sources.
func BuildSystemPrompt(sources plugingendomain.PluginSources) string {
	prompt := pluginContractPrompt
	if strings.TrimSpace(sources.Contract) != "" {
		contract := truncateRunes(sources.Contract, maxPromptSourcesRunes)
		prompt += "\n\n## Active SDK contract (authoritative)\n\n" + contract
	}
	if strings.TrimSpace(sources.Examples) != "" {
		examples := truncateRunes(sources.Examples, maxPromptSourcesRunes)
		prompt += "\n\n## Hook payload example\n\n" + examples
	} else {
		prompt += "\n\n## Hook payload example\n\n" + hookPayloadExample
	}
	prompt += outputFormatSection()
	return prompt
}

// BuildUserPrompt assembles the user prompt from the brief.
func BuildUserPrompt(brief plugingendomain.Brief) string {
	trimmed := strings.TrimSpace(brief.Description)
	if len([]rune(trimmed)) > 5000 {
		trimmed = truncateRunes(trimmed, 5000)
	}
	var b strings.Builder
	b.WriteString(languageHint(brief.Language))
	b.WriteString("\n\nHooks: ")
	b.WriteString(strings.Join(brief.Hooks, ", "))
	if len(brief.Capabilities) > 0 {
		b.WriteString("\nCapabilities: ")
		b.WriteString(strings.Join(brief.Capabilities, ", "))
	}
	b.WriteString("\n\nBrief:\n")
	b.WriteString(trimmed)
	return b.String()
}

// BuildRepairPrompt feeds validation findings back to the model.
func BuildRepairPrompt(brief plugingendomain.Brief, projectFiles map[string]string, problems []string) string {
	var b strings.Builder
	b.WriteString(languageHint(brief.Language))
	b.WriteString("\n\nThe previous plugin failed validation. Fix every problem below and return the full corrected JSON envelope.\n\nProblems:\n")
	for _, problem := range problems {
		b.WriteString("- ")
		b.WriteString(problem)
		b.WriteString("\n")
	}
	b.WriteString("\nPrevious files (fix these):\n")
	for path, content := range projectFiles {
		b.WriteString("--- ")
		b.WriteString(path)
		b.WriteString(" ---\n")
		b.WriteString(truncateRunes(content, maxRepairFileRunes))
		b.WriteString("\n")
	}
	return b.String()
}
