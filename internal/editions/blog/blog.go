// Package blog is the default (blog) edition: every first-party module —
// identity, content engine, media, comment system, SEO, the default theme,
// the SSR engine, AI text generation, and the importers — wired on top of
// the shared composition-root platform. The root main package builds this
// edition; `go install github.com/aristorinjuang/lesstruct@latest` installs it.
package blog

import (
	"cmp"
	"context"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/app"
	"github.com/aristorinjuang/lesstruct/internal/config"
	ssg "github.com/aristorinjuang/lesstruct/internal/content/ssg"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/i18n"
	"github.com/aristorinjuang/lesstruct/kernel/tiptap"
	"github.com/aristorinjuang/lesstruct/modules/comments"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	commentstore "github.com/aristorinjuang/lesstruct/modules/comments/store"
	"github.com/aristorinjuang/lesstruct/modules/importers"
	hugo "github.com/aristorinjuang/lesstruct/modules/importers/hugo"
	wordpress "github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/aristorinjuang/lesstruct/modules/seo"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	speechmodule "github.com/aristorinjuang/lesstruct/modules/speech"
	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
	speechinfra "github.com/aristorinjuang/lesstruct/modules/speech/infrastructure"
	textgenmodule "github.com/aristorinjuang/lesstruct/modules/textgen"
	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	textgeninfra "github.com/aristorinjuang/lesstruct/modules/textgen/infrastructure"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	themenmodule "github.com/aristorinjuang/lesstruct/modules/themegen"
	themenapi "github.com/aristorinjuang/lesstruct/modules/themegen/api"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	themeninfra "github.com/aristorinjuang/lesstruct/modules/themegen/infrastructure"
	themenstore "github.com/aristorinjuang/lesstruct/modules/themegen/store"
)

// loadUICatalog loads the embedded UI translation catalog for the
// configured languages. A missing catalog is fatal: every public page needs
// it for engine-generated listing strings and theme translations.
func loadUICatalog(languages []string) *i18n.Catalog {
	catalog, err := i18n.NewCatalog(i18n.Embedded(), languages)
	if err != nil {
		log.Fatalf("Failed to load UI translations: %v", err)
	}
	return catalog
}

// siteSSRConfig projects the operator's [site_config] block onto the SSR
// engine's SiteConfig: every identity field the themes read, nothing else.
// The ratio arrives pre-validated from LoadSiteConfig and is typed as
// template.CSS so the storefront layout emits it verbatim in its <style>.
func siteSSRConfig(siteConfig config.SiteConfig) sitessr.SiteConfig {
	return sitessr.SiteConfig{
		Name:              siteConfig.Name,
		Logo:              siteConfig.Logo,
		Favicon:           siteConfig.Favicon,
		ProductImageRatio: template.CSS(siteConfig.ProductImageRatio),
		HeroImage:         siteConfig.HeroImage,
		HeroTitle:         siteConfig.HeroTitle,
		HeroSubtitle:      siteConfig.HeroSubtitle,
	}
}

// resolveBodyImageVariants projects a media record's thumbnail variants onto
// the renderer seam, sorted ascending by width. The original is never
// appended: srcset carries variants only and src stays the original URL.
func resolveBodyImageVariants(m *mediadomain.Media) []tiptap.ImageVariant {
	if m == nil {
		return nil
	}
	if len(m.Variants) == 0 {
		return nil
	}
	result := make([]tiptap.ImageVariant, 0, len(m.Variants))
	for _, v := range m.Variants {
		if v.URL == "" || v.Width <= 0 {
			continue
		}
		result = append(result, tiptap.ImageVariant{
			URL:    v.URL,
			Width:  v.Width,
			Height: v.Height,
		})
	}
	slices.SortFunc(result, func(a, b tiptap.ImageVariant) int {
		if c := cmp.Compare(a.Width, b.Width); c != 0 {
			return c
		}
		return strings.Compare(a.URL, b.URL)
	})
	return result
}

// state carries the edition-built services and modules between the Wire and
// Mount hooks. Everything here is blog-specific — none of it exists in the
// leaner editions' binaries.
type state struct {
	commentService *commentdomain.Service

	commentsModule  *comments.Module
	seoModule       *seo.Module
	textgenModule   *textgenmodule.Module
	themegenModule  *themenmodule.Module
	speechModule    *speechmodule.Module
	importersModule *importers.Module

	theme *sitessr.Theme

	// Live SSR surfaces for hot theme reload (2b): set in Mount, swapped
	// in place by Reload without remounting routes.
	templates        *sitessr.Templates
	staticFiles      *sitessr.StaticFilesHandler
	rootHandler      *sitessr.ThemeRootHandler
	textgenRefresher func(baseCSS, styleCSS string)
	cfg              *config.Config

	// validator composes the domain theme checks with template render
	// validation. Boot, hot reload, and activation all share it.
	validator themegendomain.ThemeValidator

	// snapshotSlugs and snapshotCatalog feed the admin theme snapshots:
	// the installed theme's homepage rendered with populated sample data.
	// Set in Mount alongside the live surfaces.
	snapshotSlugs   []string
	snapshotCatalog *i18n.Catalog

	// Live preview (v1.5): per-request theme resolution via a signed
	// cookie. previewBuilder builds a full handler stack for a theme and
	// is set in Mount; previews caches built stacks by theme directory.
	previewKey     []byte
	previewSecure  bool
	previews       *previewCache
	previewBuilder func(theme *sitessr.Theme) (http.Handler, *sitessr.StaticFilesHandler, error)

	themeProvider kernel.ThemeProvider
}

// wire constructs the blog modules on top of the platform services and fills
// the optional handler surfaces the edition owns.
func (s *state) wire(core *app.Core) error {
	hs := core.Handlers()
	cfg := core.Cfg
	s.cfg = cfg

	// Comments live in their module since Phase 3.3b: the service is built on
	// the per-driver comment repository with the content-backed AllowComments
	// gate, and the site-ssr reader (built in Mount) reads comments through it.
	commentRepo := commentstore.New(core.DB, core.Dialect)
	s.commentService = commentdomain.NewService(
		commentRepo,
		commentdomain.WithAllowCommentsChecker(NewAllowCommentsChecker(core.ContentService)),
	)

	s.commentsModule = comments.New(
		comments.WithService(s.commentService),
		comments.WithContentResolver(NewBrowserContentResolver(core.ContentService)),
		comments.WithAgentContentLookup(NewAgentContentLookup(core.ContentService)),
		comments.WithRoles(NewCommentRoleCapabilities(core.RoleService)),
		comments.WithAgentRoles(NewAgentCommentRoleCapabilities(core.RoleService)),
		comments.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, s.commentsModule)
	hs.CommentBrowser = s.commentsModule.BrowserHandler()
	hs.CommentAgent = s.commentsModule.AgentHandler()

	s.seoModule = seo.New(
		seo.WithPages(NewSEOPublishedPages(core.ContentService)),
		seo.WithBaseURL(core.BaseURL),
		seo.WithHeadless(core.HeadlessEnabled),
		seo.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, s.seoModule)
	hs.SEO = s.seoModule.Handler()

	// v1 module system (Phase 3.5a): the edition's theme ships its layouts and
	// static assets as the ThemeProvider artifact. The module takes no
	// options — the artifact is static. The commerce edition supplies the
	// storefront theme in place of the default one.
	themeModule := s.themeProvider
	if themeModule == nil {
		themeModule = themedefault.New()
	}
	core.Candidates = append(core.Candidates, themeModule)
	// The artifact installs immediately (not only in the ThemeProvider loop
	// after InitAll): code below — ReadThemeStyles for the textgen service —
	// needs the embedded filesystem long before the registry transition.
	// Mount re-reads templates after the loop; the installer keeps the
	// post-InitAll install authoritative.
	core.ThemeInstaller = sitessr.SetEmbeddedFS
	sitessr.SetEmbeddedFS(themeModule.ThemeFiles())
	sitessr.SetThemeFuncs(themeModule.ThemeFuncs())
	if pageProvider, ok := themeModule.(kernel.ThemePageProvider); ok {
		sitessr.SetThemePages(pageProvider.ThemePages())
	} else {
		sitessr.SetThemePages(nil)
	}

	// v1 module system (Phase 3.5c): the site-ssr engine registers its
	// lifecycle and content seam.
	core.Candidates = append(core.Candidates, sitessr.New())

	// Initialize theme for content site (templates and static files).
	// Built here — ahead of module registration — so the textgen module
	// adopts the active theme's styles instead of a nil placeholder.
	// Precedence: THEME_DIR wins (legacy manual override), then [theme]
	// active at THEMES_DIR/<name>, otherwise the embedded default. A broken
	// active theme logs a warning and falls back to embedded (never bricks
	// startup); the flag surfaces on /api/health.
	//
	// The validator composes the domain CSS/file checks with template parse +
	// smoke render against the real site-ssr stack. The same instance runs at
	// boot, on hot reload, and before install/activation.
	themeValidator := newThemeValidator(core.PostTypeSlugs)
	s.validator = themeValidator
	theme, themeFailed, themeErr := resolveActiveTheme(cfg, themeValidator)
	if themeErr != nil {
		log.Printf("Warning: %v, using embedded default", themeErr)
	}
	s.theme = theme
	core.ThemeValidationFailed = themeFailed

	// Read the active theme's minified base.css + style.css so AI-generated
	// HTML/CSS reuses the site's design tokens and component classes instead
	// of inventing off-brand styles.
	themeBaseCSS, themeStyleCSS := sitessr.ReadThemeStyles(s.theme)

	// Initialize AI text generation service
	var textGenService textgendomain.TextGenerationService
	if cfg.IsTextGenerationEnabled() {
		textGenService = textgeninfra.NewOpenAITextService(
			cfg.AITextGenerationAPIKey,
			cfg.AITextGenerationBaseURL,
			cfg.AITextGenerationModel,
			themeBaseCSS,
			themeStyleCSS,
		)
	}
	// v1 module system (Phase 3.6b): the real textgen module adopts the
	// OpenAI-compatible service and the media library for context injection.
	// Core mounts the handler explicitly (see Module.Handler): the
	// textGenEnabled feature flag gates mounting.
	s.textgenModule = textgenmodule.New(
		textgenmodule.WithService(textGenService),
		textgenmodule.WithMediaLister(core.MediaService),
		textgenmodule.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, s.textgenModule)
	hs.TextGen = s.textgenModule.Handler()

	// The refresher re-bakes the textgen HTML system prompt from the active
	// theme's styles after a hot reload. Without a generation service it
	// stays nil and reload skips it.
	if textGenService != nil {
		if svc, ok := textGenService.(*textgeninfra.OpenAITextService); ok {
			s.textgenRefresher = svc.RefreshThemeStyles
		}
	}

	// Initialize AI theme generation service. The system prompt grounds on the
	// active edition's readable sources (stylesheets always, page templates
	// when the brief opts in); without sources it generates from the distilled
	// contract alone. The edition-side validator adds template parse and
	// smoke-render checks to the domain file/CSS gate.
	themGenEdition := themegendomain.EditionBlog
	if s.themeProvider != nil && s.themeProvider.Name() == "theme-storefront" {
		themGenEdition = themegendomain.EditionStorefront
	}
	var themenService *themegendomain.Service
	if cfg.IsThemeGenerationEnabled() {
		themeSources := themeBaseCSS
		if themeStyleCSS != "" {
			themeSources += "\n--- style ---\n" + themeStyleCSS
		}
		generation := themeninfra.NewOpenAIThemeService(
			cmp.Or(cfg.AIThemGenAPIKey, cfg.AITextGenerationAPIKey),
			cfg.EffectiveThemGenBaseURL(),
			cfg.EffectiveThemGenModel(),
		)
		activation := NewThemeActivation(cfg)
		store := themenstore.NewStore(cfg.ThemesRoot(), func() string {
			return activation.ActiveTheme(context.Background())
		}, func() string {
			return cfg.ThemeDir
		})
		// The generation service receives the same material the prompts used
		// to bake in: the active stylesheets plus the edition's page
		// templates. The service decides per brief whether templates matter.
		sources := themegendomain.NewThemeSources(
			themeSources,
			readThemeTemplateSources(themeModule.ThemeFiles()),
		)
		themenService = themegendomain.NewService(
			generation,
			themeValidator,
			store,
			func(context.Context, themegendomain.Edition) themegendomain.ThemeSources {
				return sources
			},
			themGenEdition,
		)
	}
	// Live preview (v1.5): the cookie-signing key is derived from the
	// server secret once — the raw secret never leaves config — and Secure
	// mirrors the site origin. The preview cache starts empty.
	s.previewKey = themenapi.DerivePreviewKey(cfg.JWTSecret)
	s.previewSecure = strings.HasPrefix(strings.ToLower(cfg.SiteURL), "https://")
	s.previews = newPreviewCache()

	s.themegenModule = themenmodule.New(
		themenmodule.WithService(themenService),
		themenmodule.WithActivations(NewThemeActivation(cfg)),
		themenmodule.WithReloader(s),
		themenmodule.WithPreviewKey(s.previewKey, s.previewSecure),
		themenmodule.WithSnapshotRenderer(s),
		themenmodule.WithEdition(themGenEdition),
		themenmodule.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, s.themegenModule)
	hs.ThemGen = s.themegenModule.Handler()

	// Initialize speech-to-text service (voice dictation). The Meta adapter
	// is the default; the OpenAI-compatible adapter serves Whisper, Groq,
	// and self-hosted endpoints. Dictation needs the text generation
	// service too (the transcript is enhanced into content) — textGenService
	// is nil-safe here because the speech routes stay unmounted unless both
	// AI keys are configured.
	var speechService speechdomain.SpeechToTextService
	if cfg.IsSpeechToTextEnabled() {
		if cfg.AISpeechToTextProvider == config.SpeechToTextProviderOpenAI {
			speechService = speechinfra.NewOpenAISpeechService(
				cfg.AISpeechToTextAPIKey,
				cfg.AISpeechToTextBaseURL,
				cfg.AISpeechToTextModel,
				nil,
			)
		} else {
			speechService = speechinfra.NewMetaSpeechService(
				cfg.AISpeechToTextAPIKey,
				cfg.AISpeechToTextBaseURL,
				cfg.AISpeechToTextModel,
				nil,
			)
		}
	}
	// v1 module system: the real speech module adopts the transcription
	// service and the text generation service as its enhancer. Core mounts
	// the handler explicitly (see Module.Handler): the speechEnabled feature
	// flag gates mounting.
	s.speechModule = speechmodule.New(
		speechmodule.WithTranscriber(speechService),
		speechmodule.WithEnhancer(textGenService),
		speechmodule.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, s.speechModule)
	hs.Speech = s.speechModule.Handler()

	// Initialize WordPress importer (admin-only content migration)
	wordpressDownloader := wordpress.NewMediaDownloader(
		&http.Client{Timeout: 30 * time.Second},
		core.MediaService,
	)
	wordpressResolver := wordpress.NewUserResolver(
		NewWordPressUserProvisioner(core.UserRepo),
		modules.NewLogger(core.Logger),
	)
	wordpressImporter := wordpress.NewImporter(
		core.ContentService,
		wordpressDownloader,
		wordpressResolver,
		core.PostTypeService,
		core.PrimaryLanguage,
		core.Logger,
	)

	// Initialize Hugo importer (admin-only content migration)
	hugoImporter := hugo.NewImporter(
		core.ContentService,
		core.AliasService,
		core.ContentService,
		core.MediaService,
		wordpressDownloader,
		core.PrimaryLanguage,
		core.Logger,
	)

	// v1 module system (Phase 3.7d): the real importers module adopts both
	// importers and builds the handlers once, so the browser and agent
	// realms share the same instances and job stores. Core mounts the
	// handlers explicitly (see Module.WordPressHandler/HugoHandler): the
	// surface spans the browser admin group and the agent realm, which a
	// single RegisterRoutes call cannot express.
	s.importersModule = importers.New(
		importers.WithWordPressImporter(wordpressImporter),
		importers.WithHugoImporter(hugoImporter),
		importers.WithMaxImportBytes(cfg.ImportMaxSize()),
		importers.WithWordPressTimeout(cfg.WordPressImportTimeout),
		importers.WithHugoTimeout(cfg.HugoImportTimeout),
		importers.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, s.importersModule)
	hs.WordPress = s.importersModule.WordPressHandler()
	hs.Hugo = s.importersModule.HugoHandler()

	return nil
}

// mount builds the SSR surface after InitAll: templates, renderer, the
// content-page handler, static files, alias redirects, and the static-site
// generator. It needs the post-cascade comments flag, the public-field
// registry, and the installed theme artifact — all platform-set by now.
func (s *state) mount(core *app.Core) error {
	hs := core.Handlers()
	cfg := core.Cfg

	uiCatalog := loadUICatalog(core.Languages)
	s.snapshotCatalog = uiCatalog
	s.snapshotSlugs = core.PostTypeSlugs

	// Initialize content site templates and renderer
	contentTemplates, err := sitessr.NewTemplates(s.theme, uiCatalog, core.PostTypeSlugs...)
	if err != nil {
		log.Fatalf("Failed to load content templates: %v", err)
	}
	s.templates = contentTemplates
	tiptapRenderer := tiptap.NewRenderer(func(src string) []tiptap.ImageVariant {
		hash := mediadomain.ExtractHashFromURL(src)
		if hash == "" {
			return nil
		}
		m, err := core.MediaRepo.FindByHashPrefix(context.Background(), hash)
		if err != nil {
			log.Printf("WARNING: FindByHashPrefix failed for hash %q: %v", hash, err)
			return nil
		}
		if m == nil {
			return nil
		}
		return resolveBodyImageVariants(m)
	})
	userProvider := sitessr.NewUserRepoAdapter(func(ctx context.Context, username string) (*sitessr.UserBasicInfo, error) {
		user, err := core.UserRepo.GetUserByUsername(ctx, username)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, nil
		}
		var avatarURL string
		if user.ProfilePicture != "" {
			avatarURL = core.ProfilePicture.GetURL(user.ProfilePicture)
		}
		return &sitessr.UserBasicInfo{
			Name:           user.Name,
			Username:       user.Username,
			CustomFields:   user.CustomFields,
			ProfilePicture: avatarURL,
		}, nil
	})

	homepageSections, err := config.LoadHomepageSections(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load homepage sections config, continuing without sections: %v", err)
		homepageSections = nil
	}
	ssrSections := make([]sitessr.HomepageSection, 0, len(homepageSections))
	for _, section := range homepageSections {
		ssrSections = append(ssrSections, sitessr.HomepageSection{
			PostType:      section.PostType,
			Limit:         section.Limit,
			Offset:        section.Offset,
			Title:         section.Title,
			TitleID:       section.TitleID,
			Description:   section.Description,
			DescriptionID: section.DescriptionID,
		})
	}

	siteConfig, err := config.LoadSiteConfig(cfg)
	if err != nil {
		return err
	}
	ssrSiteConfig := siteSSRConfig(siteConfig)

	ssrReader := NewSiteSSRContentReader(core.ContentService, s.commentService)
	contentPageHandler := sitessr.NewContentPageHandler(
		ssrReader,
		core.PostTypeService,
		core.PostTypeService,
		userProvider,
		contentTemplates,
		tiptapRenderer,
		core.MediaRepo,
		core.Languages,
		ssrSections,
		ssrSiteConfig,
		cfg.PostPerPage,
		core.CommentsEnabled,
		core.RegistrationEnabled,
	)
	contentPageHandler = contentPageHandler.WithPublicFieldRegistry(core.PublicFieldRegistry)
	contentPageHandler = contentPageHandler.WithBaseURL(core.BaseURL)
	contentPageHandler = contentPageHandler.WithUICatalog(uiCatalog)

	// Legal chrome (round 6): the same boot-time resolution the consent
	// gates use drives the banner link, the consent checkboxes, and the nav
	// placement. Rendering still re-resolves the published page set per
	// request, so a newly published legal page links immediately.
	contentPageHandler.Assembler().WithLegal(sitessr.LegalOptions{
		Overrides:           core.LegalOverrides,
		Nav:                 core.LegalNav,
		CheckoutConsent:     core.CheckoutConsent,
		RegistrationConsent: core.RegistrationConsent,
		CookieNotice:        core.LegalNotice,
	})

	// The sanitizer's iframe allowlist mirrors the CSP frame-src directive
	// (platform-set) so embeds the CSP allows to be framed are also allowed
	// through the HTML sanitizer on every path.
	contentPageHandler = contentPageHandler.WithIFrameHosts(core.IFrameHosts...)

	// Live preview (v1.5): builds a full handler stack for an installed
	// theme on demand. The preview cache calls it on a miss; failures fall
	// back to the main surfaces, so a broken preview never breaks rendering.
	s.previewBuilder = func(theme *sitessr.Theme) (http.Handler, *sitessr.StaticFilesHandler, error) {
		previewTemplates, err := sitessr.NewTemplates(theme, uiCatalog, core.PostTypeSlugs...)
		if err != nil {
			return nil, nil, err
		}
		previewHandler := sitessr.NewContentPageHandler(
			ssrReader,
			core.PostTypeService,
			core.PostTypeService,
			userProvider,
			previewTemplates,
			tiptapRenderer,
			core.MediaRepo,
			core.Languages,
			ssrSections,
			ssrSiteConfig,
			cfg.PostPerPage,
			core.CommentsEnabled,
			core.RegistrationEnabled,
		)
		previewHandler = previewHandler.WithPublicFieldRegistry(core.PublicFieldRegistry)
		previewHandler = previewHandler.WithBaseURL(core.BaseURL)
		previewHandler = previewHandler.WithUICatalog(uiCatalog)
		previewHandler.Assembler().WithLegal(sitessr.LegalOptions{
			Overrides:           core.LegalOverrides,
			Nav:                 core.LegalNav,
			CheckoutConsent:     core.CheckoutConsent,
			RegistrationConsent: core.RegistrationConsent,
			CookieNotice:        core.LegalNotice,
		})
		previewHandler = previewHandler.WithIFrameHosts(core.IFrameHosts...)
		return previewHandler, sitessr.StaticFiles(theme), nil
	}

	// Static file handler for content site (uses theme from above). The
	// concrete handler is kept so Reload can swap its filesystem in place.
	// Preview-cookie requests dispatch to the cached preview files instead.
	s.staticFiles = sitessr.StaticFiles(s.theme)
	hs.Static = &previewStaticDispatch{main: s.staticFiles, state: s}

	// Alias redirect handler wraps the content site catch-all to serve 301s.
	// Theme root files (theme's root/ directory, e.g. webpushr-sw.js) sit
	// between the alias lookup and the content renderer so they are served at
	// the site root on the dynamic server too. The wrapper is kept so Reload
	// can swap its filesystem in place. Preview-cookie requests dispatch to
	// the cached preview pages instead; root files are not previewed.
	pageHandler := &previewPageDispatch{main: contentPageHandler, state: s}
	s.rootHandler = sitessr.RootFilesHandler(s.theme, pageHandler)
	hs.AliasRedirect = handlers.NewAliasRedirectHandler(
		core.AliasService,
		core.ContentService,
		s.rootHandler,
	)

	// Initialize static site generator (Phase 3) — generates a fully rendered
	// static site with AMP variants as a tar.gz archive. Aliases are attached
	// so legacy URLs ship as meta-refresh redirect pages.
	ssgGenerator := ssg.NewGenerator(
		contentPageHandler.Assembler(),
		contentTemplates,
		ssrReader,
		filepath.Join("data", "uploads", "media"),
		s.theme,
		cfg.SiteURL,
	).WithAliases(core.AliasService)
	hs.SSG = &ssgHandler{generator: ssgGenerator, logger: core.Logger}

	return nil
}

// Edition returns the blog edition with the embedded default theme.
func Edition() app.Edition {
	return EditionWithTheme(themedefault.New())
}

// EditionWithTheme returns the blog edition with a caller-supplied theme
// provider. The commerce edition swaps in the storefront theme; blog itself
// uses the default artifact.
func EditionWithTheme(themeProvider kernel.ThemeProvider) app.Edition {
	s := &state{themeProvider: themeProvider}
	return app.Edition{
		Name:     "blog",
		Comments: true,
		Wire:     s.wire,
		Mount:    s.mount,
	}
}
