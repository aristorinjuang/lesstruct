# modules/adminpanel

Official Lesstruct module: the admin SPA shell served at `/admin`, the
frozen admin API surface, and the manifest endpoint aggregating every
feature module's admin extensions.

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
api/                 # manifest endpoint (legacy envelope, versioned JSON)
spa/                 # embedded Vue build (dist/) + ServeAdmin + dev proxy
admin/               # admin-shell manifest (empty: the shell renders others)
testutil/            # kernel fixture for module tests
```

## Manifest endpoint

`GET /api/admin/extensions` (admin-only) aggregates every registered
`kernel.AdminExtensionProvider` in registry name order into versioned JSON:

```json
{"version": 1, "modules": [{"name": "media", "menuEntries":
[{"label": "Media", "path": "/media", "icon": "photo"}],
"settingsSections": [], "editorExtensions": []}]}
```

Empty collections serialize as `[]`, never `null`. The `version` field lets
a future shell negotiate: unknown major versions must not render. Any
manifest change breaks the conformance pin — that is the freeze contract.
