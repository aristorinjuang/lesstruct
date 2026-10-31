package app

import (
	"database/sql"
	"io/fs"
	"net/http"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	agent "github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	"github.com/aristorinjuang/lesstruct/internal/api/routes"
	"github.com/aristorinjuang/lesstruct/internal/config"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	role "github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	appstorage "github.com/aristorinjuang/lesstruct/internal/storage"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	aliasdomain "github.com/aristorinjuang/lesstruct/modules/seo/alias"
	seodomain "github.com/aristorinjuang/lesstruct/modules/seo/domain"
)

// Edition is a fixed module list compiled into one binary: a curated slice of
// the first-party modules plus the wiring that mounts their handlers. The
// composition root (Run) builds the shared platform — config, database,
// identity, content engine, media stack, plugin system, boot — and hands the
// edition two hooks: Wire (construct edition services and modules, before the
// disabled-list cascade and InitAll) and Mount (build the handlers that depend
// on post-init state, after InitAll). Editions live in internal/editions/<name>
// as separate packages so a binary links only the modules its edition wires.
//
// Comments declares whether the edition carries the comment system. The flag
// folds into commentsEnabled before the content service is built (the same
// pre-cascade value the default composition always used); the [modules]
// disabled cascade still applies on top at registration time.
type Edition struct {
	Name     string
	Comments bool
	Wire     func(*Core) error
	Mount    func(*Core) error
}

// HandlerSet holds the edition-optional HTTP surfaces. A nil field unmounts
// that surface's routes (routes.Setup nil-checks each one); an edition simply
// leaves the fields it does not carry at their zero values.
type HandlerSet struct {
	SEO            routes.SEOSurface
	CommentBrowser routes.CommentBrowserHandler
	CommentAgent   routes.CommentAgentHandler
	WordPress      routes.WordPressSurface
	Hugo           routes.HugoSurface
	SSG            routes.SSGSurface
	Static         http.Handler
	AliasRedirect  *handlers.AliasRedirectHandler
	TextGen        routes.TextGenSurface

	// ConfigExtras contributes edition-specific keys to the public
	// GET /api/v1/config payload (e.g. the catalog block). Nil or empty
	// leaves the response unchanged.
	ConfigExtras func() map[string]any
}

// Core is the shared platform context an edition wires into: every service,
// repository, flag, and handler the composition root builds for all editions,
// plus the candidate list the edition appends its modules to. Edition-specific
// state (the SSR theme, the comments module, …) stays in the edition package —
// Core only carries types every binary links anyway.
type Core struct {
	Cfg    *config.Config
	Logger *util.Logger

	// Database and module machinery.
	DB       *sql.DB
	Dialect  database.Dialect
	Registry *kernel.Registry

	// Registries and services shared by every edition.
	RoleService     *role.Service
	PostTypeService *posttype.Service
	PostTypeSlugs   []string
	ContentService  *contentdomain.Service
	MediaService    *mediadomain.Service
	MediaRepo       mediadomain.Repository
	AliasService    *aliasdomain.Service
	SEOService      *seodomain.Service
	UserRepo        repository.UserRepo
	ProfilePicture  appstorage.Storage
	EmailService    *email.Service

	// Effective flags. CommentsEnabled and RegistrationEnabled are refreshed
	// to their post-cascade values before Mount runs.
	CommentsEnabled     bool
	RegistrationEnabled bool
	HeadlessEnabled     bool
	Languages           []string
	PrimaryLanguage     string
	BaseURL             string

	// Legal resolution (round 6): the conventional documents the composition
	// root resolved once at boot, the explicit URL overrides, the navigation
	// placement, and the consent gates both the SSR engine and the commerce
	// modules enforce. A zero value keeps the pre-legal behavior. LegalNotice
	// selects the banner wording: the zero value renders the neutral pair
	// and the commerce edition adopts legal.StoreNoticeCopy.
	LegalResolution     legal.Resolution
	LegalOverrides      map[legal.Page]string
	LegalNav            legal.NavPlacement
	CheckoutConsent     bool
	RegistrationConsent bool
	LegalNotice         legal.NoticeCopy

	// PublicFieldRegistry gates /api/v1/public/* custom-field queries; a
	// closed registry fails every cf_*/sort_by request. IFrameHosts mirrors
	// the CSP frame-src allowlist for the HTML sanitizer.
	PublicFieldRegistry *config.PublicFieldRegistry
	IFrameHosts         []string

	// Platform handlers every edition serves.
	ContentHandler      *handlers.ContentHandler
	AgentContentHandler *agent.ContentHandler

	// Authorize gates capability-protected routes inside edition handlers
	// (the platform resolver over the role registry; unknown capabilities
	// and roles fail closed).
	Authorize kernel.CapabilityResolver

	// Candidates accumulates kernel modules in construction order: the
	// platform appends its always-present modules around the edition's Wire
	// hook so the default composition keeps its historical registration
	// order (the registry topologically sorts anyway — the order only pins
	// the init log and migration sequence).
	Candidates []kernel.Module

	// ThemeInstaller installs a ThemeProvider's embedded filesystem into the
	// SSR engine after InitAll. Editions without an SSR engine leave it nil
	// and the install loop is skipped; platform code never imports a theme
	// or the engine directly.
	ThemeInstaller func(fs.FS)

	handlers *HandlerSet
}

// Handlers returns the edition-optional surface set. Editions fill the fields
// they carry; the rest stay nil and their routes unmount.
func (c *Core) Handlers() *HandlerSet {
	return c.handlers
}
