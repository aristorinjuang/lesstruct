# modgen

AI module generator: validated Tier 1 Go module projects (`go.mod` + `module.go` + `README.md` plus domain/api/store/admin overlays) from a natural-language brief, powered by an OpenAI-compatible API. Generated projects pass the module-contract gate (slug grammar, file allowlist, Go parse, `kernel.Module` lifecycle shape, layer import policy, httpx envelope rule, migration hygiene) plus a generated-content scan; failures feed a bounded two-attempt repair loop over the original readable output before the project is refused. Staged projects are inert source under `modules-gen/`: Tier 1 modules link at compile time, so there is no runtime install — the download ships an `INSTALL.md` with the custom-edition wiring and rebuild steps.

- `domain/` — brief/project types, validation (slug grammar, file allowlist, size caps, surface vocabulary), envelope parser, orchestration service with the repair loop, validator.
- `infrastructure/` — OpenAI-compatible client + per-call grounded prompts (distilled module contract, host kernel pin).
- `store/` — filesystem store under `modules-gen/` with `project.json` manifests and stdlib zips.
- `api/` — browser handler: generate, apply, unapply, list, download, delete.
- `admin/` — admin-shell manifest (Module Generator + Modules nav).
