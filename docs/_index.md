---
title: "Documentation Index"
weight: 1
---

# Documentation Index

_Start here. For any task, find the doc that owns it below, then read that doc's first
section — each doc states its scope at the top._

> **AI agents (Claude Code, OpenCode):** read `AGENTS.md` first (style & process), then
> this index, then [project-context.md](project-context.md) for architecture. The
> **"Where does new code go?"** decision tree lives in `project-context.md`.

## Task → doc routing

| If you are working on... | Read this first |
|---|---|
| Architecture, layers, stack versions, conventions, **where new code goes** | [project-context.md](project-context.md) |
| The `/api/v1` REST API (auth, envelope, error catalog) | [api-reference.md](api-reference.md) |
| `config.toml` schema or environment variables | [configuration.md](configuration.md) |
| Writing a WASM plugin (hooks, lifecycle, build) | [plugin-development.md](plugin-development.md) |
| Plugin capabilities / host functions | [plugin-capabilities.md](plugin-capabilities.md) |
| Building or overriding a theme (CSS, templates, JS) | [theme-development.md](theme-development.md) |

## Cross-cutting references

- **Product feature catalog** → [features.md](features.md) (the canonical list; the homepage and `README.md` curate from it).
- **Coding, testing, and doc-sync conventions** → `AGENTS.md` (repo root).
- **Build / test / mock commands** → `Makefile`: `make mock`, `make lint`, `make test`, `make vulncheck`, `make docs-serve`, `make build-cli`, `make build-admin`.
- **External agents building themes/plugins on a deployed site** → the installed skills `skills/lesstruct-theme-development/` and `skills/lesstruct-plugin-development/` (their `references/` are the user-facing snapshots of the theme/plugin docs above).

## Doc-ownership contract

Every doc declares its scope in its first section. When you change code a doc describes,
update that doc **in the same change** — the full rules and the "If you touch X → update Y"
table are in `AGENTS.md` under **Documentation Sync**.
