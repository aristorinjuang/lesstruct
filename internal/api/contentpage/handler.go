package contentpage

import (
	"context"
	"fmt"

	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
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
	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
	"github.com/aristorinjuang/lesstruct/internal/seo"
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
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].width < parts[j].width
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

// defaultPostsPerPage is the page size used when the configured POSTS_PER_PAGE
// is zero or invalid. Public listings fetch postsPerPage+1 rows so they can
// detect HasNext without a COUNT query, then trim back to postsPerPage.
const defaultPostsPerPage = 50

// defaultHomeSectionLimit is the number of items shown in a homepage section
// when its [[homepage_section]] limit is unset.
const defaultHomeSectionLimit = 6

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
}

type PostTypeResolver interface {
	GetBySlug(slug string) (posttype.PostType, error)
}

type ContentService interface {
	GetPublished(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedBySlugAny(ctx context.Context, slug string) (*contentdomain.Content, error)
	GetPublishedByID(ctx context.Context, id int) (*contentdomain.Content, error)
	GetPublishedByAuthorUsername(ctx context.Context, username string, language string, limit int, offset int) ([]*contentdomain.Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	GetPublishedPages(ctx context.Context) ([]*contentdomain.Content, error)
	GetPublishedCustomPostTypes(ctx context.Context) ([]string, error)
	GetPublishedByPostType(ctx context.Context, postType string, language string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedByTag(ctx context.Context, tag string, language string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error)
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
	contentService    ContentService
	postTypeResolver  PostTypeResolver
	userFieldResolver UserFieldResolver
	userProvider      UserProvider
	templates         *tpl.Templates
	renderer          tiptap.Renderer
	mediaRepo         mediadomain.Repository
	languages         []string
	homepageSections  []config.HomepageSection
	siteConfig        tpl.SiteConfig
	postsPerPage      int
}

// effectivePerPage returns the configured page size, falling back to the
// default when it is unset or nonsensical.
func (h *ContentPageHandler) effectivePerPage() int {
	if h.postsPerPage <= 0 {
		return defaultPostsPerPage
	}
	return h.postsPerPage
}

func (h *ContentPageHandler) resolvePostImage(imageURL string) (thumbURL, srcset, sizes string, variants map[string]string, originalURL string) {
	if h.mediaRepo == nil || imageURL == "" {
		return imageURL, "", "", nil, imageURL
	}
	hash := ExtractHashFromURL(imageURL)
	if hash == "" {
		return imageURL, "", "", nil, imageURL
	}
	m, err := h.mediaRepo.FindByHashPrefix(context.Background(), hash)
	if err != nil {
		log.Printf("WARNING: resolvePostImage FindByHashPrefix failed for hash %q: %v", hash, err)
		return imageURL, "", "", nil, imageURL
	}
	if m == nil {
		return imageURL, "", "", nil, imageURL
	}
	originalURL = m.URL
	if len(m.Variants) > 0 {
		variants = make(map[string]string, len(m.Variants))
		for k, v := range m.Variants {
			variants[k] = v.URL
		}
	}
	srcset = buildImageSrcset(m.Variants)
	if srcset != "" {
		sizes = postCardSizes
		if thumb, ok := m.Variants["_thumb"]; ok {
			thumbURL = thumb.URL
		} else {
			thumbURL = imageURL
		}
	} else {
		thumbURL = imageURL
	}
	return thumbURL, srcset, sizes, variants, originalURL
}

func (h *ContentPageHandler) isPostTypeSlug(slug string) bool {
	if h.postTypeResolver == nil {
		return false
	}
	_, err := h.postTypeResolver.GetBySlug(slug)
	return err == nil
}

func (h *ContentPageHandler) buildNavigationItems(ctx context.Context, currentPath string) []tpl.NavigationItem {
	items := []tpl.NavigationItem{
		{Title: "Home", URL: "/", IsActive: currentPath == "/"},
	}

	pages, err := h.contentService.GetPublishedPages(ctx)
	if err == nil {
		// The site nav surfaces only primary-language pages; each page still
		// links to its own translations via buildLanguageLinks, so secondary
		// languages are reachable without crowding the nav.
		primaryLang := config.PrimaryLanguage(h.languages)
		for _, page := range pages {
			if page.Language != primaryLang {
				continue
			}
			items = append(items, tpl.NavigationItem{
				Title:    page.Title,
				URL:      "/" + page.Slug,
				IsActive: currentPath == "/"+page.Slug,
			})
		}
	} else {
		log.Printf("failed to get published pages for navigation: %v", err)
	}

	postTypes, err := h.contentService.GetPublishedCustomPostTypes(ctx)
	if err == nil && h.postTypeResolver != nil {
		for _, pt := range postTypes {
			resolved, resolveErr := h.postTypeResolver.GetBySlug(pt)
			name := pt
			if resolveErr == nil && resolved.Name != "" {
				name = resolved.Name
			}
			items = append(items, tpl.NavigationItem{
				Title:    name,
				URL:      "/" + pt,
				IsActive: currentPath == "/"+pt,
			})
		}
	} else if err != nil {
		log.Printf("failed to get published custom post types for navigation: %v", err)
	}

	return items
}

func (h *ContentPageHandler) buildLanguageLinks(ctx context.Context, content *contentdomain.Content, currentLang string) []tpl.LanguageLink {
	if len(h.languages) <= 1 {
		return nil
	}

	primaryLang := h.languages[0]

	// Use primary content's ID as the translation group ID.
	// Primary content has TranslationGroupID = nil, so use its own ID.
	// Translations have TranslationGroupID set to the primary's ID.
	groupID := content.ID
	if content.TranslationGroupID != nil {
		groupID = *content.TranslationGroupID
	}

	translations, err := h.contentService.GetTranslations(ctx, groupID, content.ID)
	if err != nil {
		log.Printf("failed to get translations for group %d: %v", groupID, err)
	}

	transByLang := make(map[string]*contentdomain.Content)
	for _, t := range translations {
		transByLang[t.Language] = t
	}

	// Add primary content to the map if it's not the current content.
	if content.Language != primaryLang {
		if primary, err := h.contentService.GetPublishedByID(ctx, groupID); err == nil {
			transByLang[primary.Language] = primary
		}
	} else {
		transByLang[content.Language] = content
	}

	links := make([]tpl.LanguageLink, 0, len(h.languages)-1)
	for _, lang := range h.languages {
		if lang == currentLang {
			continue
		}
		if trans, ok := transByLang[lang]; ok {
			links = append(links, tpl.LanguageLink{
				Code: lang,
				Name: displayLanguage(lang),
				URL:  "/" + trans.Slug,
			})
		}
	}

	return links
}

func (h *ContentPageHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	primaryLang := config.PrimaryLanguage(h.languages)
	perPage := h.effectivePerPage()
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)
	offset := (page - 1) * perPage

	// Latest posts scoped to type "post" and the primary language at the query
	// level (no Go-level filtering, no wasted rows). Fetch perPage+1 to probe
	// for a next page without a COUNT query.
	contents, err := h.contentService.GetPublishedByPostType(r.Context(), "post", primaryLang, year, month, perPage+1, offset)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	contents, hasNext := trimToPage(contents, perPage)

	var ogImage string
	posts := make([]tpl.PostItem, 0, len(contents))
	for _, c := range contents {
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, h.buildPostItem(r.Context(), c))
	}

	// Tags for the homepage tag cloud (Tier 1.2 — the field was dead before).
	tags, tagsErr := h.contentService.GetPublishedTags(r.Context())
	if tagsErr != nil {
		log.Printf("failed to get published tags for index: %v", tagsErr)
		tags = nil
	}

	// Optional per-post-type sections (Tier 2.1). Only built when the operator
	// configures [[homepage_section]] blocks; otherwise the homepage renders
	// the flat latest-posts list above, fully backward compatible.
	sections := h.buildHomeSections(r.Context(), primaryLang)

	currentPath := "/"
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.IndexData{
		LayoutData: tpl.LayoutData{
			Title:           h.siteConfig.Name,
			PageTitle:       h.siteConfig.Name,
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      h.siteConfig,
		},
		Posts:          posts,
		Tags:           tags,
		Sections:       sections,
		PaginationData: buildPagination(page, hasNext, currentPath, archiveQuery(year, month)),
	}

	if err := h.templates.RenderHome(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildHomeSections assembles the configured [[homepage_section]] blocks for
// the homepage. Each section fetches its post type scoped to the primary
// language and resolves its display name via the post-type resolver. Returns
// nil when no sections are configured (the caller then renders a flat list).
func (h *ContentPageHandler) buildHomeSections(ctx context.Context, primaryLang string) []tpl.HomeSection {
	if len(h.homepageSections) == 0 {
		return nil
	}
	sections := make([]tpl.HomeSection, 0, len(h.homepageSections))
	for _, hs := range h.homepageSections {
		limit := hs.Limit
		if limit <= 0 {
			limit = defaultHomeSectionLimit
		}
		contents, err := h.contentService.GetPublishedByPostType(ctx, hs.PostType, primaryLang, 0, 0, limit, hs.Offset)
		if err != nil {
			log.Printf("failed to get homepage section %q: %v", hs.PostType, err)
			continue
		}
		if len(contents) == 0 {
			continue
		}
		title := hs.Title
		description := ""
		url := "/" + hs.PostType
		if h.postTypeResolver != nil {
			if resolved, resolveErr := h.postTypeResolver.GetBySlug(hs.PostType); resolveErr == nil {
				if title == "" {
					title = resolved.Name
				}
				description = resolved.Description
			}
		}
		if title == "" {
			title = hs.PostType
		}
		posts := make([]tpl.PostItem, 0, len(contents))
		for _, c := range contents {
			posts = append(posts, h.buildPostItem(ctx, c))
		}
		sections = append(sections, tpl.HomeSection{
			PostTypeSlug: hs.PostType,
			Title:        title,
			Description:  description,
			URL:          url,
			Posts:        posts,
		})
	}
	return sections
}

func (h *ContentPageHandler) buildPostItem(ctx context.Context, c *contentdomain.Content) tpl.PostItem {
	imageURL := seo.ExtractImageURL(c.Content)
	thumbURL, imageSrcset, imageSizes, imageVariants, originalURL := h.resolvePostImage(imageURL)

	var authorAvatarURL string
	if h.userProvider != nil && c.Username != "" {
		if user, err := h.userProvider.GetUserByUsername(ctx, c.Username); err == nil && user != nil {
			authorAvatarURL = user.ProfilePicture
		}
	}

	return tpl.PostItem{
		Slug:            c.Slug,
		Title:           c.Title,
		MetaDescription: c.MetaDescription,
		ImageURL:        thumbURL,
		ImageSrcset:     imageSrcset,
		ImageSizes:      imageSizes,
		ImageVariants:   imageVariants,
		OriginalURL:     originalURL,
		Author:          c.Author,
		Username:        c.Username,
		AuthorAvatarURL: authorAvatarURL,
		CreatedAt:       c.CreatedAt,
		PostType:        c.PostType,
		Tags:            c.Tags,
	}
}

func (h *ContentPageHandler) serveContent(w http.ResponseWriter, r *http.Request, slug string) {
	content, err := h.contentService.GetPublishedBySlugAny(r.Context(), slug)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}

	lang := content.Language
	if lang == "" {
		lang = "en"
	}

	var bodyHTML string
	switch content.Format {
	case contentdomain.FormatHTML:
		bodyHTML = sanitize.SanitizeHTMLDocument(content.Content)
	default:
		bodyHTML, err = h.renderer.Render(content.Content)
		if err != nil {
			bodyHTML = ""
		}
	}

	ogTitle := content.OGTitle
	if ogTitle == "" {
		ogTitle = content.Title
	}

	ogDesc := content.OGDescription
	if ogDesc == "" {
		ogDesc = content.MetaDescription
	}

	var featuredImage string
	switch content.Format {
	case contentdomain.FormatHTML:
		featuredImage = seo.ExtractImageURLFromHTML(content.Content)
	default:
		featuredImage = seo.ExtractImageURL(content.Content)
	}

	currentPath := "/" + slug
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	var formattedFields []tpl.FormattedField
	if h.postTypeResolver != nil && content.PostType != "" {
		if pt, ptErr := h.postTypeResolver.GetBySlug(content.PostType); ptErr == nil {
			if content.CustomFields != nil {
				formattedFields = formatCustomFields(pt.Fields, content.CustomFields, lang)
				formattedFields = append(formattedFields,
					formatCustomFields(pt.SystemFields, content.CustomFields, lang)...)
			}
		}
	}

	var commentItems []tpl.CommentItem
	if content.AllowComments {
		comments, err := h.contentService.GetCommentsForContent(r.Context(), content.ID)
		if err != nil {
			log.Printf("failed to get comments for content %d: %v", content.ID, err)
		}
		for _, c := range comments {
			commentItems = append(commentItems, tpl.CommentItem{
				Author:    c.Author,
				Text:      c.Comment,
				CreatedAt: c.CreatedAt,
			})
		}
	}

	relatedItems := make([]tpl.PostItem, 0)
	if related, err := h.contentService.GetRelated(r.Context(), content.ID, 4); err != nil {
		log.Printf("failed to get related content for content %d: %v", content.ID, err)
	} else {
		for _, c := range related {
			relatedItems = append(relatedItems, h.buildPostItem(r.Context(), c))
		}
	}

	var authorAvatarURL string
	if h.userProvider != nil && content.Username != "" {
		if user, userErr := h.userProvider.GetUserByUsername(r.Context(), content.Username); userErr == nil && user != nil {
			authorAvatarURL = user.ProfilePicture
		}
	}

	languageLinks := h.buildLanguageLinks(r.Context(), content, lang)

	data := tpl.ContentData{
		LayoutData: tpl.LayoutData{
			Title:           content.Title,
			Description:     content.MetaDescription,
			PageTitle:       fmt.Sprintf("%s - %s", content.Title, h.siteConfig.Name),
			OGTitle:         ogTitle,
			OGDesc:          ogDesc,
			OGImage:         featuredImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            lang,
			LanguageLinks:   languageLinks,
			SiteConfig:      h.siteConfig,
		},
		Slug:                  content.Slug,
		Body:                  template.HTML(bodyHTML),
		Tags:                  content.Tags,
		Author:                content.Author,
		Username:              content.Username,
		AuthorAvatarURL:       authorAvatarURL,
		CreatedAt:             content.CreatedAt,
		AllowComments:         content.AllowComments,
		CustomFields:          content.CustomFields,
		CustomFieldsFormatted: formattedFields,
		Related:               relatedItems,
		Comments:              commentItems,
		PostType:              content.PostType,
	}

	if err := h.templates.RenderContent(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveAuthor(w http.ResponseWriter, r *http.Request, username string) {
	exists, err := h.contentService.AuthorExists(r.Context(), username)
	if err != nil || !exists {
		h.serveNotFound(w, r)
		return
	}

	primaryLang := config.PrimaryLanguage(h.languages)
	perPage := h.effectivePerPage()
	page := parsePage(r)
	offset := (page - 1) * perPage

	contents, err := h.contentService.GetPublishedByAuthorUsername(r.Context(), username, primaryLang, perPage+1, offset)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	contents, hasNext := trimToPage(contents, perPage)

	authorName := ""
	var ogImage string
	posts := make([]tpl.PostItem, 0, len(contents))
	for _, c := range contents {
		if authorName == "" {
			authorName = c.Author
		}
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, h.buildPostItem(r.Context(), c))
	}

	if authorName == "" {
		authorName = username
	}

	var formattedFields []tpl.FormattedField
	var authorAvatarURL string
	var authorUser *UserBasicInfo
	if h.userProvider != nil {
		if user, userErr := h.userProvider.GetUserByUsername(r.Context(), username); userErr == nil && user != nil {
			authorUser = user
		}
	}
	if authorUser != nil {
		authorAvatarURL = authorUser.ProfilePicture
		if h.userFieldResolver != nil {
			userFields := h.userFieldResolver.GetUserFields()
			if len(userFields) > 0 && len(authorUser.CustomFields) > 0 {
				formattedFields = formatCustomFields(userFields, authorUser.CustomFields, primaryLang)
			}
		}
	}

	currentPath := "/authors/" + username
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.AuthorData{
		LayoutData: tpl.LayoutData{
			Title:           authorName,
			PageTitle:       fmt.Sprintf("%s - %s", authorName, h.siteConfig.Name),
			Description:     fmt.Sprintf("Posts by %s.", authorName),
			OGDesc:          fmt.Sprintf("Posts by %s.", authorName),
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      h.siteConfig,
		},
		AuthorName:            authorName,
		Username:              username,
		AuthorAvatarURL:       authorAvatarURL,
		Posts:                 posts,
		CustomFieldsFormatted: formattedFields,
		PaginationData:        buildPagination(page, hasNext, currentPath, ""),
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

	primaryLang := config.PrimaryLanguage(h.languages)
	perPage := h.effectivePerPage()
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)
	offset := (page - 1) * perPage

	contents, err := h.contentService.GetPublishedByTag(r.Context(), tag, primaryLang, year, month, perPage+1, offset)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	contents, hasNext := trimToPage(contents, perPage)

	posts := make([]tpl.PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, h.buildPostItem(r.Context(), c))
	}

	currentPath := "/tags/" + tag
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.TagData{
		LayoutData: tpl.LayoutData{
			Title:           tag,
			PageTitle:       fmt.Sprintf("%s - %s", tag, h.siteConfig.Name),
			Description:     fmt.Sprintf("Posts tagged %q.", tag),
			OGDesc:          fmt.Sprintf("Posts tagged %q.", tag),
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      h.siteConfig,
		},
		TagName:        tag,
		Posts:          posts,
		PaginationData: buildPagination(page, hasNext, currentPath, archiveQuery(year, month)),
	}

	if err := h.templates.RenderTag(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveNotFound(w http.ResponseWriter, r *http.Request) {
	navItems := h.buildNavigationItems(r.Context(), "")

	data := tpl.NotFoundData{
		LayoutData: tpl.LayoutData{
			Title:           "Not Found",
			PageTitle:       fmt.Sprintf("Not Found - %s", h.siteConfig.Name),
			NavigationItems: navItems,
			Lang:            config.PrimaryLanguage(h.languages),
			SiteConfig:      h.siteConfig,
		},
	}

	if err := h.templates.RenderNotFound(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) servePostTypeListing(w http.ResponseWriter, r *http.Request, postTypeSlug string) {
	primaryLang := config.PrimaryLanguage(h.languages)
	perPage := h.effectivePerPage()
	page := parsePage(r)
	year := parseYear(r)
	month := parseMonth(r)
	offset := (page - 1) * perPage

	contents, err := h.contentService.GetPublishedByPostType(r.Context(), postTypeSlug, primaryLang, year, month, perPage+1, offset)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	contents, hasNext := trimToPage(contents, perPage)

	var resolved posttype.PostType
	resolveErr := error(nil)
	if h.postTypeResolver != nil {
		resolved, resolveErr = h.postTypeResolver.GetBySlug(postTypeSlug)
	}
	pageTitle := postTypeSlug
	if resolveErr == nil && resolved.Name != "" {
		pageTitle = resolved.Name
	}

	posts := make([]tpl.PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, h.buildPostItem(r.Context(), c))
	}

	currentPath := "/" + postTypeSlug
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.IndexData{
		LayoutData: tpl.LayoutData{
			Title:           pageTitle,
			PageTitle:       fmt.Sprintf("%s - %s", pageTitle, h.siteConfig.Name),
			Description:     fmt.Sprintf("Browse %s.", pageTitle),
			OGDesc:          fmt.Sprintf("Browse %s.", pageTitle),
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      h.siteConfig,
		},
		Posts:          posts,
		PaginationData: buildPagination(page, hasNext, currentPath, archiveQuery(year, month)),
	}

	if err := h.templates.RenderIndex(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveLogin(w http.ResponseWriter, r *http.Request) {
	navItems := h.buildNavigationItems(r.Context(), "/login")
	data := tpl.AuthPageData{
		LayoutData: tpl.LayoutData{
			Title:           "Login",
			PageTitle:       fmt.Sprintf("Login - %s", h.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/login",
			Lang:            config.PrimaryLanguage(h.languages),
			SiteConfig:      h.siteConfig,
		},
	}
	if err := h.templates.RenderLogin(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveRegister(w http.ResponseWriter, r *http.Request) {
	navItems := h.buildNavigationItems(r.Context(), "/register")
	data := tpl.AuthPageData{
		LayoutData: tpl.LayoutData{
			Title:           "Register",
			PageTitle:       fmt.Sprintf("Register - %s", h.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/register",
			Lang:            config.PrimaryLanguage(h.languages),
			SiteConfig:      h.siteConfig,
		},
	}
	if err := h.templates.RenderRegister(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveForgotPassword(w http.ResponseWriter, r *http.Request) {
	navItems := h.buildNavigationItems(r.Context(), "/forgot-password")
	data := tpl.AuthPageData{
		LayoutData: tpl.LayoutData{
			Title:           "Forgot Password",
			PageTitle:       fmt.Sprintf("Forgot Password - %s", h.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/forgot-password",
			Lang:            config.PrimaryLanguage(h.languages),
			SiteConfig:      h.siteConfig,
		},
	}
	if err := h.templates.RenderForgotPassword(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveVerifyEmail(w http.ResponseWriter, r *http.Request) {
	navItems := h.buildNavigationItems(r.Context(), "/verify-email")
	data := tpl.VerifyEmailData{
		LayoutData: tpl.LayoutData{
			Title:           "Verify Email",
			PageTitle:       fmt.Sprintf("Verify Email - %s", h.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/verify-email",
			Lang:            config.PrimaryLanguage(h.languages),
			SiteConfig:      h.siteConfig,
		},
	}
	if err := h.templates.RenderVerifyEmail(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) serveResetPassword(w http.ResponseWriter, r *http.Request) {
	navItems := h.buildNavigationItems(r.Context(), "/reset-password")
	data := tpl.ResetPasswordData{
		LayoutData: tpl.LayoutData{
			Title:           "Reset Password",
			PageTitle:       fmt.Sprintf("Reset Password - %s", h.siteConfig.Name),
			NavigationItems: navItems,
			CurrentPath:     "/reset-password",
			Lang:            config.PrimaryLanguage(h.languages),
			SiteConfig:      h.siteConfig,
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
		if h.isPostTypeSlug(slug) {
			h.servePostTypeListing(w, r, slug)
			return
		}
		h.serveContent(w, r, slug)
	}
}

const postCardSizes = "(min-width: 1200px) 370px, (min-width: 768px) calc(50vw - 3rem), calc(100vw - 3rem)"

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

// defaultSiteName is the site name used when [site_config].name is not
// configured. It keeps the embedded theme's branding stable for an
// out-of-the-box install; operators override it via config.toml.
const defaultSiteName = "Lesstruct"

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
) *ContentPageHandler {
	if siteConfig.Name == "" {
		siteConfig.Name = defaultSiteName
	}
	return &ContentPageHandler{
		contentService:    contentService,
		postTypeResolver:  postTypeResolver,
		userFieldResolver: userFieldResolver,
		userProvider:      userProvider,
		templates:         templates,
		renderer:          renderer,
		mediaRepo:         mediaRepo,
		languages:         languages,
		homepageSections:  homepageSections,
		siteConfig: tpl.SiteConfig{
			Name: siteConfig.Name,
			Logo: siteConfig.Logo,
		},
		postsPerPage: postsPerPage,
	}
}
