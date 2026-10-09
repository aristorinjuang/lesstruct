# modules/seo

Official Lesstruct module: SEO metadata generation, sitemap and robots
discovery surfaces, and legacy-URL aliases.

## Layout (module template)

```
module.go            # kernel.Module + Migrator (single registry-facing file)
migrations/          # {sqlite,postgresql,mysql}/000001_aliases.{up,down}.sql (embed)
api/                 # sitemap/robots handlers, legacy envelope (3.4d)
domain/              # SEO metadata service (3.4b)
alias/               # alias entities, service, repository interfaces (3.4b)
seo/                 # meta/extraction helpers, Published Language (3.4b)
store/               # single-source alias repository on kernel.Dialect (3.4c)
admin/               # admin-shell manifest (empty: SEO renders data-driven)
testutil/            # kernel fixture for module tests
```

## Migrations

Each module versions independently in its own `schema_migrations_<module>`
table (see `internal/database/modules.go`), so modules install and upgrade
without touching the core sequence. Module migrations must stay idempotent
(`CREATE TABLE IF NOT EXISTS`): pre-module installs already carry tables
extracted from the core sequence. The MySQL alias migration folds its
indexes into the `CREATE TABLE` as `KEY` clauses — MySQL cannot re-apply
separate `CREATE INDEX` statements, so the whole statement must no-op at
once on upgrades.
