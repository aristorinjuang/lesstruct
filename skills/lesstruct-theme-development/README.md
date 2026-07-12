# lesstruct-theme-development

A skill for AI agents that helps Lesstruct users build, modify, audit, or
repair custom themes for their Lesstruct installation. It works entirely from
your `themes/<name>/` directory and the running public site; it does not
require access to the Lesstruct source tree.

## Quick start

Install with the open agent-skills CLI — it auto-detects your agent
(Claude Code, OpenCode, Cursor, Codex, Gemini CLI, Copilot, and 25+ others)
and writes the skill to the correct path:

```bash
# Install just this skill
npx skills add aristorinjuang/lesstruct --skill lesstruct-theme-development

# Or, to install all Lesstruct skills at once
npx skills add aristorinjuang/lesstruct
```

Add `-g` for a global (user-wide) install, or `-a <agent>` to target a
specific one. Restart your agent, then invoke the skill:

> "Help me build a dark theme for my Lesstruct site."

For manual `cp -r` fallbacks and the full per-agent path table, see
[`references/install-paths.md`](references/install-paths.md).

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

```bash
# If installed via the CLI
npx skills remove lesstruct-theme-development

# Or remove the directory manually
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
