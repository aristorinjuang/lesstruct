---
title: "Lesstruct"
description: "An open-source CMS powered by Go. Fast, configurable, customizable. Built for humans and AI agents, extended with WebAssembly plugins. One binary, no Docker required."
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

## Where to go next

- **Evaluating Lesstruct?** Browse the full [Features](/docs/features/) catalog, or [see it in action](/tour/) with screenshots from a running demo.
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
