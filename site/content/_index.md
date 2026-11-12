---
title: "Lesstruct"
description: "Everything is a module. The Go CMS you can shrink, extend, and ship as one binary — for humans and AI agents."
layout: landing
---

## Quick start

```bash
go install github.com/aristorinjuang/lesstruct@latest
export JWT_SECRET="$(head -c 48 /dev/urandom | base64)"
lesstruct
```

Open <http://localhost:8080/admin>, register the first account, and start publishing. Full configuration reference lives in the [Configuration](/docs/configuration/) docs.

## Install the CLI

`lesstruct-cli` is a terminal client for the REST API — useful for scripts and
AI agents.

```bash
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli@latest
```

Point it at a running server and create an API key from the admin panel. The
full command reference lives in the [API Reference](/docs/api-reference/).

## Connect your AI agent

`lesstruct-mcp` exposes the same API as 44 MCP tools over stdio, so any
MCP-native agent (Muse Code, Claude Code, OpenCode, Cursor) can operate the
site and the store with the same API key — no browser session needed.

```bash
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-mcp@latest
```

**Muse Code** — add the server under `mcpServers` in
`~/.config/muse/settings.json`:

```json
{
  "mcpServers": {
    "lesstruct": {
      "command": "lesstruct-mcp",
      "env": {
        "LESSTRUCT_API_KEY": "lesstruct_<keyID>_<secret>",
        "LESSTRUCT_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

Claude Code, OpenCode, and Cursor snippets live in the
[MCP guide](/docs/mcp/).

When the agent runs on another machine, enable `[mcp] enabled = true` in
`config.toml` and point it at `https://your-site/mcp` with the same Bearer key:
the server exposes the identical 44 tools over MCP Streamable HTTP. The
[MCP guide](/docs/mcp/) covers the host entry, protocol, and host-side limits.

## Where to go next

- **Evaluating Lesstruct?** Browse the full [Features](/docs/features/) catalog, or [see it in action](/tour/) with screenshots from a running demo.
- **New to Lesstruct?** Read the [Project Context](/docs/project-context/) for the architecture overview, then skim the [Configuration](/docs/configuration/) reference.
- **Building a site?** Start with the [Theme Development](/docs/theme-development/) guide.
- **Extending the CMS?** The [Plugin Development](/docs/plugin-development/) guide and [Plugin Capabilities](/docs/plugin-capabilities/) reference cover the WASM hook model and host functions.
- **Integrating via API?** The [API Reference](/docs/api-reference/) documents every `/api/v1` endpoint, and `lesstruct-cli` ships in the same release for terminal-first and AI-agent workflows.
- **Want to support Lesstruct?** [Sponsor the project](https://github.com/sponsors/aristorinjuang) or grab official merch at [merch.lesstruct.dev](https://merch.lesstruct.dev).

## AI agent ingestion

This site is designed to be crawlable by AI agents in addition to humans.

- [`/llms.txt`](/llms.txt) — index of every documentation page with a one-line summary.
- [`/llms-full.txt`](/llms-full.txt) — every page concatenated in section order, suitable as a single context window.
- Per-page markdown is also served at the canonical path (e.g. [`/docs/api-reference/index.md`](/docs/api-reference/index.md)) for retrieval pipelines that prefer per-page fetches.
- [`/sitemap.xml`](/sitemap.xml) — Hugo's built-in sitemap.

For an LLM working inside the repo, the source tree is the contract: `docs/` is developer-facing, `skills/*/references/` is the user-facing snapshot that ships in the binary.
