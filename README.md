# Lesstruct

> An open-source CMS powered by Go. Fast, configurable, customizable. Built for humans and AI agents, extended with WebAssembly plugins. One binary, no Docker required.

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](#license)
[![Latest Release](https://img.shields.io/github/v/release/aristorinjuang/lesstruct)](https://github.com/aristorinjuang/lesstruct/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/aristorinjuang/lesstruct/ci.yml?branch=main)](https://github.com/aristorinjuang/lesstruct/actions)
[![Docs](https://img.shields.io/badge/docs-lesstruct.dev-blue)](https://lesstruct.dev/)

## Why Lesstruct?

- **One binary, no Docker.** A single static Go binary runs the whole CMS. SQLite is built in. No runtime, no container, no `node_modules` in production.
- **Multi-database, your choice.** SQLite by default; PostgreSQL and MySQL supported via `DB_DRIVER`. Migrations run on first start.
- **Custom fields, no plugins needed.** Define post types, user profile fields, and validation rules in `config.toml`. The admin form, storage, and queries all read from that file.
- **Extend with WebAssembly plugins.** Drop a compiled `.wasm` into `plugins/` and it hooks into `before_save`, `after_create`, and `after_publish`. Plugins are sandboxed, immutable, and can call host functions for HTTP, DB, and logging.
- **AI integrations are opt-in.** Bring your own OpenAI-compatible key for text, or a Google / OpenAI key for image generation. The admin panel surfaces them as buttons; nothing else changes.
- **Built for humans AND AI agents.** A versioned REST API at `/api/v1`, a `lesstruct-cli` client, Markdown as a first-class ingest format, and skills for Claude Code / OpenCode / Hermes / OpenClaw.

## Quick start

```bash
go install github.com/aristorinjuang/lesstruct@latest
export JWT_SECRET="$(head -c 48 /dev/urandom | base64)"
lesstruct
```

Open <http://localhost:8080/admin>, register the first account, and start publishing. The full configuration split between `config.toml` (content schema) and `.env` (deployment) is in [`docs/configuration.md`](docs/configuration.md).

## Installation

### Option A — Download a release binary (recommended)

Grab the latest build for your platform from the [releases page](https://github.com/aristorinjuang/lesstruct/releases). Each release ships static binaries for `linux`, `darwin`, and `windows` on `amd64` and `arm64`:

| OS | amd64 | arm64 |
|---|---|---|
| Linux | `lesstruct-linux-amd64` | `lesstruct-linux-arm64` |
| macOS | `lesstruct-darwin-amd64` | `lesstruct-darwin-arm64` |
| Windows | `lesstruct-windows-amd64.exe` | `lesstruct-windows-arm64.exe` |

```bash
# Linux / macOS — verify, then install
curl -sSL https://github.com/aristorinjuang/lesstruct/releases/latest/download/lesstruct-linux-amd64 -o lesstruct
chmod +x lesstruct
sudo mv lesstruct /usr/local/bin/
lesstruct
```

### Option B — `go install`

```bash
go install github.com/aristorinjuang/lesstruct@latest
```

### Option C — Build from source

```bash
git clone https://github.com/aristorinjuang/lesstruct.git
cd lesstruct
make install   # builds the admin panel (npm), the server, and the CLI into $GOBIN
```

### Option D — Docker (optional)

The headline is "no Docker required," but Lesstruct runs fine in a container if you prefer one. The official image is not published yet — build your own from the Dockerfile-shaped workflow in `.github/workflows/release.yml` and the static binary it produces. The binary inside the container is `CGO_ENABLED=0`, so a `FROM scratch` or `FROM gcr.io/distroless/static` works.

## Configuration

Two layers, two responsibilities:

- **`config.toml`** — your site's *content schema*: languages, custom post types, user profile fields, thumbnail variants. Hand-edited, version-controlled.
- **`.env`** — your *deployment configuration*: host, port, database, secrets, SMTP, AI keys. Treated as deployment state, not committed.

The only required value is `JWT_SECRET` (≥ 32 characters). Everything else has a sensible default. See [`docs/configuration.md`](docs/configuration.md) for the full reference, validation rules, and troubleshooting recipes.

## AI Agent Skills

Lesstruct ships two skills that turn your AI agent into a Lesstruct expert. They work from your installed site (themes/ and plugins/ directories) — no access to the source tree required.

```bash
# Theme development — CSS, JS, HTML template overrides
cp -r skills/lesstruct-theme-development ~/.claude/skills/lesstruct-theme-development

# Plugin development — WASM hooks, host functions, capability manifests
cp -r skills/lesstruct-plugin-development ~/.claude/skills/lesstruct-plugin-development
```

Supported agents include Claude Code, OpenCode, OpenClaw, and Hermes. Per-agent install paths are in each skill's `references/install-paths.md`. Restart your agent after copying.

Once installed, ask your agent things like:

- "Help me build a dark theme for my Lesstruct site."
- "Help me write a WASM plugin that enriches content from an external API."

## Plugin Development

Plugins are sandboxed WebAssembly modules with a WordPress-familiar hook model — explicit registration, priority-based execution, immutable data flow. The currently-invoked hooks are `before_save`, `after_create`, and `after_publish`; reserved-but-unused hooks (`on_plugin_loaded`, `before_delete`) are documented for forward compatibility.

- Full contract: [`docs/plugin-development.md`](docs/plugin-development.md)
- Capability manifest & host functions: [`docs/plugin-capabilities.md`](docs/plugin-capabilities.md)
- AI-assisted workflow: [`skills/lesstruct-plugin-development/`](skills/lesstruct-plugin-development/)

## Theme Development

Themes are a `themes/<name>/` directory containing CSS, optional JS, and optional HTML template overrides — pointed at with `THEME_DIR=themes/<name>`. The default theme serves as a working starting point; the contract (CSS variables, layout/body blocks, JS DOM ids, CDN assets) is documented so a fork-and-modify workflow is safe.

- Full contract: [`docs/theme-development.md`](docs/theme-development.md)
- AI-assisted workflow: [`skills/lesstruct-theme-development/`](skills/lesstruct-theme-development/)

## API & CLI

A versioned, API-key-authenticated REST API at `/api/v1` covers Content and Media. Markdown is accepted on create/update and converted to canonical Tiptap JSON server-side.

`lesstruct-cli` is a thin Cobra client over the same API — designed for AI agents and humans who live in the terminal. Install it with Go:

```bash
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli@latest
```

Point it at a running server and create an API key from the admin panel to get started.

- API reference: [`docs/api-reference.md`](docs/api-reference.md)
- Build from source: `make build-cli` → `bin/lesstruct-cli`

## Support this project

Lesstruct is a spare-time project. I — Ari — design, build, test, and document it in the hours outside my day job. If it saves you time, the most helpful thing you can do is **[sponsor the work on GitHub Sponsors](https://github.com/sponsors/aristorinjuang)**. Sponsorship funds the small stuff that compounds: CI minutes, a real domain for documentation, and the occasional dedicated weekend for plugin/theme polish.

Starring the repo, opening well-scoped issues, and sending PRs with a failing test are all just as valuable — and they cost nothing.

## License

MIT. See the `LICENSE` file for the full text.
