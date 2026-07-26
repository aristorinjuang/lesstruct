# Features

This is the **canonical catalog** of Lesstruct's product features. The homepage
and `README.md` curate subsets from this page; when feature wording changes here,
update those surfaces too. Each feature links to the deeper reference where one
exists (`configuration.md`, `plugin-development.md`, `api-reference.md`, etc.).

> Screenshots are captured from a running demo by `make screenshots` and served
> in both light and dark themes. A referenced-but-missing screenshot fails the
> docs build on purpose, so the catalog cannot ship a broken image by accident.

## Deploy & run {#deploy-run}

- **One binary, no Docker required.** A single static Go binary (`CGO_ENABLED=0`)
  runs the whole CMS. SQLite is built in. No runtime, no container, no
  `node_modules` in production. Containerizing is fully supported if you prefer
  it — `FROM scratch` works.
- **Multi-database.** Embedded SQLite is the default; PostgreSQL and MySQL are
  first-class via `DB_DRIVER`. Schema migrations run automatically on first start,
  per driver.
- **Two-layer configuration.** `config.toml` holds your content schema (languages,
  post types, fields, thumbnails); `.env` holds deployment state (host, port, DB,
  secrets, SMTP, AI keys). Only `JWT_SECRET` is required; everything else has a
  sensible default. See [configuration.md](configuration.md).
- **Plugin hot-reload.** In `DEV_MODE`, a file watcher reloads `.wasm` plugins
  without a server restart.

## Content & authoring {#content-authoring}

{{< screenshot src="admin-hero" alt="The content editor: TipTap rich text with a custom fields panel and collapsible SEO settings." caption="The content editor — rich text, custom fields, and SEO in one view." >}}

- **Custom post types, built in.** Define post types in `config.toml` — no plugin,
  no library. The admin list, form, storage, and queries all read from that file.
  Built-in slugs (`post`, `page`, `media`, `comment`) extend instead of collide.
- **Custom fields, built in.** Add typed fields to any post type (and to user
  profiles) in `config.toml`. The admin form renders them, the service validates
  them, and they are queryable. No code required.
- **TipTap rich-text editor.** Tables, math (KaTeX), syntax-highlighted code
  blocks, emoji, YouTube embeds, links, images, and text alignment — all
  first-class.
- **Draft and publish.** A two-state workflow (`draft`, `published`) with
  publish/unpublish actions exposed in the admin and the CLI.
- **Soft-delete and restore.** Deleted content is recoverable from the admin
  trash view.
- **Per-content SEO.** Meta description, OpenGraph title and description, and a
  live preview — collapsible inside the editor. SEO metadata generation is
  format-aware: TipTap content uses JSON extraction, HTML content uses
  tag-stripping — so HTML-format imports (e.g. WordPress Elementor pages)
  get real extracted text in their meta descriptions, not empty strings.
- **Immutable URL slugs.** The slug is the public URL (`/<slug>`).
  Any authenticated user can set a custom slug when creating content (in the
  editor or via `--slug` on `lesstruct-cli content create`); otherwise the slug
  is auto-generated from the title. Once saved, the slug is **locked for
  everyone** — editing the title never regenerates it, so published URLs stay
  stable for SEO and inbound links. Uniqueness is enforced per language; a
  collision returns a clear error rather than silently suffixing.
- **HTML/CSS content authoring.** Set `format: html` on create to author raw
  HTML directly — stored and served as-is, no TipTap conversion. The admin
  editor provides a CodeMirror 6 editor with syntax highlighting and a live
  preview (sandboxed iframe). HTML content is sanitized on write (dangerous
  elements/attributes stripped via bluemonday; inline `style` and class
  attributes preserved) and on read (rendered through the same policy). Ideal
  for WordPress Elementor imports and hand-authored HTML pages.
- **Markdown as first-class ingest.** The CLI and `/api/v1` accept Markdown
  bodies; the server converts them to canonical TipTap JSON. Raw Markdown is
  never persisted.
- **WordPress importer (async).** Upload a WordPress WXR export to migrate
  posts, pages, custom post types (and their custom fields), media, and authors
  into Lesstruct. The import runs in a background goroutine and returns `202
  Accepted` immediately with a `jobId`; poll
  `GET /api/admin/wordpress/import/status/{jobId}` to track progress. Authors
  are auto-created as Contributor users and their posts are assigned to them.
  Custom post types and field schemas are read from `config.toml`; items whose
  post type is not registered are silently skipped. Featured images
  (`_thumbnail_id`) are resolved from attachment items, downloaded, transcoded
  to WebP, and prepended to each post's content body; inline body images are
  likewise downloaded and remapped (downloading is concurrent with a bounded
  worker pool; transient errors are retried with backoff). Failed downloads fall
  back to hotlinking the original WordPress URL. Elementor-built pages are
  imported as `format=html` using the rendered HTML from the Elementor cache,
  preserving their original layout.

## Media & images {#media-images}

{{< screenshot src="media-library" alt="The media library: a searchable grid of uploaded images with thumbnails and metadata." caption="The media library — search, filter, and manage uploads." >}}

- **Media library.** Browse, search, and date-filter uploads from the admin panel.
- **Automatic WebP conversion.** Every uploaded image is transcoded to WebP
  (quality 80) on upload, so images never weigh down your content.
- **Configurable thumbnail variants.** Defaults ship `_thumb` (370px),
  `_medium` (800px), `_large` (1600px); all editable in `config.toml`. The content
  site emits a responsive `srcset` from them. Post cards also expose an
  `ImageVariants` map (keyed by configured suffix) and an `OriginalURL` field
  for the unscaled original, enabling hero backgrounds and high-DPI layouts.
- **SHA-256 dedup.** Identical uploads are detected and rejected (with a
  force-upload escape hatch).
- **AI image generation.** Generate images from the media library and the content
  editor via Google Imagen, Gemini, or GPT-Image. Bring your own key.

## Internationalization {#internationalization}

- **Multilingual by default.** Declare your languages in `config.toml`
  (e.g. `languages = ["en", "id"]`). Content carries a `Language` and authors
  link translations into translation groups.
- **Translation-aware SEO.** The sitemap declares `hreflang` alternates from
  translation groups.
- **Localized date formatting.** Post dates follow each content item's
  `Language` field: Indonesian content renders `1 Januari 2026`, English content
  renders `January 2, 2006`. Custom-field dates localize automatically.
- **AI translation.** Translate content between your configured languages from
  the editor.

## AI {#ai}

- **Opt-in, bring-your-own-key.** Text via any OpenAI-compatible endpoint
  (`AI_TEXT_GENERATION_BASE_URL`); images via Google or OpenAI. Nothing runs
  without your keys; `/api/health` honestly reports which features are enabled.
- **Text enhancement and translation.** Refine or translate rich-text (TipTap) post bodies from the editor.
- **AI-powered HTML/CSS authoring.** Describe what you want in plain language — the AI generates production-ready HTML & CSS with semantic markup, responsive layouts, and accessible design. Output is on-brand by default: the AI reuses your active theme's design tokens (`var(--color-primary)`, spacing, radius, fonts) and component classes rather than inventing arbitrary colors and fonts. Includes 9 quick-start presets (hero, pricing, testimonials, features, CTA, FAQ, stats, contact, newsletter) and iterative refinement — the AI can modify existing HTML based on your follow-up instructions. Your media library images are automatically surfaced as context. This replaces the need for a drag-and-drop page builder: describe, generate, refine, ship.
- **Image generation.** Generate images from the media library and the editor.
- **Built for agents.** `lesstruct-cli` is a thin Cobra client over `/api/v1`
  designed for AI agents and terminal-first humans. Markdown ingest, cursor
  pagination, and a standard response envelope make it easy to script.
- **Agent skills.** Lesstruct ships installable skills for theme and plugin
  development that work from your installed site (no source tree needed).
  Install them with `npx skills add aristorinjuang/lesstruct` — works with
  Claude Code, OpenCode, Cursor, Codex, and 25+ other agents.
- **Crawlable docs.** This site publishes `/llms.txt` (page index),
  `/llms-full.txt` (every page concatenated), and a per-page Markdown mirror for
  retrieval pipelines.

## Themes & rendering {#themes-rendering}

- **Server-rendered by default.** The content site is rendered server-side with
  Go `html/template` — fast and SEO-friendly.
- **One default theme.** Lesstruct ships a single embedded default theme that is
  the working starting point; it does not generate a new theme per release cycle.
- **Customizable.** Point `THEME_DIR` at a `themes/<name>/` directory to override
  CSS, JS, and HTML templates. The contract (CSS variables, layout blocks, JS DOM
  ids, CDN assets) is documented so fork-and-modify is safe. See
  [theme-development.md](theme-development.md).
- **Per-post-type templates.** Each post type gets its own content template
  (e.g. `page.html`, `event.html`), falling back to `post.html`, then the
  embedded default. Theme authors can ship a `page.html` without blog chrome
  (related posts, author box, date metadata) while keeping the full layout
  for blog posts — no config changes needed.
- **Per-slug template overrides.** A theme can ship a template that applies
  to one specific content row by naming the file `<postType>-<slug>.html`
  (e.g. `page-about.html`, `article-spotlight.html`). Mirrors the WordPress
  `page-{slug}.php` convention but generalizes to every post type. Pure
  additive fallback — no existing theme breaks. See
  [theme-development.md](theme-development.md).
- **Multi-type aware.** Every post card and single-page template receives the
  item's `.PostType`, so a theme can branch layouts for articles, events, and any
  custom post type from one template set.
- **Magazine homepages.** Optional `[[homepage_section]]` blocks in `config.toml`
  render per-post-type groupings (latest articles, upcoming events, …) alongside
  the latest-posts list — backward compatible (omitted = flat list). Each section
  supports `offset` for non-overlapping content (e.g. "Featured" items 1–6,
  "Recommendations" items 7–26). The homepage renders via a dedicated
  `homepage.html` template (distinct from `index.html` used by listing pages).
- **Content archive API.** `GET /api/v1/public/archive` returns year/month
  counts for building archive widgets; listing pages accept `?year=`/`?month=`
  for date filtering.
- **Site identity from config.** An optional `[site_config]` block in
  `config.toml` sets the site `name` and an optional `logo`, which drive the
  browser-tab title suffix, `og:site_name`, the default logo text, and the
  footer. This is the one branding surface a theme override cannot reach (the
  name is baked into handler-side `PageTitle` strings); everything else
  (social links, analytics, custom `<head>`, image/multi-logo layouts) stays a
  `THEME_DIR` theme concern.
- **Paginated listings.** Homepage, author, tag, and post-type listing pages
  accept `?page=N` and expose prev/next state to templates. Page size is tuned
  via `POSTS_PER_PAGE`.
- **SEO built in.** `sitemap.xml`, `robots.txt`, JSON sitemap, and `hreflang` are
  generated for you.

## Extensibility {#extensibility}

- **WebAssembly plugins.** Drop a compiled `.wasm` into `plugins/` and it hooks
  into the content lifecycle. Any language that compiles to Wasm works.
- **Familiar hook model.** Explicit registration, priority-based execution,
  immutable data flow. Invoked hooks: `before_save`, `after_create`,
  `after_publish`; reserved for forward compatibility: `on_plugin_loaded`,
  `before_delete`.
- **Host functions.** Plugins call into the host for HTTP (`http_get`,
  `http_post`), the database (`db_query`, `db_exec`), and logging (`log_info`,
  `log_error`).
- **Sandboxed.** Each plugin declares a capability manifest (memory ceiling,
  allowed HTTP URL patterns, DB permissions) and runs under a per-call timeout.
- See [plugin-development.md](plugin-development.md) and
  [plugin-capabilities.md](plugin-capabilities.md).

## API & automation {#api-automation}

{{< screenshot src="api-keys" alt="The API keys management view in the admin profile." caption="API keys are created from the admin profile and used as Bearer tokens." >}}

- **Versioned REST API.** `/api/v1` covers Content, Media, and Comments. See
  [api-reference.md](api-reference.md).
- **Standard response envelope.** `{"data": ..., "error": {...}, "meta": {...}}`,
  with bare-array lists and cursor pagination on list endpoints.
- **API keys.** Personal `lesstruct_<keyID>_<secret>` Bearer tokens, scoped to the
  creating user, with revoke and expiry. Created from the admin profile.
- **`lesstruct-cli`.** A Cobra client for the same API — `content`, `media`,
  `comment`, and `config` subcommands; `--output text|json`; auth via `--api-key`,
  env, or config file.
- **Public content & author APIs.** Unauthenticated `/api/v1/public/*` endpoints
  serve rendered content, search, post types, and a **published-authors listing**
  (`/v1/public/authors`) for author directories and "most active contributors"
  widgets — only safe fields, never email/role/custom-fields.
- **Public custom-field filter & sort.** `/api/v1/public/content_items` and
  `/api/v1/public/authors` accept `cf_<field>`, `cf_<field>_min`,
  `cf_<field>_max`, and `sort_by=cf:<field>&order=asc|desc` so theme authors
  can build dynamic regions (recent-posts grids, "top N by ranking"
  sidebars, scoped directories) client-side without server-side queries.
  Each publicly-queryable field must be opted in via a `[[public_field]]`
  block in `config.toml` — the default is fail-closed (400
   `field_not_queryable`) so sensitive fields are never accidentally exposed.
   Fields allowlisted with the `"expose"` operation are also included in the
   response body (`publicFields` map on the authors endpoint), enabling client-side
   rendering of points, ranks, badges, and other live values.
   See [api-reference.md](api-reference.md) and [configuration.md](configuration.md).

## Users, roles & security {#users-roles-security}

- **Three roles.** Admin, Contributor, and Commentator — enforced by dedicated
  middleware on each realm.
- **First-run setup.** A default `admin/admin` account is auto-created on first
  start; the first login forces a password change. Self-registration creates
  `pending` Commentators an admin approves.
- **User management.** Admins CRUD users, assign roles, suspend/unsuspend,
  soft-delete, and moderate the registration queue (approve / reject / mark-as-spam).
- **Profiles.** Self-service profile (name, email, password, custom profile
  fields, avatar), self-service data export, and self-service account deletion.
- **JWT auth (admin realm).** Bearer-JWT sessions for the admin SPA, with Argon2id
  password hashing and transparent rehash-on-login for legacy bcrypt hashes.
- **Failed-login lockout.** An account locks for 15 minutes after 3 failed
  attempts, with an email notification.
- **Email verification and password reset.** Self-registration verifies via email
  token; forgot-password / reset-password flows are built in.
- **Rate limiting.** Separate per-minute limits for auth, API, and public realms;
  per-key limiting on the agent API.
- **CSRF and security headers.** CSRF token validation plus CSP,
  `X-Frame-Options`, `X-Content-Type-Options`, and `Referrer-Policy`. The CSP is
  [configurable from `config.toml`](configuration.md#csp) — operators append
  sources per directive, switch to Report-Only for safe rollout, add new
  directives, or override entirely (or disable when behind a CDN that manages
  its own CSP). Built-in defaults include `youtube-nocookie.com` (privacy-enhanced
  YouTube) alongside the existing `unsafe-inline` / known CDN hosts.

## Engagement {#engagement}

{{< screenshot src="dashboard" alt="The admin dashboard with published and draft counts, recent content, and moderation stats." caption="The dashboard — content and moderation at a glance." >}}

- **Comments with moderation.** Per-content comments with a moderation queue
  (`pending` / `approved` / `rejected` / `spam`) and a per-content allow/deny
  toggle.
- **Public search.** An on-site search box backed by `/api/v1/public/search`.
- **Dashboard.** Published/draft counts, users, pending registrations, media
  stats, and recent content in one view.

---

Missing something, or a feature reads stronger than it should? Features are kept
honest against the source tree — open an issue or PR. For the architecture behind
these features, read [project-context.md](project-context.md).
