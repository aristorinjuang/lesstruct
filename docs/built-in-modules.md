# Built-in Modules

Lesstruct v1 is built entirely from first-party Go modules. This page is the
reference for each one: what it owns, what it depends on, its admin-shell
entries, and — the operationally important part — what happens when you
disable it with the `[modules]` block
([configuration](configuration.md#modules)).

For authoring your own module, see the
[Module Development Guide](module-development.md); for the contracts it
compiles against, see [The Kernel](kernel.md).

## The graph at a glance

| Module | Requires | Provides | Admin menu |
|---|---|---|---|
| `users` | — | User management, profiles, profile pictures, account deletion | — |
| `auth` | `users` | Login, registration, verification, password reset, first-login setup | — |
| `comments` | `users` | Comment system end to end (agent, public, moderation) | Comments |
| `adminpanel` | `auth` | The admin SPA (embedded build) + the frozen extensions manifest API | — |
| `posttypes-basic` | — | Post-type registry, `/roles`, `/user_fields` | — |
| `media` | — | Media library, WebP pipeline, thumbnails, dedup | Media |
| `seo` | — | SEO meta, sitemap, robots, URL aliases | — |
| `site-ssr` | — | The SSR content-site engine, templates, aliases rendering | — |
| `theme-default` | — | The default theme (layouts, styles, root files) | — |
| `textgen` | — | AI text enhance/translate and HTML generation | — |
| `importers` | — | WordPress (WXR) and Hugo importers | Import |

## Disable semantics

`[modules] disabled = ["<name>"]` drops the module **and everything that
transitively requires it** (so `users` cascades away `auth`, `comments`,
and `adminpanel`). Two special cases worth knowing:

- `comments` disables exactly like the legacy `[comments] enabled = false`
  toggle — routes unmount everywhere.
- `site-ssr` is **stricter** than headless mode: no engine means no SSG
  export either. `[headless]` only suppresses public-surface mounting.

All other disables are lifecycle-level: the module skips init, migrations,
and the admin manifest, while its HTTP endpoints (if any) keep serving from
mounted handler instances — the full scope limits are documented in
[configuration.md](configuration.md#modules).

## The modules

### users

The user lifecycle: admin management (pending approval, suspension, soft
delete with content restore, admin creation with generated passwords),
self-service profile (email verification, password change, custom fields,
data export, account deletion), and profile pictures (upload, WebP
processing, admin per-user management). Leaf of the graph.

### auth

Authentication: credential login with JWT sessions and failed-login
lockout, self-service registration (with the `[registration]` block: custom
default role, admin approval), email verification, password reset, and
first-login bootstrap for default credentials. Requires `users`.

### comments

The comment system across all three realms: agent posting under content,
public per-item lists, and browser moderation (pending queue,
my-comments). Commentator-role gating flows through the same registry.
Requires `users`.

### adminpanel

Serves the single admin SPA — the embedded production build (dev-server
proxy in `DEV_MODE`) — plus the frozen extensions manifest endpoint the
shell consumes. Requires `auth`.

### posttypes-basic

The content-type registry: config-declared post types with field schemas,
served on authenticated `/post_types` and public `/post_types` endpoints,
plus the transitional `GET /roles` and `GET /user_fields` endpoints (these
transfer to the users module in a future release). Adopts the core
capability resolver for route authorization. Leaf.

### media

The media library: upload with WebP transcoding, thumbnail variants,
SHA-256 dedup, perceptual-hash duplicate detection (used by the
importers), and AI image generation. Leaf.

### seo

Discovery and identity: per-page SEO meta, OpenGraph, JSON-LD, the XML and
JSON sitemaps, robots.txt, and the `content_aliases` redirect system.
Leaf.

### site-ssr

The server-rendered content site: the template engine, content-page
assembler, listing/author/tag pages, and the embedder the SSG exporter and
`lesstruct-cli ssg` reuse. Disabling it removes rendering *and* static
export; use `[headless]` if you only want the public site suppressed.
Leaf.

### theme-default

Ships the default theme — embedded layouts, base/component stylesheets,
and root files — consumed by the site-ssr engine. Leaf.

### textgen

AI text features: enhance, translate, and on-brand HTML generation
(theme-aware prompts, media context). Bring-your-own key via the AI text
settings. Leaf.

### importers

Migrations in: WordPress (WXR — posts, custom fields, featured images via
media download + dedup) and Hugo (frontmatter, static assets, cover
images), both as async jobs with per-item warnings. Leaf.

---

Every module follows the same template and ships the same gates — build
your own with the [Module Development Guide](module-development.md).
