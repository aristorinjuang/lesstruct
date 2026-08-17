# Install Paths

> **Recommended.** Use the open agent-skills CLI — it auto-detects which
> agents you have installed and writes the skill to the correct path for
> each, so you do not need the table below.

```bash
# Install just this skill (interactive — pick your agent(s))
npx skills add aristorinjuang/lesstruct --skill lesstruct-plugin-development

# Non-interactive examples
npx skills add aristorinjuang/lesstruct --skill lesstruct-plugin-development -a claude-code -y   # project scope
npx skills add aristorinjuang/lesstruct --skill lesstruct-plugin-development -g -a opencode -y   # global scope
```

The CLI supports **30+ agents** — Claude Code, OpenCode, Cursor, Codex, Gemini
CLI, GitHub Copilot, Cline, Roo Code, Windsurf, Goose, Amp, and more. See the
[agent skills CLI](https://github.com/antfu/skills-cli#supported-agents) for
the full, up-to-date list.

## Manual install (fallback)

If you cannot run `npx` (air-gapped host, pinned toolchain, etc.), copy the
skill directory into your agent's skills folder. The path depends on the
agent and the scope:

| Agent | Project scope | Global scope |
|---|---|---|
| Claude Code | `.claude/skills/` | `~/.claude/skills/` |
| OpenCode | `.opencode/skills/` | `~/.config/opencode/skills/` |
| Cursor | `.cursor/skills/` | `~/.cursor/skills/` |
| Codex | `.codex/skills/` | `~/.codex/skills/` |
| Gemini CLI | `.gemini/skills/` | `~/.gemini/skills/` |
| GitHub Copilot | `.github/skills/` | `~/.copilot/skills/` |

```bash
# Example — Claude Code, global scope
cp -r lesstruct-plugin-development ~/.claude/skills/lesstruct-plugin-development

# Example — OpenCode, project scope
cp -r lesstruct-plugin-development .opencode/skills/lesstruct-plugin-development
```

For any other agent that loads Markdown files with YAML frontmatter from a
`skills/` directory, the minimum contract the agent must support is:

```yaml
---
name: lesstruct-plugin-development
description: <one-paragraph description>
---
```

…plus a `SKILL.md` filename. Copy the directory into that agent's
`<config-dir>/skills/` folder.

## Optional: `customize.toml`

This skill includes a `customize.toml` file for BMad agents. Agents that
do not understand BMad's customization format will ignore the file. You
can delete it without affecting the skill's behaviour in non-BMad agents:

```bash
rm lesstruct-plugin-development/customize.toml
```

## Verifying the install

After installing, restart your agent and ask it to perform a trivial
task related to Lesstruct plugins. For example:

> "List the five hooks the Lesstruct plugin host currently invokes."

If the agent responds with `before_save`, `after_create`,
`after_publish`, `before_delete`, and `after_unpublish` (and notes
that `on_plugin_loaded` is reserved), the skill is loaded correctly.
If the agent does not know what you are talking about, check the
agent's skill directory path and that the `SKILL.md` is in the right
place.
