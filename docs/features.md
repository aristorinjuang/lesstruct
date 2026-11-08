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
- **Static site generation (SSG).** Generate a fully static HTML site (tar.gz) with
  AMP variants for content pages, listing pages, author pages, tag pages, a
  sitemap, robots.txt, a `404.html` not-found page, and an RSS 2.0 feed of recent
  posts (`index.xml`) — all rendered from the same data layer as the live site.
  The archive bundles the active theme's `static/` directory overlaid on only
  the built-in assets your rendered pages actually reference (plus the two core
  stylesheets), so exports stay lean and render identically offline. Legacy
  URL aliases ship as self-contained meta-refresh redirect pages — their
  targets, like sitemap entries, AMP canonical links, and RSS feed item links,
  all use the trailing-slash directory form (`/<slug>/`) matching the
  `<slug>/index.html` export layout so they address the page rather than a
  flat alias stub; hosts should resolve directory indexes before flat `.html`
  files — and the theme's `root/` files (e.g. `webpushr-sw.js`) land at the
  archive root.
  Download from the admin panel under *Export*,
  via the agent API, or via `lesstruct-cli ssg` (use `--extract-dir` to unpack the
  files straight into a deploy directory instead of keeping the archive).
- **Multi-database.** Embedded SQLite is the default; PostgreSQL and MySQL are
  first-class via `DB_DRIVER`. Schema migrations run automatically on first start,
  per driver.
- **Two-layer configuration.** `config.toml` holds your content schema (languages,
  post types, fields, thumbnails); `.env` holds deployment state (host, port, DB,
  secrets, SMTP, AI keys). Only `JWT_SECRET` is required; everything else has a
  sensible default. See [configuration.md](configuration.md).
- **Plugin hot-reload.** In `DEV_MODE`, a file watcher reloads `.wasm` plugins
  without a server restart.
- **Headless mode.** `[headless] enabled = true` in `config.toml` turns the
  instance into a pure headless CMS: the server-rendered content site (catch-all
  and `/static/*`) is not served, only the admin panel and the REST API. The
  sitemap and robots.txt follow suit. See [configuration.md](configuration.md).
- **Comments can be switched off entirely.** `[comments] enabled = false`
  hard-disables the comment system end to end — no comment routes, no comment
  UI, `allowComments` forced off on every item, and self-registration blocked
  unless a `[registration]` block re-enables it. With a `[registration]` block,
  registration is decoupled from the comment system entirely: `enabled`,
  `default_role`, and `admin_approval` are configurable, so a site can let the
  public register into any role.
  See [configuration.md](configuration.md).
- **Modular under the hood.** Every first-party feature (users, auth, media,
  comments, SEO, AI, importers, …) is an independent kernel module with a
  declared dependency graph. Operators can shrink the running system with
  `[modules] disabled = [...]` — dropping `comments` equals the comments
  toggle, and dependents cascade with a logged warning.
  See [configuration.md](configuration.md).
- **Editions.** A fixed module list compiled into one binary. **Lesstruct**
  (the default) carries every module except commerce — the blog platform;
  **Lesstruct Core** is the headless engine (identity, content, media,
  plugins) for API-first sites, installing as `lesstruct-core`;
  **Lesstruct eCommerce** is the blog platform plus the commerce stack
  (catalog, cart, orders, payments) with the storefront theme, installing as
  `lesstruct-ecommerce`. Editions share one data dir and config; within an
  edition, `[modules] disabled` still fine-tunes.
  See [editions.md](editions.md).

## Content & authoring {#content-authoring}

{{< screenshot src="admin-hero" alt="The content editor: TipTap rich text with a custom fields panel and collapsible SEO settings." caption="The content editor — rich text, custom fields, and SEO in one view." >}}

- **Custom post types, built in.** Define post types in `config.toml` — no plugin,
  no library. The admin list, form, storage, and queries all read from that file.
  Built-in slugs (`post`, `page`, `media`, `comment`) extend instead of collide.
- **Admin content list with lazy loading.** The admin list pages through every
  item server-side (`limit`/`offset` + a total count) instead of capping at the
  first page, so sites with thousands of articles stay fully browsable. Scrolling
  loads the next page automatically, and a total-count badge on the page header
  shows how many items the current filter matches.
- **Custom fields, built in.** Add typed fields to any post type (and to user
  profiles) in `config.toml`. The admin form renders them, the service validates
  them, and they are queryable. No code required.
- **Array fields.** Three ordered, validated array types round out the
  vocabulary: `multiselect` (strings from a configured options list),
  `list` (free-form entries with per-item and item-count caps), and `gallery`
  (ordered `{url, alt}` media references with a media-panel picker in the
  admin). Legacy single-string values coerce on read and canonicalize to
  arrays on save, so switching a field's type in `config.toml` never breaks
  existing content.
- **Explicit featured image.** Every content item can carry an explicit
  featured image (set from the admin editor or the create/update APIs);
  public responses, og:image, and SEO metadata prefer it and fall back to
  the body-derived image when unset — existing sites render identically.
- **Catalog (eCommerce edition).** Products with native pricing (integer
  minor units), variant matrices, stock with low-stock thresholds, and SKUs
  — served to themes and headless frontends via a public catalog payload
  endpoint and managed from the product editor's Catalog panel (admin GET
  returns camelCase `contentId`, `salePrice`, `productContentId`,
  `updatedAt`). A tracked bucket crossing its threshold downward emails the
  configured `notify_email` once, so the operator restocks before checkout
  starts refusing. `[[catalog.display_currency]]` adds display-only
  currencies for the storefront header toggle (checkout stays base-currency).
  Translated products resolve through the translation group root when the row
  carries no record of its own (own records keep priority), and variant
  products derive in-stock/low-stock from the variant buckets — in stock when
  any variant is in stock, low when the variant sum sits within the threshold.
  The Catalog panel is a visibly separate form that saves apart from the
  content (with load-error retry and refetch-after-save): once variants exist
  the Stock field turns into a live read-only variant sum, variant images are
  picked from the media library, and `imageUrl` values must be http(s) or
  root-relative (a PUT otherwise fails 400 naming the field).
- **Guest cart (eCommerce edition).** Token-keyed server-side carts with
  live catalog pricing: no account required, prices resolve at read time
  so edits apply immediately, and out-of-stock demand is refused while
  quantities already in the cart may exceed stock.
- **Orders (eCommerce edition).** Guest checkout claims the cart with an
  atomic delete — one checkout per token, so parallel double submits can
  never mint two orders — and stores it as a first-class order with
  prices, SKUs, and titles frozen at the moment of sale; display numbers
  ship free (`ORD-000042`). An admin-set shipping line joins the frozen
  subtotal into the payable total while the order is pending — weigh the
  parcel, then charge it — and the confirmation email renders subtotal,
  shipping, total, and the order's tracking link. The domain-enforced
  lifecycle (`pending → paid → packed → shipped → delivered`, with
  cancel/refund/expiry branches) is managed from the admin orders screen
  with best-effort customer emails on every move, and guests track their
  order behind a three-factor gate — order number, checkout email, and a
  128-bit per-order tracking token delivered in the checkout response and
  the confirmation email (sequential numbers and knowable emails are
  deliberately not enough; legacy email-only tracking is gone, so orders
  placed before the token existed are untrackable through the guest gate).
  Guest payment proofs and hosted sessions pass the same gate. The orders
  list carries
  a read-time `itemCount` aggregate so the column never reads zero. Stock is claimed at
  payment (not checkout) and returns when paid or packed orders cancel or
  refund; a background sweeper expires stale pendings on a configurable
  window, and paid is reachable only through payment confirmation — no
  surface can mint a payment. Any order streams an invoice PDF on
  demand — frozen lines, shipping, and a display-only PPN breakdown
  beside an unchanged total, with seller identity and PPN rate from the
  `[invoice]` block, editable at runtime under **Settings → Invoice
  settings** (the admin write-back rewrites only that table of
  `config.toml`, atomically, and applies to the next invoice without a
  restart). Money
  renders per currency in the emails, payment instructions, and the
  invoice — zero-decimal for IDR/JPY/KRW/VND, two-decimal minor units
  otherwise; three-decimal currencies are unsupported — matching the
  storefront. Customer PII (email, name, phone, shipping address) is
  encrypted at rest (AES-256-GCM) at the store boundary and resolved for
  guest tracking through a blind-index column; the commerce edition fails
  startup closed without a valid `PII_MASTER_KEY`.
- **Payments (eCommerce edition).** Bank-transfer proof flow out of the
  box: guests submit a payment reference behind the order's tracking gate
  (order number + email + per-order tracking token),
  administrators verify or reject the pending proofs from the payments
  admin surface, and the amount snapshots with the record so later price
  edits never rewrite it. The optional `[payments.manual]` payee block
  (bank/account/name, all-or-nothing) renders a "How to pay" section in the
  confirmation email and the guest tracking panel. The Midtrans Snap driver (adopted when
  `MIDTRANS_SERVER_KEY` is set and the method is listed) mints hosted
  checkout sessions and folds signed webhook events — settlements confirm
  the order and record the transaction, refunds transition it refunded,
  forged signatures refuse. Rejection keeps the order open for a
  corrected resubmission; both decisions email the customer. Proof reference
and verifier identity are encrypted at rest (AES-256-GCM) at the store
boundary, with webhook dedup resolving through a blind-index column.
- **TipTap rich-text editor.** Tables, math (KaTeX), syntax-highlighted code
  blocks, emoji, YouTube embeds, links, images, and text alignment — all
  first-class. External links (`http://`, `https://`, `//`) always render
  exactly `rel="noopener noreferrer" target="_blank"` on the published page —
  never `nofollow` — while internal and relative links never carry `target` or
  `rel`; pasting a link leaves the next typed character as plain text (the link
  does not swallow following text).
- **Draft and publish.** A two-state workflow (`draft`, `published`) with
  publish/unpublish actions exposed in the admin and the CLI.
- **Soft-delete and restore.** Deleted content is recoverable from the admin
  trash view.
- **Per-content SEO.** Meta description, OpenGraph title and description, and a
  live preview — collapsible inside the editor. SEO metadata generation is
  format-aware: TipTap content uses JSON extraction, HTML content uses
  tag-stripping — so HTML-format imports (e.g. WordPress Elementor pages)
  get real extracted text in their meta descriptions, not empty strings.
  Image extraction accepts both absolute and root-relative (`/uploads/…`) image
  URLs; derived descriptions collapse newlines/whitespace; an unset
  og:description mirrors the resolved meta description (Hugo parity); and a
  post without any image emits no `og:image` at all instead of a bare domain,
  letting the theme's default image apply.
- **Immutable URL slugs.** The slug is the public URL (`/<slug>`). Slugs may contain lowercase letters, digits, hyphens, and dots (no leading/trailing dot, no `..`); `jquery.semantic-tabs` is a valid slug, with legacy `jquery.semantic-tabs.html` served via the alias redirect.
  Any authenticated user can set a custom slug when creating content (in the
  editor or via `--slug` on `lesstruct-cli content create`); otherwise the slug
  is auto-generated from the title. The slug stays editable while the content
  is a draft (manually, via title auto-suggest, or via Enhance with AI). Once
  **published**, the slug is **locked for everyone** — editing the title never
  regenerates it, so published URLs stay stable for SEO and inbound links.
  Uniqueness is enforced per language; a collision returns a clear error rather
  than silently suffixing.
- **HTML/CSS content authoring.** Set `format: html` on create to author raw
  HTML directly — stored and served as-is, no TipTap conversion. The admin
  editor provides a CodeMirror 6 editor with syntax highlighting and a live
  preview (sandboxed iframe). HTML content is sanitized on write (dangerous
  elements/attributes stripped via bluemonday; inline `style` and class
  attributes preserved) and on read (rendered through the same policy).
  Root-relative URLs (`/uploads/media/...`, same-site links) survive the
  sanitizer, and `<iframe>` embeds are kept when their host is allowed by the
  CSP `frame-src` directive (defaults + appends, `*.host` wildcards cover
  subdomains only — YouTube works out of the box). Ideal for WordPress
  Elementor imports and hand-authored HTML pages.
- **Markdown as first-class ingest.** The CLI and `/api/v1` accept Markdown
  bodies; the server converts them to canonical TipTap JSON (CommonMark plus
  GFM tables and strikethrough). Raw Markdown is never persisted.
- **WordPress importer (async).** Upload a WordPress WXR export to migrate
  posts, pages, custom post types (and their custom fields), media, and authors
  into Lesstruct. The import runs in a background goroutine and returns `202
  Accepted` immediately with a `jobId`; poll
  `GET /api/admin/wordpress/import/status/{jobId}` to track progress. Authors
  are auto-created as Contributor users and their posts are assigned to them.
  Custom post types and field schemas are read from `config.toml`; items whose
  post type is not registered are silently skipped. Featured images
  (`_thumbnail_id`) are resolved from attachment items, downloaded, transcoded
  to WebP, and prepended to each post's content body — unless the body already
  shows the same picture among its first images (same source URL, or a
  perceptually identical image under a different URL), in which case the
  prepend is skipped so the cover is never duplicated; perceptual skips are
  reported as `warning:` entries. Inline body images are likewise downloaded
  and remapped (downloading is concurrent with a bounded worker pool; transient
  errors are retried with backoff). Failed downloads fall back to hotlinking
  the original WordPress URL. Elementor-built pages are imported as
  `format=html` using the rendered HTML from the Elementor cache, preserving
  their original layout.
  - **Skip-media option.** When importing, you can optionally skip downloading
    media (inline images and featured images). Content imports with original
    WordPress URLs hotlinked — useful for fast imports when you already have
    the media or want to defer downloading. Available in the admin UI
    (checkbox) and the CLI (`--skip-media` flag).
  - **CLI import.** The `lesstruct-cli import wordpress --file <path> [--skip-media]`
    command imports the same WXR file via the `/api/v1` agent-realm endpoint
    using an API key. It uploads the file, then polls the status every few
    seconds, printing live progress until the job completes.
- **Hugo importer.** Import a Hugo site to migrate posts (HTML or Markdown
  with YAML frontmatter) into Lesstruct. The import runs asynchronously as a
  background job (202 Accepted + job ID) with a status endpoint for progress
  tracking — identical to the WordPress importer. Hugo `{{</* highlight */>}}`
  shortcodes (including whitespace-tolerant closing tags) are converted to
  server-rendered, syntax-highlighted code blocks via chroma — class-based HTML
  honoring `linenos=table|inline` and escaping code bodies, so themes can style
  them with their existing `.chroma` CSS; unknown languages fall back to plain
  escaped code blocks. `{{</* iframe */>}}`
  shortcodes become `<iframe>` elements. YAML frontmatter fields (`title`, `date`,
  `tags`, `description`, `url`, `aliases`, `draft`, `language`, `hasMath`,
  etc.) are mapped to their Lesstruct equivalents — including `date`, which
  becomes the content's publish date instead of the import time. Old `.html`
  URLs are stored as aliases in the `content_aliases` table so the public
  content site can issue 301 redirects from the old path to the new clean
  slug. Bilingual en/id pairs are linked as translations when they share a
  directory.
  - **Media migration.** Images referenced by the content (local `static/`
    files and remote `https://` URLs) are downloaded and re-uploaded as
    Lesstruct media (already-WebP input passes through sanitize-only —
    metadata stripped, pixels untouched; other formats are transcoded;
    SHA-256 dedup); body `<img src>` paths
    are rewritten to the new media URLs and the first frontmatter `images:`
    entry is prepended as a featured image — unless the body's first images
    already show that picture, in which case the prepend is skipped so the
    cover is never duplicated. The duplicate check compares the body's first
    three image URLs exactly and their perceptual hashes (8×8 average hash),
    catching covers that reappear under a different URL — a re-upload, resized
    export, or hotlink variant that SHA-256 dedup cannot see. Perceptual skips
    are reported as `warning:` entries in the import result. Use the
    **skip-media** option to import text only with images hotlinked — available
    in the admin UI (checkbox) and the CLI (`--skip-media` flag).
  - **Static references survive.** References that resolve to files under the
    Hugo `static/` dir — links, iframe demos, stylesheets, and images that
    could not be migrated — are rewritten to `/static/<path>`, the documented
    convention (operators mirror their Hugo `static/` into the theme's
    `static/`). Failed media migrations and references left unresolved (dead
    links, missing images) are surfaced as `warning:` entries in the import
    job's `errors` list — each exactly once — instead of failing silently;
    content permalinks and aliases stay silent.
  - **Idempotent re-runs.** Items whose slug already exists are skipped as
    "already imported", so re-running after a partial failure is safe.
  - **Legacy aliases survive delete + re-import.** Deleting a post removes its
    alias rows (no more dangling `content_aliases`), and a re-import
    automatically re-points an existing alias whose target no longer exists
    onto the freshly imported item — the documented delete-and-reimport
    remediation no longer breaks legacy `.html` redirects. Archives built with
    default `tar -czf site.tar.gz .` flags (including the leading `./` entry)
    are accepted.
  - **Admin UI.** Upload a `.tar.gz` archive (containing at least a
    `content/` directory) under *Import → Hugo*; the UI polls the job status
    and shows a progress bar plus per-item issues. The site's `config.toml` /
    `hugo.toml` is read for `baseURL` and `defaultContentLanguage`.
  - **CLI import.** `lesstruct-cli import hugo --source <path> [--skip-media]`
    accepts either a Hugo project **directory** (auto-archived: only
    `content/` and `static/` are sent) or an existing `.tar.gz` archive, and
    uploads it via the `/api/v1/hugo/import` agent-realm endpoint using an API
    key, then polls the job status until it completes.
  - **Known limitations.** Hugo sections are imported as `post` type (custom
    sections are not yet mapped to post types, and `_index.md` section
    landing pages are not skipped); directory-based i18n (`content/en/` +
    `content/id/`) is not yet detected — only the `.id.html` filename suffix
    and `language:` frontmatter are recognized; Disqus comments are not
    migrated. See [project-context.md](project-context.md) for the
    architecture and [api-reference.md](api-reference.md) for the endpoint.

- **Content export.** Download all content as a Hugo-compatible `tar.gz`
  archive (`lesstruct-export-<timestamp>.tar.gz`). Each content item becomes a
  `<postType>/<slug>.<language>.html` file with YAML frontmatter (`title`,
  `date`, `description`, `tags`, `url`, `language`, `aliases`, `draft`,
  `lastmod`, custom fields). TipTip body content is rendered to HTML before
  export. Referenced media files are bundled under `static/uploads/media/`
  with the HTML `src` attributes unchanged (Hugo serves `static/` at site
  root, matching Lesstruct's own `/uploads/media/` path). Available from the
  admin panel under *Export*, via the admin API (`/api/admin/export`), and via
  CLI (`lesstruct-cli export --output-dir <dir>`).

## Media & images {#media-images}

{{< screenshot src="media-library" alt="The media library: a searchable grid of uploaded images with thumbnails and metadata." caption="The media library — search, filter, and manage uploads." >}}

- **Media library.** Browse, search, and date-filter uploads from the admin
  panel. Admins manage the whole library; every other role sees only their own
  uploads, so one account cannot inventory another's file paths, hashes, and
  original filenames.
- **Automatic WebP conversion.** Every uploaded image is served as WebP:
  already-WebP uploads pass through byte-identical (their
  EXIF/XMP/ICC metadata chunks are stripped — the same privacy behavior the
  transcode path always had — with zero generation loss, so the stored
  colors match the source exactly). Every other format is transcoded to
  WebP (quality 80) on upload, so images never weigh down your content.
  Files uploaded before this change keep their exact stored bytes — run the
  one-time `POST /api/v1/media/reencode` (Admin only, via `lesstruct-cli
  media reencode`) to rewrite every stored original and variant in place at
  identical dimensions with no URL or hash changes (stored file sizes are
  updated to match).
  Extended VP8X/alpha WebP files are fully supported; animated WebP is
  rejected with a clear error.
- **Configurable thumbnail variants.** Defaults ship `_thumb` (64px),
  `_medium` (280px), `_large` (616px) — gallery thumbs, cards, gallery main —
  all editable in `config.toml`. Post
  bodies emit a responsive `srcset` from the variants only (`src` stays the
  original URL) with `sizes="(max-width: 768px) 100vw, 760px"` matching the
  content column, wrapped in a `<picture>` element whose mobile source caps
  small viewports (≤767px) at the `_medium` rendition — small screens get
  appropriately sized variants without
  over-fetching the full-resolution original. Post cards resolve the `_medium`
  rendition (`_thumb` fallback) and also expose an
  `ImageVariants` map (keyed by configured suffix) and an `OriginalURL` field
  for the unscaled original, enabling hero backgrounds and high-DPI layouts.
  Listing cards wrap the image in a mobile-capped `<picture>` element
  (mobile source loads `_thumb`; desktop loads `_medium` with no `srcset`
  ladder), so no viewport reaches the `_large` rendition. Gallery-main
  markup uses the same `<picture>` wrapper with a `_medium` mobile source.
  Gallery-typed custom fields resolve to per-image `_thumb`/`_large` renditions
  (matched by suffix name with original-URL fallbacks) that themes prefer over
  the legacy `lines` list.
- **SHA-256 dedup.** Identical uploads are detected and rejected (with a
  force-upload escape hatch).
- **Privacy blurring (opt-in).** Set `AI_SAM_API_KEY` to scan every media and
  profile-picture upload with Meta SAM 3.1 and blur faces, plates, and nudity
  before anything is stored — variants inherit blurred pixels, scan failures
  reject fail-closed with nothing stored, and duplicates dedup on the blurred
  hash. Faces stay visible only in profile pictures. See [configuration.md](configuration.md).
- **AI image generation.** Generate images from the media library and the content
  editor via Google Imagen, Gemini, GPT-Image, or Meta Muse Image (any
  OpenAI-compatible images endpoint). Bring your own key. Gemini,
  GPT-Image, and Muse Image models additionally accept up to 3 reference images (uploads or
  media-library picks) to guide style, subject, or composition. An "Open Graph
  image" mode generates an exact 1200x630 social preview (center-cropped) and,
  from the editor, inserts it at the top of the content — og:image is always
  the content's first image.

## Internationalization {#internationalization}

- **Multilingual by default.** Declare your languages in `config.toml`
  (e.g. `languages = ["en", "id"]`). Content carries a `Language` and authors
  link translations into translation groups.
- **Listing fallback.** Public listings (homepage, sections, post-type, tag and
  author pages) show each translation group once, preferring your configured
  language order — a post without a primary-language version still appears via
  its translation instead of disappearing.
- **Locale negotiation.** The live server negotiates per request: `?lang`
  first (sets the `lesstruct_lang` cookie unless consent is declined and redirects a post to its
  translation when one exists; bogus codes never error), then the cookie for
  returning visitors, then `Accept-Language`, then the primary. Negotiated
  listings render Indonesian-first with `<html lang="id">`, and both themes
  ship a `?lang` dropdown showing the active code uppercased (`ID`/`EN`)
  with links preserved and bogus codes ignored.
- **Engine strings localized.** Listing descriptions come from the i18n catalog (`ui.browse_posts`, `ui.posts_by`, `ui.posts_tagged` in `kernel/i18n/translations/{en,id}.toml`): Indonesian listings render `Jelajahi …` / `Postingan oleh …` / `Postingan dengan tag …`, with English fallback when a key is missing.
- **Localized config display strings.** Optional `[[post_type]] name_id`/`description_id` and `[[homepage_section]] title_id`/`description`/`description_id` override the base values when the negotiated language is Indonesian (base fallback otherwise). Homepage sections resolve via `buildHomeSections`; listing heads resolve via the post type.
- **GDPR cookie consent.** Both themes render an Accept/Decline banner (`ui.cookie_notice`/`ui.cookie_accept`/`ui.cookie_decline`). When a cookie policy — or, failing that, a privacy policy — resolves, the banner switches to `ui.cookie_notice_policy` and links the page in the visitor's language; without one it renders the same plain notice as before. The copy follows the edition: the blog banner is neutral, while the storefront keeps naming its language, currency, and cart storage (`ui.cookie_notice_store` pair). `lesstruct_consent=accepted` (1yr, `Lax`, `Secure` on HTTPS) keeps current behavior; `lesstruct_consent=declined` renders `?lang=id` Indonesian with no `Set-Cookie` and keeps the currency picker view-scoped (nothing persisted). The consent cookie itself is exempt; the cart token/JWT always works (strictly necessary, no consent gate). The global NoCookie guard stays fail-closed (allowlists `lesstruct_lang` + `lesstruct_consent`, strips and logs everything else).
- **Legal pages & server-enforced consent.** Legal documents stay ordinary bilingual `page` content: publish the conventional English slugs (`cookie-policy`, `privacy-policy`, `terms-of-service`, `returns-refunds`) and each translation group resolves in the visitor's language; `[legal]` URL overrides win where a page should live elsewhere. The footer always lists the full site map, and legal pages join the header only when `legal_nav = "header"`/`"both"`. With the terms and privacy documents resolving (or a `[legal]` flag forcing the gate), checkout and registration render a required terms/privacy checkbox **and** enforce it server-side — `400 CONSENT_REQUIRED` without an affirmative `consentAccepted`, JavaScript or not — recording the accepted revision (page ID + last-updated timestamp, plus slug and language) on the order or user row.
- **Translation-aware SEO.** The sitemap declares `hreflang` alternates from
  translation groups.
- **Localized date formatting.** Post dates follow each content item's
  `Language` field: Indonesian content renders `1 Januari 2026`, English content
  renders `January 2, 2006`. Custom-field dates localize automatically.
- **AI translation.** Translate content between your configured languages from
  the editor — translation also covers the title and SEO meta description
  (mirrored to the OG title).

## AI {#ai}

- **Opt-in, bring-your-own-key.** Text via any OpenAI-compatible endpoint
  (`AI_TEXT_GENERATION_BASE_URL`); images via Google or OpenAI. Nothing runs
  without your keys; `/api/health` honestly reports which features are enabled.
- **Text enhancement and translation.** Refine rich-text (TipTap) post bodies from the editor — enhancement also rewrites the title and SEO meta description (mirrored to the OG title) — or translate between languages.
- **Voice dictation.** Speak instead of typing: record up to five minutes in the content editor, get back formatted TipTap content (punctuation, headings, lists, links, tables, math) with a suggested title and meta description. Transcribed by Meta Muse Voice Transcribe (or any OpenAI-compatible speech endpoint), enhanced by your text model, previewed before inserting — and applied to empty fields only.
- **AI-powered HTML/CSS authoring.** Describe what you want in plain language — the AI generates production-ready HTML & CSS with semantic markup, responsive layouts, and accessible design. Output is on-brand by default: the AI reuses your active theme's design tokens (`var(--color-primary)`, spacing, radius, fonts) and component classes rather than inventing arbitrary colors and fonts. Includes 9 quick-start presets (hero, pricing, testimonials, features, CTA, FAQ, stats, contact, newsletter) and iterative refinement — the AI can modify existing HTML based on your follow-up instructions. Your media library images are automatically surfaced as context. This replaces the need for a drag-and-drop page builder: describe, generate, refine, ship. Note: saving `format: html` content strips `<style>` blocks for every author (page CSS belongs in the theme; inline `style=""` attributes are unaffected).
- **Image generation.** Generate images from the media library and the editor.
  Gemini, GPT-Image, and Muse Image models accept optional reference images (uploaded or
  picked from the library) to guide the result. Open Graph mode produces an
  exact 1200x630 preview, auto-inserted at the top of the content it was
  generated from.
- **Recommended Meta models.** AI features accept any OpenAI-compatible
  endpoint, but these are the Meta models Lesstruct recommends (configure the
  keys in [configuration](configuration.md)): **Muse Wonder** powers the
  Lesstruct Module Development skill (optimized for Muse Code and Muse Wonder);
  **Muse Spark** enhances writing and translates content into any language
  defined in `config.toml`; **Muse Voice Transcribe** turns recorded speech into
  publish-ready content, enhanced and formatted by Muse Spark; **Muse Image**
  generates any image including exact 1200x630 Open Graph previews, guided by
  up to three reference images; **Muse SAM** blurs faces, license plates, and
  nudity in uploaded images before anything is stored.
- **Built for agents.** [`lesstruct-cli`](cli.md) is a thin Cobra client over `/api/v1`
  designed for AI agents and terminal-first humans. Markdown ingest, cursor
  pagination, and a standard response envelope make it easy to script.
- **Agent skills.** Lesstruct ships installable skills for theme, plugin,
  and module development: theme and plugin skills work from your installed
  site (no source tree needed); the module skill works from a cloned
  checkout. Install them with `npx skills add aristorinjuang/lesstruct` —
  works with Claude Code, OpenCode, Cursor, Codex, and 25+ other agents.
- **Crawlable docs.** This site publishes `/llms.txt` (page index),
  `/llms-full.txt` (every page concatenated), and a per-page Markdown mirror for
  retrieval pipelines.

## Themes & rendering {#themes-rendering}

- **Server-rendered by default.** The content site is rendered server-side with
  Go `html/template` — fast and SEO-friendly.
- **One embedded theme per edition.** The blog edition ships the default blog
  theme; the eCommerce edition ships the storefront theme — brand-styled
  product cards and product pages with cart, checkout, and order-tracking
  shells. Neither generates a new theme per release cycle; both are the
  working starting point for a `THEME_DIR` child theme.
- **Commerce storefront.** The eCommerce edition's embedded storefront theme
  carries the Lesstruct brand design and turns the content site into a
  storefront: product cards hydrate price, sale price, and variant-derived
  stock posture (in stock when any variant is in stock, low when the variant
  sum sits within the threshold) from the public catalog API, product pages
  render a multi-axis variant matrix (configured axis/value order, sticky
  availability, combined price/stock/image/SKU)
  and add-to-cart, and the standalone `/cart`, `/checkout`, and `/orders/track`
  pages render through the same layout while `storefront.js` hydrates them
  from the public cart, checkout, order, and payment APIs — static-page fast
  and fully themeable. Content pages keep the comment section when comments
  are enabled. Prices format per currency (zero-decimal for
  IDR/JPY/KRW/VND, two-decimal minor units otherwise; three-decimal
  currencies are unsupported). The header runs Cart → Currency → Language →
  Log out: the currency toggle converts card and product prices through
  `[[catalog.display_currency]]` (checkout stays base-currency) alongside a
  language dropdown showing the active code uppercased, the hero
  photo/title/subtitle come from `[site_config]`,
  and the logo renders beside the site name when set.
- **Customizable.** Point `THEME_DIR` at a `themes/<name>/` directory to override
  CSS, JS, and HTML templates. The contract (CSS variables, layout blocks, JS DOM
  ids, CDN assets) is documented so fork-and-modify is safe. See
  [theme-development.md](theme-development.md).
- **Root-level theme files.** A theme's optional `root/` directory is served at
  the site root (and copied to the static-export archive root) — so fixed-URL
  files like `/webpushr-sw.js` or `.well-known/` verifications ship with the
  theme instead of needing host-level copy hacks. Existing paths always win:
  a `root/` file never shadows content pages or aliases.
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
- **Per-post scripts.** Declare a `post_script` custom field (`textarea`) on a post type in `config.toml` to opt in; its raw HTML is emitted verbatim at the end of the post via `{{.PostScripts}}` (excluded from the visible custom-fields section and stripped from AMP). Only declare it on types whose editors are fully trusted (like Ghost's per-post code injection), and ensure your CSP allows what you emit — external `src` is `'self'`-clean, inline needs `'unsafe-inline'`.
- **Multi-type aware.** Every post card and single-page template receives the
  item's `.PostType`, so a theme can branch layouts for articles, events, and any
  custom post type from one template set. Cards also carry the post's raw
  `.CustomFields` (e.g. `{{index .CustomFields "link"}}`), so list templates can
  branch links or badges on field presence server-side — no client-side JS.
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
  `config.toml` sets the site `name`, an optional `logo`, an optional
  `favicon`, an optional `product_image_ratio`, and optional storefront hero
  overrides (`hero_image`, `hero_title`, `hero_subtitle`). The name and logo drive
  the browser-tab title suffix, `og:site_name`, the default logo text, and the
  footer (the storefront header shows logo plus name together when a logo is
  set); `favicon` replaces the embedded themes' bundled icon set with one
  image for the tab, bookmarks, and home screen. `product_image_ratio`
  (`"W:H"`/`"W/H"`, validated at load) overrides the storefront product
  media aspect ratio via an inline `<style>` (unset keeps the hard-coded 4/5).
  `hero_image` swaps the homepage monogram for a photo, while `hero_title`
  and `hero_subtitle` replace the bilingual defaults.
  This is the one branding surface a theme override cannot reach (the name is
  baked into handler-side `PageTitle` strings); everything else (social links,
  analytics, custom `<head>`, image/multi-logo layouts) stays a `THEME_DIR`
  theme concern.
- **Paginated listings.** Homepage, author, tag, and post-type listing pages
  accept `?page=N` and expose prev/next state to templates. Page size is tuned
  via `POSTS_PER_PAGE`.
- **SEO built in.** `sitemap.xml`, `robots.txt`, JSON sitemap, and `hreflang` are
  generated for you.

## Extensibility {#extensibility}

- **WebAssembly plugins.** Drop a compiled `.wasm` into `plugins/` and it hooks
  into the content lifecycle. Any language that compiles to Wasm works.
- **Familiar hook model.** Explicit registration, priority-based execution,
  immutable data flow. Invoked hooks: `before_save` (create, update, and
  admin system-fields updates), `after_create`, `after_publish`,
  `before_delete`, `after_unpublish`; reserved for forward compatibility:
  `on_plugin_loaded`.
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
- **[`lesstruct-cli`](cli.md).** A Cobra client for the same API — `content`, `media`,
  `catalog`, `orders`, `payments`, `users`, `invoice-settings`, `comment`, and
  `config` subcommands; `--output text|json` (errors also JSON); auth via `--api-key`,
  env, or config file. On eCommerce builds the same Admin API key operates the store
  (dual-realm commerce endpoints plus the agent user admin), so agents can run the operator loop —
  upsert catalog data, list and transition orders, set shipping, decide payment proofs,
  triage registrations, suspend accounts, and edit invoice settings — without a browser session.
- **[`lesstruct-mcp`](mcp.md).** A stdio MCP server over the same API — 44 tools
  across content, media, catalog, orders, payments, users, invoice settings,
  comments, import/export/SSG, plus a `lesstruct_api` escape hatch — so any
  MCP-native agent (Claude Code, OpenCode, Cursor) can operate the site and the
  store with the same Admin API key, no browser session needed.
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

- **Configurable roles.** Three built-in roles — Admin, Contributor, and
  Commentator — enforced by dedicated middleware on each realm. Sites can add
  custom roles or override the built-ins via `[[role]]` entries in `config.toml`,
  each granting a per-post-type manage set plus publish, media, and comment
  capabilities. Content, media, and comment endpoints enforce these
  capabilities (Commentator is comments-only — no media — unless a site
  re-enables it with a `[[role]]` override), and the admin UI gates navigation and forms off the caller's
  derived capabilities — the content editor hides its Publish button (and the
  Published status option) for roles without the `publish` capability.
  The admin panel lands each role on a page it can use: Admins on the
  dashboard, Commentators on their profile, everyone else on the content
  list (the logo click returns there too).
- **First-run setup.** A default `admin/admin` account is auto-created on first
  start; the first login forces a password change. Self-registration creates
  `pending` Commentators an admin approves (or a configurable default role;
  see `[registration]`).
- **User management.** Admins CRUD users, assign roles, suspend/unsuspend,
  soft-delete, and moderate the registration queue (approve / reject / mark-as-spam).
  The pending queue shows each registrant's email-verification status; with
  `[registration] admin_approval = true` a registrant must verify their email
  before an admin can activate them (`409 EMAIL_NOT_VERIFIED` otherwise).
- **Profiles.** Self-service profile (name, email, password, custom profile
  fields, avatar), self-service data export, and self-service account deletion.
- **Cookie sessions (admin realm).** Browser sessions travel in the
  `lesstruct_jwt` HttpOnly cookie (`HttpOnly`, `SameSite=Lax`, `Path=/`,
  `Secure` exactly on HTTPS; lifetime matches the token expiry), set by
  MFA-verify and cleared on logout — no token lives in `localStorage` or in
  any login JSON. Bearer JWTs remain accepted for compatibility (a present
  header wins over the cookie), but programmatic clients should use API keys —
  no login endpoint returns a token to store. Argon2id
  password hashing and transparent rehash-on-login for legacy bcrypt hashes.
  Sessions are revocable: every token carries the account's `token_version`, and
  the middleware re-checks it (plus the account's existence and status) on every
  authenticated request. A password change, reset, or logout bumps the version, so older
  tokens and cookies stop working immediately instead of surviving until their 24-hour
  expiry. Suspended and soft-deleted accounts lose access on the next request,
  across both realms — API keys issued by such an account are rejected too.
- **Username or email login.** The admin SPA and the storefront both accept
  either the account's username or its unique email address in the login
  field. Usernames cannot contain `@`, so an identifier containing `@` is
  resolved as an email and everything else as a username.
- **Mandatory email MFA.** Every login — admin SPA and storefront alike —
  requires a 6-digit code emailed to the account's verified address after the
  password step; the session cookie is set only after the code matches, and no
  token is ever returned in a login body. Codes expire after
  5 minutes, are single-use, allow 5 wrong attempts, and can be re-sent after
  60 seconds. Without SMTP, login fails closed; in development only
  (`DEV_MODE=true` with SMTP unconfigured) the code is printed to stdout. The
  seeded admin before first-login setup is the sole bootstrap case: login
  returns `firstLoginRequired` with no session so the setup page can set a real
  password and email. See [SMTP](configuration.md#smtp).
- **Failed-login lockout.** An account locks for 15 minutes after 3 failed
  attempts, with an email notification.
- **Email verification and password reset.** Self-registration verifies via email
  token — verification is always mandatory — and with `admin_approval = true` the
  verified registrant stays `pending` until an administrator activates the
  account; forgot-password / reset-password flows are built in. Registration is
  enumeration-safe: a duplicate username or email gets the exact same `200`
  reply as a fresh registration (no account ID, no `ACCOUNT_EXISTS` error), and
  the existing account's holder is the one who gets told — by a notice email
  with no links and no token in it.
- **Rate limiting.** Separate per-minute limits for auth, API, and public realms;
  per-key limiting on the agent API, plus an outcome-based per-IP budget on
  Bearer 401s — rotating fake API keys trips `429 RATE_LIMITED` after 15
  failures/min from one IP while valid keys never count.
- **CSRF and security headers.** CSRF token validation plus CSP,
  `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, and
  `Permissions-Policy` (camera, microphone, geolocation, and browsing-topics
  disabled). The deprecated `X-XSS-Protection` header is gone — CSP is the
  XSS control. HSTS is derived from the public origin: an `https://` `SITE_URL`
  emits `Strict-Transport-Security: max-age=31536000` (override or disable via
  `HSTS_MAX_AGE_SECONDS`), plain-HTTP origins omit it. The CSP is
  [configurable from `config.toml`](configuration.md#csp) — operators append
  sources per directive, switch to Report-Only for safe rollout, add new
  directives, or override entirely (or disable when behind a CDN that manages
  its own CSP). Built-in defaults include `youtube-nocookie.com` (privacy-enhanced
  YouTube) alongside the existing `unsafe-inline` / known CDN hosts, and
  `img-src` is pinned to same-origin, `data:`, and `blob:` — off-site images
  (and CSS tracking beacons) need an explicit `img_src` append, with the
  S3/CDN storage origin added automatically when `STORAGE_S3_PUBLIC_BASE_URL`
  is set. The same
  `frame-src` sources also govern which `<iframe>` embeds survive the HTML
  sanitizer in HTML-format content — a full `policy` override replaces the
  sanitizer allowlist too, while `disable`/`report_only` keep the built-in
  allowlist as a safety net.
- **Configurable frame protection.** Same-origin framing is opt-in via
  [`[csp] frame_ancestors`](configuration.md#csp): `["'self'"]` relaxes the
  default `frame-ancestors 'none'` and switches `X-Frame-Options` to
  `SAMEORIGIN` (host allowlists omit the legacy header entirely, flooring to
  `DENY` under report-only so trials stay protected), so sites can embed
  their own demo/interactive pages in `<iframe>`s — with `X-Frame-Options`
  auto-derived from the same knob so the two headers never contradict.

## Engagement {#engagement}

{{< screenshot src="dashboard" alt="The admin dashboard with published and draft counts, recent content, and moderation stats." caption="The dashboard — content and moderation at a glance." >}}

- **Comments with moderation.** Per-content comments with a moderation queue
  (`pending` / `approved` / `rejected` / `spam`) and a per-content allow/deny
  toggle.
- **Public search.** An on-site search box backed by `/api/v1/public/search`, spanning every published, non-hidden post type — so a product catalog (or any custom-post-type site) is searchable out of the box, with each result labeled by its type.
- **Dashboard.** Published/draft counts, users, pending registrations, media
  stats, recent content, and a per-post-type content breakdown (each card links
  to that type's filtered list) in one view.

---

Missing something, or a feature reads stronger than it should? Features are kept
honest against the source tree — open an issue or PR. For the architecture behind
these features, read [project-context.md](project-context.md).
