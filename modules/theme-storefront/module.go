// Package themestorefront is the official storefront theme module: the
// embedded layouts, static assets, template funcs, and standalone commerce
// pages (cart, checkout, order tracking) the site-ssr engine consumes. The
// commerce edition registers it in place of the default blog theme.
package themestorefront

import (
	"html/template"
	"io/fs"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/theme-storefront/admin"
)

const (
	// Name is the registry key for this module.
	Name = "theme-storefront"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"

	// pageCart is the cart shell path the engine serves at /cart.
	pageCart = "cart"

	// pageCheckout is the checkout shell path the engine serves at /checkout.
	pageCheckout = "checkout"

	// pageOrderTrack is the guest tracking shell path the engine serves at
	// /orders/track.
	pageOrderTrack = "orders/track"
)

// lines splits a custom field value into trimmed, non-empty lines. It accepts
// both the legacy newline-separated string and the canonical gallery array
// ([]any of map[string]any{"url","alt"}): URL strings are extracted in order,
// blank URLs are dropped, and malformed items are skipped. Non-strings and
// non-gallery values yield no lines.
func lines(value any) []string {
	switch v := value.(type) {
	case string:
		out := make([]string, 0, strings.Count(v, "\n")+1)
		for line := range strings.SplitSeq(v, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				out = append(out, line)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, line := range v {
			if line = strings.TrimSpace(line); line != "" {
				out = append(out, line)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch entry := item.(type) {
			case string:
				if trimmed := strings.TrimSpace(entry); trimmed != "" {
					out = append(out, trimmed)
				}
			case map[string]any:
				rawURL, ok := entry["url"]
				if !ok {
					continue
				}
				urlStr, ok := rawURL.(string)
				if !ok {
					continue
				}
				if trimmed := strings.TrimSpace(urlStr); trimmed != "" {
					out = append(out, trimmed)
				}
			case map[string]string:
				if trimmed := strings.TrimSpace(entry["url"]); trimmed != "" {
					out = append(out, trimmed)
				}
			default:
				continue
			}
		}
		return out
	default:
		return nil
	}
}

// Module is the theme-storefront kernel module.
type Module struct{}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. The theme is a leaf artifact consumed by
// the site-ssr engine, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. None.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. There is nothing to wire beyond the embedded files.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// ThemeFiles returns the storefront artifact filesystem (layouts under
// pages/, static assets under static/). The site-ssr engine resolves
// THEME_DIR overrides over it.
func (m *Module) ThemeFiles() fs.FS {
	return Files()
}

// ThemeFuncs returns the template funcs this theme contributes beyond the
// engine's own. lines() turns a gallery field — legacy newline-separated or
// canonical gallery array — into a URL list without pushing parsing into
// templates.
func (m *Module) ThemeFuncs() template.FuncMap {
	return template.FuncMap{
		"lines": lines,
	}
}

// ThemePages declares the standalone commerce shells this theme ships. The
// engine renders them through the shared layout at the matching public paths
// and falls through to content/404 when a template is missing.
func (m *Module) ThemePages() []string {
	return []string{pageCart, pageCheckout, pageOrderTrack}
}

// AdminManifest declares this module's admin-shell extensions. A theme ships
// no admin UI of its own, so the manifest is empty.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// New builds the module. The theme artifact is static; construction takes no
// options.
func New() *Module {
	return &Module{}
}
