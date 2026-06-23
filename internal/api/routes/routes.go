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
	"github.com/aristorinjuang/lesstruct/internal/api/static"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	apikey "github.com/aristorinjuang/lesstruct/internal/domain/apikey"
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


// Setup configures and returns the HTTP router
func Setup(
	authHandler *handlers.AuthHandler,
	firstLoginHandler *handlers.FirstLoginHandler,
	notificationHandler *handlers.NotificationHandler,
	userManagementHandler *handlers.UserManagementHandler,
	profileHandler *handlers.ProfileHandler,
	contentHandler *handlers.ContentHandler,
	mediaHandler *handlers.MediaHandler,
	postTypeHandler *handlers.PostTypeHandler,
	dashboardHandler *handlers.DashboardHandler,
	seoHandler *handlers.SEOHandler,
	commentHandler *handlers.CommentHandler,
	profilePictureHandler *handlers.ProfilePictureHandler,
	wordPressHandler *handlers.WordPressHandler,
	apiKeyHandler *handlers.APIKeyHandler,
	apiKeyAuthMiddleware *middleware.APIKeyAuthMiddleware,
	agentContentHandler *agent.ContentHandler,
	agentMediaHandler *agent.MediaHandler,
	agentCommentHandler *agent.CommentHandler,
	adminMiddleware *middleware.AdminMiddleware,
	corsMiddleware *middleware.CORSMiddleware,
	noCookieMiddleware *middleware.NoCookieMiddleware,
	csrfMiddleware *middleware.CSRFMiddleware,
	rateLimitMiddleware *middleware.RateLimitMiddleware,
	jwtManager *auth.JWTManager,
	staticServer *static.StaticServer,
	staticHandler http.Handler,
	imageGenEnabled bool,
	textGenEnabled bool,
	textGenHandler *handlers.TextGenHandler,
	languages []string,
) *chi.Mux {
	r := chi.NewRouter()

	// CORS middleware (must run before authentication)
	r.Use(corsMiddleware.Handler)

	// No-Cookie enforcement middleware (strips any accidental Set-Cookie headers)
	r.Use(noCookieMiddleware.Handler)

	// Create authentication middleware for protected routes
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	// Security headers middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; img-src 'self' data: https:; font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net; connect-src 'self'; frame-src 'self' https://www.youtube.com; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
			next.ServeHTTP(w, r)
		})
	})

	// Limit request body size to 1MB
	r.Use(maxBodySizeMiddleware(1 << 20)) // 1MB

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
		textGenEnabled := textGenEnabled
		if _, err := fmt.Fprintf(w, `{"status":"ok","features":{"imageGeneration":%v,"textGeneration":%v}}`, imageGenEnabled, textGenEnabled); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	// Config endpoint (public) — exposes languages to the frontend
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
		if _, err := fmt.Fprintf(w, `{"data":{"languages":%s},"error":null}`, langJSON); err != nil {
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
		// comment methods via agent.CommentHandler.
		r.Post("/api/v1/content/{id}/comments", agentCommentHandler.Create)
		r.Get("/api/v1/content/{id}/comments", agentCommentHandler.List)
		r.Delete("/api/v1/content/{id}/comments/{commentId}", agentCommentHandler.Delete)
		r.Put("/api/v1/content/{id}/comments/{commentId}/status", agentCommentHandler.UpdateStatus)
		// Media upload is exempted from the root-level 1MB body limit (raised to 10MB) so
		// legitimate image uploads are not rejected — mirroring the admin media-upload
		// pattern. The exemption applies ONLY to the POST upload route, not the GETs.
		r.With(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
				next.ServeHTTP(w, r)
			})
		}).Post("/api/v1/media", agentMediaHandler.Upload)
		// NOTE: GET /api/v1/media and GET /api/v1/media/{id} are registered BELOW as shared
		// dual-auth dispatch routes (the browser admin media list/get co-owns the same path;
		// Chi routes by path, not auth realm, so a single registration dispatches by credential).
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

			// Admin profile picture routes (require Admin role) - Story 12.8
			r.With(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
					next.ServeHTTP(w, r)
				})
			}).Put("/users/{id}/picture", profilePictureHandler.AdminUploadUserPicture)
			r.Delete("/users/{id}/picture", profilePictureHandler.AdminDeleteUserPicture)

			// WordPress import (admin only) - Story: WP Import
			// Exempt from the global 1MB body limit; WXR files can be large.
			r.With(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 50MB
					next.ServeHTTP(w, r)
				})
			}).Post("/wordpress/import", wordPressHandler.Import)
		})

		// API key management routes (browser-realm, any authenticated role) - Story 1.1 + 1.2
		// Registered outside the /admin group so any logged-in user (not just Admins)
		// can manage their own keys. CSRF + RequireAuth protect these browser-realm endpoints.
		r.Group(func(r chi.Router) {
			r.Use(csrfMiddleware.Handler, authMiddleware.RequireAuth)
			r.Post("/admin/api-keys", apiKeyHandler.CreateAPIKey)
			r.Get("/admin/api-keys", apiKeyHandler.ListAPIKeys)
			r.Delete("/admin/api-keys/{id}", apiKeyHandler.RevokeAPIKey)
		})

		// Profile management routes (require authentication) - Story 1.7
		r.Route("/profile", func(r chi.Router) {
			r.Use(csrfMiddleware.Handler)
			r.Use(authMiddleware.RequireAuth)
			r.Get("/", profileHandler.GetProfile)
			r.Put("/email", profileHandler.UpdateEmail)
			r.Put("/password", profileHandler.ChangePassword)
			r.Get("/export", profileHandler.ExportUserData)
			r.Put("/custom-fields", profileHandler.UpdateCustomFields)
			r.Put("/name", profileHandler.UpdateName)
			r.Get("/user-fields", profileHandler.GetUserFields)
			r.Delete("/account", profileHandler.DeleteAccount)

			// Profile picture routes (require authentication) - Story 12.8
			// Upload is exempt from the global 1MB body limit; it needs up to 10MB
			r.With(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
					next.ServeHTTP(w, r)
				})
			}).Put("/picture", profilePictureHandler.UploadProfilePicture)
			r.Delete("/picture", profilePictureHandler.DeleteProfilePicture)
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
			r.Get("/authors/{username}/content_items", contentHandler.GetPublishedContentByAuthor)
			r.Get("/content_items/{slug}/comments", commentHandler.GetComments)
			r.Get("/post_types", postTypeHandler.GetPublicPostTypes)
			r.Get("/search", contentHandler.SearchPublished)
		})

		// SEO routes (public, no authentication) - Story 4.2
		r.Get("/v1/sitemap", seoHandler.GetSitemapData)
		r.Get("/robots.txt", seoHandler.GetRobotsTxt)

		// Content routes (require authentication) - Story 2.1
		r.Route("/v1", func(r chi.Router) {
			r.Use(csrfMiddleware.Handler)
			r.Use(authMiddleware.RequireAuth)
			r.Post("/content_items", contentHandler.CreateContent)
			r.Get("/content_items", contentHandler.ListContents)
			r.Get("/content_items/{id}", contentHandler.GetContent)
			r.Put("/content_items/{id}", contentHandler.UpdateContent)
			r.Delete("/content_items/{id}", contentHandler.DeleteContent)
			r.Post("/content/slug", contentHandler.GenerateSlug)
			r.Get("/content_items/{id}/seo", contentHandler.GetSEO)
			r.Post("/content_items/{slug}/comments", commentHandler.CreateComment)

			// Post types routes (require authentication) - Story 2.6
			r.Get("/post_types", postTypeHandler.GetPostTypes)

			// User field schemas route (require authentication + admin role)
			r.With(adminMiddleware.AdminOnly).Get("/user_fields", postTypeHandler.GetUserFieldsEndpoint)

			// Media routes (require authentication) - Story 2.3
			// Upload is exempt from the global 1MB body limit; it needs up to 10MB
			r.With(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
					next.ServeHTTP(w, r)
				})
			}).Post("/media/upload", mediaHandler.Upload)
			r.Post("/media/generate", mediaHandler.GenerateImage)
			// GET /media (list) and GET /media/{id} are served by the shared dual-auth
			// dispatch at the root (above) — the agent and browser admin co-own these paths.
			r.Delete("/media/{id}", mediaHandler.DeleteMedia)

			// Dashboard routes (require authentication + admin role) - Story 2.8
			r.With(adminMiddleware.AdminOnly).Get("/dashboard/stats", dashboardHandler.GetStats)

			// Admin comment moderation routes (require authentication + admin role) - Story 4.7
			r.With(adminMiddleware.ModerationOnly).Get("/content_items/{id}/comments", commentHandler.GetCommentsForModeration)
			r.With(adminMiddleware.ModerationOnly).Get("/comments/pending", commentHandler.GetPendingComments)
			r.With(adminMiddleware.ModerationOnly).Put("/comments/{id}/status", commentHandler.UpdateCommentStatus)
			r.With(adminMiddleware.ModerationOnly).Delete("/comments/{id}", commentHandler.DeleteComment)

			// Commentator user's own comments (require authentication)
			r.Get("/my-comments", commentHandler.GetMyComments)
			r.Delete("/my-comments/{id}", commentHandler.DeleteOwnComment)

			// AI text generation routes (require authentication + feature enabled)
			if textGenEnabled && textGenHandler != nil {
				r.Post("/text/enhance", textGenHandler.Enhance)
				r.Post("/text/translate", textGenHandler.Translate)
			}

		})
	})

	// Serve uploaded media files (public, no authentication required)
	uploadsDir := filepath.Join("data", "uploads", "media")
	if info, err := os.Stat(uploadsDir); err == nil && info.IsDir() {
		mediaFS := http.StripPrefix("/uploads/media/", http.FileServer(http.Dir(uploadsDir)))
		r.Handle("/uploads/media/*", mediaFS)
	}

	// Serve uploaded profile pictures (public, no authentication required)
	profilePicturesDir := filepath.Join("data", "uploads", "profile_pictures")
	if info, err := os.Stat(profilePicturesDir); err == nil && info.IsDir() {
		profilePicturesFS := http.StripPrefix("/uploads/profile_pictures/", http.FileServer(http.Dir(profilePicturesDir)))
		r.Handle("/uploads/profile_pictures/*", profilePicturesFS)
	}

	// Serve content site static assets (CSS, etc.)
	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler))

	// Admin SPA (serves embedded Vue 3 app or proxies to dev server)
	if staticServer != nil {
		r.Handle("/admin/*", http.StripPrefix("/admin", http.HandlerFunc(staticServer.ServeAdmin)))
		r.HandleFunc("/admin", staticServer.ServeAdmin)

		// Content site (Go template renderer or dev proxy)
		r.Handle("/*", http.HandlerFunc(staticServer.ServeContent))
	}

	return r
}
