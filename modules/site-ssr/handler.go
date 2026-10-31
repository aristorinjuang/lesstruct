package sitessr

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/aristorinjuang/lesstruct/kernel/i18n"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
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
				return FormatDate(lang, t)
			}
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return FormatDate(lang, t)
			}
			return s
		}
	case customfield.FieldTypeDatetime:
		if s, ok := val.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return FormatDateTime(lang, t)
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
) []FormattedField {
	result := make([]FormattedField, 0, len(fields))
	for _, f := range fields {
		if f.Slug == customfield.PostScriptSlug {
			continue
		}
		val, exists := values[f.Slug]
		if !exists || isEmptyValue(val) {
			continue
		}
		result = append(result, FormattedField{
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
func buildPagination(currentPage int, hasNext bool, baseURL string, extraQuery string) PaginationData {
	pd := PaginationData{CurrentPage: currentPage}

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
func trimToPage(items []ContentRef, perPage int) ([]ContentRef, bool) {
	if len(items) > perPage {
		return items[:perPage], true
	}
	return items, false
}

func displayLanguage(code string) string {
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}

// standalonePageTitle humanizes a theme page key for the document title:
// "cart" -> "Cart", "orders/track" -> "Track".
func standalonePageTitle(page string) string {
	name := page
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ReplaceAll(name, "-", " ")
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// ExtractHashFromURL pulls the media hash out of a stored media URL. It
// delegates to the media domain, which owns the variant-suffix vocabulary;
// kept as a thin alias for the SSR-internal call sites.
func ExtractHashFromURL(rawURL string) string {
	return mediadomain.ExtractHashFromURL(rawURL)
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

// BodyRenderer renders stored content bodies (TipTap JSON) to HTML. Core
// satisfies it with the tiptap renderer; tests stub it.
type BodyRenderer interface {
	Render(tiptapJSON string) (string, error)
}

type ContentPageHandler struct {
	assembler           *DataAssembler
	templates           *Templates
	commentsEnabled     bool
	registrationEnabled bool
}

func (h *ContentPageHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)

	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	data, err := reqAss.BuildHomeData(r.Context(), page, year, month)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data.Languages = h.assembler.siteLanguageLinks("/", r.URL.Query())

	if err := h.templates.RenderHome(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveContent(w http.ResponseWriter, r *http.Request, slug string) {
	ordered, display, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, display)
	}
	reqAss := h.assembler.forRequest(ordered)

	if setCookie {
		if content, err := reqAss.contentService.GetPublishedBySlugAny(r.Context(), slug); err == nil {
			if content.Language != "" && !strings.EqualFold(content.Language, display) {
				groupID := content.ID
				if content.TranslationGroupID != nil {
					groupID = *content.TranslationGroupID
				}
				if translations, err := reqAss.contentService.GetTranslations(r.Context(), groupID, content.ID); err == nil {
					for _, trans := range translations {
						if strings.EqualFold(trans.Language, display) {
							http.Redirect(w, r, redirectTarget(r, "/"+trans.Slug), http.StatusFound)
							return
						}
					}
				}
				if primary, err := reqAss.contentService.GetPublishedByID(r.Context(), groupID); err == nil {
					if strings.EqualFold(primary.Language, display) && primary.Slug != slug {
						http.Redirect(w, r, redirectTarget(r, "/"+primary.Slug), http.StatusFound)
						return
					}
				}
			}
		}
	}

	data, err := reqAss.BuildContentData(r.Context(), slug)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}
	data.Languages = h.assembler.siteLanguageLinks("/"+slug, r.URL.Query())

	if err := h.templates.RenderContent(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveAuthor(w http.ResponseWriter, r *http.Request, username string) {
	page := parsePage(r)

	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	data, err := reqAss.BuildAuthorData(r.Context(), username, page)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}
	data.Languages = h.assembler.siteLanguageLinks("/authors/"+username, r.URL.Query())

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

	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	data, err := reqAss.BuildTagData(r.Context(), tag, page, year, month)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}
	data.Languages = h.assembler.siteLanguageLinks("/tags/"+tag, r.URL.Query())

	if err := h.templates.RenderTag(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveNotFound(w http.ResponseWriter, r *http.Request) {
	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	currentPath := r.URL.Path
	if currentPath == "" {
		currentPath = "/"
	}
	data := reqAss.BuildNotFoundData(r.Context(), currentPath)
	data.Lang = ordered[0]
	data.Languages = h.assembler.siteLanguageLinks(currentPath, r.URL.Query())

	if err := h.templates.RenderNotFound(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) servePostTypeListing(w http.ResponseWriter, r *http.Request, postTypeSlug string) {
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)

	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	data, err := reqAss.BuildIndexData(r.Context(), postTypeSlug, page, year, month)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data.Languages = h.assembler.siteLanguageLinks("/"+postTypeSlug, r.URL.Query())

	if err := h.templates.RenderIndex(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveLogin(w http.ResponseWriter, r *http.Request) {
	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	pageChrome := reqAss.buildChrome(r.Context(), "/login")
	data := AuthPageData{
		LayoutData: LayoutData{
			Title:           "Login",
			PageTitle:       fmt.Sprintf("Login - %s", reqAss.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     "/login",
			Lang:            ordered[0],
			LanguageLinks:   nil,
			Languages:       h.assembler.siteLanguageLinks("/login", r.URL.Query()),
			SiteConfig:      reqAss.siteConfig,
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
	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	pageChrome := reqAss.buildChrome(r.Context(), "/register")
	data := AuthPageData{
		LayoutData: LayoutData{
			Title:           "Register",
			PageTitle:       fmt.Sprintf("Register - %s", reqAss.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     "/register",
			Lang:            ordered[0],
			Languages:       h.assembler.siteLanguageLinks("/register", r.URL.Query()),
			SiteConfig:      reqAss.siteConfig,
		},
		ShowRegister: true,
	}
	if err := h.templates.RenderRegister(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveForgotPassword(w http.ResponseWriter, r *http.Request) {
	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	pageChrome := reqAss.buildChrome(r.Context(), "/forgot-password")
	data := AuthPageData{
		LayoutData: LayoutData{
			Title:           "Forgot Password",
			PageTitle:       fmt.Sprintf("Forgot Password - %s", reqAss.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     "/forgot-password",
			Lang:            ordered[0],
			Languages:       h.assembler.siteLanguageLinks("/forgot-password", r.URL.Query()),
			SiteConfig:      reqAss.siteConfig,
		},
	}
	if err := h.templates.RenderForgotPassword(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveVerifyEmail(w http.ResponseWriter, r *http.Request) {
	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	pageChrome := reqAss.buildChrome(r.Context(), "/verify-email")
	data := VerifyEmailData{
		LayoutData: LayoutData{
			Title:           "Verify Email",
			PageTitle:       fmt.Sprintf("Verify Email - %s", reqAss.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     "/verify-email",
			Lang:            ordered[0],
			Languages:       h.assembler.siteLanguageLinks("/verify-email", r.URL.Query()),
			SiteConfig:      reqAss.siteConfig,
		},
	}
	if err := h.templates.RenderVerifyEmail(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveResetPassword(w http.ResponseWriter, r *http.Request) {
	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)

	pageChrome := reqAss.buildChrome(r.Context(), "/reset-password")
	data := ResetPasswordData{
		LayoutData: LayoutData{
			Title:           "Reset Password",
			PageTitle:       fmt.Sprintf("Reset Password - %s", reqAss.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     "/reset-password",
			Lang:            ordered[0],
			Languages:       h.assembler.siteLanguageLinks("/reset-password", r.URL.Query()),
			SiteConfig:      reqAss.siteConfig,
		},
	}
	if err := h.templates.RenderResetPassword(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// serveStandalonePage renders a theme-declared app page (cart, checkout,
// order tracking). The shell is server-rendered through the shared layout;
// the page's data is fetched client-side by the theme's script.
func (h *ContentPageHandler) serveStandalonePage(w http.ResponseWriter, r *http.Request, page string) {
	currentPath := "/" + page
	title := standalonePageTitle(page)

	ordered, _, setCookie := negotiateLanguage(r, h.assembler.Languages())
	if setCookie && !isConsentDeclined(r) {
		setLangCookie(w, r, ordered[0])
	}
	reqAss := h.assembler.forRequest(ordered)
	pageChrome := reqAss.buildChrome(r.Context(), currentPath)

	data := StandalonePageData{
		LayoutData: LayoutData{
			Title:           title,
			PageTitle:       fmt.Sprintf("%s - %s", title, reqAss.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     currentPath,
			Lang:            ordered[0],
			Languages:       h.assembler.siteLanguageLinks(currentPath, r.URL.Query()),
			SiteConfig:      reqAss.siteConfig,
			NoIndex:         true,
		},
		Page: page,
	}
	if err := h.templates.RenderPage(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")

	// Responses vary by negotiated language: tell shared caches that
	// Accept-Language and the language cookie select the representation.
	w.Header().Set("Vary", "Accept-Language, Cookie")

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
		if h.templates.HasPage(slug) {
			h.serveStandalonePage(w, r, slug)
			return
		}
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

// WithUICatalog attaches the i18n UI catalog for engine-generated listing
// strings. Returns the receiver for chaining at construction time.
func (h *ContentPageHandler) WithUICatalog(catalog *i18n.Catalog) *ContentPageHandler {
	h.assembler.WithUICatalog(catalog)
	return h
}

func NewContentPageHandler(
	contentService ContentReader,
	postTypeResolver PostTypeResolver,
	userFieldResolver UserFieldResolver,
	userProvider UserProvider,
	templates *Templates,
	renderer BodyRenderer,
	mediaRepo mediadomain.Repository,
	languages []string,
	homepageSections []HomepageSection,
	siteConfig SiteConfig,
	postsPerPage int,
	commentsEnabled bool,
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
