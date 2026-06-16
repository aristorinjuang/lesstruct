# Install Paths

> **Path note.** The exact skill directory location depends on the agent. The
> patterns below are based on common conventions; if your agent uses a
> different path, adjust accordingly. The `README.md` of this skill is the
> canonical install guide; this file is a quick reference.

## Claude Code

```bash
# Copy the skill into Claude Code's skill directory.
cp -r lesstruct-theme-development ~/.claude/skills/lesstruct-theme-development
```

Claude Code loads skills from `~/.claude/skills/` by default. If you have
configured a custom path, substitute it.

## OpenCode

```bash
# OpenCode uses the same skills directory as Claude Code.
cp -r lesstruct-theme-development ~/.claude/skills/lesstruct-theme-development
```

If you have multiple agents, you can symlink instead:

```bash
ln -s "$(pwd)/lesstruct-theme-development" ~/.claude/skills/lesstruct-theme-development
```

## OpenClaw

> **Note:** OpenClaw's skill directory path is hypothetical; check the OpenClaw
> documentation for the actual location.

```bash
cp -r lesstruct-theme-development ~/.openclaw/skills/lesstruct-theme-development
```

## Hermes

> **Note:** Hermes' skill directory path is hypothetical; check the Hermes
> documentation for the actual location.

```bash
cp -r lesstruct-theme-development ~/.hermes/skills/lesstruct-theme-development
```

## Generic agent

Any agent that loads Markdown files with YAML frontmatter from a `skills/`
directory should work. The minimum contract the agent must support:

```yaml
---
name: lesstruct-theme-development
description: <one-paragraph description>
---
```

Plus a `SKILL.md` filename (case-insensitive on most filesystems).

To install for a generic agent whose skill directory you know:

```bash
cp -r lesstruct-theme-development "<agent-config-dir>/skills/lesstruct-theme-development"
```

## Optional: `customize.toml`

This skill includes a `customize.toml` file for BMad agents. Agents that do
not understand BMad's customization format will ignore the file. You can
delete it without affecting the skill's behaviour in non-BMad agents:

```bash
rm lesstruct-theme-development/customize.toml
```

## Verifying the install

After installing, restart your agent and ask it to perform a trivial task
related to Lesstruct themes. For example:

> "List the CSS variables in my Lesstruct default theme."

If the agent responds with the variables from this skill's
[`theme-development.md`](theme-development.md) (Brand colors, Status colors,
Layout, etc.), the skill is loaded correctly. If the agent does not know what
you are talking about, check the agent's skill directory path and that the
`SKILL.md` is in the right place.
