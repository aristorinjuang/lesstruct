# modules/importers

Official Lesstruct module: content migration — the WordPress WXR importer
and the Hugo archive importer, each behind an async admin handler over a
shared in-memory job store.

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
api/                 # browser + agent handlers (legacy envelope, 202 job bodies)
wordpress/           # WXR parser, converter, importer, media downloader (3.7b)
hugo/                # archive importer, media mapper, parser, shortcodes (3.7c)
admin/               # admin-shell manifest (Import section)
testutil/            # kernel fixture for module tests
```

## Handlers

Both importers serve one handler each, mounted twice: the browser admin
group (`/wordpress/import`, `/hugo/import`) and the agent realm
(`/api/v1/wordpress/import`, `/api/v1/hugo/import`). The two realms share
the same handler instance — and the same in-memory job store — so a
CLI-started job is visible in the admin UI, exactly as before. Import POSTs
carry no route-level body limit; each handler applies its own
`MaxBytesReader` at `IMPORT_MAX_SIZE_MB`.

The anti-corruption direction holds: external WordPress/Hugo shapes never
leak inward — the importers project everything onto `kernel/content`
requests through narrow seams (`ContentCreator`, `AliasCreator`,
`SlugResolver`, `MediaService`, `UserProvisioner`), and platform-owned
account policy stays behind the core-side provisioner adapter. Core mounts
the handlers explicitly through structural `WordPressSurface`/`HugoSurface`
interfaces in `routes.go`, so routes stay module-agnostic and paths never
drift.
