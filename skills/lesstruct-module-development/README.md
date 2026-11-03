# lesstruct-module-development

A skill for AI agents that helps Lesstruct developers write, wire, audit,
and upgrade **Tier 1 Go modules** — first-party features compiled into
the binary (`modules/<name>/`, each its own Go module) against the
`kernel/` contract. It works from your cloned Lesstruct checkout (the
source tree); it does not work from an installed binary's `themes/` or
`plugins/` directories.

**Muse Code is the reference host** (Plan Mode + Wonder/Spark lanes);
the procedure stays portable to other agents. See
[`references/install-paths.md`](references/install-paths.md) for the
Muse-first install.

## Quick start

Install with the open agent-skills CLI — it auto-detects your agent
(Claude Code, OpenCode, Cursor, Codex, Gemini CLI, Copilot, and 25+ others)
and writes the skill to the correct path.
For Muse Code, see the verified manual path in
[`references/install-paths.md`](references/install-paths.md).

```bash
# Install just this skill
npx skills add aristorinjuang/lesstruct --skill lesstruct-module-development

# Or, to install all Lesstruct skills at once
npx skills add aristorinjuang/lesstruct
```

Add `-g` for a global (user-wide) install, or `-a <agent>` to target a
specific one. Restart your agent, then invoke the skill:

> "Help me write a Lesstruct Go module for guest wishlists in my checkout."

On Muse Code, start in Plan Mode: the skill produces a module plan
first (see [`references/module-plan.md`](references/module-plan.md)),
stops for approval, and hands off to Build.

For manual `cp -r` fallbacks and the full per-agent path table, see
[`references/install-paths.md`](references/install-paths.md).

## What it does

Walks you through:

- Triaging the intent (Tier 1 module vs WASM plugin vs theme vs kernel
  change) before touching the tree.
- Acquiring and verifying the checkout (clean tree, `go.work` markers,
  versions derived — never hardcoded).
- A design checkpoint (module portfolio, registry key, `Requires()`
  cascade, contract surface, platform-vs-edition placement).
- Plan / Build / Escalation / Diff Review phase gates with manual
  model-lane checkpoints (frontier for Plan/Review, cost-effective
  for Build).
- Scaffolding from tiered templates (core always; domain/api/store/admin
  overlays) with no `replace` directives.
- Building inward-out (domain → store → api → `module.go` → admin).
- Wiring through a private custom edition (`internal/editions/<custom>/`
  + thin `cmd/<custom>/main.go`) — never `internal/app` by default.
- Gates (`depguard`, 100% domain coverage, conformance fixtures,
  `kernel/httpx` envelope, idempotent per-driver migrations) and
  verification (in-workspace + `GOWORK=off`).
- Auditing an existing module and upgrading across kernel versions.

The full workflow is in `SKILL.md`. The working references are the
adapted module snapshot in
[`references/module-development.md`](references/module-development.md)
and the capability reality matrix in
[`references/kernel-contracts.md`](references/kernel-contracts.md).

## Directory layout

```
lesstruct-module-development/
  SKILL.md                              # The workflow
  customize.toml                        # BMad-specific extension (optional)
  README.md                             # This file
  references/
    module-development.md               # Adapted snapshot of docs/module-development.md
    kernel-contracts.md                 # Capability reality matrix
    module-plan.md                      # Plan artifact template (session state)
    integration-recipes.md              # Custom-edition recipe, go.work/SUBMODULES, GOWORK=off
    troubleshooting.md                  # Dirty tree, version drift, mount failures, cascade
    module-checklist.md                 # Ship + audit checklist
    upgrade-guide.md                    # Kernel-versioning digest
    install-paths.md                    # Per-agent install snippets
    scaffold/                           # Tiered templates with INSTRUCTIONS.txt
      core/                             # go.mod, module.go, README (always)
      domain/                           # types, repository, service, service_test
      api/                              # seams, handler, handler_test (+ agent/ Bearer realm)
      store/                            # single-source store
      admin/                            # manifest, manifest_test
      testutil/                         # NewModule()/Kernel() fixture
      migrations/                       # embed.go + sqlite/postgresql/mysql
```

## Uninstall

```bash
# If installed via the CLI
npx skills remove lesstruct-module-development

# Or remove the directory manually
rm -rf ~/.claude/skills/lesstruct-module-development
```

## Where to get help

- **Lesstruct repository**: [github.com/aristorinjuang/lesstruct](https://github.com/aristorinjuang/lesstruct)
- **Issue tracker**: open an issue in the Lesstruct repo with the
  `module-development` label.
- **Developer-facing docs**: the Lesstruct repo's `docs/module-development.md`
  (the canonical, checkout-authoritative version of the contract this skill uses).
- **The loaded references**: when the skill runs, the working snapshot is
  at `references/module-development.md` and the reality matrix at
  `references/kernel-contracts.md` inside this skill.

## License

This skill is part of the Lesstruct project and is released under the same
license as Lesstruct itself.
