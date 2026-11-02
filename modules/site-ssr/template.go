package sitessr

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/i18n"
)

var (
	// embeddedFS is the default theme artifact filesystem installed once at
	// startup from the theme-default module (layouts under pages/, static assets
	// under static/). THEME_DIR overrides resolve over it via resolveFS; without
	// an installed theme only on-disk files resolve.
	embeddedFS fs.FS

	// themeFuncs holds the extra template funcs contributed by the installed
	// theme. Engine funcs always win on name collisions, so a theme cannot break
	// the layout contract.
	themeFuncs template.FuncMap

	// themePages lists the standalone pages the installed theme serves at fixed
	// public paths (e.g. "cart" renders /cart from pages/cart.gohtml). The
	// engine owns no page names; themes declare what they ship.
	themePages []string
)

// standalonePageTemplate maps a theme page key to its template filename:
// "cart" -> "cart.html", "orders/track" -> "orders-track.html".
func standalonePageTemplate(page string) string {
	return strings.ReplaceAll(page, "/", "-") + ".html"
}

// inlineThemeStyles concatenates the resolved base.css and style.css into a
// single style element for layouts to inline instead of render-blocking on.
// It reuses ReadThemeStyles, so THEME_DIR overrides resolve exactly like the
// linked stylesheets they replace. A missing file contributes nothing; the
// stylesheets never contain a closing style tag (pinned by tests), so no
// escaping is needed.
func inlineThemeStyles(theme *Theme) template.HTML {
	baseCSS, styleCSS := ReadThemeStyles(theme)
	if baseCSS == "" && styleCSS == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("<style>")
	b.WriteString(baseCSS)
	b.WriteString(styleCSS)
	b.WriteString("</style>")
	return template.HTML(b.String())
}

// SetEmbeddedFS installs the default theme artifact filesystem. The
// composition root calls it once after resolving the ThemeProvider; test
// fixtures call it with fixture filesystems.
func SetEmbeddedFS(files fs.FS) {
	embeddedFS = files
}

// SetThemeFuncs installs the active theme's extra template funcs. The
// composition root calls it once alongside SetEmbeddedFS. Passing nil keeps
// only the engine funcs.
func SetThemeFuncs(funcs template.FuncMap) {
	themeFuncs = funcs
}

// SetThemePages installs the standalone page paths the active theme serves.
// Orders is preserved; the composition root calls it once alongside
// SetEmbeddedFS. Passing nil means the theme ships no standalone pages.
func SetThemePages(pages []string) {
	themePages = pages
}

// ComputeAssetHash reads base.css and style.css from the resolved static
// filesystem and returns a short combined hex hash for filename-based cache
// versioning. When either file is not found (e.g. broken theme), it returns
// "dev".
func ComputeAssetHash(theme *Theme) string {
	resolved := resolveFS(theme, embeddedFS, "static")
	if resolved == nil {
		return "dev"
	}

	baseData, err1 := fs.ReadFile(resolved, "base.css")
	styleData, err2 := fs.ReadFile(resolved, "style.css")
	if err1 != nil && err2 != nil {
		return "dev"
	}

	h := sha256.New()
	h.Write(baseData)
	h.Write(styleData)
	return fmt.Sprintf("%x", h.Sum(nil)[:6])
}

// ReadThemeStyles reads base.css and style.css from the resolved static
// filesystem (theme overrides first, embedded defaults as fallback). The
// returned CSS is injected into AI text generation so generated HTML reuses
// the site's design tokens and component classes instead of inventing
// off-brand styles. A missing file yields an empty string for that slot —
// callers handle the empty case gracefully.
func ReadThemeStyles(theme *Theme) (baseCSS, styleCSS string) {
	resolved := resolveFS(theme, embeddedFS, "static")
	if resolved == nil {
		return "", ""
	}

	if data, err := fs.ReadFile(resolved, "base.css"); err == nil {
		baseCSS = string(data)
	}

	if data, err := fs.ReadFile(resolved, "style.css"); err == nil {
		styleCSS = string(data)
	}

	return baseCSS, styleCSS
}

// staticHandler rewrites versioned CSS requests (<name>.<hash>.css) to serve
// <name>.css with immutable Cache-Control headers. It handles both base.css
// and style.css (and any future hashed assets). Every other file gets a
// bounded one-day TTL: plain /static/ URLs change across releases, so they
// must not be immutable.
type staticHandler struct {
	fs        fs.FS
	assetHash string
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.fs == nil {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if h.assetHash != "" && strings.HasSuffix(path, ".css") {
		hashSuffix := "." + h.assetHash + ".css"
		if strings.Contains(path, hashSuffix) {
			originalName := strings.Replace(path, hashSuffix, ".css", 1)
			r2 := *r
			r2.URL = &url.URL{Path: originalName}
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.FileServer(http.FS(h.fs)).ServeHTTP(w, &r2)
			return
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.FileServer(http.FS(h.fs)).ServeHTTP(w, r)
}

type NavigationItem struct {
	Title    string
	URL      string
	IsActive bool
}

// CookieNotice is the consent banner's notice split around its policy link.
// Every piece is a plain string, so the layout escapes each one and no
// translation can inject markup. An empty URL renders the link label as
// plain text: the notice keeps its meaning without a dead link.
type CookieNotice struct {
	Before   string
	LinkText string
	After    string
	URL      string
}

// LegalData is the page-wide legal projection: where each conventional
// document lives (resolved, overridden, or empty when absent) and whether
// checkout and registration must demand an affirmative consent act. The
// consent gates mirror the server-side enforcement exactly — both read the
// same boot-time resolution.
type LegalData struct {
	CookiePolicyURL     string
	PrivacyPolicyURL    string
	TermsOfServiceURL   string
	ReturnsPolicyURL    string
	CookieNotice        CookieNotice
	CheckoutConsent     bool
	RegistrationConsent bool
}

type LanguageLink struct {
	Code string
	Name string
	URL  string
}

// SiteConfig carries the site-wide identity (name, optional logo, optional
// favicon, optional product image ratio, optional hero) read from the optional
// [site_config] block in config.toml. It is the same on every page, so it lives on
// LayoutData rather than per-page data. Name is always populated (the handler
// defaults it to the application name); Logo is empty unless the operator
// configures one, in which case themes render an <img> (using Name as the alt
// text) instead of the name as text. Favicon is likewise empty unless
// configured, in which case themes use it for the icon links instead of their
// bundled favicon set. ProductImageRatio is the canonical "W / H" form
// validated at load (empty means the theme's hard-coded 4/5 applies); the
// storefront layout emits it as an inline aspect-ratio override. HeroImage,
// HeroTitle, and HeroSubtitle are the optional storefront hero overrides: an
// empty HeroImage keeps the monogram panel, a set one renders the photo
// instead; empty HeroTitle/HeroSubtitle keep the bilingual defaults. It is typed
// as template.CSS so the layout style block renders verbatim.
type SiteConfig struct {
	Name              string
	Logo              string
	Favicon           string
	ProductImageRatio template.CSS
	HeroImage         string
	HeroTitle         string
	HeroSubtitle      string
}

type LayoutData struct {
	Title           string
	Description     string
	OGTitle         string
	OGDesc          string
	OGImage         string
	PageTitle       string
	NavigationItems []NavigationItem
	// FooterItems is the full site map the footer lists: every published page
	// (conventional legal pages included) and every listing post type.
	FooterItems   []NavigationItem
	Legal         LegalData
	CurrentPath   string
	Lang            string
	LanguageLinks   []LanguageLink
	Languages       []LanguageLink
	SiteConfig      SiteConfig
	// NoIndex asks the layout to emit a robots noindex directive. Standalone
	// application pages (cart, checkout, order tracking) set it because their
	// server-rendered shells carry no indexable content.
	NoIndex bool
}

type PostItem struct {
	Slug            string
	Title           string
	MetaDescription string
	ImageURL        string
	ImageSrcset     string
	ImageSizes      string
	ImageVariants   map[string]string
	// ImageMobileURL is the rendition mobile browsers (<=767px) must load:
	// the _thumb variant when the pipeline generated one, _medium
	// otherwise. Empty when no variants exist (templates omit the mobile
	// source then). It guarantees small viewports fetch the smallest
	// rendition (srcset alone cannot promise that: a high-DPR phone
	// resolves a 342px slot against the 616w entry).
	ImageMobileURL  string
	OriginalURL     string
	Author          string
	Username        string
	AuthorAvatarURL string
	CreatedAt       time.Time
	PostType        string
	Tags            []string
	CustomFields    map[string]any
}

// HomeSection is a per-post-type grouping rendered on the homepage. It is only
// populated when [[homepage_section]] blocks are configured in config.toml; an
// empty slice means the theme should fall back to the flat .Posts list.
type HomeSection struct {
	PostTypeSlug string
	Title        string
	Description  string
	URL          string
	Posts        []PostItem
}

// PaginationData carries prev/next state for paginated public listings. It is
// embedded in IndexData, AuthorData, and TagData so templates reach the fields
// directly (e.g. {{.NextURL}}). HasNext is derived from the fetch-limit+1
// trick, so no COUNT query is required.
type PaginationData struct {
	CurrentPage int
	HasPrev     bool
	HasNext     bool
	PrevURL     string
	NextURL     string
}

type IndexData struct {
	LayoutData
	Posts    []PostItem
	Tags     []string
	Sections []HomeSection
	PaginationData
}

type FormattedField struct {
	Label string
	Value string
}

type CommentItem struct {
	Author    string
	Text      string
	CreatedAt time.Time
}

// GalleryImage is one resolved gallery custom-field entry: the stored URL
// and alt text plus the thumbnail-pipeline renditions. Thumb/Medium/Large
// carry the variant URLs keyed by suffix name (never pixel widths, so they
// adapt to any [[thumbnail]] config); each falls back to the original URL
// when the media record is missing or has no such variant (external URLs
// always render as-is). Srcset spreads whatever variants exist. URL is the
// canonical display URL (the media record's URL when resolved, else the
// stored URL); Original pins that same fallback target explicitly so
// themes can always recover the unmodified source.
type GalleryImage struct {
	URL      string
	Alt      string
	Thumb    string
	Medium   string
	Large    string
	Srcset   string
	Original string
}

type ContentData struct {
	LayoutData
	Slug                  string
	Body                  template.HTML
	Tags                  []string
	Author                string
	Username              string
	AuthorAvatarURL       string
	CreatedAt             time.Time
	AllowComments         bool
	CustomFields          map[string]any
	CustomFieldsFormatted []FormattedField
	// CustomFieldGalleries carries the resolved gallery-typed custom fields,
	// keyed by field slug in schema order. Themes prefer it over the raw
	// lines fallback for gallery fields (e.g. product images).
	CustomFieldGalleries  map[string][]GalleryImage
	Related               []PostItem
	Comments              []CommentItem
	PostType              string
	PostScripts           template.HTML
}

type AuthorData struct {
	LayoutData
	AuthorName            string
	Username              string
	AuthorAvatarURL       string
	Posts                 []PostItem
	CustomFieldsFormatted []FormattedField
	PaginationData
}

type TagData struct {
	LayoutData
	TagName string
	Posts   []PostItem
	PaginationData
}

type AuthPageData struct {
	LayoutData
	// ShowRegister toggles the "create account" link on the login page. It is
	// false when self-registration is disabled (comments off), so visitors are
	// not pointed at a registration form that always fails.
	ShowRegister bool
}

type NotFoundData struct {
	LayoutData
}

type VerifyEmailData struct {
	LayoutData
}

type ResetPasswordData struct {
	LayoutData
}

// StandalonePageData is the render context for a theme-declared standalone
// page (cart, checkout, order tracking). The shell is server-rendered for
// consistent chrome and caching; the commerce data hydrates client-side from
// the public JSON APIs. Page is the theme's page key (e.g. "orders/track").
type StandalonePageData struct {
	LayoutData
	Page string
}

type Templates struct {
	layout         *template.Template
	index          *template.Template
	home           *template.Template
	pages          map[string]*template.Template
	content        map[string]*template.Template
	contentBySlug  map[string]*template.Template
	contentDefault *template.Template
	author         *template.Template
	tag            *template.Template
	notFound       *template.Template
	login          *template.Template
	register       *template.Template
	forgotPassword *template.Template
	verifyEmail    *template.Template
	resetPassword  *template.Template
	catalog        *i18n.Catalog
}

func (t *Templates) RenderIndex(w http.ResponseWriter, data IndexData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.index.Execute(w, data)
}

func (t *Templates) RenderHome(w http.ResponseWriter, data IndexData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.home.Execute(w, data)
}

// HasPage reports whether the installed theme declares a standalone page at
// the given key ("cart", "checkout", "orders/track"). The handler checks it
// before falling through to content so a declared page always renders.
func (t *Templates) HasPage(page string) bool {
	_, ok := t.pages[page]
	return ok
}

// RenderPage renders one theme-declared standalone page. The page key must
// be declared by the theme; NewTemplates already skipped undeclared or
// templateless pages, so a miss here is a wiring bug, not a 404.
func (t *Templates) RenderPage(w http.ResponseWriter, data StandalonePageData) error {
	tpl, ok := t.pages[data.Page]
	if !ok {
		return fmt.Errorf("theme does not declare the %q page", data.Page)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tpl.Execute(w, data)
}

func (t *Templates) RenderContent(w http.ResponseWriter, data ContentData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if data.PostType != "" && data.Slug != "" {
		if tpl, ok := t.contentBySlug[data.PostType+":"+data.Slug]; ok {
			return tpl.Execute(w, data)
		}
	}

	tpl := t.contentDefault
	if specific, ok := t.content[data.PostType]; ok {
		tpl = specific
	}
	return tpl.Execute(w, data)
}

func (t *Templates) RenderAuthor(w http.ResponseWriter, data AuthorData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.author.Execute(w, data)
}

func (t *Templates) RenderTag(w http.ResponseWriter, data TagData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.tag.Execute(w, data)
}

func (t *Templates) RenderNotFound(w http.ResponseWriter, data NotFoundData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	return t.notFound.Execute(w, data)
}

func (t *Templates) RenderLogin(w http.ResponseWriter, data AuthPageData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.login.Execute(w, data)
}

func (t *Templates) RenderRegister(w http.ResponseWriter, data AuthPageData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.register.Execute(w, data)
}

func (t *Templates) RenderForgotPassword(w http.ResponseWriter, data AuthPageData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.forgotPassword.Execute(w, data)
}

func (t *Templates) RenderVerifyEmail(w http.ResponseWriter, data VerifyEmailData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.verifyEmail.Execute(w, data)
}

func (t *Templates) RenderResetPassword(w http.ResponseWriter, data ResetPasswordData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.resetPassword.Execute(w, data)
}

func (t *Templates) renderToString(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (t *Templates) RenderIndexToString(data IndexData) (string, error) {
	return t.renderToString(t.index, data)
}

func (t *Templates) RenderHomeToString(data IndexData) (string, error) {
	return t.renderToString(t.home, data)
}

func (t *Templates) RenderContentToString(data ContentData) (string, error) {
	if data.PostType != "" && data.Slug != "" {
		if tpl, ok := t.contentBySlug[data.PostType+":"+data.Slug]; ok {
			return t.renderToString(tpl, data)
		}
	}

	tmpl := t.contentDefault
	if specific, ok := t.content[data.PostType]; ok {
		tmpl = specific
	}
	return t.renderToString(tmpl, data)
}

func (t *Templates) RenderAuthorToString(data AuthorData) (string, error) {
	return t.renderToString(t.author, data)
}

func (t *Templates) RenderTagToString(data TagData) (string, error) {
	return t.renderToString(t.tag, data)
}

func (t *Templates) RenderNotFoundToString(data NotFoundData) (string, error) {
	return t.renderToString(t.notFound, data)
}

func (t *Templates) RenderLoginToString(data AuthPageData) (string, error) {
	return t.renderToString(t.login, data)
}

func (t *Templates) RenderRegisterToString(data AuthPageData) (string, error) {
	return t.renderToString(t.register, data)
}

// NewTemplates builds the template set from the theme filesystem and the
// i18n catalog. It loads the shared layout, the fixed pages, the content
// templates (per post type plus per-slug overrides), and the standalone pages
// the theme declares through ThemePageProvider.
func NewTemplates(theme *Theme, catalog *i18n.Catalog, postTypeSlugs ...string) (*Templates, error) {
	tFunc := func(lang, key string) string { return key }
	if catalog != nil {
		tFunc = catalog.T
	}

	assetHash := ComputeAssetHash(theme)

	inlineCSS := inlineThemeStyles(theme)

	funcs := template.FuncMap{
		"urlpath": url.PathEscape,
		"t":       tFunc,
		"upper":   strings.ToUpper,
		"assetURL": func(name string) string {
			ext := ".css"
			base := strings.TrimSuffix(name, ext)
			return "/static/" + base + "." + assetHash + ext
		},
		"inlineStyles": func() template.HTML {
			return inlineCSS
		},
		"formatDate":     FormatDate,
		"formatDateTime": FormatDateTime,
	}
	// Theme funcs extend the engine vocabulary; engine names always win so a
	// theme cannot shadow urlpath, t, upper, assetURL, inlineStyles, or the
	// date formatters.
	for name, fn := range themeFuncs {
		if _, exists := funcs[name]; !exists {
			funcs[name] = fn
		}
	}

	layout := template.New("layout").Funcs(funcs)
	for _, source := range readLayoutSources(theme) {
		layout = template.Must(layout.Parse(source))
	}

	t := &Templates{
		layout:  layout,
		catalog: catalog,
	}

	t.index = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "index.html")))
	t.home = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "homepage.html")))
	t.author = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "author.html")))
	t.tag = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "tag.html")))
	t.notFound = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "not_found.html")))
	t.login = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "login.html")))
	t.register = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "register.html")))
	t.forgotPassword = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "forgot_password.html")))
	t.verifyEmail = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "verify_email.html")))
	t.resetPassword = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "reset_password.html")))

	// Load the default content template (post.gohtml).
	t.contentDefault = template.Must(template.Must(t.layout.Clone()).Parse(readContentTemplate(theme, "post")))

	// Load per-post-type content templates.
	t.content = make(map[string]*template.Template)
	if len(postTypeSlugs) == 0 {
		postTypeSlugs = []string{"post"}
	}
	for _, slug := range postTypeSlugs {
		if slug == "post" {
			continue
		}
		tplContent := readContentTemplate(theme, slug)
		t.content[slug] = template.Must(template.Must(t.layout.Clone()).Parse(tplContent))
	}

	// Load per-slug content template overrides (e.g. page-about.html). These
	// take precedence over the per-post-type templates above when both the
	// post type and the slug match. Files without a matching post-type prefix
	// are ignored; per-post-type templates themselves (e.g. page.html) do not
	// have a hyphen and so are skipped.
	knownPostTypes := make(map[string]bool, len(postTypeSlugs)+1)
	knownPostTypes["post"] = true
	for _, slug := range postTypeSlugs {
		knownPostTypes[slug] = true
	}
	t.contentBySlug = make(map[string]*template.Template)
	for key, tplContent := range findPerSlugTemplateOverrides(theme, knownPostTypes) {
		t.contentBySlug[key] = template.Must(template.Must(t.layout.Clone()).Parse(tplContent))
	}

	// Load the standalone pages the theme declares (cart, checkout, order
	// tracking). They render through the shared layout so chrome and site
	// config stay consistent; their data hydrates client-side. A declared
	// page without a template is skipped, so a partial theme still boots and
	// the path falls through to content/404.
	t.pages = make(map[string]*template.Template, len(themePages))
	for _, page := range themePages {
		page = strings.Trim(page, "/")
		if page == "" {
			continue
		}
		body := readThemeFile(theme, standalonePageTemplate(page))
		if body == "" {
			continue
		}
		t.pages[page] = template.Must(template.Must(t.layout.Clone()).Parse(body))
	}

	return t, nil
}

// StaticFiles returns an http.Handler that serves the content site's static
// assets (CSS, JS). When a non-nil Theme with a non-empty Dir is provided,
// files on disk in that directory are served first, falling back to the
// embedded defaults for any file not present in the theme directory.
func StaticFiles(theme *Theme) http.Handler {
	handlerFS := resolveFS(theme, embeddedFS, "static")
	assetHash := ComputeAssetHash(theme)

	return &staticHandler{
		fs:        handlerFS,
		assetHash: assetHash,
	}
}

// EmbeddedStaticFS returns the built-in static assets served under /static/
// when no theme overrides them. The SSG exporter copies this tree as the base
// layer of the exported /static/ directory. A nil install yields nil.
func EmbeddedStaticFS() fs.FS {
	if embeddedFS == nil {
		return nil
	}
	sub, _ := fs.Sub(embeddedFS, "static")

	return sub
}

// ThemeStaticFS returns the active theme's static subtree on disk, or nil when
// no theme is configured or the theme ships no static directory. The SSG
// exporter copies it over the embedded layer so theme files win on name
// conflicts, mirroring the live server's resolution order.
func ThemeStaticFS(theme *Theme) fs.FS {
	if theme == nil || theme.Dir == "" {
		return nil
	}

	dir := filepath.Join(theme.Dir, "static")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	return os.DirFS(dir)
}
