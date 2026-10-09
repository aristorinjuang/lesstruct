# modules/comments

Official Lesstruct module: the comment system — per-content discussion with
moderation (pending/approved/rejected/spam).

## Layout (module template)

```
module.go            # kernel.Module + Migrator (single registry-facing file)
migrations/          # {sqlite,postgresql,mysql}/000001_comments.{up,down}.sql (embed)
api/                 # browser handlers (legacy envelope) + mocks
api/agent/           # Bearer-realm handlers (UPPER_SNAKE envelope) + dto + mocks
domain/              # service, entities, validation, repository interfaces (3.3b)
domain/mocks/        # CommentRepository testify mock (moved with the domain)
store/               # single-source repository on kernel.Dialect (3.3c)
admin/               # admin-shell manifest (comments moderation entry)
testutil/            # kernel fixture for module tests
```

## Handlers

Both auth realms are served from this module: `api.CommentHandler` (browser
JWT/CSRF, legacy `error:null` envelope) and `api/agent.CommentHandler`
(Bearer API key, UPPER_SNAKE envelope). Content lookups cross the module
boundary through narrow func seams (`ContentResolver` for slug resolution,
`ContentLookup` for ID resolution) that project only comment-relevant facts.
Core mounts the handlers explicitly — the surface spans four middleware
groups plus the agent realm, which a single `RegisterRoutes` call cannot
express — through structural `CommentBrowserHandler`/`CommentAgentHandler`
interfaces in `routes.go`, so routes stay module-agnostic and paths never
drift.

## Migrations

Each module versions independently in its own `schema_migrations_<module>`
table (see `internal/database/modules.go`), so modules install and upgrade
without touching the core sequence. Module migrations must stay idempotent
(`CREATE TABLE IF NOT EXISTS`): pre-module installs already carry tables
extracted from the core sequence.
