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
| `slug` | string | URL slug. Auto-generated from the title; the `slug` you send in create/update is **not honored**. |
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
| `body` | yes | The content. With `format: markdown` it is Markdown (converted server-side to Tiptap); with `format: tiptap` (the default) it must be a valid Tiptap JSON document string. |
| `format` | no | `"markdown"` or `"tiptap"`. Defaults to `"tiptap"`. Matched case-insensitively after trimming leading/trailing whitespace. |
| `postType` | no | Content type. |
| `tags` | no | Array of tag strings. Server normalizes (trim, lowercase, dedupe, length-bound) via `ValidateTags`; an invalid tag returns 400 `VALIDATION_ERROR`. |
| `language` | no | Language code (e.g. `"en"`, `"id"`). Must be in the server's configured languages list (`config.toml` `[languages]`); an unknown code returns 400 `VALIDATION_ERROR` (`ErrInvalidLanguage`). |
| `slug` | no | Accepted for API stability but **not honored** — the server auto-generates the slug from the title. |
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

Accepts `title`, `body`, `format`, `postType`, `customFields`, `isPublished`, `tags`, and `language`. SEO metadata (`metaDescription`, `ogTitle`, `ogDescription`), `allowComments`, and `translationGroupId` are **preserved from the existing item** and cannot be changed via this endpoint — any values you send for them are ignored. `slug` is accepted for API stability but not honored (the server auto-generates the slug from the title). `format: markdown` converts the body to Tiptap before storing.

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

> **Shared path note.** `GET /api/v1/media` and `GET /api/v1/media/{id}` are shared with the browser admin panel; the server dispatches to the agent handler when the request presents a `lesstruct_`-prefixed Bearer token, and to the browser handler otherwise. For agent clients this is transparent — always send the Bearer key. `POST /api/v1/media` (upload) is agent/Bearer-only.

## Comments

Create, list, delete, and moderate comments on a content item. The agent comment surface is **nested under the content namespace** (`/api/v1/content/{id}/comments`) so it is collision-free with the browser admin's `/api/v1/content_items/.../comments` and `/api/v1/comments` routes, and consistent with the rest of the agent surface (which keys everything by content id). New comments always start in the `pending` moderation status.

> **Browser-admin moderation queue.** The admin panel additionally exposes `GET /api/v1/comments/pending` (JWT + CSRF, **Admin only** — not part of the Bearer `/api/v1` agent surface). It returns every comment currently in the `pending` status across all content, each enriched with `contentId`, `contentTitle`, and `contentSlug` so the global moderation queue can link back to the originating post. The same response shape applies to the per-content admin route `GET /api/v1/content_items/{id}/comments`.

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

Returns every comment on content `{id}` (any moderation status — the management view), scoped to content the caller may see (published, owned, or Admin). Envelope is a bare `data` array (always present, even when empty):

```json
{ "data": [ { "id": 1, "comment": "ok", "status": "approved", "createdAt": "…" }, { "id": 2, "comment": "waiting", "status": "pending", "createdAt": "…" } ] }
```

| Status | Code | When |
|---|---|---|
| `200` | — | The comment list (possibly empty). |
| `400` | `VALIDATION_ERROR` | Invalid content id. |
| `404` | `NOT_FOUND` | Content does not exist or is not visible to the caller. |

### Delete comment

```http
DELETE /api/v1/content/{id}/comments/{commentId}
```

Deletes the comment. An Admin key may delete any comment; any other key only its own. The server returns `404` (no disclosure) when the comment is missing or belongs to someone else.

**Response** `204 No Content` (empty body).

| Status | Code | When |
|---|---|---|
| `204` | — | Deleted. |
| `404` | `NOT_FOUND` | Comment does not exist, or is not yours (and you are not Admin). |

### Moderate comment (admin only)

```http
PUT /api/v1/content/{id}/comments/{commentId}/status
Content-Type: application/json
```

```json
{ "status": "approved" }
```

Sets a comment's moderation status. `status` must be a valid value (`pending`, `approved`, `rejected`, `spam`). **Admin only** — a non-admin key gets `403 FORBIDDEN`. The updated comment is returned.

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
| `404` | `NOT_FOUND` | Comment does not exist. |

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
                body: { type: string, description: "Tiptap JSON (format=tiptap) or Markdown (format=markdown)" }
                format: { type: string, enum: [markdown, tiptap], default: tiptap, description: "Server matches case-insensitively after trimming whitespace." }
                postType: { type: string }
                slug: { type: string, description: "Accepted but not honored; slug is auto-generated." }
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

