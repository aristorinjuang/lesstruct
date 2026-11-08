---
title: "Lesstruct MCP Server"
linkTitle: "MCP"
---

# Lesstruct MCP Server

> **Scope.** This page documents `lesstruct-mcp`, the Model Context Protocol
> (stdio) server that exposes Lesstruct to AI agents. Endpoint semantics live
> in [api-reference.md](api-reference.md); the terminal client is documented
> in [cli.md](cli.md).

`lesstruct-mcp` is a stdio MCP server over `/api/v1` for AI agents. The agent
spawns the binary, speaks JSON-RPC over stdin/stdout, and every tool call runs
as the API key's user — the same RBAC scoping as the browser admin and
[`lesstruct-cli`](cli.md). It shares the typed `pkg/apiv1` client with the
CLI, so one setup serves both binaries.

## Install

```bash
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-mcp@latest
```

Or from source (`make build-mcp` drops the binary into `bin/`):

```bash
make build-mcp    # bin/lesstruct-mcp
```

`lesstruct-mcp --version` reports the shipped version (release builds bake it
in via `-ldflags`; a bare `go build` reports `dev`).

## Authentication and configuration

Every tool call carries one API key as a Bearer token. Create it from the
admin panel: **Profile → API keys** (`lesstruct_<keyID>_<secret>`), scoped to
your user — an Admin key unlocks the admin-only tools below; the server never
sends an unauthenticated request.

Resolution order for **both** the key and the base URL (first hit wins):

| Precedence | API key | Base URL |
|---|---|---|
| 1. Flag | `--api-key` | `--base-url` |
| 2. Environment | `LESSTRUCT_API_KEY` | `LESSTRUCT_BASE_URL` |
| 3. Config file | `[default] api_key` | `[default] base_url` |
| 4. Fallback | — (hard error) | `http://localhost:8080` |

The config file is the same TOML the CLI reads (`$LESSTRUCT_CONFIG`, else
`$XDG_CONFIG_HOME/lesstruct/config.toml`, else `~/.config/lesstruct/config.toml`);
a key-bearing file that is group/world-readable triggers a warning on stderr
(`chmod 600` it). Stdout is protocol-only — every diagnostic goes to stderr,
so logs never corrupt the JSON-RPC stream.

## Connecting an agent

The server speaks stdio: the agent launches it and talks over its
stdin/stdout. Pass the key via the environment (never the command line —
argv is visible to other local users and lands in shell history).

**Muse Code** — add the server under `mcpServers` in
`~/.config/muse/settings.json`:

```json
{
  "mcpServers": {
    "lesstruct": {
      "command": "lesstruct-mcp",
      "env": {
        "LESSTRUCT_API_KEY": "lesstruct_<keyID>_<secret>",
        "LESSTRUCT_BASE_URL": "https://cms.example.com"
      }
    }
  }
}
```

**Claude Code:**

```bash
claude mcp add --transport stdio lesstruct \
  --env LESSTRUCT_API_KEY=lesstruct_<keyID>_<secret> \
  --env LESSTRUCT_BASE_URL=https://cms.example.com \
  -- lesstruct-mcp
```

**OpenCode** (`opencode.json`):

```json
{
  "mcp": {
    "lesstruct": {
      "type": "local",
      "command": ["lesstruct-mcp"],
      "environment": {
        "LESSTRUCT_API_KEY": "lesstruct_<keyID>_<secret>",
        "LESSTRUCT_BASE_URL": "https://cms.example.com"
      }
    }
  }
}
```

**Cursor** (`~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "lesstruct": {
      "command": "lesstruct-mcp",
      "env": {
        "LESSTRUCT_API_KEY": "lesstruct_<keyID>_<secret>",
        "LESSTRUCT_BASE_URL": "https://cms.example.com"
      }
    }
  }
}
```

## Tool catalog

44 tools, grouped by resource. Successful results are the server's
`{data, meta}` envelope as JSON (follow `meta.pagination.nextCursor` while
`meta.pagination.hasMore` is true); failures are MCP tool errors carrying the
`CODE: message` line (e.g. `FORBIDDEN: …`, `NOT_FOUND: …`) so the agent can
react to them.

| Group | Tools |
|---|---|
| content | `content_list`, `content_get`, `content_create`, `content_update`, `content_delete`, `content_publish`, `content_unpublish`, `content_system_fields` (admin only) |
| media | `media_list`, `media_get`, `media_upload` (base64), `media_delete`, `media_reencode` (admin only, minutes-long) |
| catalog (eCommerce) | `catalog_get`, `catalog_upsert` (merges: omitted fields keep server values; `price` required without a record) |
| orders (eCommerce) | `orders_list`, `orders_get`, `orders_transition`, `orders_set_shipping` (pending only) |
| payments (eCommerce) | `payments_list`, `payments_get`, `payments_verify`, `payments_reject` |
| invoice (eCommerce) | `invoice_settings_get`, `invoice_settings_set` |
| users | `users_list`, `users_pending`, `users_approve`, `users_reject`, `users_mark_spam`, `users_suspend`, `users_unsuspend`, `users_soft_delete` (`reject`/`mark_spam`/`soft_delete` require `confirm: true`) |
| comment | `comment_create`, `comment_list`, `comment_delete`, `comment_moderate` (admin only: `approved`/`rejected`/`spam`) |
| transfer | `import_wordpress`, `import_status`, `import_hugo`, `hugo_import_status`, `export_content`, `ssg_generate` |
| escape hatch | `lesstruct_api` — any `GET`/`POST`/`PUT`/`PATCH`/`DELETE` on a `/api/v1/` path with query/body |

Conventions:

- **Markdown is the authoring format.** `content_create`/`content_update`
  default to `format: markdown`; pass `format: html` or `format: tiptap`
  only with a matching body.
- **Patch semantics on update.** Absent fields keep stored values (a
  body-only edit never wipes tags or unpublishes); pass `published: false`
  to unpublish and `tags: []` to clear all tags. Flags-only edits work
  without resending the body.
- **IDs are numeric.** Content/comment/media take `int` ids; order, payment,
  and user tools take `int64` admin ids (orders use the numeric id, not the
  `ORD-000042` display number).
- **Uploads ride base64.** `media_upload` takes `filename` +
  `contentBase64` (+ optional `altText`); imports take `filename` +
  `contentBase64` (+ optional `skipMedia`) and return a job id to poll.
- **Archives ride base64.** `export_content`/`ssg_generate` return
  `{filename, bytes, contentBase64}` with a 50 MiB default cap — raise
  `maxBytes` explicitly for large sites.

## Troubleshooting

- `no API key found` — no key resolved from flag/env/config; create one at
  **Profile → API keys**.
- Tool error `UNAUTHORIZED` — revoked, expired, or mistyped key (or a key
  whose owner was suspended); only the key ID is ever logged.
- Tool error `FORBIDDEN`/`INSUFFICIENT_PERMISSIONS` — the key's user lacks
  the capability (admin-only tools need an Admin key).
- Tool error `NOT_FOUND` on a shared path — the item does not exist or the
  caller may not see it (existence is never disclosed to unauthorized callers).
