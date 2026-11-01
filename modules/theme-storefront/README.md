# modules/theme-storefront

Official Lesstruct module: the storefront theme artifact — embedded layouts,
static assets, template funcs, and the standalone commerce pages (cart,
checkout, order tracking) consumed by the site-ssr engine. The eCommerce
edition registers it in place of `theme-default`.

## Layout (module template)

```
module.go            # kernel.Module + ThemeProvider + ThemePageProvider (single registry-facing file)
files.go             # embedded pages/ + static/ artifact + Files() accessor
pages/               # embedded *.gohtml layouts, including the commerce shells
static/              # embedded CSS/JS assets (storefront.js, minified from storefront.src.js via `make js`, and base.css/style.css, minified from their `.src.css` sources via `make css`; together they style and hydrate the commerce pages)
admin/               # admin-shell manifest (empty: themes ship no admin UI)
testutil/            # kernel fixture for module tests
```

## Files

The artifact mirrors the theme-default layout: `pages/<name>.gohtml` and
`static/<name>` are resolved by the engine with `THEME_DIR` overrides layered
on top, so a child theme can shadow any page or asset by name.

The commerce pages are declared through `ThemePages()` — `cart`, `checkout`,
and `orders/track` — and render at `/cart`, `/checkout`, and `/orders/track`
through the shared layout with `StandalonePageData` (every layout field plus
the page key). `storefront.js` hydrates them client-side from the public
catalog, cart, checkout, order, and payment APIs; prices format with zero
decimals for IDR/JPY/KRW/VND and two decimals elsewhere. The readable source
is `static/storefront.src.js` — edit it, then run `make js` to regenerate the
minified `static/storefront.js` browsers receive.

Content posts keep the shared comment section: `post.gohtml` renders existing
comments and, when `AllowComments` is true, loads `comments.js` page-scoped.
The public search box, KaTeX math, and highlight.js syntax highlighting stay
default-theme-only.

`ThemeFuncs()` contributes `lines`, which splits a newline-separated
custom-field value into a trimmed list (used for gallery-style fields).
