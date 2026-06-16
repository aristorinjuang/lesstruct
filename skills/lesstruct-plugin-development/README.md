# lesstruct-plugin-development

A skill for AI agents (Claude Code, OpenCode, OpenClaw, Hermes, and others)
that helps Lesstruct users write, debug, audit, and upgrade WASM plugins for
their Lesstruct installation.

The skill is developed in the [Lesstruct](https://github.com/aristorinjuang/lesstruct)
repository and ships as a copy-paste directory. It works entirely from your
`plugins/` directory and the running Lesstruct install; it does not require
access to the Lesstruct source tree.

## Quick start (Claude Code / OpenCode)

```bash
# From a clone of the Lesstruct repo, or after downloading this directory
# from a Lesstruct release:
cp -r skills/lesstruct-plugin-development ~/.claude/skills/lesstruct-plugin-development
```

Restart your agent. The skill is now available. To invoke it, ask your agent
something like:

> "Help me write a WASM plugin for my Lesstruct installation that enriches content from an external API."

For other agents, see [`references/install-paths.md`](references/install-paths.md).

## What it does

Walks you through:

- Picking the right hook (the 3 currently-invoked hooks are
  `before_save`, `after_create`, `after_publish`).
- Implementing the memory protocol (input at offset 65536, return result
  offset; optional `__hook_result_len`).
- Calling host functions (HTTP, DB, logging) with the `//go:wasmimport`
  bindings.
- Writing a capability manifest.
- Building with TinyGo, Rust, or C/C++.
- Deploying to `plugins/` and verifying.
- Auditing an existing plugin.

The full workflow is in `SKILL.md`. The user-facing contract is in
[`references/plugin-development.md`](references/plugin-development.md) and
[`references/plugin-capabilities.md`](references/plugin-capabilities.md).

## What ships with the skill

- `references/examples/` — four Go/TinyGo example plugins, copied from
  `pkg/sdk/examples/`. Three of them use invoked hooks; one uses a
  reserved hook (see `SKILL.md` Step 4).
- `references/host-function-imports.go.txt` — a copy-pasteable
  `//go:wasmimport` block. The Lesstruct SDK does not currently ship
  these declarations.
- `references/hook-data-example.json` — the actual JSON shape the host
  sends to a `before_save` hook.

## Directory layout

```
lesstruct-plugin-development/
  SKILL.md                              # The workflow
  customize.toml                        # BMad-specific extension (optional)
  README.md                             # This file
  references/
    plugin-development.md               # User-facing plugin contract
    plugin-capabilities.md              # User-facing capabilities contract
    host-function-imports.go.txt        # //go:wasmimport declarations
    hook-data-example.json              # Real hookData shape
    dev-vs-prod.md                      # DEV_MODE caveats
    plugin-checklist.md                 # Pre-flight audit checklist
    install-paths.md                    # Per-agent install snippets
    examples/                           # Four bundled Go/TinyGo plugins
      go-hello-world/
      go-content-transform/
      go-system-fields/
      go-validation/
```

## Uninstall

Remove the directory from your agent's skills folder:

```bash
rm -rf ~/.claude/skills/lesstruct-plugin-development
```

## Where to get help

- **Lesstruct repository**: [github.com/aristorinjuang/lesstruct](https://github.com/aristorinjuang/lesstruct)
- **Issue tracker**: open an issue in the Lesstruct repo with the
  `plugin-development` label.
- **Developer-facing docs**: the Lesstruct repo's `docs/plugin-development.md`
  and `docs/plugin-capabilities.md` (the canonical, source-tree-aware
  versions of the contract this skill uses).
- **The loaded reference**: when the skill runs, the user-facing contract
  is at `references/plugin-development.md` and
  `references/plugin-capabilities.md` inside this skill.

## License

This skill is part of the Lesstruct project and is released under the same
license as Lesstruct itself.
