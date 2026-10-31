package kernel

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Module is the only interface every Lesstruct module must implement.
//
// Name is the stable registry key ("users", "comments", "site-ssr").
// Requires lists hard dependencies resolved topologically at startup;
// Optional lists soft dependencies the module degrades without.
// Init wires the module (routes, subscriptions, services); Shutdown releases it.
type Module interface {
	Name() string
	Version() string
	Requires() []string
	Optional() []string
	Init(kernel Kernel) error
	Shutdown() error
}

// Kernel is the minimal host surface a module receives at Init.
type Kernel interface {
	Logger() Logger
	Registry() *Registry
}

// RouteProvider mounts HTTP routes onto the shared Chi router. Core passes
// the protected router (auth middleware pre-chained), so everything mounted
// here requires authentication. For anonymous surfaces, implement
// PublicRouteProvider instead.
type RouteProvider interface {
	Module
	RegisterRoutes(router chi.Router)
}

// PublicRouteProvider mounts HTTP routes onto the shared Chi router. Core
// passes the public router (no auth chain), so everything mounted here is
// anonymous. Prefer RouteProvider unless the endpoint is genuinely public.
//
// The method is deliberately named differently from RouteProvider's so one
// module can serve both surfaces and core can discover each independently:
// a single RegisterRoutes method would satisfy both interfaces at once and
// mount the same routes twice.
type PublicRouteProvider interface {
	Module
	RegisterPublicRoutes(router chi.Router)
}

// DualAuthRegistrar is the narrow route sink core passes to dual-realm
// modules: only the flat verb registrations a shared admin path needs.
// Core mounts every registration twice — once on the agent stack (Bearer
// API key auth plus per-key rate limits) and once on the browser stack
// (CSRF plus JWT auth) — dispatched by credential at the shared path,
// exactly like the media co-owned paths. Chi routes by path, not auth
// realm, so registering the same path twice would shadow instead of
// dispatch; going through this sink keeps the two chains in one place.
type DualAuthRegistrar interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
}

// DualAuthProvider is implemented by modules whose admin surface both
// realms must serve with the same handler methods — typically commerce
// surfaces an AI agent operates with an API key while the admin SPA drives
// them with a JWT cookie. Patterns must be full /api/v1/... paths (the
// mount point carries no sub-router prefix) and must not collide with any
// other registration. Handlers keep their in-handler capability checks,
// which resolve API-key principals exactly like JWT ones — a non-admin key
// answers 403 on both realms with no extra middleware.
type DualAuthProvider interface {
	Module
	RegisterDualRoutes(r DualAuthRegistrar)
}

// Migrator ships namespaced migrations as an embedded filesystem.
//
// Layout convention: {sqlite,postgresql,mysql}/NNN_name.{up,down}.sql.
// The core migration runner selects the subdir via Driver.MigrationDir.
type Migrator interface {
	Module
	Migrations() fs.FS
}

// PostTypeDef is the minimal post-type registration the kernel understands.
// Richer definitions (fields, templates) live in the posttypes module.
// Protected marks slugs reserved by built-in modules (post, page, media,
// comment): the kernel index records them so later phases can refuse
// overrides, and config entries may not hide them.
type PostTypeDef struct {
	Slug      string
	Name      string
	Protected bool
}

// ContentTypeProvider registers content types (post, page, product, ...).
type ContentTypeProvider interface {
	Module
	PostTypes() []PostTypeDef
}

// AdminMenuEntry is one navigation item contributed to the admin shell.
type AdminMenuEntry struct {
	Label string
	Path  string
	Icon  string
}

// AdminExtensionManifest is the declarative UI a feature module contributes to
// the admin shell: navigation, settings sections, and editor extensions.
// One shell renders everything; modules never ship their own mini-SPA.
type AdminExtensionManifest struct {
	MenuEntries      []AdminMenuEntry
	SettingsSections []string
	EditorExtensions []string
}

// Validate ensures a manifest's entries are well-formed: menu entries need a
// label, an absolute path, and no duplicate paths. It mirrors the shell's
// own checks so module authors catch malformed manifests in unit tests (see
// kernel/conformance CheckAdminExtension for the third-party fixture).
func (m AdminExtensionManifest) Validate() error {
	seen := make(map[string]bool, len(m.MenuEntries))
	for i, entry := range m.MenuEntries {
		if entry.Label == "" {
			return fmt.Errorf("admin: menu entry %d has no label", i)
		}
		if entry.Path == "" {
			return fmt.Errorf("admin: menu entry %q has no path", entry.Label)
		}
		if !strings.HasPrefix(entry.Path, "/") {
			return fmt.Errorf("admin: menu entry %q path must be absolute", entry.Label)
		}
		if seen[entry.Path] {
			return fmt.Errorf("admin: menu entry %q duplicates path %q", entry.Label, entry.Path)
		}
		seen[entry.Path] = true
	}
	return nil
}

// AdminExtensionProvider contributes UI to the admin shell via manifest.
type AdminExtensionProvider interface {
	Module
	AdminManifest() AdminExtensionManifest
}

// ThemeProvider registers a base theme consumed by the site-ssr engine:
// embedded layouts and CSS, template funcs, and site-root files.
type ThemeProvider interface {
	Module
	ThemeFiles() fs.FS
	ThemeFuncs() template.FuncMap
}

// ThemePageProvider is implemented by themes that ship standalone pages the
// site-ssr engine serves at fixed public paths. Keys are slash-separated
// paths without a leading slash ("cart", "orders/track"); each maps to the
// embedded page file named after the key with slashes replaced by hyphens
// ("cart.gohtml", "orders-track.gohtml"). The optional interface keeps
// existing themes source-compatible.
type ThemePageProvider interface {
	ThemePages() []string
}

// RoleProvider contributes capability strings to the permission registry.
type RoleProvider interface {
	Module
	Capabilities() []string
}
