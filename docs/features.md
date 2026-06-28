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
  live preview — collapsible inside the editor.
- **Markdown as first-class ingest.** The CLI and `/api/v1` accept Markdown
  bodies; the server converts them to canonical TipTap JSON. Raw Markdown is
  never persisted.
- **WordPress importer.** Upload a WordPress WXR export to migrate posts, pages,
  and media into Lesstruct.

## Media & images {#media-images}

{{< screenshot src="media-library" alt="The media library: a searchable grid of uploaded images with thumbnails and metadata." caption="The media library — search, filter, and manage uploads." >}}

- **Media library.** Browse, search, and date-filter uploads from the admin panel.
- **Automatic WebP conversion.** Every uploaded image is transcoded to WebP
  (quality 80) on upload, so images never weigh down your content.
- **Configurable thumbnail variants.** Defaults ship `_thumb` (370px),
  `_medium` (800px), `_large` (1600px); all editable in `config.toml`. The content
  site emits a responsive `srcset` from them.
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
- **AI translation.** Translate content between your configured languages from
  the editor.

## AI {#ai}

- **Opt-in, bring-your-own-key.** Text via any OpenAI-compatible endpoint
  (`AI_TEXT_GENERATION_BASE_URL`); images via Google or OpenAI. Nothing runs
  without your keys; `/api/health` honestly reports which features are enabled.
- **Text enhancement and translation.** Refine or translate post bodies from the
  editor.
- **Image generation.** Generate images from the media library and the editor.
- **Built for agents.** `lesstruct-cli` is a thin Cobra client over `/api/v1`
  designed for AI agents and terminal-first humans. Markdown ingest, cursor
  pagination, and a standard response envelope make it easy to script.
- **Agent skills.** Lesstruct ships installable skills for theme and plugin
  development that work from your installed site (no source tree needed), for
  Claude Code, OpenCode, OpenClaw, and Hermes.
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
  `X-Frame-Options`, `X-Content-Type-Options`, and `Referrer-Policy`.

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
