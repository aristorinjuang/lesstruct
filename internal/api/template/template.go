package template

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/i18n"
)

//go:embed all:static
var staticFS embed.FS

//go:embed all:pages
var pagesFS embed.FS

// ComputeAssetHash reads base.css and style.css from the resolved static
// filesystem and returns a short combined hex hash for filename-based cache
// versioning. When either file is not found (e.g. broken theme), it returns
// "dev".
func ComputeAssetHash(theme *Theme) string {
	resolved := resolveFS(theme, staticFS, "static")

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
	resolved := resolveFS(theme, staticFS, "static")

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
// and style.css (and any future hashed assets).
type staticHandler struct {
	fs        fs.FS
	assetHash string
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	http.FileServer(http.FS(h.fs)).ServeHTTP(w, r)
}

type NavigationItem struct {
	Title    string
	URL      string
	IsActive bool
}

type LanguageLink struct {
	Code string
	Name string
	URL  string
}

// SiteConfig carries the site-wide identity (name + optional logo) read from
// the optional [site_config] block in config.toml. It is the same on every
// page, so it lives on LayoutData rather than per-page data. Name is always
// populated (the handler defaults it to the application name); Logo is empty
// unless the operator configures one, in which case themes render an <img>
// (using Name as the alt text) instead of the name as text.
type SiteConfig struct {
	Name string
	Logo string
}

type LayoutData struct {
	Title           string
	Description     string
	OGTitle         string
	OGDesc          string
	OGImage         string
	PageTitle       string
	NavigationItems []NavigationItem
	CurrentPath     string
	Lang            string
	LanguageLinks   []LanguageLink
	SiteConfig      SiteConfig
}

type PostItem struct {
	Slug            string
	Title           string
	MetaDescription string
	ImageURL        string
	ImageSrcset     string
	ImageSizes      string
	ImageVariants   map[string]string
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
	Related               []PostItem
	Comments              []CommentItem
	PostType              string
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

type Templates struct {
	layout         *template.Template
	index          *template.Template
	home           *template.Template
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

func NewTemplates(theme *Theme, catalog *i18n.Catalog, postTypeSlugs ...string) (*Templates, error) {
	tFunc := func(lang, key string) string { return key }
	if catalog != nil {
		tFunc = catalog.T
	}

	assetHash := ComputeAssetHash(theme)

	layout := template.Must(template.New("layout").Funcs(template.FuncMap{
		"urlpath": url.PathEscape,
		"t":       tFunc,
		"assetURL": func(name string) string {
			ext := ".css"
			base := strings.TrimSuffix(name, ext)
			return "/static/" + base + "." + assetHash + ext
		},
		"formatDate":     FormatDate,
		"formatDateTime": FormatDateTime,
	}).Parse(readThemeFile(theme, "layout.html")))

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

	return t, nil
}

// StaticFiles returns an http.Handler that serves the content site's static
// assets (CSS, JS). When a non-nil Theme with a non-empty Dir is provided,
// files on disk in that directory are served first, falling back to the
// embedded defaults for any file not present in the theme directory.
func StaticFiles(theme *Theme) http.Handler {
	handlerFS := resolveFS(theme, staticFS, "static")
	assetHash := ComputeAssetHash(theme)

	return &staticHandler{
		fs:        handlerFS,
		assetHash: assetHash,
	}
}
