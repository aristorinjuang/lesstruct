# Install Paths — Muse Code first

**Muse Code is the reference host for this skill.** The Plan / Build /
Escalation / Diff Review gates in `SKILL.md` map directly onto Muse's
Plan Mode and model lanes; the procedure stays portable to other
agents that lack them.

> **Recommended.** Use the open agent-skills CLI — auto-detect your agent(s):

```bash
# Install just this skill (interactive — pick your agent(s))
npx skills add aristorinjuang/lesstruct --skill lesstruct-module-development
```

For Muse Code specifically, or if the CLI does not list it, install manually:

```bash
# Global scope (verified path — bundled and user skills live here)
cp -r lesstruct-module-development ~/.local/share/muse/skills/lesstruct-module-development
```

> Project scope: place the skill directory in your Muse project's
> skills location per your setup. Either scope, run the check below —
> if the agent answers correctly, it is registered.

Model lanes (switch manually at phase boundaries — the skill never
assumes the host can switch models itself):

| Phase | Lane | Suggested model |
|---|---|---|
| Plan (Steps 1–3) | Frontier | Muse Wonder (`muse-wonder-1.0`) |
| Build (Steps 4–7) | Cost-effective | Muse Spark (`muse-spark-1.3`) |
| Escalation | Frontier | Muse Wonder, or the human |
| Verify (Step 8) | Build lane continues | Same model as Build |
| Diff Review (Step 9) | Frontier, fresh session | Muse Wonder |
| Audit / upgrade (Step 10) | Frontier | Muse Wonder, or the build model for mechanical re-verification |

## Other agents

The same CLI covers **30+ agents** — Claude Code, OpenCode, Cursor, Codex, Gemini
CLI, GitHub Copilot, Cline, Roo Code, Windsurf, Goose, Amp, and more. See the
[agent skills CLI](https://github.com/antfu/skills-cli#supported-agents) for
the full, up-to-date list.

```bash
# Non-interactive examples
npx skills add aristorinjuang/lesstruct --skill lesstruct-module-development -a claude-code -y   # project scope
npx skills add aristorinjuang/lesstruct --skill lesstruct-module-development -g -a opencode -y   # global scope
```

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
cp -r lesstruct-module-development ~/.claude/skills/lesstruct-module-development

# Example — OpenCode, project scope
cp -r lesstruct-module-development .opencode/skills/lesstruct-module-development
```

For any other agent that loads Markdown files with YAML frontmatter from a
`skills/` directory, the minimum contract the agent must support is:

```yaml
---
name: lesstruct-module-development
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
rm lesstruct-module-development/customize.toml
```

## Verifying the install

After installing, restart your agent and ask it to perform a trivial
task related to Lesstruct modules. For example:

> "What is the difference between a Tier 1 module and a Tier 2 plugin, and where does a private module get wired?"

If the agent answers Tier 1 = Go module compiled into the binary
(`modules/<name>/`) vs Tier 2 = WASM plugin dropped into `plugins/`,
and private modules wire via a custom edition in
`internal/editions/<custom>/` (never `internal/app` by default), the
skill is loaded correctly. If the agent does not know what you are
talking about, check the agent's skill directory path and that the
`SKILL.md` is in the right place.
