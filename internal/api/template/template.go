package template

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"

	"github.com/aristorinjuang/lesstruct/internal/i18n"
)

//go:embed all:static
var staticFS embed.FS

//go:embed all:pages
var pagesFS embed.FS

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

type LayoutData struct {
	Title              string
	Description        string
	OGTitle            string
	OGDesc             string
	OGImage            string
	PageTitle          string
	NavigationItems    []NavigationItem
	CurrentPath        string
	Lang               string
	LanguageLinks      []LanguageLink
}

type PostItem struct {
	Slug            string
	Title           string
	MetaDescription string
	ImageURL        string
	ImageSrcset     string
	ImageSizes      string
	Author          string
	Username        string
	AuthorAvatarURL string
	CreatedAt       string
}

type IndexData struct {
	LayoutData
	Posts []PostItem
	Tags  []string
}

type FormattedField struct {
	Label string
	Value string
}

type CommentItem struct {
	Author    string
	Text      string
	CreatedAt string
}

type ContentData struct {
	LayoutData
	Slug                  string
	Body                  template.HTML
	Tags                  []string
	Author                string
	Username              string
	AuthorAvatarURL       string
	CreatedAt             string
	AllowComments         bool
	CustomFields          map[string]any
	CustomFieldsFormatted []FormattedField
	Comments              []CommentItem
}

type AuthorData struct {
	LayoutData
	AuthorName             string
	Username               string
	AuthorAvatarURL        string
	Posts                  []PostItem
	CustomFieldsFormatted []FormattedField
}

type TagData struct {
	LayoutData
	TagName string
	Posts   []PostItem
}

type AuthPageData struct {
	LayoutData
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
	content        *template.Template
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

func (t *Templates) RenderContent(w http.ResponseWriter, data ContentData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.content.Execute(w, data)
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

func NewTemplates(theme *Theme, catalog *i18n.Catalog) (*Templates, error) {
	tFunc := func(lang, key string) string { return key }
	if catalog != nil {
		tFunc = catalog.T
	}

	layout := template.Must(template.New("layout").Funcs(template.FuncMap{
		"urlpath": url.PathEscape,
		"t":       tFunc,
	}).Parse(readThemeFile(theme, "layout.html")))

	t := &Templates{
		layout:  layout,
		catalog: catalog,
	}

	t.index = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "index.html")))
	t.content = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "content.html")))
	t.author = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "author.html")))
	t.tag = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "tag.html")))
	t.notFound = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "not_found.html")))
	t.login = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "login.html")))
	t.register = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "register.html")))
	t.forgotPassword = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "forgot_password.html")))
	t.verifyEmail = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "verify_email.html")))
	t.resetPassword = template.Must(template.Must(t.layout.Clone()).Parse(readThemeFile(theme, "reset_password.html")))

	return t, nil
}

// StaticFiles returns an http.Handler that serves the content site's static
// assets (CSS, JS). When a non-nil Theme with a non-empty Dir is provided,
// files on disk in that directory are served first, falling back to the
// embedded defaults for any file not present in the theme directory.
func StaticFiles(theme *Theme) http.Handler {
	handlerFS := resolveFS(theme, staticFS, "static")

	return http.FileServer(http.FS(handlerFS))
}
