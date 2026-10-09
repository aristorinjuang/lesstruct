# modules/posttypes-basic

Official Lesstruct module: the `post` + `page` content types and the
post-type registry host.

## Layout (module template)

```
module.go            # kernel.Module + capabilities (single registry-facing file)
api/                 # HTTP handlers + legacy envelope (delivery layer)
domain/              # registry service, entities, validation (moved from internal/domain/posttype)
admin/               # admin-shell manifest (empty: the UI is data-driven)
testutil/            # kernel fixture for module tests
```

Deliberately absent: `store/` and `migrations/` — the registry is
in-memory, so there is nothing to persist or migrate. If post types ever
grow stored state, those directories appear here following the template.

## Capabilities

- `ContentTypeProvider` — declares `post` + `page` (protected) plus custom
  types (unprotected). The `media`/`comment` placeholders stay out: the
  media and comments modules claim those slugs when they land.
- `RouteProvider` — `GET /post_types`, `GET /roles`, `GET /user_fields`
  (admin-gated via the injected resolver).
- `PublicRouteProvider` — `GET /post_types` (anonymous).
- `AdminExtensionProvider` — empty manifest (the admin UI reads the API).

## Transfers

`GET /roles` and `GET /user_fields` are served here only until the users
module lands, which takes them over unchanged.
