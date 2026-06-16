# lesstruct-theme-development

A skill for AI agents (Claude Code, OpenCode, OpenClaw, Hermes, and others)
that helps Lesstruct users build, modify, audit, or repair custom themes for
their Lesstruct installation.

The skill is developed in the [Lesstruct](https://github.com/aristorinjuang/lesstruct)
repository and ships as a copy-paste directory. It works entirely from your
`themes/<name>/` directory and the running public site; it does not require
access to the Lesstruct source tree.

## Quick start (Claude Code / OpenCode)

```bash
# From a clone of the Lesstruct repo, or after downloading this directory
# from a Lesstruct release:
cp -r skills/lesstruct-theme-development ~/.claude/skills/lesstruct-theme-development
```

Restart your agent. The skill is now available. To invoke it, ask your agent
something like:

> "Help me build a dark theme for my Lesstruct site."

For other agents, see [`references/install-paths.md`](references/install-paths.md).

## What it does

Walks you through:

- Setting up `themes/<name>/` and the `THEME_DIR` environment variable.
- Overriding CSS, JavaScript, and HTML templates.
- Preserving the layout / body block contract.
- Preserving the JavaScript DOM contract (the ids and classes the default
  templates and scripts expect).
- Handling CDN assets (katex, highlight.js) the default layout pulls in.
- Verifying your theme against all 10 page types.
- Maintaining your theme across Lesstruct upgrades.

The full workflow is in `SKILL.md`. The user-facing contract is in
[`references/theme-development.md`](references/theme-development.md).

## Directory layout

```
lesstruct-theme-development/
  SKILL.md                              # The workflow
  customize.toml                        # BMad-specific extension (optional)
  README.md                             # This file
  references/
    theme-development.md                # User-facing theme contract
    default-style-reference.css         # Verbatim readable default CSS
    theme-audit-checklist.md            # Pre-flight audit checklist
    page-render-smoke-test.md           # Manual page-by-page smoke test
    install-paths.md                    # Per-agent install snippets
```

## Uninstall

Remove the directory from your agent's skills folder:

```bash
rm -rf ~/.claude/skills/lesstruct-theme-development
```

## Where to get help

- **Lesstruct repository**: [github.com/aristorinjuang/lesstruct](https://github.com/aristorinjuang/lesstruct)
- **Issue tracker**: open an issue in the Lesstruct repo with the
  `theme-development` label.
- **Developer-facing docs**: the Lesstruct repo's `docs/theme-development.md`
  (the canonical, source-tree-aware version of the contract this skill uses).
- **The loaded reference**: when the skill runs, the user-facing contract is
  at `references/theme-development.md` inside this skill.

## License

This skill is part of the Lesstruct project and is released under the same
license as Lesstruct itself.
