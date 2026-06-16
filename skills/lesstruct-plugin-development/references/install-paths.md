# Install Paths

> **Path note.** The exact skill directory location depends on the agent.
> The patterns below are based on common conventions; if your agent uses
> a different path, adjust accordingly. The `README.md` of this skill is
> the canonical install guide; this file is a quick reference.

## Claude Code

```bash
# Copy the skill into Claude Code's skill directory.
cp -r lesstruct-plugin-development ~/.claude/skills/lesstruct-plugin-development
```

Claude Code loads skills from `~/.claude/skills/` by default. If you have
configured a custom path, substitute it.

## OpenCode

```bash
# OpenCode uses the same skills directory as Claude Code.
cp -r lesstruct-plugin-development ~/.claude/skills/lesstruct-plugin-development
```

If you have multiple agents, you can symlink instead:

```bash
ln -s "$(pwd)/lesstruct-plugin-development" ~/.claude/skills/lesstruct-plugin-development
```

## OpenClaw

> **Note:** OpenClaw's skill directory path is hypothetical; check the
> OpenClaw documentation for the actual location.

```bash
cp -r lesstruct-plugin-development ~/.openclaw/skills/lesstruct-plugin-development
```

## Hermes

> **Note:** Hermes' skill directory path is hypothetical; check the
> Hermes documentation for the actual location.

```bash
cp -r lesstruct-plugin-development ~/.hermes/skills/lesstruct-plugin-development
```

## Generic agent

Any agent that loads Markdown files with YAML frontmatter from a
`skills/` directory should work. The minimum contract the agent must
support:

```yaml
---
name: lesstruct-plugin-development
description: <one-paragraph description>
---
```

Plus a `SKILL.md` filename (case-insensitive on most filesystems).

To install for a generic agent whose skill directory you know:

```bash
cp -r lesstruct-plugin-development "<agent-config-dir>/skills/lesstruct-plugin-development"
```

## Optional: `customize.toml`

This skill includes a `customize.toml` file for BMad agents. Agents that
do not understand BMad's customization format will ignore the file. You
can delete it without affecting the skill's behaviour in non-BMad
agents:

```bash
rm lesstruct-plugin-development/customize.toml
```

## Verifying the install

After installing, restart your agent and ask it to perform a trivial
task related to Lesstruct plugins. For example:

> "List the three hooks the Lesstruct plugin host currently invokes."

If the agent responds with `before_save`, `after_create`, and
`after_publish` (and notes that `on_plugin_loaded` and `before_delete`
are reserved), the skill is loaded correctly. If the agent does not
know what you are talking about, check the agent's skill directory
path and that the `SKILL.md` is in the right place.
