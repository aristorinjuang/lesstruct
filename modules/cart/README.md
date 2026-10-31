# cart

The commerce cart module: guest-first, server-side carts with live catalog
pricing. Carts are keyed by opaque bearer tokens; only the token hash is
stored. Prices are never persisted — totals resolve live from the catalog
at read time, so price edits apply immediately.

## Owns

- `cart_carts` / `cart_items` tables (module migration,
  `schema_migrations_cart`); variant 0 means the base-product line so the
  line uniqueness constraint behaves identically on every driver.
- `POST /api/v1/public/cart` — mints a token (response body and
  `X-Cart-Token` header) with an empty cart.
- `GET /api/v1/public/cart` — the live-priced cart: lines, subtotal,
  item count, currency.
- `DELETE /api/v1/public/cart` — empties and removes the cart.
- `POST /api/v1/public/cart/items` — resolves the sellable line and merges
  it (same product and variant add up, capped at 99 per line); out-of-stock
  demand is refused.
- `PATCH /api/v1/public/cart/items/{itemId}` — replaces a line quantity.
- `DELETE /api/v1/public/cart/items/{itemId}` — drops one line.

Tokens ride the `X-Cart-Token` header, falling back to a body `token`
field on bodyless requests. Unknown or missing tokens read as
unauthorized; unknown products and variants as not found.

## Seams

- `CatalogSource` — sellable-line resolution (composition root adapts the
  catalog service; unknown lookups wrap the cart sentinels).
- `CartStore` — persistence (the single-source store satisfies it).
- Display currency (adopted from the catalog configuration).

## Not here (yet)

The checkout handoff lands with the orders module; tax, shipping, and
vouchers stay out of totals until the pricing phase. No admin surface,
no config block, no capabilities — the surface is fully anonymous.
