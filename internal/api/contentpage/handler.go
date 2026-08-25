package contentpage

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	tpl "github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
)

const (
	// defaultPostsPerPage is the page size used when the configured POSTS_PER_PAGE
	// is zero or invalid. Public listings fetch postsPerPage+1 rows so they can
	// detect HasNext without a COUNT query, then trim back to postsPerPage.
	defaultPostsPerPage = 50

	// defaultHomeSectionLimit is the number of items shown in a homepage section
	// when its [[homepage_section]] limit is unset.
	defaultHomeSectionLimit = 6

	postCardSizes = "(min-width: 1200px) 370px, (min-width: 768px) calc(50vw - 3rem), calc(100vw - 3rem)"

	// defaultSiteName is the site name used when [site_config].name is not
	// configured. It keeps the embedded theme's branding stable for an
	// out-of-the-box install; operators override it via config.toml.
	defaultSiteName = "Lesstruct"
)

func isEmptyValue(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	}
	return false
}

func formatFieldValue(fieldType customfield.FieldType, val any, lang string) string {
	switch fieldType {
	case customfield.FieldTypeCheckbox:
		if b, ok := val.(bool); ok && b {
			return "Yes"
		}
	case customfield.FieldTypeDate:
		if s, ok := val.(string); ok {
			if t, err := time.Parse("2006-01-02", s); err == nil {
				return tpl.FormatDate(lang, t)
			}
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return tpl.FormatDate(lang, t)
			}
			return s
		}
	case customfield.FieldTypeDatetime:
		if s, ok := val.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return tpl.FormatDateTime(lang, t)
			}
			return s
		}
	}
	return fmt.Sprintf("%v", val)
}

func formatCustomFields(
	fields []customfield.FieldSchema,
	values map[string]any,
	lang string,
) []tpl.FormattedField {
	result := make([]tpl.FormattedField, 0, len(fields))
	for _, f := range fields {
		if f.Slug == customfield.PostScriptSlug {
			continue
		}
		val, exists := values[f.Slug]
		if !exists || isEmptyValue(val) {
			continue
		}
		result = append(result, tpl.FormattedField{
			Label: f.Name,
			Value: formatFieldValue(f.Type, val, lang),
		})
	}
	return result
}

func buildImageSrcset(variants map[string]mediadomain.MediaVariant) string {
	if len(variants) == 0 {
		return ""
	}
	type entry struct {
		url   string
		width int
	}
	parts := make([]entry, 0, len(variants))
	for _, v := range variants {
		parts = append(parts, entry{url: v.URL, width: v.Width})
	}
	slices.SortFunc(parts, func(a, b entry) int {
		return cmp.Compare(a.width, b.width)
	})
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%s %dw", p.url, p.width)
	}
	return sb.String()
}

// parsePage extracts a 1-based page number from the ?page= query parameter,
// clamped to a minimum of 1. Missing or invalid values default to page 1.
func parsePage(r *http.Request) int {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// parseYear extracts a 4-digit year from the ?year= query parameter. Missing
// or invalid values default to 0 (meaning "no year filter").
func parseYear(r *http.Request) int {
	raw := r.URL.Query().Get("year")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 9999 {
		return 0
	}
	return n
}

// parseMonth extracts a month (1-12) from the ?month= query parameter. Missing
// or invalid values default to 0 (meaning "no month filter").
func parseMonth(r *http.Request) int {
	raw := r.URL.Query().Get("month")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 12 {
		return 0
	}
	return n
}

// archiveQuery builds the extra query-string fragment used to preserve
// ?year= and ?month= across pagination links. Returns "" when no filter is
// active so buildPagination produces clean URLs (backward compatible).
func archiveQuery(year, month int) string {
	if year > 0 && month > 0 {
		return fmt.Sprintf("year=%d&month=%d", year, month)
	}
	return ""
}

// buildPagination assembles prev/next state from the current page and the
// HasNext flag produced by the fetch-limit+1 probe. baseURL is the bare path
// (e.g. "/authors/admin"); page 1 always links to the bare URL when extraQuery
// is empty. When extraQuery is non-empty (e.g. "year=2026&month=7"), it is
// preserved across all pagination links.
func buildPagination(currentPage int, hasNext bool, baseURL string, extraQuery string) tpl.PaginationData {
	pd := tpl.PaginationData{CurrentPage: currentPage}

	pageURL := func(page int) string {
		if extraQuery == "" {
			if page == 1 {
				return baseURL
			}
			return fmt.Sprintf("%s?page=%d", baseURL, page)
		}
		if page == 1 {
			return fmt.Sprintf("%s?%s", baseURL, extraQuery)
		}
		return fmt.Sprintf("%s?%s&page=%d", baseURL, extraQuery, page)
	}

	if currentPage > 1 {
		pd.HasPrev = true
		pd.PrevURL = pageURL(currentPage - 1)
	}
	if hasNext {
		pd.HasNext = true
		pd.NextURL = pageURL(currentPage + 1)
	}
	return pd
}

// trimToPage applies the fetch-limit+1 HasNext probe: if more than perPage rows
// were returned, there is another page and the extra row is dropped.
func trimToPage(items []*contentdomain.Content, perPage int) ([]*contentdomain.Content, bool) {
	if len(items) > perPage {
		return items[:perPage], true
	}
	return items, false
}

func ExtractHashFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return ""
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for _, suffix := range []string{"_large", "_medium", "_thumb"} {
		if rest, ok := strings.CutSuffix(name, suffix); ok {
			name = rest
			break
		}
	}
	return name
}

type UserBasicInfo struct {
	Name           string
	Username       string
	CustomFields   map[string]any
	ProfilePicture string
}

type UserProvider interface {
	GetUserByUsername(ctx context.Context, username string) (*UserBasicInfo, error)
}

type UserFieldResolver interface {
	GetUserFields() []customfield.FieldSchema
	GetUserSystemFields() []customfield.FieldSchema
}

// PublicFieldLookup is the interface the ContentPageHandler uses to discover
// which user custom/system field slugs are allowlisted with the "expose"
// operation in the [[public_field]] config. Defining it locally keeps the
// contentpage package decoupled from the config package and testable with a stub.
type PublicFieldLookup interface {
	ExposedFields(resource, postType string) []string
}

type PostTypeResolver interface {
	GetBySlug(slug string) (posttype.PostType, error)
}

type ContentService interface {
	GetPublished(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedBySlugAny(ctx context.Context, slug string) (*contentdomain.Content, error)
	GetPublishedByID(ctx context.Context, id int) (*contentdomain.Content, error)
	GetPublishedByAuthorUsername(ctx context.Context, username string, languages []string, limit int, offset int) ([]*contentdomain.Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	GetPublishedPages(ctx context.Context) ([]*contentdomain.Content, error)
	GetPublishedCustomPostTypes(ctx context.Context) ([]string, error)
	GetPublishedByPostType(ctx context.Context, postType string, languages []string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedByTag(ctx context.Context, tag string, languages []string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedTags(ctx context.Context) ([]string, error)
	GetCommentsForContent(ctx context.Context, contentID int) ([]*contentdomain.Comment, error)
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error)
	GetRelated(ctx context.Context, id int, limit int) ([]*contentdomain.Content, error)
}

var languageNames = map[string]string{
	"en": "English",
	"id": "Indonesian",
	"fr": "French",
	"de": "German",
	"es": "Spanish",
	"zh": "Chinese",
	"ja": "Japanese",
	"ko": "Korean",
	"ar": "Arabic",
	"ru": "Russian",
	"pt": "Portuguese",
	"it": "Italian",
	"nl": "Dutch",
	"th": "Thai",
	"vi": "Vietnamese",
	"ms": "Malay",
	"hi": "Hindi",
	"tr": "Turkish",
	"pl": "Polish",
	"sv": "Swedish",
	"da": "Danish",
	"fi": "Finnish",
	"nb": "Norwegian",
	"cs": "Czech",
	"hu": "Hungarian",
	"ro": "Romanian",
	"bg": "Bulgarian",
	"el": "Greek",
	"he": "Hebrew",
	"uk": "Ukrainian",
}

func displayLanguage(code string) string {
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}

type ContentPageHandler struct {
	assembler           *DataAssembler
	templates           *tpl.Templates
	commentsEnabled     bool
	registrationEnabled bool
}

func (h *ContentPageHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)

	data, err := h.assembler.BuildHomeData(r.Context(), page, year, month)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.templates.RenderHome(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveContent(w http.ResponseWriter, r *http.Request, slug string) {
	data, err := h.assembler.BuildContentData(r.Context(), slug)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}

	if err := h.templates.RenderContent(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveAuthor(w http.ResponseWriter, r *http.Request, username string) {
	page := parsePage(r)

	data, err := h.assembler.BuildAuthorData(r.Context(), username, page)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}

	if err := h.templates.RenderAuthor(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveTag(w http.ResponseWriter, r *http.Request, tag string) {
	if tag == "" {
		h.serveNotFound(w, r)
		return
	}

	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)

	data, err := h.assembler.BuildTagData(r.Context(), tag, page, year, month)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}

	if err := h.templates.RenderTag(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveNotFound(w http.ResponseWriter, r *http.Request) {
	data := h.assembler.BuildNotFoundData(r.Context(), "")

	if err := h.templates.RenderNotFound(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) servePostTypeListing(w http.ResponseWriter, r *http.Request, postTypeSlug string) {
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)

	data, err := h.assembler.BuildIndexData(r.Context(), postTypeSlug, page, year, month)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.templates.RenderIndex(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveLogin(w http.ResponseWriter, r *http.Request) {
	navItems := h.assembler.buildNavigationItems(r.Context(), "/login")
	data := tpl.AuthPageData{
		LayoutData: tpl.LayoutData{
			Title:           "Login",
			PageTitle:       fmt.Sprintf("Login - %s", h.assembler.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/login",
			Lang:            h.assembler.PrimaryLanguage(),
			SiteConfig:      h.assembler.siteConfig,
		},
		ShowRegister: h.registrationEnabled,
	}
	if err := h.templates.RenderLogin(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveRegister(w http.ResponseWriter, r *http.Request) {
	if !h.registrationEnabled {
		// Self-registration is disabled (via [registration] enabled = false or,
		// by legacy default, when the comment system is off); the page has no
		// purpose.
		http.NotFound(w, r)
		return
	}
	navItems := h.assembler.buildNavigationItems(r.Context(), "/register")
	data := tpl.AuthPageData{
		LayoutData: tpl.LayoutData{
			Title:           "Register",
			PageTitle:       fmt.Sprintf("Register - %s", h.assembler.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/register",
			Lang:            h.assembler.PrimaryLanguage(),
			SiteConfig:      h.assembler.siteConfig,
		},
		ShowRegister: true,
	}
	if err := h.templates.RenderRegister(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveForgotPassword(w http.ResponseWriter, r *http.Request) {
	navItems := h.assembler.buildNavigationItems(r.Context(), "/forgot-password")
	data := tpl.AuthPageData{
		LayoutData: tpl.LayoutData{
			Title:           "Forgot Password",
			PageTitle:       fmt.Sprintf("Forgot Password - %s", h.assembler.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/forgot-password",
			Lang:            h.assembler.PrimaryLanguage(),
			SiteConfig:      h.assembler.siteConfig,
		},
	}
	if err := h.templates.RenderForgotPassword(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveVerifyEmail(w http.ResponseWriter, r *http.Request) {
	navItems := h.assembler.buildNavigationItems(r.Context(), "/verify-email")
	data := tpl.VerifyEmailData{
		LayoutData: tpl.LayoutData{
			Title:           "Verify Email",
			PageTitle:       fmt.Sprintf("Verify Email - %s", h.assembler.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/verify-email",
			Lang:            h.assembler.PrimaryLanguage(),
			SiteConfig:      h.assembler.siteConfig,
		},
	}
	if err := h.templates.RenderVerifyEmail(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveResetPassword(w http.ResponseWriter, r *http.Request) {
	navItems := h.assembler.buildNavigationItems(r.Context(), "/reset-password")
	data := tpl.ResetPasswordData{
		LayoutData: tpl.LayoutData{
			Title:           "Reset Password",
			PageTitle:       fmt.Sprintf("Reset Password - %s", h.assembler.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/reset-password",
			Lang:            h.assembler.PrimaryLanguage(),
			SiteConfig:      h.assembler.siteConfig,
		},
	}
	if err := h.templates.RenderResetPassword(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")

	switch {
	case path == "" || path == "/":
		h.serveIndex(w, r)
	case path == "login":
		h.serveLogin(w, r)
	case path == "register":
		h.serveRegister(w, r)
	case path == "forgot-password":
		h.serveForgotPassword(w, r)
	case path == "verify-email":
		h.serveVerifyEmail(w, r)
	case path == "reset-password":
		h.serveResetPassword(w, r)
	case strings.HasPrefix(path, "authors/"):
		username := strings.TrimPrefix(path, "authors/")
		username = strings.TrimRight(username, "/")
		h.serveAuthor(w, r, username)
	case strings.HasPrefix(path, "tags/"):
		tag := strings.TrimPrefix(path, "tags/")
		tag = strings.TrimRight(tag, "/")
		h.serveTag(w, r, tag)
	default:
		slug := strings.TrimRight(path, "/")
		if h.assembler.isPostTypeSlug(slug) {
			h.servePostTypeListing(w, r, slug)
			return
		}
		h.serveContent(w, r, slug)
	}
}

func (h *ContentPageHandler) Assembler() *DataAssembler {
	return h.assembler
}

// WithPublicFieldRegistry attaches a [[public_field]] registry to the handler
// so the author HTML page can discover which user system fields are allowlisted
// with the "expose" operation and render them. When not called (or called with
// nil), no system fields appear — the safe default. Returns the receiver for
// chaining at construction time.
func (h *ContentPageHandler) WithPublicFieldRegistry(registry PublicFieldLookup) *ContentPageHandler {
	h.assembler.WithPublicFieldRegistry(registry)
	return h
}

// WithIFrameHosts attaches the sanitizer's iframe host allowlist (derived from
// the CSP frame-src directive) so HTML-format content can render allowed
// embeds on the read path. When not called, iframes stay stripped. Returns the
// receiver for chaining at construction time.
func (h *ContentPageHandler) WithIFrameHosts(hosts ...string) *ContentPageHandler {
	h.assembler.WithIFrameHosts(hosts...)
	return h
}

// WithBaseURL sets the canonical site URL used to absolutize media URLs in
// SEO meta tags. Returns the receiver for chaining at construction time.
func (h *ContentPageHandler) WithBaseURL(baseURL string) *ContentPageHandler {
	h.assembler.WithBaseURL(baseURL)
	return h
}

func NewContentPageHandler(
	contentService ContentService,
	postTypeResolver PostTypeResolver,
	userFieldResolver UserFieldResolver,
	userProvider UserProvider,
	templates *tpl.Templates,
	renderer tiptap.Renderer,
	mediaRepo mediadomain.Repository,
	languages []string,
	homepageSections []config.HomepageSection,
	siteConfig config.SiteConfig,
	postsPerPage int,
	commentsEnabled     bool,
	registrationEnabled bool,
) *ContentPageHandler {
	return &ContentPageHandler{
		assembler: NewDataAssembler(
			contentService,
			postTypeResolver,
			userFieldResolver,
			userProvider,
			renderer,
			mediaRepo,
			languages,
			homepageSections,
			siteConfig,
			postsPerPage,
		),
		templates:           templates,
		commentsEnabled:     commentsEnabled,
		registrationEnabled: registrationEnabled,
	}
}
