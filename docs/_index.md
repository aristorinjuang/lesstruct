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
| Domain model: bounded contexts, aggregates, DDD layers, context map | [ddd/domain-context.md](ddd/domain-context.md) |
| Domain vocabulary (the term to use in code, docs, and UI) | [ddd/ubiquitous-language.md](ddd/ubiquitous-language.md) |
| Commerce order model: checkout invariants, lifecycle, Money, invoices | [ddd/orders-bounded-context.md](ddd/orders-bounded-context.md) |
| The `/api/v1` REST API (auth, envelope, error catalog) | [api-reference.md](api-reference.md) |
| The `lesstruct-cli` terminal client (install, auth, commands, workflows) | [cli.md](cli.md) |
| The `lesstruct-mcp` MCP server for AI agents (setup, tool catalog) | [mcp.md](mcp.md) |
| `config.toml` schema or environment variables | [configuration.md](configuration.md) |
| Writing a Go module (Tier 1: layout, seams, gating, wiring) | [module-development.md](module-development.md) |
| What a built-in module owns / what disabling it does | [built-in-modules.md](built-in-modules.md) |
| Which edition to install / what each edition carries | [editions.md](editions.md) |
| Setting up the eCommerce edition: products, payments, fulfillment | [getting-started-ecommerce.md](getting-started-ecommerce.md) |
| Understanding or evolving the kernel | [kernel.md](kernel.md) |
| Writing a WASM plugin (hooks, lifecycle, build) | [plugin-development.md](plugin-development.md) |
| Plugin capabilities / host functions | [plugin-capabilities.md](plugin-capabilities.md) |
| Building or overriding a theme (CSS, templates, JS) | [theme-development.md](theme-development.md) |
| GOHTML / Go template formatting (`.gohtml` files) | [coding-standards/html.md](coding-standards/html.md) |
| Brand assets (logo, colors, typography) | [brands.md](brands.md) |

## Cross-cutting references

- **Product feature catalog** → [features.md](features.md) (the canonical list; the homepage and `README.md` curate from it).
- **Coding, testing, and doc-sync conventions** → `AGENTS.md` (repo root).
- **Build / test / mock commands** → `Makefile`: `make mock`, `make lint`, `make test`, `make vulncheck`, `make docs-serve`, `make build-cli`, `make build-admin`, `make worktree <path>`, `make worktree-sync DIR=...`.
- **External agents building themes/plugins on a deployed site** → the installed skills `skills/lesstruct-theme-development/` and `skills/lesstruct-plugin-development/` (their `references/` are the user-facing snapshots of the theme/plugin docs above); **agents developing Tier 1 Go modules in a checkout** → the installed skill `skills/lesstruct-module-development/` (its `references/` snapshot the module doc plus the kernel reality matrix and scaffold).

## Doc-ownership contract

Every doc declares its scope in its first section. When you change code a doc describes,
update that doc **in the same change** — the full rules and the "If you touch X → update Y"
table are in `AGENTS.md` under **Documentation Sync**.
