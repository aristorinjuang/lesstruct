# Ubiquitous Language

> **Audience.** Anyone naming a type, a method, a config key, an API field, or
> a test case. Use the term in the **Term** column everywhere — code,
> comments, commits, docs, and UI copy. If a concept is missing, add it here
> before inventing a synonym.
>
> Ownership: the context that defines a term owns its meaning. A term used by
> two contexts (e.g. *product*) is linked to the ID that crosses the boundary,
> not shared as a mutable model.

## Content Publishing

| Term | Meaning | Code name |
|---|---|---|
| Content item | One authored document in one language. The aggregate root of publishing. | `contentdomain.Content` |
| Post type | A content category with its own field schema (`post`, `page`, custom slugs). | `PostType` / `post_type` |
| Custom field | Author-editable data declared by a post type. | `customfield.FieldSchema` |
| System field | A field owned by a plugin or module, hidden from authors and protected from edits. | `FieldSchema.System` |
| Field schema | The declaration (type, required, bounds, options) that validates a field value. | `customfield.FieldSchema` |
| Custom-field value | The stored value, canonicalised to its declared type. | `Content.CustomFields` |
| Format | The body representation: `tiptap` (canonical JSON), `html`, or `markdown` at the import edge. | `contentdomain.Format` |
| Status | Editorial state: `draft` or `published`. | `contentdomain.Status` |
| Slug | The public URL segment, unique per language; editable while draft, immutable once published. | `Content.Slug` |
| Translation group | The identifier tying language variants of one logical item. | `TranslationGroupID` |
| Alias | A legacy URL that redirects to a content item. | `alias.Alias` |
| Tag | A free-form label used for related-content reads and public filtering. | `Content.Tags` |
| Featured image | The explicitly chosen cover image (http(s) or root-relative URL); falls back to body extraction. | `Content.FeaturedImage` |
| Comment | A visitor response on a content item, gated by `allowComments` and the comment system toggle. | `comments.Comment` |
| Plain text | The extracted, whitespace-collapsed body text used for SEO descriptions. | `seo.ExtractPlainText` |
| SEO metadata | Derived title/description/OG/canonical data produced at publish time or on demand. | `seo.GeneratedMetadata` |
| Import job | An asynchronous WordPress/Hugo migration with warnings and a terminal status. | importer job store |

## Identity & Access

| Term | Meaning | Code name |
|---|---|---|
| Principal | The acting identity attached to a request: ID, username, roles. | `kernel.Principal` |
| Role | A named capability set loaded from `config.toml` at startup (`Admin`, `Contributor`, `Commentator`, or custom). | `role.Role` |
| Capability | A permission string such as `content.publish` or `catalog.manage`. | `kernel/capability.Capability` |
| Primary role | The role used for coarse checks when a principal carries several. | `identity.PrimaryRole` |
| API key | A machine credential (`lesstruct_<keyID>_<secret>`) whose secret is stored hashed. | `apikey.APIKey` |
| MFA challenge | A short-lived, single-use login challenge holding hashed token and code. | `auth.MFAChallenge` |
| Verification token | A hashed, expiring token for email verification or password reset. | `auth.VerificationToken` |
| First login | The seeded admin's bootstrap state before a real password and email exist. | `firstLoginRequired` |
| Login identifier | The value a user signs in with: a username, or an email address when it contains `@`. | `auth.IsEmailIdentifier` |
| Pending approval | A registrant who verified email but awaits admin activation (`admin_approval = true`). | `users` status `pending` |

## Media Library

| Term | Meaning | Code name |
|---|---|---|
| Media file | One stored image plus metadata and its thumbnail set. The aggregate root. | `media.Media` |
| Variant / thumbnail | A resized render of the original under a thumbnail spec suffix. | `media.MediaVariant`, `thumbnail.Spec` |
| Thumbnail spec | A configured suffix + max width pair. | `thumbnail.ThumbnailConfig` |
| Content hash | SHA-256 of the original bytes; drives dedup and filenames. | `Media.Hash` |
| Dedup | Returning the existing record instead of storing a duplicate (unless forced). | `ErrDuplicateMedia` |
| Perceptual hash | 64-bit average hash used by importers to spot visually duplicate covers. | `media.PerceptualHash` |
| WebP conversion | Transcoding uploads to WebP (quality 80) with metadata stripped. | `ConvertToWebP` |
| Pixel budget | The 60-megapixel cap probed before decoding, guarding decompression bombs. | `maxDecodedPixels` |
| Image generation | AI-produced imagery through the provider port. | `ImageGenerationService` |
| Privacy redaction | SAM 3.1 scan that blurs faces/plates/nudity before anything is stored; fail-closed on scan error. | `privacy.Service`, `ErrPrivacyScanFailed` |
| Detection | One sensitive region in pixel coordinates for a concept. | `privacy.Detection` |
| Segmenter | Port that finds detections, one Responses-API call per concept with boxes extracted. | `privacy.Segmenter` |

## AI Assistance

| Term | Meaning | Code name |
|---|---|---|
| Enhancement | Rewriting a body into sharper copy plus SEO title/meta, same language. | `EnhanceText` / `enhanceContent` |
| Translation | Rendering body, title, and meta into another language, preserving structure. | `TranslateText` / `translateContent` |
| Dictation | Recording speech to author content instead of typing. | `speech` module / "Dictate with AI" |
| Transcription | The raw text (plus detected turns) a speech provider returns for audio. | `speechdomain.Transcription` |
| Speech turn | One detected speech segment with millisecond boundaries and an optional speaker label. | `speechdomain.Turn` |

## Catalog

| Term | Meaning | Code name |
|---|---|---|
| Catalog product | Commerce data attached to one product content item. | `catalog.Product` |
| Variant | One concrete option combination with its own SKU, price override, stock, weight, image. | `catalog.Variant` |
| Variant axis | An option dimension (`size`, `color`) and its allowed values. | `Product.Options` |
| Variant matrix | The cross-product of axes, capped by `MaxVariants`. | `Product.Variants` |
| Price | Regular sellable price in integer minor units. | `Product.Price` |
| Sale price | Optional discounted price; must not exceed `Price`. | `Product.SalePrice` |
| Effective price | Variant override, else sale price, else price. | `Product.EffectivePrice` |
| Stock bucket | The counted unit: product stock when `variantID == nil`, else variant stock. | `Product.Stock`, `Variant.Stock` |
| Track inventory | Whether stock gates sellability and moves on payment. | `Product.TrackInventory` |
| Low-stock threshold | The level at which a downward move emails the configured recipient. | `Product.LowStockThreshold` |
| Adjustment | A signed stock delta applied conditionally (never below zero). | `Service.AdjustStock` |
| SKU | Stock-keeping unit, unique per product when non-empty, ≤100 runes. | `Product.SKU`, `Variant.SKU` |

## Cart

| Term | Meaning | Code name |
|---|---|---|
| Cart | A guest-owned basket identified by an opaque token. The aggregate root. | `cart.Cart` / `StoredCart` |
| Cart token | A 32-byte base64url secret shown once; only its SHA-256 hash is stored. | `TokenBytes`, `hashToken` |
| Stored line | The persisted `(product, variant) → quantity` row with no price. | `StoredLine` |
| Cart line | The read shape: stored quantity plus live catalog projection. | `CartLine` |
| Catalog item | The narrowed catalog projection the cart needs (price, stock, identity). | `CatalogItem` |
| Available | Whether the line's product/variant still resolves; unavailable lines stay visible but are excluded from totals. | `CartLine.Available` |
| Soft stock gate | New demand for an out-of-stock line is refused, but an existing quantity may exceed stock; checkout requires every line to still be in stock, and payment claims the exact quantities from the catalog. | `ErrOutOfStock` |
| Live pricing | Prices are never persisted; totals resolve from the catalog at read time. | `Cart.Subtotal` |

## Orders

| Term | Meaning | Code name |
|---|---|---|
| Order | The checkout snapshot plus lifecycle state. The aggregate root. | `orders.Order` |
| Order item | An immutable snapshot line: product ID, optional variant ID, SKU, title, unit price, quantity. | `OrderItem` |
| Snapshot | The frozen copy of cart facts at checkout; later catalog edits never rewrite it. | `Checkout` |
| Order number | The derived display ID `ORD-000042`; never stored. | `Order.DisplayNumber` |
| Subtotal | Sum of item line totals, frozen at checkout (minor units). | `Order.Subtotal` |
| Shipping line | Admin-entered shipping charge, pending orders only, empty by default. | `Order.ShippingMinor` |
| Total | `Subtotal + ShippingMinor`, derived at read. | `Order.TotalMinor` |
| Checkout | The use case that validates contact/shipping, claims the cart, and stores the pending order. | `Service.Checkout` |
| Claim / consume | The atomic delete of the cart that makes one checkout win a race on a token. | `CartSource.Consume` |
| Tracking | Guest lookup by order number + email + the per-order tracking token, with non-enumerating failures — every mismatch reads not-found. | `Service.Tracking` |
| Transition | A legal status move validated by the lifecycle map. | `Service.Transition` |
| Lifecycle | `pending → paid → packed → shipped → delivered`, with cancel/refund/expiry branches. | `lifecycle` |
| Confirm paid | The only path into `paid`: reserve stock, then conditionally flip pending→paid. | `Service.ConfirmPaid` |
| Reserve / release | Inventory claims taken on payment and returned on failed flips or fulfilment unwind. | `Inventory` |
| Expire stale | Flip abandoned pending orders to `expired` and notify (best-effort). | `Service.ExpireStale` |
| Invoice | A derived, never-persisted render document (`INV-000042`). | `InvoiceDocument` |
| PPN | Indonesian VAT, a display-only invoice breakdown at the configured rate. | `InvoiceSettings.TaxRate` |
| Invoice settings | The live `[invoice]` block (seller identity + PPN rate): module state behind a mutex, persisted through `config.SaveInvoice` so an admin edit applies without a restart. | `Module.InvoiceSettings` |
| Order mailer | Best-effort confirmation/status email delivery. | `Mailer` |

## Legal & consent

| Term | Meaning | Code name |
|---|---|---|
| Legal document | A conventional store policy page (cookie, privacy, terms, returns) published as ordinary bilingual `page` content. | `legal.Page` |
| Conventional slug | The canonical English slug that anchors a document's translation group (`cookie-policy`, `privacy-policy`, `terms-of-service`, `returns-refunds`). | `legal.PageCookiePolicy` … |
| Legal resolution | Mapping each document to the published translation-group member that best fits the visitor's language chain; absent when the canonical slug is unpublished. | `legal.Resolve` |
| Override | An explicit `[legal] *_url` value that wins over the resolved page (validated site-relative or http(s)). | `LegalOptions.Overrides` |
| Consent banner | The Accept/Decline cookie notice; links a resolved cookie (or privacy) policy and degrades to the plain notice when none resolves. | `LegalData.CookieNotice` |
| Consent gate | The server-enforced requirement that checkout/registration carry an affirmative acceptance when terms and privacy resolve (or `[legal]` forces it). | `ConsentPolicy` |
| Consent evidence | The recorded acceptance: timestamp plus the accepted revision of each document (page ID, slug, language, last-updated). | `legal.Consent` |
| Legal placement | Where conventional legal pages appear in navigation: footer (default), header, or both; the footer always lists the full site map. | `legal.NavPlacement` |

## Payments

| Term | Meaning | Code name |
|---|---|---|
| Payment | One submitted proof or gateway settlement, append-only. The aggregate root. | `payments.Payment` |
| Method | How the payment was made: `manual` or `midtrans`. | `Method` |
| Payment status | Verification state: `pending`, `verified`, or `rejected`. | `Status` |
| Reference | The verbatim proof text (transfer reference or transaction ID), ≤500 runes. | `Payment.Reference` |
| Proof | A manual submission quoting the order number, email, tracking token, and a reference. | `Service.Submit` |
| Verify / reject | Pending-only admin decisions on pending orders (a gateway settlement that landed first refuses both); rejection keeps the order open for resubmission. | `Service.Verify`, `Service.Reject` |
| Order ref | The payments-side read view of an order: ID, number, status, payable total, currency. | `OrderRef` |
| Gateway session | A minted hosted-checkout token and redirect URL. | `GatewaySession` |
| Notification | A verified gateway webhook event folded onto the order lifecycle. | `Notification` |
| Settlement | A gateway event that confirms payment and calls orders' `Confirm`. | `settleNotification` |
| Refund | A gateway event that moves the order to refunded. | `refundNotification` |

## Platform & delivery

| Term | Meaning | Code name |
|---|---|---|
| Module | A compiled-in, first-party bounded context (`modules/<name>/`, own `go.mod`). | `kernel.Module` |
| Edition | A fixed module list compiled into one binary (`blog`, `core`, `commerce`). Internal vocabulary; product docs say "server build". | `internal/editions/<name>` |
| Kernel | The frozen shared contract module (`kernel/vX.Y.Z`). | `kernel/` |
| Seam | A narrow interface a module declares for a collaborator it does not own. | e.g. `CartSource` |
| Adapter | An implementation of a seam, usually in an edition or `store/`. | e.g. `internal/editions/commerce` |
| Principal bridge | Convert a `kernel.Principal` into an integer acting-user ID. | `identity.IntID` |
| Plugin | A sandboxed WASM extension; never a Go module. | `internal/domain/plugin` |
| Hook | A plugin event (`before_save`, `after_publish`, …). | `plugin.HookName` |
| Capability resolver | Fail-closed mapping from principal to capability set. | `kernel.CapabilityResolver` |
| Migrator | A module's versioned schema migrations. | `kernel.Migrator` |
| Site identity | The operator-configured name, logo, and favicon shared by every public page and passed to themes as `SiteConfig`. | `config.SiteConfig` |

## Naming rules

- Prefer the domain noun over its transport spelling: *order*, not *order
  record*; *payment proof*, not *payment transaction row*.
- "Minor units" always means the smallest denomination of the currency
  (IDR has none; USD cents). Never say *cents* for a non-USD amount.
- `paid` is a state and `ConfirmPaid` is the only action that produces it.
  Do not name an admin endpoint *pay order*.
- *Consume* is the cart claim; *delete* is a plain removal. They are not
  interchangeable.
- A *variant* belongs to a catalog product; a *media variant* belongs to a
  media file. Keep the qualifier when the context is not obvious.
- Use *reviewer* for the admin deciding a payment, *customer* for the payer,
  and *buyer* nowhere.
