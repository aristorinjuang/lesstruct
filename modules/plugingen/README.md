# plugingen

AI plugin generator: validated WASM plugin projects (`main.go` + `go.mod` + manifest sibling) from a natural-language brief, powered by an OpenAI-compatible API. Generated projects pass the file-contract gate plus Go parse and host-function policy, plus a generated-content scan; failures feed a bounded two-attempt repair loop over the original readable output before the project is refused.

- `domain/` — brief/project types, validation (slug grammar, allowlist, size caps, hook/capability vocab), envelope parser, orchestration service with the repair loop, validator.
- `infrastructure/` — OpenAI-compatible client + per-call grounded prompts (distilled SDK contract + hook payload example).
- `store/` — filesystem store under `plugins/src` with `project.json` manifests and stdlib zips.
- `api/` — browser handler: generate, list, download, delete, status.
- `admin/` — admin-shell manifest (Plugin Generator + Plugins nav).
