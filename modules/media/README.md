# modules/media

Official Lesstruct module: the media library — uploads, listing, AI image
generation, and thumbnail variants.

## Layout (module template)

```
module.go            # kernel.Module + Migrator (single registry-facing file)
migrations/          # {sqlite,postgresql,mysql}/000001_media.{up,down}.sql (embed)
api/                 # browser handlers (legacy envelope) + mocks
api/agent/           # Bearer-realm handlers (UPPER_SNAKE envelope) + dto + mocks
domain/              # service, processor, image providers, thumbnail (3.2d)
store/               # single-source repository on kernel.Dialect (3.2c)
admin/               # admin-shell manifest (media library entry)
testutil/            # kernel fixture for module tests
```

## Handlers

Both auth realms are served from this module: `api.MediaHandler` (browser
JWT/CSRF, legacy `error:null` envelope) and `api/agent.MediaHandler`
(Bearer API key, UPPER_SNAKE envelope). Cursor pagination is shared via
`kernel/httpx`. Core mounts the handlers explicitly — the surface spans
three body-limit groups plus the dual-auth dispatch, which a single
`RouteProvider` call cannot express — through structural
`MediaBrowserHandler`/`MediaAgentHandler` interfaces in `routes.go`, so
routes stay module-agnostic and paths never drift.

## Migrations

Each module versions independently in its own `schema_migrations_<module>`
table (see `internal/database/modules.go`), so modules install and upgrade
without touching the core sequence. Module migrations must stay idempotent
(`CREATE TABLE IF NOT EXISTS`): pre-module installs already carry tables
extracted from the core sequence.
