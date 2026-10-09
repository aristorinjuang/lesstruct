package routes

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	apikey "github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/kernel"
	adminpanelapi "github.com/aristorinjuang/lesstruct/modules/adminpanel/api"
	"github.com/go-chi/chi/v5"
)

// maxBodySizeMiddleware limits request body size to 1MB
func maxBodySizeMiddleware(maxBytes int64) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// isAgentRequest reports whether the request presents a Bearer API-key token (the agent
// realm). It is a fast structural check on the Authorization header — the
// APIKeyAuthMiddleware performs the real verification. The browser admin realm (JWT in a
// cookie or a non-API-key Bearer) never carries the apikey.KeyPrefix, so this cleanly
// separates the two realms that co-own the shared /api/v1/media GET paths.
func isAgentRequest(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "+apikey.KeyPrefix)
}

// dispatchByAuth returns a handler that routes to agentChain when the request presents a
// Bearer API key, otherwise to browserChain. It resolves the path collision between the
// agent v1 surface and the browser admin endpoints at the SAME path: Chi routes by path
// (not auth realm), so the two realms cannot each register the path. Each chain carries its
// own auth stack, so both realms work end-to-end at the shared path.
func dispatchByAuth(agentChain, browserChain http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isAgentRequest(r) {
			agentChain.ServeHTTP(w, r)
			return
		}
		browserChain.ServeHTTP(w, r)
	}
}

// agentAdminOnly is a middleware for the agent (Bearer API key) realm that rejects
// non-admin users. The decision delegates to the capability resolver so the
// meaning of "admin" stays in one place (the users module); the response is
// byte-identical to the previous direct role comparison.
func agentAdminOnly(resolve kernel.CapabilityResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := middleware.Principal(r)
			if !ok || resolve == nil || !resolve(principal, string(middleware.CapabilityAdmin)) {
				response.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "Only administrators can perform this action", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MediaBrowserHandler is the browser-realm (JWT/CSRF) media surface the
// routes mount. It is satisfied by the media module's browser handler; the
// structural seam keeps routes.go from importing the module so the surface
// stays removable. Method values (handler.GetMedia, ...) mount directly.
type MediaBrowserHandler interface {
	Upload(http.ResponseWriter, *http.Request)
	GetMedia(http.ResponseWriter, *http.Request)
	GetMediaByID(http.ResponseWriter, *http.Request)
	DeleteMedia(http.ResponseWriter, *http.Request)
	GenerateImage(http.ResponseWriter, *http.Request)
}

// MediaAgentHandler is the Bearer-realm (API key) media surface the routes
// mount. It is satisfied by the media module's agent handler, for the same
// removability reason as MediaBrowserHandler.
type MediaAgentHandler interface {
	Upload(http.ResponseWriter, *http.Request)
	List(http.ResponseWriter, *http.Request)
	Get(http.ResponseWriter, *http.Request)
}

// CommentBrowserHandler is the browser-realm (JWT/CSRF) comment surface the
// routes mount. It is satisfied by the comments module's browser handler; the
// structural seam keeps routes.go from importing the module so the surface
// stays removable. Method values (handler.GetComments, ...) mount directly.
type CommentBrowserHandler interface {
	GetComments(http.ResponseWriter, *http.Request)
	CreateComment(http.ResponseWriter, *http.Request)
	GetCommentsForModeration(http.ResponseWriter, *http.Request)
	GetPendingComments(http.ResponseWriter, *http.Request)
	UpdateCommentStatus(http.ResponseWriter, *http.Request)
	DeleteComment(http.ResponseWriter, *http.Request)
	GetMyComments(http.ResponseWriter, *http.Request)
	DeleteOwnComment(http.ResponseWriter, *http.Request)
}

// CommentAgentHandler is the Bearer-realm (API key) comment surface the
// routes mount. It is satisfied by the comments module's agent handler, for
// the same removability reason as CommentBrowserHandler.
type CommentAgentHandler interface {
	Create(http.ResponseWriter, *http.Request)
	List(http.ResponseWriter, *http.Request)
	Delete(http.ResponseWriter, *http.Request)
	UpdateStatus(http.ResponseWriter, *http.Request)
}

// SEOSurface is the SEO discovery surface the routes mount: the crawler XML
// sitemap, robots.txt, and the programmatic JSON sitemap. It is satisfied by
// the seo module's handler; the structural seam keeps routes.go from
// importing the module so the surface stays removable.
type SEOSurface interface {
	GetSitemapXML(http.ResponseWriter, *http.Request)
	GetRobotsTxt(http.ResponseWriter, *http.Request)
	GetSitemapData(http.ResponseWriter, *http.Request)
}

// TextGenSurface is the browser-realm AI text generation surface the routes
// mount. It is satisfied by the textgen module's handler; the structural
// seam keeps routes.go from importing the module so the surface stays
// removable. The textGenEnabled feature flag gates mounting.
type TextGenSurface interface {
	Enhance(http.ResponseWriter, *http.Request)
	Translate(http.ResponseWriter, *http.Request)
}

// WordPressSurface is the browser-and-agent WordPress import surface the
// routes mount. It is satisfied by the importers module's handler; the
// structural seam keeps routes.go from importing the module so the surface
// stays removable. The same instance serves both realms, sharing one
// in-memory job store.
type WordPressSurface interface {
	Import(http.ResponseWriter, *http.Request)
	ImportStatus(http.ResponseWriter, *http.Request)
}

// HugoSurface is the browser-and-agent Hugo import surface the routes mount.
// Same dual-realm sharing contract as the WordPress surface.
type HugoSurface interface {
	Import(http.ResponseWriter, *http.Request)
	ImportStatus(http.ResponseWriter, *http.Request)
}

// AdminPanelSurface is the admin SPA surface the routes mount. It is
// satisfied by the adminpanel module's SPA handler; the structural seam
// keeps routes.go from importing the module so the surface stays removable.
// A nil handler unmounts the /admin premises entirely, mirroring the
// pre-module static-server behavior the headless tests pin.
type AdminPanelSurface interface {
	ServeAdmin(http.ResponseWriter, *http.Request)
}

// AuthSurface is the public auth surface the routes mount: login,
// registration, verification, and password reset. It is satisfied by the
// auth module's handler; the structural seam keeps routes.go from importing
// the module so the surface stays removable.
type AuthSurface interface {
	Login(http.ResponseWriter, *http.Request)
	RegisterUser(http.ResponseWriter, *http.Request)
	VerifyEmail(http.ResponseWriter, *http.Request)
	ResendVerificationEmail(http.ResponseWriter, *http.Request)
	ForgotPassword(http.ResponseWriter, *http.Request)
	ResetPassword(http.ResponseWriter, *http.Request)
}

// FirstLoginSurface is the first-login bootstrap surface the routes mount.
// Same structural contract as the auth surface.
type FirstLoginSurface interface {
	GetStatus(http.ResponseWriter, *http.Request)
	CompleteSetup(http.ResponseWriter, *http.Request)
}

// UserManagementSurface is the admin user-lifecycle surface the routes
// mount: approval, suspension, soft delete with content restore, and admin
// creation. It is satisfied by the users module's handler; the structural
// seam keeps routes.go from importing the module so the surface stays
// removable.
type UserManagementSurface interface {
	GetPendingUsers(http.ResponseWriter, *http.Request)
	ApproveUser(http.ResponseWriter, *http.Request)
	RejectUser(http.ResponseWriter, *http.Request)
	MarkUserAsSpam(http.ResponseWriter, *http.Request)
	GetAllUsers(http.ResponseWriter, *http.Request)
	SuspendUser(http.ResponseWriter, *http.Request)
	UnsuspendUser(http.ResponseWriter, *http.Request)
	SoftDeleteUser(http.ResponseWriter, *http.Request)
	GetSoftDeletedContent(http.ResponseWriter, *http.Request)
	RestoreContent(http.ResponseWriter, *http.Request)
	CreateUser(http.ResponseWriter, *http.Request)
	UpdateUser(http.ResponseWriter, *http.Request)
}

// ProfileSurface is the self-service profile surface the routes mount.
// Same structural contract as the management surface.
type ProfileSurface interface {
	GetProfile(http.ResponseWriter, *http.Request)
	UpdateEmail(http.ResponseWriter, *http.Request)
	ChangePassword(http.ResponseWriter, *http.Request)
	ExportUserData(http.ResponseWriter, *http.Request)
	VerifyEmailUpdate(http.ResponseWriter, *http.Request)
	DeleteAccount(http.ResponseWriter, *http.Request)
	UpdateCustomFields(http.ResponseWriter, *http.Request)
	UpdateName(http.ResponseWriter, *http.Request)
	GetUserFields(http.ResponseWriter, *http.Request)
}

// ProfilePictureSurface is the profile picture surface the routes mount:
// self-service upload/delete plus the admin per-user endpoints. Same
// structural contract as the management surface.
type ProfilePictureSurface interface {
	UploadProfilePicture(http.ResponseWriter, *http.Request)
	DeleteProfilePicture(http.ResponseWriter, *http.Request)
	AdminUploadUserPicture(http.ResponseWriter, *http.Request)
	AdminDeleteUserPicture(http.ResponseWriter, *http.Request)
}

// Setup configures and returns the HTTP router
func Setup(
	authHandler AuthSurface,
	firstLoginHandler FirstLoginSurface,
	notificationHandler *handlers.NotificationHandler,
	userManagementHandler UserManagementSurface,
	profileHandler ProfileSurface,
	contentHandler *handlers.ContentHandler,
	mediaHandler MediaBrowserHandler,
	dashboardHandler *handlers.DashboardHandler,
	seoHandler SEOSurface,
	commentHandler CommentBrowserHandler,
	profilePictureHandler ProfilePictureSurface,
	wordPressHandler WordPressSurface,
	apiKeyHandler *handlers.APIKeyHandler,
	hugoHandler HugoSurface,
	exportHandler *handlers.ExportHandler,
	ssgHandler *handlers.SSGHandler,
	apiKeyAuthMiddleware *middleware.APIKeyAuthMiddleware,
	agentContentHandler *agent.ContentHandler,
	agentMediaHandler MediaAgentHandler,
	agentCommentHandler CommentAgentHandler,
	adminMiddleware *middleware.AdminMiddleware,
	corsMiddleware *middleware.CORSMiddleware,
	noCookieMiddleware *middleware.NoCookieMiddleware,
	csrfMiddleware *middleware.CSRFMiddleware,
	rateLimitMiddleware *middleware.RateLimitMiddleware,
	securityHeadersMiddleware *middleware.SecurityHeadersMiddleware,
	jwtManager *auth.JWTManager,
	spaHandler AdminPanelSurface,
	staticHandler http.Handler,
	imageGenEnabled bool,
	imageReferenceEnabled bool,
	textGenEnabled bool,
	textGenHandler TextGenSurface,
	languages []string,
	aliasRedirectHandler *handlers.AliasRedirectHandler,
	headlessEnabled bool,
	commentsEnabled bool,
	storageDriver string,
	authorize kernel.CapabilityResolver,
	moduleRegistry *kernel.Registry,
) *chi.Mux {
	r := chi.NewRouter()

	// CORS middleware (must run before authentication)
	r.Use(corsMiddleware.Handler)

	// No-Cookie enforcement middleware (strips any accidental Set-Cookie headers)
	r.Use(noCookieMiddleware.Handler)

	// Create authentication middleware for protected routes
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	// Security headers middleware
	r.Use(securityHeadersMiddleware.Handler)

	// Body-size limits are applied per-route-group below, NOT globally. A global
	// MaxBytesReader would run before any per-route exemption and silently cap
	// upload routes at the global value (the inner, smaller reader always wins),
	// which is what broke the WordPress importer. Each group applies exactly one
	// limit: 1MB for JSON endpoints, higher for upload endpoints.

	// Health endpoint is NOT rate-limited
	// SEO discovery endpoints live at the router ROOT (not under /api) so crawlers
	// find them at the canonical paths: /sitemap.xml (XML sitemap, advertised by
	// robots.txt) and /robots.txt. They are registered before the /* content-site
	// catchall so they win over it. The JSON sitemap at /api/v1/sitemap (below,
	// for programmatic callers) stays where it is.
	r.Get("/sitemap.xml", seoHandler.GetSitemapXML)
	r.Get("/robots.txt", seoHandler.GetRobotsTxt)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		imageGenEnabled := imageGenEnabled
		imageReferenceEnabled := imageReferenceEnabled
		textGenEnabled := textGenEnabled
		if _, err := fmt.Fprintf(w, `{"status":"ok","features":{"imageGeneration":%v,"imageReference":%v,"textGeneration":%v}}`, imageGenEnabled, imageReferenceEnabled, textGenEnabled); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	// Config endpoint (public) — exposes languages and feature flags to the
	// frontend (admin SPA and public content site).
	r.Get("/api/v1/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		langJSON := []byte{'['}
		for i, lang := range languages {
			if i > 0 {
				langJSON = append(langJSON, ',')
			}
			langJSON = append(langJSON, '"')
			langJSON = append(langJSON, []byte(lang)...)
			langJSON = append(langJSON, '"')
		}
		langJSON = append(langJSON, ']')
		if _, err := fmt.Fprintf(w, `{"data":{"languages":%s,"commentsEnabled":%v,"headless":%v},"error":null}`, langJSON, commentsEnabled, headlessEnabled); err != nil {
			log.Printf("Failed to write config response: %v", err)
		}
	})

	// Bearer-only /api/v1 (Epic 2) — mounted at the router ROOT so it inherits ONLY
	// the root global middleware (CORS ✓, NoCookie = harmless no-op, security headers,
	// 1MB body limit) and NOT the /api subrouter's per-IP rate limit, csrfMiddleware,
	// or authMiddleware.RequireAuth. Precedent: /api/health and /api/v1/config are
	// also root-level. Path leaves (/content, /content/{id}) are collision-free vs the
	// browser-realm leaves (/content_items, /content_items/{id}, /content/slug).
	//
	// Middleware ORDER matters: the per-key rate limiter is registered FIRST so it is
	// the outermost wrapper and runs BEFORE the Bearer auth middleware. The
	// APIKeyAuthMiddleware short-circuits on every failure path (missing/invalid key →
	// 401, it never calls next), so if auth were outermost the limiter would only ever
	// see already-authenticated requests and invalid/repeating key attempts would NOT
	// be throttled. Keeping the limiter outermost keys the bucket by the presented
	// keyID (see keyByAPIKeyOrIP) so brute-force attempts are counted even when the key
	// never verifies.
	r.Group(func(r chi.Router) {
		r.Use(rateLimitMiddleware.APIKeyHandler)
		r.Use(apiKeyAuthMiddleware.Handler)

		// JSON endpoints — 1MB body limit.
		r.Group(func(r chi.Router) {
			r.Use(maxBodySizeMiddleware(1 << 20))
			r.Post("/api/v1/content", agentContentHandler.Create)
			r.Get("/api/v1/content", agentContentHandler.List)
			r.Get("/api/v1/content/{id}", agentContentHandler.Get)
			r.Put("/api/v1/content/{id}", agentContentHandler.Update)
			// Admin-only (a non-admin API key gets 403): lets the CLI set the
			// admin-managed system fields for a content item (mirror of the browser
			// admin's system-fields endpoint, in the Bearer/agent realm).
			r.Put("/api/v1/content/{id}/system-fields", agentContentHandler.SetSystemFields)
			r.Delete("/api/v1/content/{id}", agentContentHandler.Delete)
			// Standalone status-toggle actions — let agents flip published/draft
			// without resending the body. Both accept an empty request body and
			// are idempotent. Sharing the Bearer-only group above so the per-key
			// rate limit + API-key auth apply on the same footing as Update/Delete.
			r.Post("/api/v1/content/{id}/publish", agentContentHandler.Publish)
			r.Post("/api/v1/content/{id}/unpublish", agentContentHandler.Unpublish)
			// Agent comment surface — nested under the content-keyed namespace so it is
			// collision-free vs the browser realm's /api/v1/content_items/.../comments and
			// /api/v1/comments routes (Chi routes by path, not auth realm). Create/List are
			// any authenticated caller (scoped to visible content); Delete is own-or-admin;
			// UpdateStatus is admin-only moderation. Reuses the existing content domain
			// comment methods via agent.CommentHandler. Unmounted when comments are disabled.
			if commentsEnabled {
				r.Post("/api/v1/content/{id}/comments", agentCommentHandler.Create)
				r.Get("/api/v1/content/{id}/comments", agentCommentHandler.List)
				r.Delete("/api/v1/content/{id}/comments/{commentId}", agentCommentHandler.Delete)
				r.Put("/api/v1/content/{id}/comments/{commentId}/status", agentCommentHandler.UpdateStatus)
			}
		})

		// Media upload — 10MB (sibling of the 1MB group so the higher limit is not
		// silently capped by a competing smaller MaxBytesReader).
		// NOTE: GET /api/v1/media and GET /api/v1/media/{id} are registered BELOW as shared
		// dual-auth dispatch routes (the browser admin media list/get co-owns the same path;
		// Chi routes by path, not auth realm, so a single registration dispatches by credential).
		r.With(maxBodySizeMiddleware(10<<20)).Post("/api/v1/media", agentMediaHandler.Upload)

		// Content export (agent-only) — streams a tar.gz of all content as
		// Hugo-compatible source files with bundled media.
		r.Get("/api/v1/export", exportHandler.Export)

		// Static site generation (agent-only) — streams a tar.gz of the
		// fully rendered static site with AMP variants.
		r.Get("/api/v1/ssg", ssgHandler.Generate)

		// WordPress import — large upload, no body-size middleware (handler applies
		// its own MaxBytesReader at IMPORT_MAX_SIZE_MB). Admin-only (only administrators
		// may import content via API key). Reuses the same handler and in-memory job
		// store as the browser realm, so a CLI-started job is visible in the admin UI.
		r.With(agentAdminOnly(authorize)).Post("/api/v1/wordpress/import", wordPressHandler.Import)
		r.With(agentAdminOnly(authorize)).Get("/api/v1/wordpress/import/status/{jobId}", wordPressHandler.ImportStatus)
		r.With(agentAdminOnly(authorize)).Get("/api/v1/wordpress/import/status", wordPressHandler.ImportStatus)

		// Hugo import — same contract as WordPress: admin-only, agent realm, and
		// shares the browser realm's handler and job store.
		r.With(agentAdminOnly(authorize)).Post("/api/v1/hugo/import", hugoHandler.Import)
		r.With(agentAdminOnly(authorize)).Get("/api/v1/hugo/import/status/{jobId}", hugoHandler.ImportStatus)
		r.With(agentAdminOnly(authorize)).Get("/api/v1/hugo/import/status", hugoHandler.ImportStatus)
	})

	// Shared /api/v1/media GET routes — co-owned by the agent (Bearer API key) and browser
	// admin (JWT/CSRF) realms at the SAME path. Chi routes by path, not auth realm, so the
	// two realms cannot each register the path; a single registration dispatches by the
	// presented credential (Authorization: Bearer lesstruct_… → agent; otherwise → browser).
	// POST /api/v1/media (upload) stays Bearer-only above; browser POST /media/upload,
	// /media/generate and DELETE /media/{id} stay in the /api/v1 subrouter — those paths do
	// not collide. Each branch runs its OWN auth stack (agent: per-key rate limit + API-key
	// auth; browser: per-IP rate limit + CSRF + JWT) so both realms work end-to-end.
	r.Get("/api/v1/media", dispatchByAuth(
		rateLimitMiddleware.APIKeyHandler(apiKeyAuthMiddleware.Handler(http.HandlerFunc(agentMediaHandler.List))),
		rateLimitMiddleware.Handler(csrfMiddleware.Handler(authMiddleware.RequireAuth(http.HandlerFunc(mediaHandler.GetMedia)))),
	))
	r.Get("/api/v1/media/{id}", dispatchByAuth(
		rateLimitMiddleware.APIKeyHandler(apiKeyAuthMiddleware.Handler(http.HandlerFunc(agentMediaHandler.Get))),
		rateLimitMiddleware.Handler(csrfMiddleware.Handler(authMiddleware.RequireAuth(http.HandlerFunc(mediaHandler.GetMediaByID)))),
	))

	// API routes (highest priority) — all rate-limited
	r.Route("/api", func(r chi.Router) {
		// Apply global API rate limit to all /api/* routes
		r.Use(rateLimitMiddleware.Handler)

		r.Route("/auth", func(r chi.Router) {
			// Stricter rate limit for auth endpoints
			r.Use(rateLimitMiddleware.AuthHandler)
			r.Post("/login", authHandler.Login)
			r.Post("/register", authHandler.RegisterUser)
			r.Get("/verify-email", authHandler.VerifyEmail)
			r.Post("/resend-verification", authHandler.ResendVerificationEmail)
			r.Post("/forgot-password", authHandler.ForgotPassword)
			r.Post("/reset-password", authHandler.ResetPassword)
			r.Get("/first-login", firstLoginHandler.GetStatus)
			r.Post("/first-login", firstLoginHandler.CompleteSetup)
		})
		r.Get("/notifications", notificationHandler.GetNotifications)

		// Admin user management routes (require Admin role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(csrfMiddleware.Handler)
			r.Use(adminMiddleware.AdminOnly)

			// JSON endpoints — 1MB body limit.
			r.Group(func(r chi.Router) {
				r.Use(maxBodySizeMiddleware(1 << 20))
				r.Get("/pending-users", userManagementHandler.GetPendingUsers)
				r.Post("/users/{id}/approve", userManagementHandler.ApproveUser)
				r.Post("/users/{id}/reject", userManagementHandler.RejectUser)
				r.Post("/users/{id}/mark-spam", userManagementHandler.MarkUserAsSpam)
				// Account administration routes (Story 1.6)
				r.Post("/users", userManagementHandler.CreateUser)
				r.Get("/users", userManagementHandler.GetAllUsers)
				r.Post("/users/{id}/suspend", userManagementHandler.SuspendUser)
				r.Post("/users/{id}/unsuspend", userManagementHandler.UnsuspendUser)
				r.Post("/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)
				r.Put("/users/{id}", userManagementHandler.UpdateUser)
				r.Get("/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)
				r.Post("/content/{id}/restore", userManagementHandler.RestoreContent)
				r.Put("/content/{id}/system-fields", contentHandler.SetSystemFields)
				r.Delete("/users/{id}/picture", profilePictureHandler.AdminDeleteUserPicture)
			})

			// Admin profile picture upload — 10MB (Story 12.8).
			r.With(maxBodySizeMiddleware(10<<20)).Put("/users/{id}/picture", profilePictureHandler.AdminUploadUserPicture)

			// WordPress import — the handler is async (202 Accepted). The handler
			// applies MaxBytesReader at IMPORT_MAX_SIZE_MB, so no route-level
			// wrapper is needed here. Status polling endpoints let the admin UI
			// or the caller track progress of a running import job.
			r.Post("/wordpress/import", wordPressHandler.Import)
			r.Get("/wordpress/import/status/{jobId}", wordPressHandler.ImportStatus)
			r.Get("/wordpress/import/status", wordPressHandler.ImportStatus)
			// Hugo import (admin only) — same import ceiling as WordPress.
			r.Post("/hugo/import", hugoHandler.Import)
			r.Get("/hugo/import/status/{jobId}", hugoHandler.ImportStatus)
			r.Get("/hugo/import/status", hugoHandler.ImportStatus)
			// Content export (admin only) — streams a tar.gz of all content as
			// Hugo-compatible source files with bundled media.
			r.Get("/export", exportHandler.Export)
			// Static site generation (admin only) — streams a tar.gz of the
			// fully rendered static site with AMP variants.
			r.Get("/ssg", ssgHandler.Generate)
			// Admin manifest aggregation (admin only) — the frozen
			// extensions contract: every registered AdminExtensionProvider
			// in registry name order, versioned JSON. Aggregation reads
			// the registry live, so the handler is built here rather
			// than adopted from a module.
			r.Get("/extensions", adminpanelapi.NewExtensionsHandler(moduleRegistry).GetExtensions)
		})

		// API key management routes (browser-realm, any authenticated role) - Story 1.1 + 1.2
		// Registered outside the /admin group so any logged-in user (not just Admins)
		// can manage their own keys. CSRF + RequireAuth protect these browser-realm endpoints.
		r.Group(func(r chi.Router) {
			r.Use(csrfMiddleware.Handler, authMiddleware.RequireAuth)
			r.Use(maxBodySizeMiddleware(1 << 20))
			r.Post("/admin/api-keys", apiKeyHandler.CreateAPIKey)
			r.Get("/admin/api-keys", apiKeyHandler.ListAPIKeys)
			r.Delete("/admin/api-keys/{id}", apiKeyHandler.RevokeAPIKey)
		})

		// Profile management routes (require authentication) - Story 1.7
		r.Route("/profile", func(r chi.Router) {
			r.Use(csrfMiddleware.Handler)
			r.Use(authMiddleware.RequireAuth)

			// JSON endpoints — 1MB body limit.
			r.Group(func(r chi.Router) {
				r.Use(maxBodySizeMiddleware(1 << 20))
				r.Get("/", profileHandler.GetProfile)
				r.Put("/email", profileHandler.UpdateEmail)
				r.Put("/password", profileHandler.ChangePassword)
				r.Get("/export", profileHandler.ExportUserData)
				r.Put("/custom-fields", profileHandler.UpdateCustomFields)
				r.Put("/name", profileHandler.UpdateName)
				r.Get("/user-fields", profileHandler.GetUserFields)
				r.Delete("/account", profileHandler.DeleteAccount)
				r.Delete("/picture", profilePictureHandler.DeleteProfilePicture)
			})

			// Profile picture upload — 10MB (Story 12.8).
			r.With(maxBodySizeMiddleware(10<<20)).Put("/picture", profilePictureHandler.UploadProfilePicture)
		})

		// Email update verification route (public, token-based) - Story 1.7
		// Does NOT require authentication - the verification token is sufficient
		r.Get("/profile/verify-email", profileHandler.VerifyEmailUpdate)

		// Public content routes (no authentication) - Story 4.1
		r.Route("/v1/public", func(r chi.Router) {
			// Public rate limit for content delivery endpoints
			r.Use(rateLimitMiddleware.PublicHandler)
			r.Get("/content_items", contentHandler.ListPublishedContents)
			r.Get("/content_items/{slug}", contentHandler.GetPublishedContent)
			r.Get("/authors", contentHandler.ListPublishedAuthors)
			r.Get("/authors/{username}", contentHandler.GetPublishedAuthor)
			r.Get("/authors/{username}/content_items", contentHandler.GetPublishedContentByAuthor)
			if commentsEnabled {
				r.Get("/content_items/{slug}/comments", commentHandler.GetComments)
			}
			// First-party modules mount their anonymous surfaces here, on the
			// public router (rate-limited, no auth chain).
			if moduleRegistry != nil {
				for _, provider := range kernel.Capabilities[kernel.PublicRouteProvider](moduleRegistry) {
					provider.RegisterPublicRoutes(r)
				}
			}
			r.Get("/search", contentHandler.SearchPublished)
			r.Get("/archive", contentHandler.ListPublishedArchive)
		})

		// SEO routes (public, no authentication) - Story 4.2
		r.Get("/v1/sitemap", seoHandler.GetSitemapData)
		r.Get("/robots.txt", seoHandler.GetRobotsTxt)

		// Content routes (require authentication) - Story 2.1
		r.Route("/v1", func(r chi.Router) {
			r.Use(csrfMiddleware.Handler)
			r.Use(authMiddleware.RequireAuth)

			// JSON endpoints — 1MB body limit.
			r.Group(func(r chi.Router) {
				r.Use(maxBodySizeMiddleware(1 << 20))
				r.Post("/content_items", contentHandler.CreateContent)
				r.Get("/content_items", contentHandler.ListContents)
				r.Get("/content_items/{id}", contentHandler.GetContent)
				r.Put("/content_items/{id}", contentHandler.UpdateContent)
				r.Delete("/content_items/{id}", contentHandler.DeleteContent)
				r.Post("/content/slug", contentHandler.GenerateSlug)
				r.Get("/content_items/{id}/seo", contentHandler.GetSEO)
				if commentsEnabled {
					r.Post("/content_items/{slug}/comments", commentHandler.CreateComment)
				}

				// First-party modules mount here, on the authenticated group
				// (CSRF + JWT + 1MB pre-chained above). The posttypes-basic module
				// serves post types, roles, and user fields from here.
				if moduleRegistry != nil {
					for _, provider := range kernel.Capabilities[kernel.RouteProvider](moduleRegistry) {
						provider.RegisterRoutes(r)
					}
				}

				// GET /media (list) and GET /media/{id} are served by the shared dual-auth
				// dispatch at the root (above) — the agent and browser admin co-own these paths.
				r.Delete("/media/{id}", mediaHandler.DeleteMedia)

				// Dashboard routes (require authentication + admin role) - Story 2.8
				r.With(adminMiddleware.AdminOnly).Get("/dashboard/stats", dashboardHandler.GetStats)

				// Admin comment moderation routes (require authentication + admin role) - Story 4.7
				// and the Commentator's own-comment routes are unmounted when comments are
				// disabled, mirroring the agent/public comment gates above.
				if commentsEnabled {
					r.With(adminMiddleware.ModerationOnly).Get("/content_items/{id}/comments", commentHandler.GetCommentsForModeration)
					r.With(adminMiddleware.ModerationOnly).Get("/comments/pending", commentHandler.GetPendingComments)
					r.With(adminMiddleware.ModerationOnly).Put("/comments/{id}/status", commentHandler.UpdateCommentStatus)
					r.With(adminMiddleware.ModerationOnly).Delete("/comments/{id}", commentHandler.DeleteComment)

					// Commentator user's own comments (require authentication)
					r.Get("/my-comments", commentHandler.GetMyComments)
					r.Delete("/my-comments/{id}", commentHandler.DeleteOwnComment)
				}

				// AI text generation routes (require authentication + feature enabled)
				if textGenEnabled && textGenHandler != nil {
					r.Post("/text/enhance", textGenHandler.Enhance)
					r.Post("/text/translate", textGenHandler.Translate)
				}
			})

			// Media upload — 10MB (Story 2.3).
			r.With(maxBodySizeMiddleware(10<<20)).Post("/media/upload", mediaHandler.Upload)

			// AI image generation — 32MB. The JSON prompt-only form is tiny, but the
			// multipart form may carry up to three 10MB reference images.
			r.With(maxBodySizeMiddleware(32<<20)).Post("/media/generate", mediaHandler.GenerateImage)
		})
	})

	// Serve uploaded media and profile pictures (public, no authentication
	// required) — ONLY for the local driver. With STORAGE_DRIVER=s3 the bytes
	// live in the bucket and are served to clients via the public base URL /
	// CDN produced by Storage.GetURL, so no local fileserver is mounted.
	if storageDriver == "local" {
		uploadsDir := filepath.Join("data", "uploads", "media")
		if info, err := os.Stat(uploadsDir); err == nil && info.IsDir() {
			mediaFS := http.StripPrefix("/uploads/media/", http.FileServer(http.Dir(uploadsDir)))
			r.Handle("/uploads/media/*", mediaFS)
		}

		profilePicturesDir := filepath.Join("data", "uploads", "profile_pictures")
		if info, err := os.Stat(profilePicturesDir); err == nil && info.IsDir() {
			profilePicturesFS := http.StripPrefix("/uploads/profile_pictures/", http.FileServer(http.Dir(profilePicturesDir)))
			r.Handle("/uploads/profile_pictures/*", profilePicturesFS)
		}
	}

	// Serve content site static assets (CSS, etc.) — only when the content site
	// itself is enabled (i.e. not in headless mode).
	if !headlessEnabled {
		r.Handle("/static/*", http.StripPrefix("/static/", staticHandler))
	}

	// Admin SPA (serves embedded Vue 3 app or proxies to dev server)
	if spaHandler != nil {
		r.Handle("/admin/*", http.StripPrefix("/admin", http.HandlerFunc(spaHandler.ServeAdmin)))
		r.HandleFunc("/admin", spaHandler.ServeAdmin)

		// Content site (Go template renderer or dev proxy) with alias redirect
		// wrapping. In headless mode the catch-all is NOT mounted — the instance
		// serves only the admin panel and the REST API. (The content page handler
		// and alias handler are still constructed for the SSG generator; they are
		// simply unreachable over HTTP.)
		if !headlessEnabled {
			r.Handle("/*", aliasRedirectHandler)
		}
	}

	return r
}
