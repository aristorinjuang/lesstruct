# Lesstruct API Reference (`/api/v1`)

Lesstruct exposes a versioned, API-key-authenticated REST API at `/api/v1` for creating, reading, updating, and deleting **Content** and **Media**. It is designed for programmatic consumers — the `lesstruct-cli`, MCP servers, AI agents (Claude Code, OpenCode, Hermes, …), and human integrators — and accepts Markdown as a first-class authoring format.

This reference documents the **implemented** surface. For the design intent, see `_bmad-output/planning-artifacts/architecture-ai-cli.md`.

## Overview

- **Base URL.** The API is served from the same origin as your Lesstruct server, under the `/api/v1` prefix. Example: `https://your-lesstruct.example/api/v1/content`.
- **Transport.** HTTPS in production. All request and response bodies are `application/json`, except media upload which is `multipart/form-data`.
- **Authentication.** Every `/api/v1` request carries an API key as a Bearer token (see [Authentication](#authentication)). `/api/v1` is **Bearer-only** — there is no cookie/JWT fallback.
- **Versioning.** The `v1` URL segment pins the contract. Breaking changes ship under a new version segment.
- **JSON conventions.** Keys are `camelCase`. Strings are UTF-8. Timestamps are ISO 8601 strings.

## Authentication

Requests authenticate with a personal API key in the `Authorization` header:

```http
Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>
```

The key string has the format `lesstruct_<keyID>_<secret>`:

- `lesstruct_` — a recognizable prefix (like GitHub's `ghp_`), so keys are easy to detect in logs and scanners.
- `keyID` — 12 hex characters (e.g. `a1b2c3d4e5f6`). This is the public, safely-displayable identifier.
- `secret` — 32 hex characters (≥128 bits). It is stored only as a salted hash and is **never** logged.

### Creating keys

API keys are created in the admin panel under **Profile → API Keys** (this is a browser/JWT action, not part of `/api/v1`). When you create a key:

1. You give it a human-readable **name** (e.g. "Claude Code").
2. The **full key string** is shown **exactly once**, with a copy button and a "you won't see this again" warning. Save it immediately.
3. Thereafter, the key is displayed only as its prefix (`lesstruct_a1b2c3d4e5f6••••`).

You can **revoke** a key at any time from the same view; revoked keys immediately stop authenticating.

### Logging hygiene

Only the `keyID` is ever logged — the secret and the full key string are redacted in all log output. Integrators should apply the same redaction in their own logs.

### Authorization

A key acts **as the user who created it**. It inherits that user's role and permissions, and every operation is scoped to that user's own resources (you can only list/read your own content and media, unless your role is Admin). Lesstruct's existing role-based access control governs every request unchanged.

## Conventions

### Response envelope

All responses use a uniform envelope with three optional top-level keys: `data`, `error`, and `meta`.

**Single resource** (create / get / update, and single media get):

```json
{
  "data": { "content": { "id": 7, "title": "Hello", "..." : "..." } }
}
```

**List** (content list, media list) — `data` is a **bare array**, not wrapped in an object:

```json
{
  "data": [ { "id": 7, "..." : "..." }, { "id": 6, "..." : "..." } ],
  "meta": { "pagination": { "nextCursor": "Ng", "hasMore": true } }
}
```

> **Watch the asymmetry.** A single content item is `{"data":{"content":{…}}}` (wrapped under `content`), but a list is `{"data":[…]}` (bare array). This is intentional and is the most common source of client bugs. Empty lists render as `"data":[]` (the key is always present).

**Error:**

```json
{
  "error": { "code": "VALIDATION_ERROR", "message": "title is required and must be between 1 and 200 characters" }
}
```

### Pagination

List endpoints use **cursor** (keyset) pagination, which is stable across inserts and deletes (unlike offset pagination).

| Parameter | Default | Range | Notes |
|---|---|---|---|
| `limit` | `50` | `1`–`100` | Missing/invalid/negative → `50`; over `100` → clamped to `100`. |
| `cursor` | _(omit)_ | opaque | Omit for the first page. Pass the `nextCursor` from the previous response. |

The cursor is an opaque, unpadded base64url token encoding the id of the last item on the current page. Do not construct or inspect it — treat it as opaque and echo it back. An invalid cursor returns `400 VALIDATION_ERROR "Invalid cursor"`.

The response includes `meta.pagination`:

- `nextCursor` — present **only** when `hasMore` is `true`. Pass it as the next request's `cursor`.
- `hasMore` — whether another page exists.

```bash
# First page
curl -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  "https://your-lesstruct.example/api/v1/content?limit=50"

# Next page (use the nextCursor from the previous response)
curl -H "Authorization: Bearer lesstruct_<...>" \
  "https://your-lesstruct.example/api/v1/content?limit=50&cursor=Ng"
```

Lists are **scoped to the caller's own resources** — an API key cannot enumerate another user's content or media (Admin-role keys excepted, per the role inheritance above).

### Visibility (no-enumeration model)

To avoid disclosing which resources exist, operations on a resource you don't own (and aren't an Admin for) return `404 NOT_FOUND` — **never** `403 FORBIDDEN`:

- **Drafts** are readable only by their owner.
- **Published** content is readable by any authenticated key.
- `GET`/`PUT`/`DELETE` on a resource you don't own → `404 NOT_FOUND` (existence is not disclosed).

## Content

The Content resource lets you publish posts, pages, and other content types over the API. Content is stored as canonical **Tiptap JSON**; you may submit Markdown and let the server convert it (see [Authoring in Markdown](#authoring-in-markdown)).

### Content object

```json
{
  "id": 7,
  "title": "Hello world",
  "slug": "hello-world",
  "body": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",...}]}",
  "status": "published",
  "postType": "post",
  "language": "en",
  "tags": ["intro", "demo"],
  "customFields": { "subtitle": "My first post" },
  "author": "Ari",
  "createdAt": "2026-06-15T10:00:00Z",
  "updatedAt": "2026-06-15T10:00:00Z"
}
```

| Field | Type | Notes |
|---|---|---|
| `id` | int | Stable identifier. |
| `title` | string | 1–200 chars. |
| `slug` | string | URL slug. **Immutable**: on create, any authenticated user may supply a custom slug (validated, unique per language); otherwise it is auto-generated from the title. The slug can never be changed via update — editing the title does not regenerate it (it is the public URL). |
| `body` | string | The canonical content — a Tiptap JSON document string. |
| `status` | string | `"draft"` or `"published"`. |
| `postType` | string | Content type (e.g. `post`, `page`), from your configured post types. |
| `language` | string | Language code. Settable on create/update; must be in the server's configured `languages` list, else 400 `ErrInvalidLanguage`. |
| `tags` | string[] | Tags. Settable on create/update; normalized server-side (lowercased, trimmed, deduped, length-bounded) via `ValidateTags`. |
| `customFields` | object | TOML-defined, server-validated custom-field values. |
| `author` | string | Display name of the author. Read-only — derived from the API key's user. |
| `createdAt` / `updatedAt` | string | ISO 8601 timestamps. |

### Create content

```http
POST /api/v1/content
```

**Request body:**

```json
{
  "title": "Hello world",
  "body": "# Hello\n\nThis is my first post.",
  "format": "markdown",
  "postType": "post",
  "customFields": { "subtitle": "My first post" },
  "isPublished": true
}
```

| Field | Required | Notes |
|---|---|---|
| `title` | yes | 1–200 chars. |
| `body` | yes | The content. With `format: markdown` it is Markdown (converted server-side to Tiptap); with `format: tiptap` (the default) it must be a valid Tiptap JSON document string; with `format: html` it is raw HTML stored as-is (sanitized on write). |
| `format` | no | `"markdown"`, `"tiptap"`, or `"html"`. Defaults to `"tiptap"`. Matched case-insensitively after trimming leading/trailing whitespace. `"html"` stores raw HTML directly — no TipTap conversion. |
| `postType` | no | Content type. |
| `tags` | no | Array of tag strings. Server normalizes (trim, lowercase, dedupe, length-bound) via `ValidateTags`; an invalid tag returns 400 `VALIDATION_ERROR`. |
| `language` | no | Language code (e.g. `"en"`, `"id"`). Must be in the server's configured languages list (`config.toml` `[languages]`); an unknown code returns 400 `VALIDATION_ERROR` (`ErrInvalidLanguage`). |
| `slug` | no | A custom slug (lowercase letters, digits, and hyphens; 1–200 chars; unique per language, else 400 `ErrSlugAlreadyExists`). Omit to auto-generate from the title. The slug is **immutable after creation** — see [Update content](#update-content). |
| `customFields` | no | Custom-field values, validated through the same path the admin uses. Admin-managed **system fields** (declared per post type) are rejected here with `400 VALIDATION_ERROR` — set them via [Set system fields](#set-system-fields). |
| `translationGroupId` | no | ID of an existing content item whose translation group this item joins. The server validates the ID exists; a miss returns 400 `ErrTranslationGroupNotFound`. |
| `isPublished` | no | `true` → `"published"`; `false`/omitted → `"draft"`. |

**Response** `200 OK`:

```json
{ "data": { "content": { "id": 7, "title": "Hello world", "slug": "hello-world", "..." : "..." } } }
```

> Create returns `200 OK` (not `201 Created`) by design — consistent with the other `/api/v1` success responses.

> Creating directly with `isPublished: true` runs the full publish pipeline: SEO metadata is auto-generated (when the SEO service is configured) and the `AfterPublish` plugin hook fires — equivalent to create + [`/publish`](#publish-content). Creating as a draft (the default) only fires `AfterCreate`.

Errors: `400 VALIDATION_ERROR` (bad/missing fields, invalid Tiptap, custom-field validation, or Markdown that converts to Tiptap the server rejects — see [Authoring in Markdown](#authoring-in-markdown)).

### Get content

```http
GET /api/v1/content/{id}
```

**Response** `200 OK`: `{"data":{"content":{…}}}`.

Returns `404 NOT_FOUND` if the content does not exist **or** you are not allowed to read it (a draft owned by someone else). Published content is readable by any authenticated key.

### List content

```http
GET /api/v1/content?limit=50&cursor=<cursor>&tag=foo&tag=bar&language=en&status=draft&post_type=post&author=alice&search=golang
```

Returns the caller's own content (drafts and published), newest-first, using [cursor pagination](#pagination). All filters AND together with the cursor; pass multiple `tag` values to AND-of-tags (the post must carry every tag).

Query parameters:

| Param        | Type                | Notes |
|--------------|---------------------|-------|
| `limit`      | int                 | Default 50, max 100. |
| `cursor`     | string              | Opaque token from a previous list call. |
| `tag`        | string (repeatable) | AND-of-tags — the post must carry every tag. |
| `language`   | string              | Filter by language code. |
| `status`     | `draft` \| `published` | Unknown values return `400 VALIDATION_ERROR`. |
| `post_type`  | string              | Filter by post type. |
| `author`     | string              | **Admin only.** Non-admins receive `403 FORBIDDEN`. |
| `search`     | string              | Title / meta-description substring (case-insensitive). Min length 2; shorter values are dropped. |

**Response** `200 OK`:

```json
{
  "data": [ { "id": 7, "..." : "..." }, { "id": 6, "..." : "..." } ],
  "meta": { "pagination": { "nextCursor": "Ng", "hasMore": true } }
}
```

### Update content

```http
PUT /api/v1/content/{id}
```

Accepts `title`, `body`, `format`, `postType`, `customFields`, `isPublished`, `tags`, and `language`. SEO metadata (`metaDescription`, `ogTitle`, `ogDescription`), `allowComments`, and `translationGroupId` are **preserved from the existing item** and cannot be changed via this endpoint — any values you send for them are ignored. The `slug` is **immutable** and never changes on update (editing the title does not regenerate it — it is the public URL); any `slug` you send here is ignored. `format: markdown` converts the body to Tiptap before storing. `format: html` stores raw HTML directly.

**Response** `200 OK`: `{"data":{"content":{…}}}` with the updated item.

Returns `404 NOT_FOUND` if the item does not exist or you are not its owner (and not Admin) — existence is not disclosed (see [Visibility](#visibility-no-enumeration-model)). Errors: `400 VALIDATION_ERROR`.

### Delete content

```http
DELETE /api/v1/content/{id}
```

**Response** `204 No Content` (empty body) on success. A subsequent `GET` returns `404 NOT_FOUND`.

Returns `404 NOT_FOUND` if the item does not exist or you are not its owner (and not Admin).

### Publish content

```http
POST /api/v1/content/{id}/publish
```

Standalone status-toggle verb. No request body. On the **draft → published** transition the server auto-generates SEO metadata (when the SEO service is configured) and fires the `AfterPublish` plugin hook. Publishing an already-published post is a **200 no-op**: the row is persisted unchanged, no hook fires, no SEO is regenerated.

**Response** `200 OK`: `{"data":{"content":{…}}}` with the item now in `status: "published"`.

Returns `404 NOT_FOUND` if the item does not exist or you are not its owner (and not Admin) — existence is not disclosed. Errors: `400 VALIDATION_ERROR` (bad id).

```bash
curl -X POST -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  "https://your-lesstruct.example/api/v1/content/7/publish"
```

### Unpublish content

```http
POST /api/v1/content/{id}/unpublish
```

Standalone status-toggle verb. No request body. Sets `status: "draft"`. Never fires the `AfterPublish` hook (the hook is wired to the draft → published edge only). Unpublishing an already-draft post is a **200 no-op**.

**Response** `200 OK`: `{"data":{"content":{…}}}` with the item now in `status: "draft"`.

Returns `404 NOT_FOUND` if the item does not exist or you are not its owner (and not Admin). Errors: `400 VALIDATION_ERROR` (bad id).

```bash
curl -X POST -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  "https://your-lesstruct.example/api/v1/content/7/unpublish"
```

### Set system fields

```http
PUT /api/v1/content/{id}/system-fields
```

Sets the admin-managed **system fields** (e.g. `editorial_status`, `internal_notes` — declared per post type in `config.toml`) on a content item. **Admin only:** a non-Admin API key receives `403 FORBIDDEN`. This is the agent/Bearer-realm mirror of the admin panel's system-fields editor, so the CLI (`lesstruct-cli content system-fields <id> --field key=value …`) can set them with an Admin API key.

**Request body:**

```json
{ "systemFields": { "editorial_status": "published" } }
```

The server validates every key against the item's post-type system-field schema and every value's type — an unknown key returns `400 VALIDATION_ERROR` (`ErrUnknownSystemFieldKey`) and a value that fails the field schema returns `400 VALIDATION_ERROR` (`ErrSystemFieldValidation`).

**Response** `200 OK`: `{"data":{"content":{…}}}` with the updated item.

Returns `403 FORBIDDEN` if the key does not belong to an Admin. Returns `404 NOT_FOUND` if the item does not exist. Errors: `400 VALIDATION_ERROR`.

> System fields are **not** accepted inside `customFields` on [create](#create-content) or [update](#update-content) — they are rejected with a `400 VALIDATION_ERROR` naming the offending key. Use this endpoint instead.

```bash
curl -X PUT -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  -H "Content-Type: application/json" \
  -d '{"systemFields":{"editorial_status":"published"}}' \
  "https://your-lesstruct.example/api/v1/content/7/system-fields"
```

## Media

Upload, retrieve, and list media (images). Media is deduplicated by content hash and stored with generated variants (e.g. WebP + thumbnails).

### Media object

```json
{
  "id": 12,
  "filename": "a1b2c3d4.webp",
  "originalFilename": "photo.jpg",
  "mimeType": "image/jpeg",
  "fileSize": 204800,
  "width": 1200,
  "height": 800,
  "altText": "A scenic mountain view",
  "isWebp": true,
  "hash": "a1b2c3d4e5f6...",
  "url": "https://your-lesstruct.example/uploads/media/a1b2c3d4.webp",
  "variants": {
    "thumbnail": { "url": "https://your-lesstruct.example/uploads/media/a1b2c3d4-200.webp", "width": 200 }
  },
  "createdAt": "2026-06-15T10:00:00Z",
  "updatedAt": "2026-06-15T10:00:00Z"
}
```

| Field | Type | Notes |
|---|---|---|
| `id` | int | Stable identifier. |
| `filename` / `originalFilename` | string | Stored / uploaded filename. |
| `mimeType` | string | Source MIME type (JPG/PNG/GIF/WebP). |
| `fileSize` | int | Bytes. |
| `width` / `height` | int | Pixel dimensions. |
| `altText` | string | Accessibility text. |
| `isWebp` | bool | Whether the primary stored file is WebP. (Note the key `isWebp`, not `isWebP`.) |
| `hash` | string | Content hash used for dedup. |
| `url` | string | The absolute URL to reference this media in content (e.g. `https://your-lesstruct.example/uploads/media/<file>`). |
| `variants` | object | Map of variant name → `{ "url", "width" }` (e.g. `thumbnail`). |
| `createdAt` / `updatedAt` | string | ISO 8601 timestamps. |

### Upload media

```http
POST /api/v1/media
Content-Type: multipart/form-data
```

The body is `multipart/form-data` with:

- **`file`** (required) — the image part (JPG, PNG, GIF, or WebP). A missing `file` part returns `400 VALIDATION_ERROR "file part is required"`.
- **`metadata`** — a JSON part: `{"altText":"A scenic mountain view"}`. A non-empty **`altText` is required** for accessibility. The part is optional only in the multipart sense: omitting it (or sending empty `altText`) causes the service to reject the upload with `400 VALIDATION_ERROR`. **Always send it.**

```bash
curl -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  -F "file=@photo.jpg" \
  -F 'metadata={"altText":"A scenic mountain view"}' \
  "https://your-lesstruct.example/api/v1/media"
```

**Response** `200 OK`: `{"data":{"media":{…}}}`.

| Status | Code | When |
|---|---|---|
| `200` | — | Uploaded; a new media item was created and stored. |
| `400` | `VALIDATION_ERROR` | Missing `file` part; unsupported/oversized file; empty `altText`. |
| `409` | `CONFLICT` | A file with the same content hash already exists. The upload is not stored; upload a different file or use the existing media's `url`. |

> **Duplicate handling.** The API returns `409 CONFLICT` for a duplicate upload (rather than returning the existing item, as the admin panel does). This keeps the contract honest: the upload was not stored.

### Get media

```http
GET /api/v1/media/{id}
```

**Response** `200 OK`: `{"data":{"media":{…}}}`.

Returns `404 NOT_FOUND` if the media does not exist or you are not its owner (and not Admin) — existence is not disclosed.

### List media

```http
GET /api/v1/media?limit=50&cursor=<cursor>
```

Returns the caller's own media, newest-first, using [cursor pagination](#pagination). Same envelope as the [content list](#list-content): a bare `data` array plus `meta.pagination`.

> **Shared path note.** `GET /api/v1/media` and `GET /api/v1/media/{id}` are shared with the browser admin panel; the server dispatches to the agent handler when the request presents a `lesstruct_`-prefixed Bearer token, and to the browser handler otherwise. For agent clients this is transparent — always send the Bearer key. `POST /api/v1/media` (upload) is agent/Bearer-only. The browser admin handler speaks the **same contract**: `limit` (default `50`, clamped to `100`) plus `cursor`, returning the same bare-array list with `meta.pagination` — the admin media library pages through it with infinite scroll.

## Comments

Create, list, delete, and moderate comments on a content item. The agent comment surface is **nested under the content namespace** (`/api/v1/content/{id}/comments`) so it is collision-free with the browser admin's `/api/v1/content_items/.../comments` and `/api/v1/comments` routes, and consistent with the rest of the agent surface (which keys everything by content id). New comments always start in the `pending` moderation status.

> **Browser-admin moderation queue.** The admin panel additionally exposes `GET /api/v1/comments/pending` (JWT + CSRF, **Admin only** — not part of the Bearer `/api/v1` agent surface). It returns every comment currently in the `pending` status across all content, each enriched with `contentId`, `contentTitle`, and `contentSlug` so the global moderation queue can link back to the originating post. The same response shape applies to the per-content admin route `GET /api/v1/content_items/{id}/comments`.

> **Rendering comment text.** Comment text is validated on input (1–2000 chars, no HTML) and the built-in Go `html/template` theme auto-escapes it on output. It is, however, returned **verbatim** in the JSON API's `comment` field. Any consumer that renders it — a custom theme, or an agent frontend (React/Vue/Angular) — must HTML-escape it on output: never bind it with `v-html`, `[innerHTML]`, or `dangerouslySetInnerHTML`. The server's input validation is a first layer, not a guarantee that a downstream renderer is safe.

### Comment object

```json
{
  "id": 9,
  "comment": "Great post!",
  "author": "Alice",
  "username": "alice",
  "role": "admin",
  "status": "pending",
  "createdAt": "2026-06-23T12:00:00Z"
}
```

| Field | Type | Notes |
|---|---|---|
| `id` | int | Stable identifier. |
| `comment` | string | The comment text (1–2000 chars, no HTML). |
| `author` / `username` / `role` | string | Author display name / handle / role. Omitted when not applicable. |
| `status` | string | Moderation status: `pending`, `approved`, `rejected`, `spam`. |
| `createdAt` | string | ISO 8601 timestamp. |

### Create comment

```http
POST /api/v1/content/{id}/comments
Content-Type: application/json
```

```json
{ "comment": "Great post!" }
```

Creates a comment on content `{id}` attributed to the API-key-owning user, in the `pending` status. The content must be visible to the caller (published, owned, or Admin) and have comments enabled.

```bash
curl -X POST -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  -H "Content-Type: application/json" \
  -d '{"comment":"Great post!"}' \
  "https://your-lesstruct.example/api/v1/content/5/comments"
```

**Response** `200 OK`: `{"data":{"comment":{…}}}`.

| Status | Code | When |
|---|---|---|
| `200` | — | Created; the new comment is returned. |
| `400` | `VALIDATION_ERROR` | Empty/oversized comment text (1–2000 chars), HTML in the text, invalid content id, malformed body. |
| `403` | `FORBIDDEN` | Comments are disabled on this content (`allowComments=false`). |
| `404` | `NOT_FOUND` | Content does not exist, or is a draft the caller may not see (existence not disclosed). |

### List comments

```http
GET /api/v1/content/{id}/comments
```

Returns the comments on content `{id}`, scoped to content the caller may see (published, owned, or Admin). **What is returned depends on the key's role:** an Admin key gets the full moderation queue (every status, including `pending`/`rejected`/`spam`), while a non-Admin key gets only `approved` comments — mirroring the public `GET /api/v1/public/content_items/{slug}/comments`, so the pre-moderation queue is never exposed to a Commentator-level key. Comments-disabled content (`allowComments=false`) returns an empty list for non-Admin keys; an Admin key still sees the queue so it can moderate. Envelope is a bare `data` array (always present, even when empty):

```json
{ "data": [ { "id": 1, "comment": "ok", "status": "approved", "createdAt": "…" }, { "id": 2, "comment": "waiting", "status": "pending", "createdAt": "…" } ] }
```

> The `pending`/`rejected`/`spam` items in the example above are only visible to an Admin key.

| Status | Code | When |
|---|---|---|
| `200` | — | The comment list (possibly empty). |
| `400` | `VALIDATION_ERROR` | Invalid content id. |
| `404` | `NOT_FOUND` | Content does not exist or is not visible to the caller. |

### Delete comment

```http
DELETE /api/v1/content/{id}/comments/{commentId}
```

Deletes the comment. The path `{id}` must be the comment's actual content — a mismatch (the comment belongs to different content) returns `404` with no disclosure. An Admin key may delete any comment; any other key only its own, and a missing or someone else's comment also returns `404` (no disclosure).

**Response** `204 No Content` (empty body).

| Status | Code | When |
|---|---|---|
| `204` | — | Deleted. |
| `400` | `VALIDATION_ERROR` | Invalid content or comment id. |
| `404` | `NOT_FOUND` | Comment does not exist, is bound to different content, or is not yours (and you are not Admin). |

### Moderate comment (admin only)

```http
PUT /api/v1/content/{id}/comments/{commentId}/status
Content-Type: application/json
```

```json
{ "status": "approved" }
```

Sets a comment's moderation status. The path `{id}` must be the comment's actual content — a mismatch returns `404` with no disclosure. `status` must be a valid value (`pending`, `approved`, `rejected`, `spam`). **Admin only** — a non-admin key gets `403 FORBIDDEN`. The updated comment is returned.

```bash
curl -X PUT -H "Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>" \
  -H "Content-Type: application/json" \
  -d '{"status":"approved"}' \
  "https://your-lesstruct.example/api/v1/content/5/comments/9/status"
```

**Response** `200 OK`: `{"data":{"comment":{…}}}`.

| Status | Code | When |
|---|---|---|
| `200` | — | Updated; the comment is returned with its new status. |
| `400` | `VALIDATION_ERROR` | Unknown `status`, invalid id, malformed body. |
| `403` | `FORBIDDEN` | Caller is not an Admin. |
| `404` | `NOT_FOUND` | Comment does not exist or is bound to different content. |

## Errors

Errors use the envelope's `error` object: `{"error":{"code":"…","message":"…"}}`. (A `details` field is reserved on the object but is not currently populated by the `/api/v1` handlers.)

### Error catalog

| HTTP | Code | Meaning | Emitted by |
|---|---|---|---|
| `401` | `UNAUTHORIZED` | No / undecodable identity. | handler / middleware |
| `401` | `INVALID_API_KEY` | The key is malformed or unknown. | auth middleware |
| `401` | `REVOKED_KEY` | The key has been revoked. | auth middleware |
| `401` | `EXPIRED_KEY` | The key has expired. | auth middleware |
| `400` | `VALIDATION_ERROR` | Bad request body, invalid Tiptap, custom-field validation, invalid cursor, invalid id, missing `file` part, bad alt text, etc. | handler |
| `404` | `NOT_FOUND` | Resource does not exist, or you don't own it (and aren't Admin) — existence is not disclosed. | handler |
| `403` | `FORBIDDEN` | Reserved for service-layer rejections. Not the response for resources you don't own — those return `404` (no-enumeration). Rarely emitted on the agent surface. | handler |
| `409` | `CONFLICT` | Duplicate media upload. | media handler |
| `429` | `RATE_LIMITED` | You have exceeded the per-key rate limit. | rate-limit middleware |
| `500` | `INTERNAL_ERROR` | Unexpected server error. | handler |

### No-enumeration

Resource existence is never disclosed: a request for a resource you don't own (and aren't Admin for) returns `404 NOT_FOUND`, not `403 FORBIDDEN`. Treat `404` on `GET`/`PUT`/`DELETE` as "not found **or** not yours".

## Authoring in Markdown

Set `format: "markdown"` on create/update to author content in Markdown. The server parses it with [goldmark](https://github.com/yuin/goldmark) (core CommonMark) and converts it to canonical **Tiptap JSON**, which is what is stored. **Raw Markdown is never persisted.**

```bash
curl -X POST -H "Authorization: Bearer lesstruct_<...>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello","body":"# Hello\n\n**Bold** and *italic*.","format":"markdown","isPublished":true}' \
  "https://your-lesstruct.example/api/v1/content"
```

### Supported Markdown

| Markdown | Result |
|---|---|
| `# H1` … `###### H6` | Headings (levels 1–6). |
| Plain text | Paragraphs. |
| `**bold**`, `__bold__` | Bold. |
| `*italic*`, `_italic_` | Italic. |
| `` `code` `` | Inline code. |
| ` ```lang … ``` ` (fenced) | Code block, with `language` from the info string. |
| Indented code | Code block (no language). |
| `---`, `***`, `___` | Horizontal rule. |
| `> quote` | Blockquote (nestable). |
| `- a` / `* a` / `+ a` | Bullet list. |
| `1. a` | Ordered list. |
| `![alt](url "title")` | Image (`src`/`alt`/`title`). |
| `[text](url "title")` | Link (`href`/`title`). |
| `<https://example.com>` | Autolink (→ link). |
| Hard line break (`··\n` or `text\`) | Hard break. |

### Sanitized / not enabled

- **Raw HTML is sanitized.** Inline and block raw HTML is reduced to **safe plain text** (tags are stripped, visible text is kept) via [bluemonday](https://github.com/microcosm-cc/bluemonday). Raw HTML markup is never stored. Converting rich HTML formatting to Tiptap marks is out of scope — only the visible text survives.
- **Tables, task lists, and strikethrough** are not enabled (core CommonMark only). They render as plain text/paragraphs.

### URL safety

The converted document must pass Lesstruct's Tiptap validator, which restricts URL schemes:

- **Link `href`** must be `http`, `https`, `mailto`, or empty.
- **Image `src`** must be `http`, `https`, or empty.

A link or image with another scheme (`javascript:`, `data:`, `file:`, …) causes the converted document to fail validation and the request returns `400 VALIDATION_ERROR`. This is intentional and applies site-wide (including admin-authored content). Use an `http(s)` URL or upload the media first (see [Images](#images)).

> Markdown is an **ingest format only**. It is always converted to Tiptap JSON before storage; you cannot retrieve the original Markdown. Round-trip (Tiptap → Markdown) is out of scope.

## Images

- **External images** — `![alt](https://cdn.example.com/img.png)` passes through unchanged; the `src` is stored as-is (subject to the `http(s)` scheme rule above).
- **Local media** — to embed an image you upload, first upload it via `POST /api/v1/media`, then reference the returned `url` in your Markdown:

  ```bash
  # 1. Upload
  curl -H "Authorization: Bearer lesstruct_<...>" \
    -F "file=@photo.jpg" -F 'metadata={"altText":"..."}' \
    "https://your-lesstruct.example/api/v1/media"
  # → { "data": { "media": { "url": "https://your-lesstruct.example/uploads/media/a1b2c3d4.webp", ... } } }

  # 2. Reference the returned url
  ![A scenic view](https://your-lesstruct.example/uploads/media/a1b2c3d4.webp)
  ```

## Rate limiting

`/api/v1` is rate-limited **per API key** (not per IP) for attribution and fairness, using the same token-bucket limiter as the rest of the API. When you exceed the limit you receive `429 RATE_LIMITED`. Browser/admin routes are rate-limited per IP.

If you hit the limit, wait and retry with backoff. The limit is shared across all requests made with a given key.

## AI text generation

> Requires `AI_TEXT_GENERATION_API_KEY` (see [Configuration](../configuration.md)).

### Enhance / Generate

```
POST /api/v1/text/enhance
```

Enhance existing rich-text content or generate HTML/CSS from a natural-language prompt.

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | For `tiptap`: TipTap JSON to enhance. For `html`: natural-language prompt describing what to generate. |
| `format` | string | no | `"tiptap"` (default) or `"html"`. |
| `existingHtml` | string | no | Existing HTML to refine (HTML format only). Sent alongside the prompt for iterative refinement. |

**Response:** `200 OK` with `{ "data": { "content": "..." } }`.

- `format=tiptap` → returns enhanced TipTap JSON.
- `format=html` → returns an HTML fragment with `<style>` block first. The AI surfaces the user's media library images as context.

### Translate

```
POST /api/v1/text/translate
```

Translate content between languages.

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | Content to translate (TipTap JSON or HTML). |
| `sourceLang` | string | yes | Source language code (e.g. `"en"`). |
| `targetLang` | string | yes | Target language code (e.g. `"fr"`). |
| `format` | string | no | `"tiptap"` (default) or `"html"`. |

**Response:** `200 OK` with `{ "data": { "content": "..." } }`.

- `format=tiptap` → returns translated TipTap JSON preserving structure.
- `format=html` → returns translated HTML preserving tags, styles, and URLs — only visible text and `alt` attributes are translated.

## OpenAPI snippet

A machine-readable OpenAPI fragment for the Content create endpoint. A full OpenAPI specification is deferred to post-MVP; this snippet is suitable for agent tooling consumption.

```yaml
openapi: 3.0.3
info:
  title: Lesstruct API
  version: "1.0"
  description: >
    Versioned, API-key-authenticated Content and Media API.
    Auth: HTTP Bearer scheme with a `lesstruct_`-prefixed API key.
servers:
  - url: https://your-lesstruct.example
components:
  securitySchemes:
    apiKey:
      type: http
      scheme: bearer
      description: "Authorization: Bearer lesstruct_<keyID>_<secret>"
  schemas:
    Content:
      type: object
      description: "A content item (see Content object above for the full field set)."
      properties:
        id: { type: integer }
        title: { type: string }
        slug: { type: string }
        body: { type: string, description: "Canonical Tiptap JSON document string." }
        status: { type: string, enum: [draft, published] }
  responses:
    Error:
      description: Error
      content:
        application/json:
          schema:
            type: object
            properties:
              error:
                type: object
                properties:
                  code: { type: string }
                  message: { type: string }
security:
  - apiKey: []
paths:
  /api/v1/content:
    post:
      summary: Create content
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [title, body]
              properties:
                title: { type: string, minLength: 1, maxLength: 200 }
                body: { type: string, description: "Tiptap JSON (format=tiptap), Markdown (format=markdown), or raw HTML (format=html)" }
                format: { type: string, enum: [markdown, tiptap, html], default: tiptap, description: "Server matches case-insensitively after trimming whitespace. 'html' stores raw HTML directly." }
                postType: { type: string }
                slug: { type: string, description: "Create-only. A custom slug (validated, unique per language); omit to auto-generate from the title. Immutable after creation." }
                customFields: { type: object, additionalProperties: true }
                isPublished: { type: boolean, default: false }
      responses:
        "200":
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: object
                    properties:
                      content: { $ref: "#/components/schemas/Content" }
        "400": { $ref: "#/components/responses/Error" }
        "401": { $ref: "#/components/responses/Error" }
        "429": { $ref: "#/components/responses/Error" }
```

The same pattern extends to the remaining `/api/v1/content[/{id}]`, `/api/v1/media`, and `/api/v1/content/{id}/comments[/{commentId}[/status]]` operations described above. A complete OpenAPI document will be generated in a follow-up.

## Public SEO endpoints (no auth)

These are unauthenticated, served at the site root (not under `/api/v1`) so crawlers find them at their canonical paths:

```http
GET /sitemap.xml
```

Returns the [sitemaps.org](https://www.sitemaps.org/protocol.html) XML `<urlset>` of every published content item with a public page — `post`, `page`, and any custom post type (e.g. `tutorial`, `showcase`). Each `<loc>` is the item's root URL (`/<slug>`, where the public site serves it). The homepage is the first entry. `Content-Type: application/xml`. (A JSON shape is also available at `GET /api/v1/sitemap` for programmatic callers.)

Translated pages declare their language variants: each `<url>` in a translation group carries `<xhtml:link rel="alternate" hreflang="…" href="…"/>` entries for every published translation (including itself), so search engines can serve the right locale. Pages with no published translations emit no `hreflang`.

```http
GET /robots.txt
```

Returns a permissive `robots.txt` that allows all crawlers, disallows `/admin`, and points at the sitemap: `Sitemap: <site URL>/sitemap.xml`.

## Public content endpoints (no auth)

Lesstruct also exposes a family of **unauthenticated** JSON endpoints under `/api/v1/public/*` for content delivery, search, post-type discovery, and published-author listings. These exist *outside* the Bearer API documented above — no API key, no `Authorization` header, rate-limited by the public per-IP bucket (`RATE_LIMIT_PUBLIC_PER_MINUTE`, default 60).

> **Envelope divergence from the Bearer API.** The public endpoints use a slightly different response shape than the Bearer `/api/v1` surface above. Where the Bearer API emits `meta.pagination` (cursor) and `UPPER_SNAKE` error codes, the public endpoints emit `meta: {"timestamp": "…"}` only (no pagination metadata — use `limit`/`offset`) and `lower_snake` error codes. Mirror this exactly when writing a client.

```http
GET /api/v1/public/authors
```

Returns the users who have published at least one content item, with **only safe, public fields** — never email, role, or status. Custom and system fields that are allowlisted with the `"expose"` operation in the `[[public_field]]` config are included in the `publicFields` map (see [Public custom-field query](#public-custom-field-query) below). With no `cf_*` or `sort_by` parameter, results are ordered by published-content count (desc) then username (asc), so the first entries are the most active contributors — useful for author directories and "most active" widgets. When `sort_by=cf:<field>` is supplied, the order is driven by that custom-field's numeric value instead (see [Public custom-field query](#public-custom-field-query) below).

**Query parameters:**

| Parameter | Default | Range | Notes |
|---|---|---|---|
| `limit` | `100` | `1`–`100` | Missing/invalid/negative → `100`; over `100` → clamped to `100`. |
| `offset` | `0` | `≥ 0` | Standard offset pagination (use `limit` + `offset` to page through). |
| `cf_<field>` | *(unset)* | string | Exact-match filter on a user custom field (e.g. `cf_tier=gold`). Allowlisted via `[[public_field resource="user"]]`. |
| `cf_<field>_min` | *(unset)* | numeric | Inclusive lower-bound filter on a numeric user custom field (e.g. `cf_points_min=10`). |
| `cf_<field>_max` | *(unset)* | numeric | Inclusive upper-bound filter on a numeric user custom field. |
| `sort_by` | *(unset)* | `cf:<field>` | Sort by a user custom field. The `cf:` prefix is required; bare field names are rejected with `400 invalid_sort`. |
| `order` | `desc` | `asc` \| `desc` | Sort direction. Defaults to `desc` when omitted — the natural choice for "top N" rankings. |

**Response** (`200`):

```json
{
  "data": [
    {
      "username": "johndoe",
      "displayName": "John Doe",
      "avatarURL": "http://your-lesstruct.example/uploads/profile_pictures/abc.jpg",
      "profileURL": "http://your-lesstruct.example/authors/johndoe",
      "contentCount": 42,
      "postTypes": ["article", "event"],
      "publicFields": {
        "tier_point": 500,
        "current_point": 3200,
        "stars": 4
      }
    }
  ],
  "error": null,
  "meta": { "timestamp": "2026-07-12T09:30:00Z" }
}
```

| Field | Description |
|---|---|
| `username` | Author username. |
| `displayName` | `users.name`, falling back to `username` when name is unset. |
| `avatarURL` | Absolute profile-picture URL; empty string when the author has no picture. |
| `profileURL` | Absolute URL of the server-rendered author page (`<baseURL>/authors/<username>`), which renders profile custom and exposed system fields server-side. |
| `contentCount` | Number of published content items by the author. |
| `postTypes` | Distinct post types the author publishes under. Always a non-nil array (renders `[]` when single-type). |
| `publicFields` | Map of custom/system field slugs and their raw values. Only fields that are allowlisted with the `"expose"` operation in `[[public_field]]` are included. The key is omitted entirely when no field has been opted in (backward-compatible default). |

An empty result returns `"data": []`. On a server failure the endpoint returns `500` with `{"error":{"code":"internal_error","message":"Failed to list published authors"}}`.

**Related public endpoints** (same envelope, same rate-limit bucket): `GET /api/v1/public/content_items`, `GET /api/v1/public/content_items/{slug}`, `GET /api/v1/public/authors/{username}`, `GET /api/v1/public/authors/{username}/content_items`, `GET /api/v1/public/content_items/{slug}/comments`, `GET /api/v1/public/post_types`, `GET /api/v1/public/search`, `GET /api/v1/public/archive`.

---

```http
GET /api/v1/public/authors/{username}
```

Returns a single published author's public profile, using the same response shape as the list endpoint above. Returns `404` when the author has no published content or the username does not exist.

**Path parameter:**

| Parameter | Description |
|---|---|
| `username` | The username of the author to fetch. |

**Response** (`200`): Same as `GET /api/v1/public/authors` but a single object in `data` instead of an array. The same `publicFields` map is populated when `"expose"`-allowlisted fields are configured.

```json
{
  "data": {
    "username": "johndoe",
    "displayName": "John Doe",
    "avatarURL": "http://your-lesstruct.example/uploads/profile_pictures/abc.jpg",
    "profileURL": "http://your-lesstruct.example/authors/johndoe",
    "contentCount": 42,
    "postTypes": ["article", "event"],
    "publicFields": {
      "tier_point": 500,
      "stars": 4
    }
  },
  "error": null,
  "meta": { "timestamp": "2026-07-12T09:30:00Z" }
}
```

**Error codes:** `404 author_not_found`, `400 invalid_username`, `500 internal_error`.

---

```http
GET /api/v1/public/content_items
```

Returns published content items. With no `cf_*` or `sort_by` parameter, results are newest first. When `sort_by=cf:<field>` is supplied, results are ordered by that custom-field's numeric value instead (see [Public custom-field query](#public-custom-field-query) below). Each item includes all content fields plus a `featuredImage` URL when the item has an image in its body. Useful for sidebar widgets, "latest posts" lists, and external app rendering.

**Query parameters:**

| Parameter | Default | Notes |
|---|---|---|
| `limit` | 100 | Max items returned (clamped to 1–1000). |
| `offset` | 0 | Pagination offset. |
| `post_type` | *(all types)* | Restrict to a single post type (e.g. `article`). |
| `cf_<field>` | *(unset)* | Exact-match filter on a content custom field (e.g. `cf_category=News`). Allowlisted via `[[public_field resource="content"]]`; entries can be post-type-scoped. |
| `cf_<field>_min` | *(unset)* | Inclusive lower-bound filter on a numeric content custom field (e.g. `cf_price_min=5`). |
| `cf_<field>_max` | *(unset)* | Inclusive upper-bound filter on a numeric content custom field. |
| `sort_by` | *(unset)* | `cf:<field>` to sort by a content custom field. The `cf:` prefix is required. |
| `order` | `desc` | `asc` \| `desc`. Sort direction. |

**Response** (`200`):

```json
{
  "data": [
    {
      "id": 1,
      "title": "Article Title",
      "slug": "article-title",
      "postType": "article",
      "featuredImage": "http://your-lesstruct.example/uploads/abc123_thumb.webp",
      "createdAt": "2026-07-15T10:00:00Z"
    }
  ],
  "error": null,
  "meta": { "timestamp": "2026-07-15T10:00:00Z" }
}
```

| Field | Description |
|---|---|
| `featuredImage` | Absolute thumbnail URL resolved from the item's media record, or omitted when the content body contains no image. |

An empty result returns `"data": []`. On a server failure the endpoint returns `500` with `{"error":{"code":"internal_error","message":"An internal error occurred"}}`.

---

```http
GET /api/v1/public/archive
```

Returns published-content counts grouped by year and month, newest first — for building archive widgets (e.g. a sidebar "Arsip" list with month names and post counts). Each entry includes a `url` pointing to the matching listing page with `?year=` and `?month=` params so the user can browse that month's posts.

**Query parameters:**

| Parameter | Default | Notes |
|---|---|---|
| `post_type` | *(all types)* | Restrict to a single post type (e.g. `article`). When omitted, counts span every post type. |
| `language` | *(all languages)* | Restrict to a single language code. |

**Response** (`200`):

```json
{
  "data": [
    { "year": 2026, "month": 7, "count": 12, "url": "http://your-lesstruct.example/article?year=2026&month=7" },
    { "year": 2026, "month": 6, "count": 8, "url": "http://your-lesstruct.example/article?year=2026&month=6" }
  ],
  "error": null,
  "meta": { "timestamp": "2026-07-14T10:00:00Z" }
}
```

| Field | Description |
|---|---|
| `year` | 4-digit year. |
| `month` | 1–12 month number. |
| `count` | Number of published items in that month (and post type / language if filtered). |
| `url` | Absolute URL of the listing page for that month. When `post_type` is set, points to `/<post_type>?year=…&month=…`; otherwise points to `/?year=…&month=…`. |

An empty result returns `"data": []`. On a server failure the endpoint returns `500` with `{"error":{"code":"internal_error","message":"Failed to list published archive"}}`.

---

## Public custom-field query

`GET /api/v1/public/content_items` and `GET /api/v1/public/authors` accept four custom-field parameter shapes that let theme authors build dynamic regions client-side:

| Parameter shape | Operation | Example |
|---|---|---|
| `cf_<field>=<value>` | equality filter (works best on string fields) | `cf_category=News` |
| `cf_<field>_min=<n>` | inclusive numeric lower bound | `cf_points_min=10` |
| `cf_<field>_max=<n>` | inclusive numeric upper bound | `cf_price_max=20` |
| `sort_by=cf:<field>&order=<asc|desc>` | numeric sort (non-numeric values sort as 0) | `sort_by=cf:points&order=desc` |

Multiple filters AND together. `sort_by` and the cf filters are independent — both can be supplied, and either can be omitted.

> **Admin-managed system fields** (declared under `[[post_type.system_fields]]` or
> `[user_fields].system_fields` in `config.toml`) are also queryable via `cf_*` and
> `sort_by=cf:*`, since they are stored in the same `custom_fields` JSON column as
> regular custom fields. A field like `total_point` declared under
> `[user_fields].system_fields` can be sorted on the public authors endpoint with
> `sort_by=cf:total_point&order=desc` — the only requirement is a `[[public_field]]`
> allowlist entry with `resource = "user"`.

### The `[[public_field]]` allowlist

Every `cf_*` and `sort_by=cf:*` parameter is **rejected by default** with `400 field_not_queryable`. The site operator must opt fields in by adding one or more `[[public_field]]` blocks to `config.toml`:

```toml
# Allow sorting users by their numeric "points" system field.
[[public_field]]
resource   = "user"
field      = "points"
operations = ["sort"]

# Include the user's tier_point value in the author response body
# and also allow sorting by it.
[[public_field]]
resource   = "user"
field      = "tier_point"
operations = ["sort", "expose"]

# Allow filtering and sorting articles by "views", scoped to the article post type.
[[public_field]]
resource   = "content"
field      = "views"
post_type  = "article"
operations = ["sort", "filter"]
```

The `"expose"` operation additionally includes the field's value in the public response body (in the `publicFields` map on the `authors` endpoint). Without it, fields can only be used for sort/filter queries (the original behaviour). Currently only the `"user"` resource supports the `"expose"` operation.

See [`docs/configuration.md`](configuration.md#public_field) for the full schema. Admin endpoints (e.g. `GET /api/v1/content_items`) are **not** gated — they remain unrestricted for operator-side tooling.

### Numeric safety

`cf_<field>_min`, `cf_<field>_max`, and `sort_by=cf:<field>` all cast the field value to a number before comparing or ordering. Non-numeric values are treated as `0`:

- SQLite: silent cast via `CAST(json_extract(...) AS REAL)`.
- PostgreSQL: a `CASE WHEN … ~ '^-?[0-9]+(\.[0-9]+)?$'` wrapper casts matching values and falls back to `0` for the rest, so a single bad row does not poison the query.
- MySQL: equivalent `CASE WHEN … REGEXP …` wrapper.

`cf_<field>=<value>` (equality) does no cast — it compares the raw JSON value against the literal string the caller sent. This is the right shape for string fields like `category` or `tier`, but it does **not** match when the JSON value is a number and the URL param is its string form (so `cf_points=87` will not match `{"points":87}` — use `cf_points_min=87&cf_points_max=87` instead, or store the value as a string).

### Error catalog (public query)

| HTTP | `error.code` | When |
|---|---|---|
| `400` | `field_not_queryable` | The referenced field is not in the `[[public_field]]` allowlist (or the allowlist is empty / the registry was not wired in `main.go`). The message tells the operator to add an entry to `config.toml`. |
| `400` | `invalid_sort` | `sort_by` is non-empty but is not of the form `cf:<field>`, or `<field>` does not match `^[a-z][a-z0-9_]*$`, or `order` is not one of `asc`/`desc`/empty. |
| `400` | `invalid_filter_field` / `invalid_filter_operator` / `invalid_filter_value` | The cf filter failed domain-level validation (empty field, unknown operator, empty value). |
| `500` | `internal_error` | Repository failure. |

## WordPress import (`/api/v1/wordpress/import`)

The WordPress import endpoint is also available in the agent (Bearer API key) realm, so the `lesstruct-cli` and programmatic callers can trigger and track imports without a browser session.

### Start an import

`POST /api/v1/wordpress/import` — multipart/form-data with a `file` part (the WXR XML) and an optional `skipMedia` form field.

**Auth:** API key belonging to an **Admin** role (non-admin keys get `403 INSUFFICIENT_PERMISSIONS`).

**Request**

```
POST /api/v1/wordpress/import
Authorization: Bearer lesstruct_a1b2c3d4e5f6_<secret>
Content-Type: multipart/form-data; boundary=----boundary

------boundary
Content-Disposition: form-data; name="file"; filename="export.xml"

<WXR XML content>
------boundary
Content-Disposition: form-data; name="skipMedia"

true
------boundary--
```

The `skipMedia` field is optional — when set to `"true"` the server skips downloading inline images and featured images during import (content is imported with original WordPress URLs hotlinked).

**Response** (`202 Accepted`):

```json
{
  "data": {
    "jobId": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
    "state": "running"
  },
  "error": null
}
```

### Track import progress

`GET /api/v1/wordpress/import/status/{jobId}` — poll for the current state of an import job. When `{jobId}` is omitted, returns the most recent job (if any).

**Auth:** API key belonging to an **Admin** role (same as the import endpoint).

**Response** (`200 OK`):

```json
{
  "data": {
    "jobId": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
    "job": {
      "state": "running",
      "imported": 42,
      "skipped": 0,
      "usersImported": 3,
      "total": 150,
      "startedAt": "2026-07-26T10:00:00Z"
    }
  },
  "error": null
}
```

`state` is one of `"running"`, `"done"`, or `"failed"`. When `total` is zero the import is still in its initialisation phase (parsing the WXR). When `state` is `"done"` or `"failed"`, `finishedAt` is also populated.


