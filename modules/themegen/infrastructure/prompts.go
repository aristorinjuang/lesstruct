// Package infrastructure adapts the themegen domain port onto
// OpenAI-compatible chat-completion APIs. Prompt construction grounds the
// model on the active edition's readable sources; response validation stays
// in the domain's envelope parser.
package infrastructure

import (
	"fmt"
	"strings"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

const (
	// themeContractPrompt is the distilled theme contract injected into every
	// system prompt: the token catalog from docs/theme-development.md. The
	// model derives (token overrides + component restyling) instead of
	// inventing from scratch.
	themeContractPrompt = `You generate Lesstruct content-site themes. Every theme ships two
stylesheets: static/base.css (design tokens + base elements) and
static/style.css (components + layout). The engine layers the installed
theme over the embedded default per file (resolveFS fallback), so a
stylesheet-only theme is already complete.

Token catalog (reuse verbatim, never invent new --accent/--bg tokens,
never hardcode hex anywhere except token definitions in base.css):
- Color: var(--color-primary), var(--color-primary-hover), var(--color-secondary), var(--color-accent), var(--color-text), var(--color-text-muted), var(--color-bg), var(--color-card-bg), var(--color-border), var(--color-success), var(--color-danger), var(--color-link)
- Spacing: var(--space-1) (0.25rem) ... var(--space-8) (3rem)
- Radius: var(--radius-sm), var(--radius-md), var(--radius-lg)
- Shadow: var(--shadow-sm), var(--shadow-md), var(--shadow-lg)
- Focus ring: var(--ring)
- Fonts: var(--font-sans) (body, already set), var(--font-mono)
- Transition: var(--transition-fast)

Component classes the layout expects (restyle, do not rename):
.container (max-width page wrapper), .btn (already wired to
--color-primary, hover, and focus ring), .form-control, .alert and
.alert--success/.alert--error, header/nav/footer shells, post cards.

Rules:
- base.css holds :root tokens, @font-face (self-hosted
  /static/inter-latin-var.woff2 only), reset, body typography.
- style.css holds header, nav, hero, cards, buttons, forms, footer,
  responsive breakpoints, prefers-reduced-motion.
- No <script>, no on* handlers, no javascript: URLs, no external
  resources (no @import, no external fonts, no remote url() hosts).
- url() may reference /static/... paths and data: URIs only.
- Dark recipe when requested: flip --color-bg/--color-text/
  --color-text-muted/--color-card-bg/--color-border, keep brand hues.
- Minimal CSS: prefer token overrides over full rewrites.`

	// templateOverridesContract is appended only when the brief asks for page
	// templates. It documents the partial-theme fallback, the file map, the
	// {{define "body"}} convention, the data surface, and the funcs.
	templateOverridesContract = `

## Template overrides (opt-in)

This brief also asks for page templates. Ship templates/ files for the pages
the design changes; every file you omit falls back to the embedded default, so
a partial template set is valid.

Files (relative to templates/ in the JSON map):
- layout.html — the shared page shell. Define {{define "layout"}}...{{end}}
  and render the page body with {{template "body" .}} inside <main>.
- index.html — the main listing. homepage.html — the configured-sections
  homepage.
- post.html — the default content page. <postType>.html — a per-post-type
  content page (e.g. page.html). <postType>-<slug>.html — a per-slug override
  (e.g. page-about.html).
- author.html, tag.html, not_found.html.
- login.html, register.html, forgot_password.html, verify_email.html,
  reset_password.html.
Every page file other than layout.html must be exactly one
{{define "body"}}...{{end}} block; the shared layout renders it inside
<main>. A file without that wrapper fails validation.

Data surface (html/template auto-escapes every value):
- Every page embeds LayoutData: .Title, .Description, .OGTitle, .OGDesc,
  .OGImage, .PageTitle, .NavigationItems and .FooterItems ([]{Title,URL,IsActive}),
  .Legal {CookiePolicyURL, PrivacyPolicyURL, TermsOfServiceURL, ReturnsPolicyURL,
  CookieNotice{Before,LinkText,After,URL}, CheckoutConsent, RegistrationConsent},
  .CurrentPath, .Lang, .LanguageLinks and .Languages ([]{Code,Name,URL}),
  .SiteConfig {Name,Logo,Favicon,ProductImageRatio,HeroImage,HeroTitle,HeroSubtitle},
  .NoIndex.
- index.html and homepage.html: IndexData — .Posts []PostItem, .Tags []string,
  .Sections []HomeSection {PostTypeSlug,Title,Description,URL,Posts}, plus
  .CurrentPage, .HasPrev, .HasNext, .PrevURL, .NextURL.
- post.html and post-type pages: ContentData — .Slug, .Title, .Body (safe HTML),
  .Tags, .Author, .Username, .AuthorAvatarURL, .CreatedAt, .AllowComments,
  .CustomFields map, .CustomFieldsFormatted []{Label,Value},
  .CustomFieldGalleries map[string][]GalleryImage, .Related []PostItem,
  .Comments []CommentItem {Author,Text,CreatedAt}, .PostType, .PostScripts
  (safe HTML).
- author.html: AuthorData — .AuthorName, .Username, .AuthorAvatarURL, .Posts,
  .CustomFieldsFormatted, plus the pagination fields.
- tag.html: TagData — .TagName, .Posts, plus the pagination fields.
- not_found.html: NotFoundData (layout fields only).
- login.html: AuthPageData (layout fields plus .ShowRegister);
  register.html takes AuthPageData too; forgot_password/verify_email/
  reset_password take the layout fields.
PostItem fields: .Slug, .Title, .MetaDescription, .ImageURL, .ImageSrcset,
.ImageSizes, .ImageVariants map, .ImageMobileURL, .OriginalURL, .Author,
.Username, .AuthorAvatarURL, .CreatedAt, .PostType, .Tags, .CustomFields.

Functions: urlpath (path-escapes a segment), t (lang, key — UI string),
upper, assetURL (adds the stylesheet content hash to a /static/*.css name),
inlineStyles (inlines base+style CSS), formatDate (lang, time),
formatDateTime (lang, time). Use only these; edition-specific funcs are not
guaranteed.

Rules:
- Prefer pure markup changes: keep the layout's root-relative URLs and the
  existing static script includes; do not add new inline <script> blocks,
  inline event handlers, or javascript: URLs, and do not introduce external
  resources. Keep any JavaScript strings single-line and double-quoted so
  html/template never JS-escapes the following markup.
- Reuse the engine funcs and the .SiteConfig/.Legal fields instead of
  hardcoding names, URLs, or languages.
- Keep the {{define "body"}} wrapper on every page file.`

	// themeSourcesSection wraps the edition's readable stylesheet sources when
	// available. The two %s placeholders receive base and style sources.
	themeSourcesSection = `

## Active edition sources (authoritative — derive from these)

Below are the edition's readable base sources. Internalize the palette,
spacing, radius, and component patterns, then derive the new theme from
them. Do NOT copy them verbatim — override tokens and restyle components.

### base source
%s

### style source
%s`

	// themeTemplateSourcesSection wraps the edition's readable page templates
	// when template overrides are requested. The %s placeholder receives them.
	themeTemplateSourcesSection = `

## Active edition template sources (authoritative — derive from these)

Below are the edition's current page templates. Reuse their partial names,
class hooks, and data access patterns, then derive the new markup from them.
Do NOT copy unrelated pages verbatim — override only the pages the design
changes.

### template sources
%s`

	// maxPromptSourcesRunes bounds injected stylesheet sources so the system
	// prompt stays within model context.
	maxPromptSourcesRunes = 12000

	// maxPromptTemplateRunes bounds injected template sources. The full page
	// set is larger than the stylesheet pair, so this cap is separate.
	maxPromptTemplateRunes = 24000

	// maxRepairFileRunes bounds each file echoed back in a repair prompt.
	// Templates are larger than stylesheets; the failing line usually sits
	// within this window.
	maxRepairFileRunes = 16000
)

// stylePresetHint maps the UI style preset to an extra directive. Empty
// means auto (follow the brief).
func stylePresetHint(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "dark":
		return "Style preset: dark. Ship a dark theme (dark surfaces, light text, brand hues kept, WCAG AA contrast)."
	case "light":
		return "Style preset: light. Ship a light theme (light surfaces, dark text, airy spacing)."
	case "minimal":
		return "Style preset: minimal. Ship a restrained theme: quiet surfaces, one accent, generous whitespace, small radius."
	case "playful":
		return "Style preset: playful. Ship a lively theme: rounded shapes, warm accents, bold headings — still token-driven."
	case "editorial":
		return "Style preset: editorial. Ship a magazine theme: serif display stack, strong hierarchy, hairline rules."
	case "brutalist":
		return "Style preset: brutalist. Ship a raw theme: hard borders, no shadows, monospace accents, high contrast."
	default:
		return "Style preset: auto. Follow the brief's mood; default to a clean light theme unless it asks otherwise."
	}
}

// truncateRunes shortens s to at most max runes.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func formatSourcesSection(base, style string) string {
	if style == "" {
		style = "(no separate style source provided)"
	}
	return fmt.Sprintf(themeSourcesSection, base, style)
}

// outputFormatSection renders the JSON envelope instructions, naming a
// template file when overrides are requested.
func outputFormatSection(templateOverrides bool) string {
	files := `{"static/base.css": "...", "static/style.css": "..."}`
	if templateOverrides {
		files = `{"static/base.css": "...", "static/style.css": "...", "templates/index.html": "..."}`
	}
	return `

## Output format

Output ONLY a JSON object with exactly these keys — no markdown fences, no explanation, no commentary:
{"name": "<theme-slug>", "rationale": "<one-paragraph design explanation>", "files": ` + files + `}`
}

// BuildSystemPrompt assembles the theme generation system prompt from the
// distilled contract, the edition's readable stylesheet sources, and — when
// the brief asks for template overrides — the template contract and page
// sources.
func BuildSystemPrompt(sources themegendomain.ThemeSources, templateOverrides bool) string {
	prompt := themeContractPrompt
	if strings.TrimSpace(sources.CSS) != "" {
		css := truncateRunes(sources.CSS, maxPromptSourcesRunes)
		base, style := css, ""
		if parts := strings.SplitN(css, "\n--- style ---\n", 2); len(parts) == 2 {
			base, style = parts[0], parts[1]
		}
		prompt += "\n" + formatSourcesSection(base, style)
	}
	if templateOverrides {
		prompt += templateOverridesContract
		if strings.TrimSpace(sources.Templates) != "" {
			templates := truncateRunes(sources.Templates, maxPromptTemplateRunes)
			prompt += fmt.Sprintf(themeTemplateSourcesSection, templates)
		}
	}
	prompt += outputFormatSection(templateOverrides)
	return prompt
}

// BuildUserPrompt assembles the user prompt from the brief and style preset.
func BuildUserPrompt(description, style string) string {
	trimmed := strings.TrimSpace(description)
	if len([]rune(trimmed)) > 5000 {
		trimmed = truncateRunes(trimmed, 5000)
	}
	var b strings.Builder
	b.WriteString(stylePresetHint(style))
	b.WriteString("\n\nBrief:\n")
	b.WriteString(trimmed)
	return b.String()
}

// BuildRepairPrompt feeds compiler/validation findings back to the model.
// The brief's style preset rides along so a repair does not drift from the
// requested look; theme files arrive verbatim (minification happens at
// install time) so braces and structure stay readable.
func BuildRepairPrompt(brief themegendomain.Brief, themeFiles map[string]string, problems []string) string {
	var b strings.Builder
	b.WriteString(stylePresetHint(brief.Style))
	b.WriteString("\n\nThe previous theme failed validation. Fix every problem below and return the full corrected JSON envelope.\n\nProblems:\n")
	for _, problem := range problems {
		b.WriteString("- ")
		b.WriteString(problem)
		b.WriteString("\n")
	}
	b.WriteString("\nPrevious files (fix these):\n")
	for path, content := range themeFiles {
		b.WriteString("--- ")
		b.WriteString(path)
		b.WriteString(" ---\n")
		b.WriteString(truncateRunes(content, maxRepairFileRunes))
		b.WriteString("\n")
	}
	return b.String()
}
