package contentpage

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tpl "github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/seo"
)

func formatDate(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Format("January 2, 2006")
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

func formatFieldValue(fieldType customfield.FieldType, val any) string {
	switch fieldType {
	case customfield.FieldTypeCheckbox:
		if b, ok := val.(bool); ok && b {
			return "Yes"
		}
	case customfield.FieldTypeDate:
		if s, ok := val.(string); ok {
			return formatDate(s)
		}
	}
	return fmt.Sprintf("%v", val)
}

func formatCustomFields(
	fields []customfield.FieldSchema,
	values map[string]any,
) []tpl.FormattedField {
	result := make([]tpl.FormattedField, 0, len(fields))
	for _, f := range fields {
		val, exists := values[f.Slug]
		if !exists || isEmptyValue(val) {
			continue
		}
		result = append(result, tpl.FormattedField{
			Label: f.Name,
			Value: formatFieldValue(f.Type, val),
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
	GetPublishedByAuthorUsername(ctx context.Context, username string, limit int, offset int) ([]*contentdomain.Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	GetPublishedPages(ctx context.Context) ([]*contentdomain.Content, error)
	GetPublishedCustomPostTypes(ctx context.Context) ([]string, error)
	GetPublishedByPostType(ctx context.Context, postType string, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedByTag(ctx context.Context, tag string, limit int, offset int) ([]*contentdomain.Content, error)
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
}

func (h *ContentPageHandler) resolvePostImage(imageURL string) (thumbURL, srcset, sizes string) {
	if h.mediaRepo == nil || imageURL == "" {
		return imageURL, "", ""
	}
	hash := ExtractHashFromURL(imageURL)
	if hash == "" {
		return imageURL, "", ""
	}
	m, err := h.mediaRepo.FindByHashPrefix(context.Background(), hash)
	if err != nil {
		log.Printf("WARNING: resolvePostImage FindByHashPrefix failed for hash %q: %v", hash, err)
		return imageURL, "", ""
	}
	if m == nil {
		return imageURL, "", ""
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
	return thumbURL, srcset, sizes
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
	contents, err := h.contentService.GetPublished(r.Context(), 50, 0)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	primaryLang := config.PrimaryLanguage(h.languages)

	posts := make([]tpl.PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if c.PostType != "post" || c.Language != primaryLang {
			continue
		}
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, h.buildPostItem(r.Context(), c))
	}

	currentPath := "/"
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.IndexData{
		LayoutData: tpl.LayoutData{
			Title:           "Lesstruct",
			PageTitle:       "Lesstruct",
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
		},
		Posts: posts,
	}

	if err := h.templates.RenderIndex(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) buildPostItem(ctx context.Context, c *contentdomain.Content) tpl.PostItem {
	imageURL := seo.ExtractImageURL(c.Content)
	thumbURL, imageSrcset, imageSizes := h.resolvePostImage(imageURL)

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
		Author:          c.Author,
		Username:        c.Username,
		AuthorAvatarURL: authorAvatarURL,
		CreatedAt:       formatDate(c.CreatedAt),
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

	bodyHTML, err := h.renderer.Render(content.Content)
	if err != nil {
		bodyHTML = ""
	}

	ogTitle := content.OGTitle
	if ogTitle == "" {
		ogTitle = content.Title
	}

	ogDesc := content.OGDescription
	if ogDesc == "" {
		ogDesc = content.MetaDescription
	}

	featuredImage := seo.ExtractImageURL(content.Content)

	currentPath := "/" + slug
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	var formattedFields []tpl.FormattedField
	if h.postTypeResolver != nil && content.PostType != "" {
		if pt, ptErr := h.postTypeResolver.GetBySlug(content.PostType); ptErr == nil {
			if content.CustomFields != nil {
				formattedFields = formatCustomFields(pt.Fields, content.CustomFields)
				formattedFields = append(formattedFields,
					formatCustomFields(pt.SystemFields, content.CustomFields)...)
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
				CreatedAt: formatDate(c.CreatedAt),
			})
		}
	}

	relatedItems := make([]tpl.PostItem, 0)
	if related, err := h.contentService.GetRelated(r.Context(), content.ID, 5); err != nil {
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
			PageTitle:       content.Title + " - Lesstruct",
			OGTitle:         ogTitle,
			OGDesc:          ogDesc,
			OGImage:         featuredImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            lang,
			LanguageLinks:   languageLinks,
		},
		Slug:                  content.Slug,
		Body:                  template.HTML(bodyHTML),
		Tags:                  content.Tags,
		Author:                content.Author,
		Username:              content.Username,
		AuthorAvatarURL:       authorAvatarURL,
		CreatedAt:             formatDate(content.CreatedAt),
		AllowComments:         content.AllowComments,
		CustomFields:          content.CustomFields,
		CustomFieldsFormatted: formattedFields,
		Related:               relatedItems,
		Comments:              commentItems,
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

	contents, err := h.contentService.GetPublishedByAuthorUsername(r.Context(), username, 50, 0)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	primaryLang := config.PrimaryLanguage(h.languages)

	posts := make([]tpl.PostItem, 0, len(contents))
	authorName := ""
	var ogImage string
	for _, c := range contents {
		if c.Language != primaryLang {
			continue
		}
		if authorName == "" {
			authorName = c.Author
		}
		imageURL := seo.ExtractImageURL(c.Content)
		if imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		thumbURL, imageSrcset, imageSizes := h.resolvePostImage(imageURL)
		posts = append(posts, tpl.PostItem{
			Slug:            c.Slug,
			Title:           c.Title,
			MetaDescription: c.MetaDescription,
			ImageURL:        thumbURL,
			ImageSrcset:     imageSrcset,
			ImageSizes:      imageSizes,
			Author:          c.Author,
			Username:        c.Username,
			CreatedAt:       formatDate(c.CreatedAt),
		})
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
				formattedFields = formatCustomFields(userFields, authorUser.CustomFields)
			}
		}
	}

	currentPath := "/authors/" + username
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.AuthorData{
		LayoutData: tpl.LayoutData{
			Title:           authorName,
			PageTitle:       authorName + " - Lesstruct",
			Description:     fmt.Sprintf("Posts by %s.", authorName),
			OGDesc:          fmt.Sprintf("Posts by %s.", authorName),
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
		},
		AuthorName:             authorName,
		Username:               username,
		AuthorAvatarURL:        authorAvatarURL,
		Posts:                  posts,
		CustomFieldsFormatted: formattedFields,
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

	contents, err := h.contentService.GetPublishedByTag(r.Context(), tag, 50, 0)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	primaryLang := config.PrimaryLanguage(h.languages)

	posts := make([]tpl.PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if c.Language != primaryLang {
			continue
		}
		imageURL := seo.ExtractImageURL(c.Content)
		if imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		thumbURL, imageSrcset, imageSizes := h.resolvePostImage(imageURL)

		var postAuthorAvatarURL string
		if h.userProvider != nil && c.Username != "" {
			if user, userErr := h.userProvider.GetUserByUsername(r.Context(), c.Username); userErr == nil && user != nil {
				postAuthorAvatarURL = user.ProfilePicture
			}
		}

		posts = append(posts, tpl.PostItem{
			Slug:            c.Slug,
			Title:           c.Title,
			MetaDescription: c.MetaDescription,
			ImageURL:        thumbURL,
			ImageSrcset:     imageSrcset,
			ImageSizes:      imageSizes,
			Author:          c.Author,
			Username:        c.Username,
			AuthorAvatarURL: postAuthorAvatarURL,
			CreatedAt:       formatDate(c.CreatedAt),
		})
	}

	currentPath := "/tags/" + tag
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.TagData{
		LayoutData: tpl.LayoutData{
			Title:           tag,
			PageTitle:       tag + " - Lesstruct",
			Description:     fmt.Sprintf("Posts tagged %q.", tag),
			OGDesc:          fmt.Sprintf("Posts tagged %q.", tag),
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
		},
		TagName: tag,
		Posts:   posts,
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
			PageTitle:       "Not Found - Lesstruct",
			NavigationItems: navItems,
			Lang:            config.PrimaryLanguage(h.languages),
		},
	}

	if err := h.templates.RenderNotFound(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *ContentPageHandler) servePostTypeListing(w http.ResponseWriter, r *http.Request, postTypeSlug string) {
	contents, err := h.contentService.GetPublishedByPostType(r.Context(), postTypeSlug, 50, 0)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var resolved posttype.PostType
	resolveErr := error(nil)
	if h.postTypeResolver != nil {
		resolved, resolveErr = h.postTypeResolver.GetBySlug(postTypeSlug)
	}
	pageTitle := postTypeSlug
	if resolveErr == nil && resolved.Name != "" {
		pageTitle = resolved.Name
	}

	primaryLang := config.PrimaryLanguage(h.languages)

	posts := make([]tpl.PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if c.Language != primaryLang {
			continue
		}
		imageURL := seo.ExtractImageURL(c.Content)
		if imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		thumbURL, imageSrcset, imageSizes := h.resolvePostImage(imageURL)

		var postAuthorAvatarURL string
		if h.userProvider != nil && c.Username != "" {
			if user, userErr := h.userProvider.GetUserByUsername(r.Context(), c.Username); userErr == nil && user != nil {
				postAuthorAvatarURL = user.ProfilePicture
			}
		}

		posts = append(posts, tpl.PostItem{
			Slug:            c.Slug,
			Title:           c.Title,
			MetaDescription: c.MetaDescription,
			ImageURL:        thumbURL,
			ImageSrcset:     imageSrcset,
			ImageSizes:      imageSizes,
			Author:          c.Author,
			Username:        c.Username,
			AuthorAvatarURL: postAuthorAvatarURL,
			CreatedAt:       formatDate(c.CreatedAt),
		})
	}

	currentPath := "/" + postTypeSlug
	navItems := h.buildNavigationItems(r.Context(), currentPath)

	data := tpl.IndexData{
		LayoutData: tpl.LayoutData{
			Title:           pageTitle,
			PageTitle:       pageTitle + " - Lesstruct",
			Description:     fmt.Sprintf("Browse %s.", pageTitle),
			OGDesc:          fmt.Sprintf("Browse %s.", pageTitle),
			OGImage:         ogImage,
			NavigationItems: navItems,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
		},
		Posts: posts,
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
			PageTitle:       "Login - Lesstruct",
			NavigationItems: navItems,
			CurrentPath:     "/login",
			Lang:            config.PrimaryLanguage(h.languages),
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
			PageTitle:       "Register - Lesstruct",
			NavigationItems: navItems,
			CurrentPath:     "/register",
			Lang:            config.PrimaryLanguage(h.languages),
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
			PageTitle:       "Forgot Password - Lesstruct",
			NavigationItems: navItems,
			CurrentPath:     "/forgot-password",
			Lang:            config.PrimaryLanguage(h.languages),
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
			PageTitle:       "Verify Email - Lesstruct",
			NavigationItems: navItems,
			CurrentPath:     "/verify-email",
			Lang:            config.PrimaryLanguage(h.languages),
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
			PageTitle:       "Reset Password - Lesstruct",
			NavigationItems: navItems,
			CurrentPath:     "/reset-password",
			Lang:            config.PrimaryLanguage(h.languages),
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

func NewContentPageHandler(
	contentService ContentService,
	postTypeResolver PostTypeResolver,
	userFieldResolver UserFieldResolver,
	userProvider UserProvider,
	templates *tpl.Templates,
	renderer tiptap.Renderer,
	mediaRepo mediadomain.Repository,
	languages []string,
) *ContentPageHandler {
	return &ContentPageHandler{
		contentService:    contentService,
		postTypeResolver:  postTypeResolver,
		userFieldResolver: userFieldResolver,
		userProvider:      userProvider,
		templates:         templates,
		renderer:          renderer,
		mediaRepo:         mediaRepo,
		languages:         languages,
	}
}
