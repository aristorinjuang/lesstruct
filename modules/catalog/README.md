# catalog

The commerce catalog module: native pricing, variant matrices, stock, and
SKUs keyed to a configured product post type. Products are content items —
this module owns the commerce structure and its surfaces.

## Config

```toml
[catalog]
post_type = "product"   # the registered post type carrying products (required)
currency = "IDR"        # display currency for public payloads (required)
```

Init fails closed when the post type is not registered or the currency is
missing — a misconfigured commerce edition never serves an empty catalog
silently.

## Owns

- `catalog_products` / `catalog_variants` tables (module migration,
  `schema_migrations_catalog`); prices are integer minor units.
- `GET/PUT /api/v1/catalog/products/{contentID}` — admin commerce data
  (protected realm; mutations gated by the `catalog.manage` capability).
- `GET /api/v1/public/catalog/products/{slug}` — public commerce payload
  (price, sale price, stock posture, variant matrix with effective prices,
  currency) for a published product.
- `catalog.manage` capability (contributed via `RoleProvider`).

## Seams

- `PostTypeChecker` — registry check (composition root adapts the
  post-type service).
- `ProductSlugResolver` — published-product slug → content ID
  (composition root adapts the content service).
- `kernel.CapabilityResolver` — admin mutation gate.
- `kernel.Logger`.

## Not here (yet)

Variant stock decrements ride the payments phase; storefront rendering rides
the theme-storefront module; customer accounts and vouchers are v2 per the
commerce proposal.
