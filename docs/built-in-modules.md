# Built-in Modules

Lesstruct v1 is built entirely from first-party Go modules. This page is the
reference for each one: what it owns, what it depends on, its admin-shell
entries, and — the operationally important part — what happens when you
disable it with the `[modules]` block
([configuration](configuration.md#modules)).

Modules reach a binary through its **edition** — a fixed module list compiled
into one binary ([module development](module-development.md#composition-root-wiring)).
The default `lesstruct` binary (the **blog** edition) carries every module in
the table below except the commerce stack and `theme-storefront`; the **Core**
edition (`lesstruct-core`) carries the engine subset only — `users`, `auth`,
`adminpanel`, `posttypes-basic`, and `media` — leaving out `comments`, `seo`,
`site-ssr`, `theme-default`, `textgen`, and `importers`; the **eCommerce**
edition (`lesstruct-ecommerce`) is the blog list plus the commerce stack —
`catalog`, `cart`, `orders`, and `payments` — with `theme-storefront` in place
of `theme-default`. Within any edition, `[modules] disabled` still prunes
further.

For authoring your own module, see the
[Module Development Guide](module-development.md); for the contracts it
compiles against, see [The Kernel](kernel.md).

## The graph at a glance

| Module | Requires | Provides | Admin menu |
|---|---|---|---|
| `users` | — | User management, profiles, profile pictures, account deletion | — |
| `auth` | `users` | Login (password + emailed code), registration, verification, password reset, first-login setup | — |
| `comments` | `users` | Comment system end to end (agent, public, moderation) | Comments |
| `adminpanel` | `auth` | The admin SPA (embedded build) + the frozen extensions manifest API | — |
| `posttypes-basic` | — | Post-type registry, `/roles`, `/user_fields` | — |
| `media` | — | Media library, WebP pipeline, thumbnails, dedup | Media |
| `seo` | — | SEO meta, sitemap, robots, URL aliases | — |
| `site-ssr` | — | The SSR content-site engine, templates, aliases rendering | — |
| `theme-default` | — | The default blog theme (layouts, styles, root files) | — |
| `theme-storefront` | — | The eCommerce storefront theme (layouts, styles, cart/checkout/tracking pages) | — |
| `textgen` | — | AI text enhance/translate and HTML generation | — |
| `importers` | — | WordPress (WXR) and Hugo importers | Import |
| `catalog` | — | Commerce catalog (pricing, variants, stock, SKUs) | Catalog panel |
| `cart` | `catalog` | Token-keyed guest carts, live-priced | — |
| `orders` | `cart` | Checkout, order lifecycle, expiry sweeper | Orders |
| `payments` | `orders` | Payment proofs, verification, hosted checkout, webhooks | — |

## Disable semantics

`[modules] disabled = ["<name>"]` drops the module **and everything that
transitively requires it** (so `users` cascades away `auth`, `comments`,
and `adminpanel`). Two special cases worth knowing:

- `comments` disables exactly like the legacy `[comments] enabled = false`
  toggle — routes unmount everywhere.
- `site-ssr` is **stricter** than headless mode: no engine means no SSG
  export either. `[headless]` only suppresses public-surface mounting.

All other disables are lifecycle-level: the module skips init, migrations,
and the admin manifest, while its HTTP endpoints (if any) keep serving from
mounted handler instances — the full scope limits are documented in
[configuration.md](configuration.md#modules).

## The modules

### users

The user lifecycle: admin management (pending approval, suspension, soft
delete with content restore, admin creation with generated passwords),
self-service profile (email verification, password change, custom fields,
data export, account deletion), and profile pictures (upload, WebP
processing, admin per-user management). Leaf of the graph.

### auth

Authentication: two-step credential login (password, then a 6-digit code
emailed to the account's verified address), revocable JWT sessions
(`users.token_version`, bumped on every password change), failed-login
lockout, enumeration-safe self-registration (a duplicate username or email
gets the same 200 as success and the existing holder gets a notice email;
the `[registration]` block adds a custom default role and admin approval),
email verification, password reset, and
first-login bootstrap for default credentials. Requires `users`.

### comments

The comment system across all three realms: agent posting under content,
public per-item lists, and browser moderation (pending queue,
my-comments). Commentator-role gating flows through the same registry.
Requires `users`.

### adminpanel

Serves the single admin SPA — the embedded production build (dev-server
proxy in `DEV_MODE`) — plus the frozen extensions manifest endpoint the
shell consumes. Requires `auth`.

### posttypes-basic

The content-type registry: config-declared post types with field schemas,
served on authenticated `/post_types` and public `/post_types` endpoints,
plus the transitional `GET /roles` and `GET /user_fields` endpoints (these
transfer to the users module in a future release). Adopts the core
capability resolver for route authorization. Leaf.

### media

The media library: upload with WebP transcoding, thumbnail variants,
SHA-256 dedup, perceptual-hash duplicate detection (used by the
importers), and AI image generation. Leaf.

### seo

Discovery and identity: per-page SEO meta, OpenGraph, JSON-LD, the XML and
JSON sitemaps, robots.txt, and the `content_aliases` redirect system.
Leaf.

### site-ssr

The server-rendered content site: the template engine, content-page
assembler, listing/author/tag pages, and the embedder the SSG exporter and
`lesstruct-cli ssg` reuse. Disabling it removes rendering *and* static
export; use `[headless]` if you only want the public site suppressed.
Leaf.

### theme-default

Ships the default blog theme — embedded layouts, base/component stylesheets,
and root files — consumed by the site-ssr engine. Leaf.

### theme-storefront

Ships the official storefront theme (eCommerce edition): embedded layouts,
brand styles, static assets (including `storefront.js`, which hydrates
catalog prices, the cart, checkout, and order tracking from the public JSON
APIs, and `comments.js`, loaded by content posts when comments are enabled),
one extra template func (`lines`, for newline-separated custom fields), and
the three standalone commerce pages — `/cart`, `/checkout`, and
`/orders/track` — declared through `ThemePageProvider`. The commerce edition
registers it in place of `theme-default`, so a commerce binary never mounts
the blog theme; `THEME_DIR` still shadows any page or asset, which is the
supported child-theme path. Leaf.

### textgen

AI text features: enhance, translate, and on-brand HTML generation
(theme-aware prompts, media context). Bring-your-own key via the AI text
settings. Leaf.

### importers

Migrations in: WordPress (WXR — posts, custom fields, featured images via
media download + dedup) and Hugo (frontmatter, static assets, cover
images), both as async jobs with per-item warnings. Leaf.

### catalog

The commerce catalog (eCommerce edition): pricing, variant matrices, stock,
and SKUs for a configured product post type
(`[catalog]` `post_type`/`currency`/`notify_email`; startup fails closed
when the type is unregistered). Prices are integer minor units. A tracked
bucket crossing its threshold downward emails `notify_email` once (unset
disables); restocks and already-below buckets stay quiet. Admin surface
`GET/PUT /api/v1/catalog/products/{contentID}` (admin capability) and a
public payload `GET /api/v1/public/catalog/products/{slug}` (price, sale
price, stock posture, variants with effective prices, currency) for themes
and headless frontends. Requires nothing; the cart module requires it.
Leaf.

### cart

The commerce cart (eCommerce edition): guest-first, token-keyed
server-side carts with live catalog pricing. Token minting, cart reads,
line add (merge on same product and variant, capped at 99 per line),
quantity updates, line removal, and cart deletion — all on the anonymous
public router under `/api/v1/public/cart*`, priced from the catalog at
read time. No admin surface, no config, no capabilities. Requires
`catalog` (disabling catalog cascades the cart off the registry). Leaf.

### orders

The commerce orders (eCommerce edition): guest checkout that claims the
cart with an atomic delete and stores it as a first-class order with
frozen prices, SKUs, and titles — one checkout per token, so parallel
double submits on the same cart can never mint two orders. The lifecycle
(`pending → paid → packed → shipped →
delivered` with cancel/refund/expiry branches; terminal statuses closed)
is admin-driven except `paid`: payment confirmation is the only path in,
through the payments module — the admin transition surface refuses it
outright, so no surface can mint a payment. An admin-set shipping line
joins the frozen subtotal into the payable total while the order is
pending (pending-only writes, pending-guarded at the statement); the
payable total is always subtotal + shipping, derived, never stored twice.
Stock claims at payment
confirmation through the catalog and returns when paid or packed orders
cancel or refund; shipment and delivery end the unwind. Display order
numbers are derived (`ORD-000042`), customer emails ride every move on a
best-effort basis, and a background sweeper expires pendings older than
the `[orders] pending_expiry` window (default 24h, batched with
best-effort customer emails). Invoices stream on demand as PDF
(`GET /api/v1/orders/{orderID}/invoice`): frozen lines, shipping, and a
display-only PPN breakdown at the `[invoice]` rate beside an unchanged
total, with seller identity from the same block. Public surface on the anonymous router:
`POST /api/v1/public/checkout` and `GET /api/v1/public/orders/{orderNumber}`
(three-factor guest tracking: order number + checkout email + the per-order
tracking token minted at checkout and delivered once in the checkout
response and the confirmation email — the store keeps only its hash, so
sequential numbers and knowable emails alone open nothing). Admin surface behind
the admin capability: `GET /api/v1/orders` (keyset cursor pagination with
an optional status filter), `GET /api/v1/orders/{orderID}`,
`POST /api/v1/orders/{orderID}/status`,
`POST /api/v1/orders/{orderID}/shipping`,
`GET /api/v1/orders/{orderID}/invoice`, and
`GET/PUT /api/v1/orders/invoice-settings` (live seller identity + PPN rate;
the write persists through `config.SaveInvoice` and applies without a
restart). Checkout carries an affirmative `consentAccepted` act when the
`[legal]` gate is active (`400 CONSENT_REQUIRED` otherwise), and the
accepted terms/privacy revision is stored on the order. Requires `cart`
(disabling cart cascades orders off the registry; the admin SPA reads the
`orders` config block the commerce edition contributes). Customer PII (email, name, phone, shipping snapshot) is encrypted at rest (AES-256-GCM) at the store boundary through the shared `kernel/pii` cipher, provisioned from `PII_MASTER_KEY` (the commerce edition fails startup closed without a valid key); guest tracking resolves through the `customer_email_idx` blind-index column, and a startup sweep backfills legacy plaintext rows.

### payments

The commerce payments (eCommerce edition): payment records carry the
proof text, the method, the verification status, and the amount snapshot
taken with the submission — append-only history, so a later price edit
can never rewrite a proof. The offered methods adopt from the
`[payments]` block (`manual` first; `midtrans` joins when the
environment carries `MIDTRANS_SERVER_KEY`); unknown values ride through
config and drop at adoption, so gateway entries can pre-list before
their drivers exist. Manual flow: guests submit a transfer reference
against order number + email + tracking token (the guest-tracking gate —
lookups never
enumerate), administrators verify or reject pending proofs behind the
admin capability (`GET /api/v1/payments`, `GET /api/v1/payments/{id}`,
`POST /api/v1/payments/{id}/verify`,
`POST /api/v1/payments/{id}/reject`). The detail read carries the order
context (number, order status, payable total) so a stale proof shows its
amount against the current total, and `?orderId=` narrows the list to one
order's history. Verification confirms the order
through the orders payment-gated path — a stock shortfall leaves the
order pending for fulfillment; rejection keeps it open for a corrected
resubmission. Both decisions email the customer. Midtrans flow: the
module mints Snap hosted sessions
(`POST /api/v1/public/orders/{orderNumber}/payments/session`) whose
expiry mirrors `[orders] pending_expiry`, receives webhook events at
`POST /api/v1/public/payments/midtrans/webhook` (SHA-512 server-key
checksum on the raw wire strings; settlements record self-verifying rows
deduplicated on the transaction reference and confirm the order; refunds
transition it refunded; forged signatures refuse), and records
`midtrans` as the verifier. Method enumeration for themes lives at
`GET /api/v1/public/payments/methods`. Requires `orders` (disabling
orders cascades payments off the registry; orders never imports payments —
they meet only through the composition root's adapters). Proof reference and verifier identity are encrypted at rest (AES-256-GCM) at the store boundary through the shared `kernel/pii` cipher, provisioned from `PII_MASTER_KEY`; gateway webhook dedup resolves through the `reference_idx` blind-index column, and a startup sweep backfills legacy plaintext rows.

---

Every module follows the same template and ships the same gates — build
your own with the [Module Development Guide](module-development.md).
