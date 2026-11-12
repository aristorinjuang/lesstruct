---
title: "Lesstruct MCP Server"
linkTitle: "MCP"
---

# Lesstruct MCP Server

> **Scope.** This page documents `lesstruct-mcp` (the stdio server) and the
> server's optional Streamable HTTP MCP endpoint (`POST /mcp`) that exposes
> Lesstruct to AI agents. Endpoint semantics live in
> [api-reference.md](api-reference.md); the terminal client is documented
> in [cli.md](cli.md).

Lesstruct speaks MCP two ways. `lesstruct-mcp` is a stdio server: the agent
spawns the binary, speaks JSON-RPC over stdin/stdout, and every tool call runs
as the API key's user — the same RBAC scoping as the browser admin and
[`lesstruct-cli`](cli.md). The server binary also carries the same tool surface
as an opt-in **Streamable HTTP endpoint** at `POST /mcp` for hosts that connect
over the network. Both front-ends share the `pkg/mcpserver` tool surface, so a
tool added once serves both.

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

## Connecting a local agent (stdio)

The stdio server is launched by the agent and talks over its stdin/stdout. Pass
the key via the environment (never the command line — argv is visible to other
local users and lands in shell history).

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

## Connecting a remote host (Streamable HTTP)

The running server can expose the same tools over the network. Enable the
endpoint in `config.toml` and restart:

```toml
[mcp]
enabled = true
```

The endpoint answers at `POST https://cms.example.com/mcp` — register the full
path exactly as written; nothing is appended. Every request authenticates with
the same `Authorization: Bearer lesstruct_<keyID>_<secret>` API key, and each
tool call runs as that key's user through the same middleware, rate limits,
capability checks, and edition gating as a direct `/api/v1` call.

Properties:

- **Transport.** MCP Streamable HTTP (JSON-RPC 2.0 over `POST`), stateless,
  answering `application/json`. No separate process, port, or SSE stream is
  needed beyond the site itself.
- **Protocol.** `initialize` → `tools/list` → `tools/call`; the server
  negotiates the protocol version the host announces (`2025-06-18` and the
  versions this SDK supports).
- **No loopback hop.** Tools dispatch into the server's own router in-process,
  so there is no self-DNS or proxy dependency and the API key is not replayed
  to a second address.
- **Default off.** Without `[mcp] enabled = true`, `/mcp` is not mounted and
  returns 404.

A typical remote-host entry (keys vary by host — consult its documentation):

```json
{
  "mcpServers": {
    "lesstruct": {
      "url": "https://cms.example.com/mcp",
      "headers": {
        "Authorization": "Bearer lesstruct_<keyID>_<secret>"
      }
    }
  }
}
```

### Host-side limits

Hosted MCP clients impose their own budgets, which can be tighter than the API:

| Limit | Effect on Lesstruct |
|---|---|
| ~15s per request, ~20s per turn | `media_reencode` (minutes), `export_content`/`ssg_generate` (archives), and large imports exceed the window. Start an import, then poll `import_status`/`hugo_import_status`; run re-encode from the CLI. |
| ~1 MiB response body | Large `content_get`/`content_list` payloads are truncated host-side. Paginate with a small `limit` and fetch one item at a time. |
| ~20,000 chars of tool text | The host truncates long tool results; keep list calls small and fetch one item at a time. |
| HTTPS + public IP, no redirects | Put `/mcp` behind your normal TLS reverse proxy and register the final public URL; some hosts do not follow 3xx responses. |
| Protocol version | A host that hard-rejects `2025-06-18` cannot connect yet; the server speaks the version the host announces. |

These are host-side constraints, not Lesstruct settings.

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
- `404` on `POST /mcp` — the endpoint is not enabled: set `[mcp] enabled =
  true` in `config.toml` and restart.
- `401 UNAUTHORIZED` on `POST /mcp` — missing or invalid Bearer key; the
  endpoint accepts API keys only (no browser session cookie).
- `405 Method Not Allowed` on `GET /mcp` — the stateless endpoint serves
  `POST` only; hosts that open a standalone SSE stream should disable it.
