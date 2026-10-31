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
	textgenmodule "github.com/aristorinjuang/lesstruct/modules/textgen"
	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	textgeninfra "github.com/aristorinjuang/lesstruct/modules/textgen/infrastructure"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
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

// state carries the edition-built services and modules between the Wire and
// Mount hooks. Everything here is blog-specific — none of it exists in the
// leaner editions' binaries.
type state struct {
	commentService *commentdomain.Service

	commentsModule  *comments.Module
	seoModule       *seo.Module
	textgenModule   *textgenmodule.Module
	importersModule *importers.Module

	theme *sitessr.Theme

	themeProvider kernel.ThemeProvider
}

// wire constructs the blog modules on top of the platform services and fills
// the optional handler surfaces the edition owns.
func (s *state) wire(core *app.Core) error {
	hs := core.Handlers()
	cfg := core.Cfg

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
	if cfg.ThemeDir != "" {
		s.theme = &sitessr.Theme{Dir: cfg.ThemeDir}
	}

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

	// Initialize content site templates and renderer
	contentTemplates, err := sitessr.NewTemplates(s.theme, uiCatalog, core.PostTypeSlugs...)
	if err != nil {
		log.Fatalf("Failed to load content templates: %v", err)
	}
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
		variants := m.Variants
		if len(variants) == 0 {
			return nil
		}
		result := make([]tiptap.ImageVariant, 0, len(variants))
		for _, v := range variants {
			result = append(result, tiptap.ImageVariant{
				URL:    v.URL,
				Width:  v.Width,
				Height: v.Height,
			})
		}
		slices.SortFunc(result, func(a, b tiptap.ImageVariant) int {
			return cmp.Compare(a.Width, b.Width)
		})
		if m.URL != "" && m.Width > 0 && m.Width > result[len(result)-1].Width {
			result = append(result, tiptap.ImageVariant{
				URL:    m.URL,
				Width:  m.Width,
				Height: m.Height,
			})
		}
		return result
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

	// Static file handler for content site (uses theme from above)
	hs.Static = sitessr.StaticFiles(s.theme)

	// Alias redirect handler wraps the content site catch-all to serve 301s.
	// Theme root files (theme's root/ directory, e.g. webpushr-sw.js) sit
	// between the alias lookup and the content renderer so they are served at
	// the site root on the dynamic server too.
	hs.AliasRedirect = handlers.NewAliasRedirectHandler(
		core.AliasService,
		core.ContentService,
		sitessr.RootFilesHandler(s.theme, contentPageHandler),
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
