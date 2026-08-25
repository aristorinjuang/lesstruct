---
project_name: 'lesstruct'
user_name: 'Ari'
date: '2026-06-16'
sections_completed: ['technology_stack', 'language_rules', 'framework_rules']
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

### Backend (Go)
- **Go** 1.26 — module: `github.com/aristorinjuang/lesstruct`
- **Chi** 5.2.5 — HTTP router; **httprate** 0.15.0 — per-route rate limiting
- **Databases** (driver selected via `DB_DRIVER` env: `sqlite` | `postgres` | `mysql`):
  - **SQLite** (modernc.org/sqlite v1.50.0) — default, embedded
  - **PostgreSQL** (jackc/pgx/v5 v5.10.0)
  - **MySQL** (go-sql-driver/mysql v1.9.2) — DSN MUST contain `parseTime=true` AND `multiStatements=true`
- **golang-migrate** 4.19.1 — DB migrations via `iofs` embedded filesystem, per-driver subdirs under `internal/database/migrations/{sqlite,postgresql,mysql}/`
- **BurntSushi/toml** 1.6.0 — `config.toml` parsing
- **joho/godotenv** 1.5.1 — `.env` loader (called from `config.Load()`)
- **fsnotify** 1.10.0 — plugin hot-reload in `DEV_MODE` (config itself is read once at startup, not watched)
- **golang-jwt** 5.3.1 — JWT auth (browser admin realm)
- **bluemonday** 1.0.27 — HTML sanitization
- **chroma** (alecthomas/chroma/v2) 2.27.0 — server-side syntax highlighting; the Hugo importer's `{{</* highlight … */>}}` shortcode converter (`internal/content/hugo/shortcodes.go`) renders class-based chroma HTML honoring `linenos=table|inline`, styled by the theme's `.chroma` CSS
- **goldmark** 1.8.2 — Markdown parser (Markdown → TipTap JSON converter in `internal/content/markdown/`)
- **wazero** 1.11.0 — WebAssembly runtime (plugin system)
- **google.golang.org/genai** 1.59.0 — Google Imagen image generation
- **openai/openai-go** 1.12.0 — text generation (OpenAI-compatible APIs via `AI_TEXT_GENERATION_BASE_URL`)
- **deepteams/webp** 1.2.1 + **golang.org/x/image** 0.45.0 — image transcoding for media uploads
- **spf13/cobra** 1.10.2 — CLI framework (`cmd/lesstruct-cli`)
- **golang.org/x/crypto** 0.52.0, **golang.org/x/net** 0.55.0
- **stretchr/testify** 1.11.1 — test assertions
- **mockery** — mock generation (`make mock`)
- **govulncheck** — vulnerability scanning (`make vulncheck`)
- **golangci-lint** v2.11.4 — linter (`make lint`); config in `.golangci.yml`

### CLI (`cmd/lesstruct-cli`)
- Thin Cobra-based client over `/api/v1`; imports **no server internals**
- Subcommands: `content` (create/get/list/update/delete/publish/unpublish), `media` (upload/get/list), `export` (download all content as Hugo-compatible tar.gz), `config`
- Auth via `--api-key` flag, `LESSTRUCT_API_KEY` env, or config file (precedence in that order)
- Output mode: `--output text|json` (default `text`)
- `--version` prints the build version (injected via `-ldflags` in `make build-cli`/`install`, git-derived; defaults to `dev`)
- Built via `make build-cli` → `bin/lesstruct-cli`; integration tests via `make test-cli` (tag `integration`)

### Admin Panel (Frontend) — `web/admin/`
- **Vue 3** 3.5.31 — Composition API + `<script setup>` only
- **TypeScript** 6.0 — strict mode
- **Vite** 8 — build tool; base: `/admin/`, output to `internal/api/static/admin/`
- **Pinia** 3.0.4 — state management
- **Vue Router** 5.0.4
- **TipTap** 3.22+ (Vue 3) — rich text editor (starter-kit + code-block-lowlight, emoji, image, link, mathematics, placeholder, table, table-cell, table-header, table-row, text-align, underline)
- **Headless UI** 1.7.23 — accessible primitives
- **KaTeX** 0.16.47 — math rendering
- **lowlight** 3.3.0 — syntax highlighting
- **Vitest** 4.1.2 + jsdom 29 — unit tests
- **Prettier** 3.8.1 + ESLint 10 + Oxlint ~1.57 — linting/formatting
- Node engine: `^20.19.0 || >=22.12.0`

### Content Theme
- Go `html/template` — server-rendered content site via `internal/api/template/` (layouts/pages) and `internal/api/contentpage/` (data assembly)
- Per-post-type content templates: each post type resolves via `readContentTemplate` (theme `<type>.html` → theme `post.html` → embedded `<type>.gohtml` → embedded `post.gohtml`). A per-slug override layer (`<type>-<slug>.html`, e.g. `page-about.html`) sits in front of the per-post-type chain — discovered by `findPerSlugTemplateOverrides` at startup and keyed by `<postType>:<slug>` in `Templates.contentBySlug`
- Theme overrides via `THEME_DIR` env var or theme plugin architecture
- Theme `root/` directory (optional): regular files under `<THEME_DIR>/root/` are served at the **site root** by the dynamic server (`template.RootFilesHandler`, wired between the alias redirect and the content renderer in `main.go`) and copied to the archive root by the SSG — for fixed-scope files like `webpushr-sw.js` that cannot live under `/static/`
- Default theme CSS minified via `make css` (tdewolff/minify)

### Architecture
- **Domain-Driven Design**: `internal/domain/<name>/` holds business logic, types, sentinel errors, interfaces. Current domains: `apikey`, `alias`, `auth`, `content`, `customfield`, `dashboard`, `media`, `plugin`, `posttype`, `profilepicture`, `role`, `sanitize`, `seo`, `textgen`, `thumbnail`, `user`
- **Repository pattern**: interfaces in domain, per-driver implementations in `internal/repository/{sqlite,mysql,postgresql}/`. Shared cross-driver helpers (e.g., `soft_delete.go`, `user.go`) live directly in `internal/repository/`
- **File storage**: cross-cutting `internal/storage/` package (like `internal/repository/`, outside `domain/`) holds the shared `Storage` interface (`Save`/`Delete`/`GetURL`) plus its backends — `local.go` (disk under `data/uploads/`) and `s3.go` (AWS S3 **or** MinIO via aws-sdk-go-v2; selected by `STORAGE_DRIVER=local|s3`, with MinIO differing only by `STORAGE_S3_ENDPOINT` + `STORAGE_S3_USE_PATH_STYLE`). Both the media domain and the profilepicture domain consume `storage.Storage` (each resource gets its own instance with a distinct key prefix / URL prefix: `media/` and `profile_pictures/`). The driver decides the URL shape baked into stored data: in S3 mode, `GetURL` returns `<STORAGE_S3_PUBLIC_BASE_URL>/<key>` (bucket/CDN URL) and the local `/uploads/*` fileservers are not mounted; in local mode they are mounted and `GetURL` returns root-relative URLs (`/<prefix>/<file>`) that survive reverse proxies/HTTPS unchanged. Contexts requiring absolute values absolutize on read: SEO meta tags via the contentpage assembler's `WithBaseURL(SITE_URL)` + `seo.BuildURL`, publish-time metadata via the seo service, and public-API featured images via the content handler's baseURL.
- **HTTP handlers**: `internal/api/handlers/` (browser admin realm) and `internal/api/handlers/agent/` (Bearer API-key realm, `/api/v1`); routes registered in `internal/api/routes/routes.go`
- **Auth realms**: two co-exist on shared paths and are dispatched by `dispatchByAuth()` based on the `Authorization` header prefix (`Bearer lesstruct_…` = agent realm, JWT cookie or other Bearer = browser realm). Each chain carries its own auth middleware
- **Middleware** (`internal/api/middleware/`): `auth` (JWT), `apikey` (Bearer API key), `admin`, `commentator`, `cors`, `csrf`, `nocookie`, `ratelimit` (via httprate). `securityheaders` emits the static security headers plus the CSP; `X-Frame-Options` is not hardcoded — it is derived from the `[csp]` framing configuration (`CSPConfig.XFrameOptions()`: `DENY` default, `frame_ancestors=["'self'"]` → `SAMEORIGIN`, host lists omit the legacy header except under `report_only` which floors to `DENY`; a `policy` override takes precedence), so the two framing headers can never contradict.
- **Response envelope** (`internal/api/response/`): `{"data": ..., "error": {...}, "meta": {...}}`. Lists use `SuccessList()` which uses a dedicated `listResponse` type WITHOUT `omitempty` on `data` so empty lists serialize as `"data":[]`
- **Plugin system**: wazero WASM runtime in `internal/plugin/` with hook execution (`before_save`, `after_create`, `after_publish`, `before_delete`, `after_unpublish`; `on_plugin_loaded` defined but not invoked). Subpackages: `bootstrap`, `capability`, `devmode`, `hostfunctions`, `loader`, `registry`, `runtime`
- **Media pipeline**: `internal/domain/media/processor.go` owns image processing. Decoding is dispatched **explicitly** — WebP always goes through `golang.org/x/image/webp`, every other format through `image.Decode` — never left to registration order: `deepteams/webp` (the encoder) also registers a `webp` decoder whose VP8X/ALPH path fails on real-world files, and `image.Decode` picks the first matching driver. `ConvertToWebP` passes already-WebP input through without re-encoding (a RIFF chunk-walker — `sanitizeWebP` — strips EXIF/XMP/ICC metadata chunks and rejects animated or frame-less containers with clear errors; files without metadata pass byte-identical); other formats are transcoded at quality 80. `Resize` (thumbnail variants) and `profilepicture`'s `CropAndConvertToWebP` share the same explicit dispatch. `phash.go` computes 64-bit average (aHash) perceptual hashes on the same decode path; the content importers use them to detect featured-image covers that visually duplicate a body image under a different URL (`MaxPerceptualDistance` bounds the Hamming distance).
- **Content pipeline**: `internal/content/` holds format converters — `tiptap/` (canonical), `markdown/` (Markdown→TipTap via goldmark), `wordpress/` (WordPress importer), `hugo/` (Hugo importer), `export/` (Hugo-compatible source file exporter), `ssg/` (static site generator — renders the full site to HTML with AMP variants, packaged as tar.gz; emits meta-refresh redirect pages for `content_aliases` targets that are published — skipping aliases that would shadow emitted pages or reserved root files and excluding them from the sitemap; alias redirect targets, sitemap entries, AMP canonical links, and RSS feed item links all use the trailing-slash directory form (`/<slug>/`) matching the `<slug>/index.html` export layout so they address the page rather than a flat alias stub; hosts serving the export should resolve directory indexes before flat `.html` files — slash-less targets self-loop on hosts that prefer the flat file; ships only embedded `/static/` assets referenced by rendered pages plus the always-needed stylesheets, never the `*.src.css` dev sources; copies the theme's optional `root/` directory to the archive root). Content items carry a `format` field (`tiptap`, `html`, or `markdown`). HTML-format content is stored and served as-is — no TipTap conversion. The WXR importer accepts any post type registered in `config.toml` (built-in `post`/`page` plus custom types), parses `<wp:postmeta>` custom fields, and converts values to the declared field types (e.g. WordPress `YYYY-MM-DD HH:MM:SS` datetimes → RFC 3339; numeric strings → `float64`) before passing them to the content service as `CustomFields`. Featured images (`_thumbnail_id`) are resolved from attachment items, downloaded via the media downloader (WebP transcode + SHA-256 dedup), and prepended to the post's TipTap content — unless the body's first images already show that picture (exact source-URL match, or a perceptual-hash match via the downloader's recorded 8×8 average hashes; perceptual skips emit a `warning:` entry); inline body images are likewise downloaded and remapped. Attachment items themselves are captured as a lookup table (post ID → URL) in `WXRDocument.Attachments` but never become content posts. The Hugo importer (`internal/content/hugo/`) runs async (202 + job store, mirroring WordPress), reads `hugo.toml`/`config.toml` for `baseURL` + `defaultContentLanguage`, imports posts as `format=html`, preserves the frontmatter `date` as `PublishedAt`, and migrates images (local `static/` files and remote URLs) through `media.go`'s `MediaMapper` — reusing the WordPress `MediaDownloader` for remote URLs, rewriting body `<img src>` references, and prepending the first frontmatter `images:` entry as a leading `<figure><img>` cover (skipped when the rewritten body's first three images already show that picture — an exact mapped-URL match, or a perceptual-hash match recorded by the mapper at ingest time from re-uploaded/downloaded/static-file bytes; perceptual skips emit a `warning:` entry). The mapper also rewrites any root-relative `href`/`src` that resolves to a file under the extracted `static/` dir to `/static/<path>` (`RewriteStaticRefs` — the documented mirror convention; applies to skip-media imports and failed-image fallbacks too), and records every migration failure (ref → reason + resulting URL; failures are reported exactly once per import via `TakeUnreportedFailures`) which the importer surfaces as `warning:` entries in the job's `errors` list. A post-rewrite scan additionally warns about references left unresolved — dead links and missing images — while staying silent for the import's own URLs and aliases (`knownTargets`). The Hugo archive extractor tolerates GNU tar's leading `./` root entry (`tar -czf site.tar.gz .`), and the alias loop re-points an existing alias whose target no longer exists onto the freshly imported item (never stealing from a live one). Highlight shortcodes (`{{</* highlight … */>}}`, whitespace-tolerant closing tags) are converted to server-rendered chroma HTML (class-based, `linenos` honored, code bodies escaped; unknown languages fall back to plain escaped `<pre><code class="language-x">`) — chroma blocks carry a `nohighlight` marker so client-side highlighters skip them. Deleting content cascades to `content_aliases` via the content service's `WithAliasDeleter` option (aliases are removed before the content row, fail-closed — the SQLite schema's `ON DELETE CASCADE` does not fire because SQLite foreign keys are not enforced), and self-service account deletion (`internal/repository/user_deletion.go` + per-driver variants) deletes its aliases in the same transaction for the same reason. Both importers skip items whose slug already exists (idempotent re-runs via the content service's `SlugExists`); a post type that declares a `post_script` textarea field has its raw value emitted verbatim at the end of the post via `{{.PostScripts}}` (excluded from the visible custom-fields section and from AMP; only declared types emit), and the WordPress importer now preserves `pubDate` as `PublishedAt`.
- **Content sanitization**: HTML content (format `html`) is sanitized on **write** (both browser and agent API handlers run `SanitizeHTMLDocument` before persisting) and on **read** (the public contentpage handler re-sanitizes before rendering). The policy (`internal/domain/sanitize/htmldocument.go`) allows all HTML5 elements (including the code phrase elements `code`/`kbd`/`samp`/`var`) and inline `style`/`class` attributes, root-relative URLs (via `AllowRelativeURLs`), and `<iframe>` embeds whose host appears in the iframe allowlist, while blocking `<script>`, event handlers (`on*`), and `javascript:` URLs. The iframe allowlist is derived once at startup from the `[csp]` `frame-src` directive (built-in defaults + operator appends; `*.host` entries allow subdomains only, and a full `policy` override replaces the defaults — an override without `frame-src` keeps iframes stripped) and injected into the assembler and both content handlers via `WithIFrameHosts`; without an allowlist iframes stay stripped. TipTap content is sanitized differently — raw HTML within TipTap bodies is stripped to plain text via bluemonday's `UGCPolicy`. AI-generated HTML is also sanitized server-side in `callChatCompletionHTML` (in `internal/domain/textgen/service.go`) before being returned to the client.
- **AI text generation**: `internal/domain/textgen/` provides the `TextGenerationService` interface with `EnhanceText` and `TranslateText` methods, both parameterized by a `format` field (`tiptap` or `html`). For HTML format, the service is constructed with the active theme's minified `base.css` (design tokens) and `style.css` (component styles), read once at startup by `template.ReadThemeStyles` (`internal/api/template/template.go`) and baked into the system prompt so generated HTML reuses the site's CSS custom properties (`var(--color-primary)`, `var(--space-*)`, etc.) and component classes (`.btn`, `.container`, `.form-control`) instead of inventing off-brand styles. The handler also injects a keyword-filtered subset of the user's media library (up to 20 images, matched by alt-text overlap with the user's prompt) as context in the user prompt. The HTML system prompt instructs the AI to produce production-ready fragments with `<style>` blocks at the top, scoped class names (`.ls-<topic>`), responsive CSS, and accessible semantic HTML5. The handler (`internal/api/handlers/textgen.go`) owns the `MediaLister` interface and the `buildMediaContext` helper (`textgen_media.go`).
- **Config**: `.env` + env vars loaded via `internal/config/config.go` (`Config` struct, `Load()`); storage backend selected via `STORAGE_DRIVER` (`local`|`s3`, the latter covering both AWS S3 and MinIO); user-facing `config.toml` in project root loaded **once** at startup from `${CONFIG_DIR}/${CONFIG_FILE}` (no hot-reload — restart the server to pick up changes); post types/languages/thumbnails/CSP schemas in `internal/config/`
- **Migrations**: numbered `.up.sql`/`.down.sql` pairs in `internal/database/migrations/{driver}/`, embedded via `embed.go`

### Request flow & auth realms

Two auth realms co-exist on shared paths. `dispatchByAuth()` inspects the `Authorization` header **prefix** to route each request through one chain before it reaches a handler — downstream code is auth-agnostic because both inject the same context keys (`UserIDKey`, `UsernameKey`, `RoleKey`).

```mermaid
flowchart TD
    Client([Client request])
    Disp{"Authorization<br/>header prefix?"}
    Browser["Browser realm<br/>JWT cookie / plain Bearer<br/>→ auth middleware"]
    Agent["Agent realm<br/>Bearer lesstruct_…<br/>→ apikey middleware"]
    Handler["Handler<br/>internal/api/handlers/  (browser)<br/>internal/api/handlers/agent/  (/api/v1)"]
    Service["Domain service<br/>internal/domain/&lt;name&gt;/"]
    Repo["Repository iface → per-driver impl<br/>internal/repository/{sqlite,mysql,postgresql}/"]
    DB[("SQLite / PostgreSQL / MySQL")]

    Client --> Disp
    Disp -- "JWT cookie / other Bearer" --> Browser --> Handler
    Disp -- "Bearer lesstruct_" --> Agent --> Handler
    Handler --> Service --> Repo --> DB
```

---

## Where Does New Code Go?

Quick routing — confirm the exact package by reading the matching `internal/` tree. When a change spans layers, work outside-in (handler → service → repository) and add a test per layer.

| You are adding... | It goes in... |
|---|---|
| A business rule, domain type, sentinel error, or repository interface | `internal/domain/<name>/` |
| A file storage backend (local disk, S3, MinIO, …) | `internal/storage/` (the `Storage` interface already lives there; add the backend next to `local.go`/`s3.go`) |
| Database access for that interface | the interface in `internal/domain/<name>/`, plus an implementation in **all three** `internal/repository/{sqlite,mysql,postgresql}/` (cross-driver helper → `internal/repository/`) |
| An HTTP endpoint | a handler in `internal/api/handlers/` (browser/admin realm) or `internal/api/handlers/agent/` (`/api/v1`), the route in `internal/api/routes/routes.go`, and the error in the realm's mapper |
| Request middleware | `internal/api/middleware/` |
| A content format converter | `internal/content/<format>/` (tipTap JSON converters); HTML-format content is stored directly via the content service (no converter needed) |
| A plugin host function or hook | `internal/plugin/` |
| A CLI subcommand | `cmd/lesstruct-cli/` |
| Admin UI | `web/admin/src/` following atomic design (`atoms/` → `molecules/` → `organisms/` → `views/`); the **Pinia store action** makes the API call — components only call store actions |

---

## Critical Implementation Rules

### Language-Specific Rules

#### Go
- Use `any`, never `interface{}`
- Never use `panic()` — use `log.Fatalf()`/`log.Panicf()` only in `main.go` (and only in `cmd/lesstruct-cli/main.go` for the CLI)
- Private structs/functions before public ones in every file
- Constructors (`New*`) go AFTER all methods on the struct
- Multi-line function arguments when ≥3 params (one arg per line)
- Always use constants for HTTP methods: `http.MethodDelete`, not `"DELETE"`
- `internal/config/` holds env-based config; `config.toml` holds user-facing config
- Domain errors are sentinel errors (`var ErrSomething = errors.New(...)`) in the domain package; when propagating, wrap with `fmt.Errorf("failed to X: %w", err)` so `errors.Is`/`errors.As` chains stay intact
- Handlers map domain errors to HTTP responses via a `switch` over `errors.Is`. **Two error-code casings exist — match the realm you are in:** the agent API (`/api/v1`, `internal/api/handlers/agent/errors.go`) emits `UPPER_SNAKE` codes (`NOT_FOUND`, `FORBIDDEN`, `VALIDATION_ERROR`, `INTERNAL_ERROR`); the browser/admin API (`internal/api/handlers/`, per-resource `handleXxxError()`) emits `lowercase_snake` codes (`content_not_found`, `invalid_title`). When you add a new domain sentinel, **register it in BOTH mappers**
- JSON responses use the envelope from `internal/api/response/` — call `Success`, `Error`, or `SuccessList`; never hand-roll the envelope
- Logging uses the injected `util.Logger`, which is **printf-style**: `h.logger.Error("failed to X: %v", err)`. Never use `fmt.Println` or `log.*` outside `main.go`
- Cross-driver repository code must work for SQLite, PostgreSQL, AND MySQL — beware driver-specific SQL (placeholders, `RETURNING`, time handling). Use the per-driver subpackage when behavior must diverge

#### TypeScript/Vue
- Use `<script setup lang="ts">` exclusively
- Use `defineProps<T>()`, `defineEmits<T>()` typed interfaces
- `composables/` for reusable stateful logic (e.g., `useAuth`)
- `stores/` for Pinia stores, organized by domain under `stores/domain/` and UI under `stores/ui/`
- `types/` for shared TypeScript interfaces
- TipTap content is always a JSON string (`"{\"type\":\"doc\",\"content\":[...]}"`)

### Framework-Specific Rules

#### Backend (Chi + Domain-Driven Design)
- **No framework**: Chi is a lightweight router, not a framework — handlers receive `http.ResponseWriter, *http.Request`
- Routes registered in `internal/api/routes/routes.go`, grouped by resource and by auth realm
- Two auth realms co-own some `/api/v1/media` paths — when adding routes that may collide, register via `dispatchByAuth(agentChain, browserChain)` rather than duplicating the path
- Agent realm (`/api/v1/*`) requires Bearer `lesstruct_<keyID>_<secret>` tokens verified by `APIKeyAuthMiddleware`; identity is injected into context using the SAME context keys (`UserIDKey`, `UsernameKey`, `RoleKey`) as the JWT middleware so downstream code is auth-agnostic
- Content services require a `HookExecutor` — always pass plugin hooks through, don't bypass
- Custom field validation flows through `content.Service.validateCustomFields()` — never call `validateFieldValue()` directly from handlers
- Post types loaded from `config.toml` **once** at startup via `internal/config/posttypes.go` (restart to pick up changes); built-in slugs (`post`/`page`/`media`/`comment`) extend instead of duplicating. A `hidden = true` flag on a `[[post_type]]` entry hides the type from presentation surfaces (admin tabs/editor dropdown, dashboard breakdown, public post-type list) while the registry keeps serving it; `page`/`media`/`comment` reject `hidden = true` (`internal/domain/posttype/service.go` `protectedSlugs`)
- Headless mode and the comment-system toggle are optional `config.toml` blocks loaded at startup: `internal/config/headless.go` (`[headless] enabled`) and `internal/config/comments.go` (`[comments] enabled`, `IsEnabled()` defaults to true when absent). The two flags flow through `routes.Setup(headlessEnabled, commentsEnabled)` — headless unmounts the content-site catch-all + `/static/*` and makes `sitemap.xml` 404 / `robots.txt` `Disallow: /` (JSON sitemap stays); comments-disabled unmounts every comment route in all three realms, forces `allowComments=false` in the content service (`WithCommentsEnabled` option), blocks self-registration with `403 REGISTRATION_DISABLED`, and removes the `Commentator` role from admin creation (`admin_creation.go` `isAllowedRole`). The flags are also exposed to the SPA via `GET /api/v1/config` (`commentsEnabled`, `headless`)
- **User roles are config-driven**: `internal/config/roles.go` (`[[role]]`) loads role definitions once at startup (via `config.LoadRoles(cfg, postTypeSlugs)`) into `internal/domain/role/`, which validates them against the post-type registry and seeds three built-ins — `Admin` (reserved, all capabilities), `Contributor` (all types, publish/media/comments), `Commentator` (no types, media + comments). A `[[role]]` entry overrides a built-in (except `Admin`) or adds a custom role; roles may only reference post types defined in config (typo fails closed). Enforcement is per-capability through the three gate helpers: the content service takes a `RoleChecker` option (`WithRoleChecker`) gating create/update/publish/unpublish/delete by manageable post types and the `publish` flag; `handlers/{media,comment,posttype}.go` and `handlers/agent/{media,comment}.go` take `WithMediaRoleService`/`WithCommentRoleService`/`WithPostTypeRoleService` options gating those endpoints; user admin uses `WithRoleService` to allow/assign any registered role. `[registration]` (`internal/config/registration.go`) decouples self-registration from the comment toggle — `enabled`, `default_role` (validated against the role registry at startup, `Admin` rejected), and `admin_approval` — wired through `internal/domain/auth/registration.go` (variadic `WithEnabled`/`WithDefaultRole`) and `internal/domain/auth/verification.go`/`internal/domain/user/service.go` (variadic `WithAdminApprovalRequired`/`WithApprovalEmailRequired`). Email verification is always mandatory (`users.email_verified` boolean, migration `000005`); with `admin_approval = true` a registrant stays `pending` after verifying and the admin approval is the only activation path (approve fails `409 EMAIL_NOT_VERIFIED` until the email is verified; the verify-email page shows a "pending approval" message via `VerifyEmailResponse.awaitingApproval`). The contentpage handler receives `registrationEnabled` so the public `/register` page and the login page's "create account" link follow the `[registration]` block too, not just the comment toggle. `GET /api/v1/roles` (auth-gated) returns `{roles, me}` where `me` carries the caller's derived capabilities (`isAdmin`, `media`, `comments`, `postTypes`) so the SPA gates navigation/forms without hardcoding role names. Absent config = exact legacy behavior
- Homepage sections (`[[homepage_section]]`), public listing page size (`POSTS_PER_PAGE` env), site-wide identity (`[site_config]` → `name`/`logo`), and the public custom-field query allowlist (`[[public_field]]`) are loaded the same way — `internal/config/homepage.go`, `siteconfig.go`, and `publicfield.go`; the site name defaults to `Lesstruct` in the contentpage handler constructor and drives `PageTitle` suffixes + `og:site_name` (the one branding value a `THEME_DIR` override cannot reach). Public listing queries (`GetPublishedByPostType`/`GetPublishedByAuthorUsername`/`GetPublishedByTag`/`GetPublishedArchive`) take an optional priority-ordered `languages []string` argument so the language scope and the pagination HasNext probe happen at the SQL level, not in Go: empty = every language, one element = exact match, multiple elements = Hugo-style fallback where each translation group appears once under its best-ranked configured language (shared predicate in `internal/repository/language_filter.go`, applied before ORDER BY/LIMIT/OFFSET so counts stay exact). Public custom-field filter/sort on `/api/v1/public/{content_items,authors}` is gated by `PublicFieldRegistry.IsQueryable` — without an entry, the request fails closed with `400 field_not_queryable`. Public field values can also be opted into the response body via the `"expose"` operation (`PublicFieldRegistry.ExposedFields`), projected at the handler level and rendered on both JSON and HTML author pages.
- SEO auto-extraction: `ExtractPlainText()` and `ExtractImageURL()` consume TipTap JSON from content; HTML content uses tag-stripping for meta description extraction, and `ExtractImageURLFromHTML` accepts both http(s) and root-relative (`/uploads/...`) image srcs (protocol-relative `//` is rejected). Derived descriptions collapse internal whitespace/newlines before truncation, and an unset og:description falls back to the resolved meta description (Hugo parity) rather than re-deriving from body text. The contentpage assembler's `absoluteImageURL` leaves empty inputs empty so failed extraction falls back to the theme default instead of emitting the bare site URL as og:image
- Markdown bodies on the agent create surface are converted to canonical TipTap JSON via `internal/content/markdown` — raw Markdown is NEVER persisted; HTML bodies (`format: html`) are stored as-is after sanitization
- **Slug is immutable**: on create, any authenticated user may supply `CreateContentRequest.Slug` (validated by `ValidateSlug` — `[a-z0-9.-]`, 1–200 chars, no leading/trailing dot, no `..`; uniqueness checked per language via `Repository.CheckSlugUnique`); if omitted the service auto-generates from the title (`Service.GenerateSlug`). On update the slug **never** changes — the title-change→regenerate path was removed so the public URL stays stable for SEO. The CLI exposes it via `lesstruct-cli content create --slug`.
- Rate limits configurable per realm via `RATE_LIMIT_{AUTH,API,PUBLIC}_PER_MINUTE`; toggle via `RATE_LIMIT_ENABLED`

#### Frontend (Vue 3 + Pinia)
- **Atomic design**: `atoms/` → `molecules/` → `organisms/` → `views/` under `web/admin/src/components/` and `web/admin/src/views/`
- **Buttons are centralized**: every ad-hoc `<button>` in views/organisms is migrated to the shared `Button.vue` atom (`atoms/Button.vue`) or `IconButton.vue` for icon-only controls. Variants: `primary | neutral | danger | ghost | link | subtle` (legacy `secondary` maps to `neutral`); `subtle`/`ghost`/`link` take a `tone` (`success | danger | warning | info | neutral`); sizes `small | medium | large`; `isLoading`, `fullWidth` props. Button colors come exclusively from the `--btn-*` / `--nav-*` design tokens in `assets/base.css` — never hardcode button colors in components. Primary text is fixed dark (`--btn-primary-fg: #0f172a`) in BOTH themes because the cyan `--brand-primary` background never darkens; `--brand-dark-*`/`--brand-light-*` are theme-swapped and must only be used on theme-adaptive surfaces (backgrounds, borders, text-on-surface). Buttons have no `min-height` — padding defines height
- Content editor: `ContentEditor.vue` is the single organism for create + edit (shared component, not separate views); format selector toggles between TipTap editor and `HtmlCodeEditor.vue` (CodeMirror 6 with live iframe preview)
- Custom field rendering: `CustomFieldRenderer.vue` in molecules handles all field types
- Media upload: `MediaPanel.vue` organism, opened as a slideover from `ContentEditor`
- SEO settings are collapsible within `ContentEditor` (`isSEOSettingsOpen`)
- Slug field in `ContentEditor` is enabled for any user on new content (`:disabled="!isNewContent"`); editing existing content always shows it disabled (immutable). The `slugManuallyEdited` ref stops the title→slug auto-suggest watcher once a user types a custom slug or an existing item is loaded
- Store actions (e.g., `contentStore.create()`) make API calls; components only call store actions
- Toast notifications via `Toast.vue` molecule with `displayToast(message, type)` pattern
