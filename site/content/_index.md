---
title: "Lesstruct"
description: "An open-source CMS powered by Go. Fast, configurable, customizable. Built for humans and AI agents, extended with WebAssembly plugins. One binary, no Docker required."
layout: landing
---

{{< columns >}}
- ## One binary, no Docker

  A single static Go binary runs the whole CMS. SQLite is built in. No runtime, no container, no `node_modules` in production.

- ## Custom fields, no plugins

  Define post types, user profile fields, and validation rules in `config.toml`. The admin form, storage, and queries all read from that file.

- ## Extend with WebAssembly

  Drop a compiled `.wasm` into `plugins/` and it hooks into `before_save`, `after_create`, and `after_publish`. Plugins are sandboxed, immutable, and can call host functions for HTTP, DB, and logging.

- ## Built for humans AND AI

  A versioned REST API at `/api/v1`, a `lesstruct-cli` client, Markdown as a first-class ingest format, and skills for Claude Code, OpenCode, and other agents.
{{< /columns >}}

## Quick start

```bash
go install github.com/aristorinjuang/lesstruct@latest
export JWT_SECRET="$(head -c 48 /dev/urandom | base64)"
lesstruct
```

Open <http://localhost:8080/admin>, register the first account, and start publishing. Full configuration reference lives in the [Configuration](/docs/configuration/) docs.

## Where to go next

- **New to Lesstruct?** Read the [Project Context](/docs/project-context/) for the architecture overview, then skim the [Configuration](/docs/configuration/) reference.
- **Building a site?** Start with the [Theme Development](/docs/theme-development/) guide.
- **Extending the CMS?** The [Plugin Development](/docs/plugin-development/) guide and [Plugin Capabilities](/docs/plugin-capabilities/) reference cover the WASM hook model and host functions.
- **Integrating via API?** The [API Reference](/docs/api-reference/) documents every `/api/v1` endpoint, and `lesstruct-cli` ships in the same release for terminal-first and AI-agent workflows.

## AI agent ingestion

This site is designed to be crawlable by AI agents in addition to humans.

- [`/llms.txt`](/llms.txt) — index of every documentation page with a one-line summary.
- [`/llms-full.txt`](/llms-full.txt) — every page concatenated in section order, suitable as a single context window.
- Per-page markdown is also served at the canonical path (e.g. [`/docs/api-reference/index.md`](/docs/api-reference/index.md)) for retrieval pipelines that prefer per-page fetches.
- [`/sitemap.xml`](/sitemap.xml) — Hugo's built-in sitemap.

For an LLM working inside the repo, the source tree is the contract: `docs/` is developer-facing, `skills/*/references/` is the user-facing snapshot that ships in the binary.
