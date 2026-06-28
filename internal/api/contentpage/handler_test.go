package contentpage_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/contentpage"
	"github.com/aristorinjuang/lesstruct/internal/api/contentpage/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	mediaMocks "github.com/aristorinjuang/lesstruct/internal/domain/media/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTemplates(t *testing.T) *template.Templates {
	t.Helper()
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)
	return templates
}

func setupHandler(t *testing.T, mockService *mocks.MockContentService) *contentpage.ContentPageHandler {
	t.Helper()
	return setupHandlerWithLanguages(t, mockService, []string{"en"})
}

func setupHandlerWithLanguages(t *testing.T, mockService *mocks.MockContentService, languages []string) *contentpage.ContentPageHandler {
	t.Helper()
	mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]*contentdomain.Content{}, nil).Maybe()
	renderer := tiptap.NewRenderer(nil)
	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).Return(posttype.PostType{}, assert.AnError)
	return contentpage.NewContentPageHandler(mockService, mockResolver, nil, nil, newTemplates(t), renderer, nil, languages)
}

func setupHandlerWithResolver(t *testing.T, mockService *mocks.MockContentService, mockResolver *mocks.MockPostTypeResolver) *contentpage.ContentPageHandler {
	t.Helper()
	mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]*contentdomain.Content{}, nil).Maybe()
	renderer := tiptap.NewRenderer(nil)
	return contentpage.NewContentPageHandler(mockService, mockResolver, nil, nil, newTemplates(t), renderer, nil, []string{"en"})
}

func setupHandlerWithLanguagesAndResolver(t *testing.T, mockService *mocks.MockContentService, languages []string, mockResolver *mocks.MockPostTypeResolver) *contentpage.ContentPageHandler {
	t.Helper()
	mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]*contentdomain.Content{}, nil).Maybe()
	renderer := tiptap.NewRenderer(nil)
	return contentpage.NewContentPageHandler(mockService, mockResolver, nil, nil, newTemplates(t), renderer, nil, languages)
}

func setupNavMocks(mockService *mocks.MockContentService) {
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)
}

func TestServeIndex(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
	assert.Contains(t, w.Body.String(), `href="/hello-world"`)
	mockService.AssertExpectations(t)
}

func TestServeIndex_FiltersNonPosts(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "about-page", Title: "About", PostType: "page", Language: "en"},
		{Slug: "hello-world", Title: "Hello World", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Hello World")
	assert.NotContains(t, body, "About")
	mockService.AssertExpectations(t)
}

func TestServeContent(t *testing.T) {
	mockService := new(mocks.MockContentService)
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService.On("GetPublishedBySlugAny", mock.Anything, "hello-world").Return(&contentdomain.Content{
		Slug: "hello-world", Title: "Hello World", Content: tiptapJSON, Language: "en",
		Author: "Admin", Tags: []string{"go"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/hello-world", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
	assert.Contains(t, w.Body.String(), `<p class="content-wrapper">Hello</p>`)
	assert.Contains(t, w.Body.String(), "go")
	mockService.AssertExpectations(t)
}

func TestServeContent_NotFound(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "nonexistent").Return(nil, assert.AnError)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "404")
	mockService.AssertExpectations(t)
}

func TestServeAuthor(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin User", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Admin User")
	assert.Contains(t, w.Body.String(), "Post One")
	mockService.AssertExpectations(t)
}

func TestServeAuthor_NotFound(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "unknown").Return(false, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/authors/unknown", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

func TestServeAuthor_FiltersByPrimaryLanguage(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", Author: "Admin", PostType: "post", Language: "en"},
		{Slug: "halo-dunia", Title: "Halo Dunia", Author: "Admin", PostType: "post", Language: "id"},
		{Slug: "second-post", Title: "Second Post", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Hello World")
	assert.Contains(t, body, "Second Post")
	assert.NotContains(t, body, "Halo Dunia")
	mockService.AssertExpectations(t)
}

func TestServeTag_FiltersByPrimaryLanguage(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedByTag", mock.Anything, "news", 50, 0).Return([]*contentdomain.Content{
		{Slug: "en-news", Title: "EN News", PostType: "post", Language: "en"},
		{Slug: "id-berita", Title: "ID Berita", PostType: "post", Language: "id"},
		{Slug: "en-update", Title: "EN Update", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

	req := httptest.NewRequest(http.MethodGet, "/tags/news", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "EN News")
	assert.Contains(t, body, "EN Update")
	assert.NotContains(t, body, "ID Berita")
	mockService.AssertExpectations(t)
}

func TestServeIndex_NavigationIncludesPages(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", PostType: "post", Language: "en"},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{
		{Slug: "about", Title: "About", PostType: "page", Language: "en"},
		{Slug: "contact", Title: "Contact", PostType: "page", Language: "en"},
	}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `href="/"`)
	assert.Contains(t, body, `href="/about"`)
	assert.Contains(t, body, `href="/contact"`)
	assert.Contains(t, body, "About")
	assert.Contains(t, body, "Contact")
	mockService.AssertExpectations(t)
}

func TestServeIndex_NavigationExcludesSecondaryLanguage(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{
		{Slug: "about", Title: "About", PostType: "page", Language: "en"},
		{Slug: "tentang", Title: "Tentang", PostType: "page", Language: "id"},
	}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)

	handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `href="/about"`)
	assert.NotContains(t, body, `href="/tentang"`)
	mockService.AssertExpectations(t)
}

func TestServeIndex_NavigationIncludesCustomPostTypes(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"menu-item", "team-member"}, nil)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)
	mockResolver.On("GetBySlug", "team-member").Return(posttype.PostType{Name: "Team Member", Slug: "team-member"}, nil)
	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).Return(posttype.PostType{}, assert.AnError)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `href="/menu-item"`)
	assert.Contains(t, body, `href="/team-member"`)
	assert.Contains(t, body, "Menu Item")
	assert.Contains(t, body, "Team Member")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeIndex_NavigationActiveClass(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "about").Return(&contentdomain.Content{
		Slug: "about", Title: "About Us",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"About"}]}]}`,
		Tags: []string{},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{
		{Slug: "about", Title: "About", PostType: "page", Language: "en"},
	}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "About Us")
	assert.Contains(t, body, `href="/about"`)
	assert.Contains(t, body, `class="active"`)
	mockService.AssertExpectations(t)
}

func TestServePostTypeListing(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedByPostType", mock.Anything, "menu-item", 50, 0).Return([]*contentdomain.Content{
		{Slug: "croissant", Title: "Croissant", PostType: "menu-item", MetaDescription: "A pastry", Language: "en"},
		{Slug: "eclair", Title: "Eclair", PostType: "menu-item", Language: "en"},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"menu-item"}, nil)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/menu-item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Croissant")
	assert.Contains(t, body, "Eclair")
	assert.Contains(t, body, `href="/croissant"`)
	assert.Contains(t, body, "Menu Item")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServePostTypeListing_NotPostTypeSlug_FallsThroughToContent(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "my-post").Return(&contentdomain.Content{
		Slug: "my-post", Title: "My Post",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
		Tags: []string{},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "my-post").Return(posttype.PostType{}, assert.AnError)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/my-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "My Post")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeIndex_PostCardImage(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/photo.webp"}}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "photo-post", Title: "Photo Post", PostType: "post", Content: tiptapJSON, Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<img src="https://example.com/photo.webp"`)
	assert.Contains(t, body, `class="post-card__image"`)
	assert.Contains(t, body, `loading="lazy"`)
	mockService.AssertExpectations(t)
}

func TestServeIndex_OGImageFromFirstPost(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/first.webp"}}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "first-post", Title: "First Post", PostType: "post", Content: tiptapJSON, Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<meta property="og:image" content="https://example.com/first.webp">`)
	assert.Contains(t, body, `<meta name="twitter:image" content="https://example.com/first.webp">`)
	mockService.AssertExpectations(t)
}

func TestServeIndex_NoImage_GracefulEmptyState(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "text-post", Title: "Text Post", PostType: "post", Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`, Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `class="post-card__image"`)
	assert.NotContains(t, body, `og:image`)
	assert.NotContains(t, body, `twitter:image`)
	mockService.AssertExpectations(t)
}

func TestServeContent_OGImageAndTwitterImage(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/hero.webp"}},{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "photo-article").Return(&contentdomain.Content{
		Slug: "photo-article", Title: "Photo Article", Content: tiptapJSON,
		Author: "Admin", Tags: []string{},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/photo-article", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<meta property="og:image" content="https://example.com/hero.webp">`)
	assert.Contains(t, body, `<meta name="twitter:image" content="https://example.com/hero.webp">`)
	mockService.AssertExpectations(t)
}

func TestServeContent_HeroImage(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/hero.webp"}},{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "photo-article").Return(&contentdomain.Content{
		Slug: "photo-article", Title: "Photo Article", Content: tiptapJSON,
		Author: "Admin", Tags: []string{},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/photo-article", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<img src="https://example.com/hero.webp"`)
	mockService.AssertExpectations(t)
}

func TestServeContent_NoImage_GracefulEmptyState(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Just text"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "text-article").Return(&contentdomain.Content{
		Slug: "text-article", Title: "Text Article", Content: tiptapJSON,
		Tags: []string{},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/text-article", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `og:image`)
	assert.NotContains(t, body, `twitter:image`)
	mockService.AssertExpectations(t)
}

func TestServeAuthor_PostCardImage(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/author-photo.webp"}}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin User", PostType: "post", Content: tiptapJSON, Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<img src="https://example.com/author-photo.webp"`)
	assert.Contains(t, body, `class="post-card__image"`)
	assert.Contains(t, body, `<meta property="og:image" content="https://example.com/author-photo.webp">`)
	assert.Contains(t, body, `<meta name="twitter:image" content="https://example.com/author-photo.webp">`)
	mockService.AssertExpectations(t)
}

func TestServeIndex_DateFormatting(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", PostType: "post", CreatedAt: "2026-05-12T10:30:00Z", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "May 12, 2026")
	assert.NotContains(t, body, "2026-05-12T10:30:00Z")
	mockService.AssertExpectations(t)
}

func TestServeContent_DateFormatting(t *testing.T) {
	mockService := new(mocks.MockContentService)
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService.On("GetPublishedBySlugAny", mock.Anything, "dated-post").Return(&contentdomain.Content{
		Slug: "dated-post", Title: "Dated Post", Content: tiptapJSON,
		Author: "Admin", Tags: []string{}, CreatedAt: "2026-05-12T10:30:00Z",
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/dated-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "May 12, 2026")
	assert.NotContains(t, body, "2026-05-12T10:30:00Z")
	mockService.AssertExpectations(t)
}

func TestServeAuthor_DateFormatting(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin User", PostType: "post", CreatedAt: "2026-05-12T10:30:00Z", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "May 12, 2026")
	assert.NotContains(t, body, "2026-05-12T10:30:00Z")
	mockService.AssertExpectations(t)
}

func TestServePostTypeListing_DateFormatting(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedByPostType", mock.Anything, "menu-item", 50, 0).Return([]*contentdomain.Content{
		{Slug: "croissant", Title: "Croissant", PostType: "menu-item", CreatedAt: "2026-05-12T10:30:00Z", Language: "en"},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"menu-item"}, nil)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/menu-item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "May 12, 2026")
	assert.NotContains(t, body, "2026-05-12T10:30:00Z")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServePostTypeListing_FiltersByPrimaryLanguage(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedByPostType", mock.Anything, "menu-item", 50, 0).Return([]*contentdomain.Content{
		{Slug: "croissant", Title: "Croissant", PostType: "menu-item", Language: "en"},
		{Slug: "kue-lapis", Title: "Kue Lapis", PostType: "menu-item", Language: "id"},
		{Slug: "eclair", Title: "Eclair", PostType: "menu-item", Language: "en"},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"menu-item"}, nil)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)

	handler := setupHandlerWithLanguagesAndResolver(t, mockService, []string{"en", "id"}, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/menu-item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Croissant")
	assert.Contains(t, body, "Eclair")
	assert.NotContains(t, body, "Kue Lapis")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeIndex_EmptyCreatedAt_NoPanic(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "no-date", Title: "No Date Post", PostType: "post", CreatedAt: "", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestServeIndex_InvalidCreatedAt_PassThrough(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "bad-date", Title: "Bad Date Post", PostType: "post", CreatedAt: "not-a-date", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "not-a-date")
	mockService.AssertExpectations(t)
}

func TestServeContent_AuthorDisplayName(t *testing.T) {
	mockService := new(mocks.MockContentService)
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService.On("GetPublishedBySlugAny", mock.Anything, "author-post").Return(&contentdomain.Content{
		Slug: "author-post", Title: "Author Post", Content: tiptapJSON,
		Author: "Jane Doe", Tags: []string{}, CreatedAt: "2026-05-12T10:30:00Z",
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/author-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Jane Doe")
	mockService.AssertExpectations(t)
}

func TestServeContent_AuthorFallbackToUsername(t *testing.T) {
	mockService := new(mocks.MockContentService)
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService.On("GetPublishedBySlugAny", mock.Anything, "fallback-post").Return(&contentdomain.Content{
		Slug: "fallback-post", Title: "Fallback Post", Content: tiptapJSON,
		Author: "adminuser", Tags: []string{}, CreatedAt: "2026-05-12T10:30:00Z",
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/fallback-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "adminuser")
	mockService.AssertExpectations(t)
}

func TestServePostTypeListing_PostCardImage(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/item.webp"}}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedByPostType", mock.Anything, "menu-item", 50, 0).Return([]*contentdomain.Content{
		{Slug: "croissant", Title: "Croissant", PostType: "menu-item", Content: tiptapJSON, Language: "en"},
	}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{"menu-item"}, nil)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/menu-item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<img src="https://example.com/item.webp"`)
	assert.Contains(t, body, `class="post-card__image"`)
	assert.Contains(t, body, `<meta property="og:image" content="https://example.com/item.webp">`)
	assert.Contains(t, body, `<meta name="twitter:image" content="https://example.com/item.webp">`)
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsRendered(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "croissant").Return(&contentdomain.Content{
		Slug:     "croissant",
		Title:    "Croissant",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"price":    float64(4.5),
			"category": "Pastry",
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "croissant").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
			{Name: "Category", Slug: "category", Type: customfield.FieldTypeText},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/croissant", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `aria-label="Custom Fields"`)
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.Contains(t, body, "<dd>4.5</dd>")
	assert.Contains(t, body, "<dt>Category</dt>")
	assert.Contains(t, body, "<dd>Pastry</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsEmptyValuesOmitted(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:     "item",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"price":    float64(10),
			"category": "",
			"notes":    nil,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
			{Name: "Category", Slug: "category", Type: customfield.FieldTypeText},
			{Name: "Notes", Slug: "notes", Type: customfield.FieldTypeText},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.NotContains(t, body, "<dt>Category</dt>")
	assert.NotContains(t, body, "<dt>Notes</dt>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsCheckboxFormatting(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:     "item",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"available": true,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Available", Slug: "available", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>Yes</dd>")
	assert.NotContains(t, body, "true")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsCheckboxFalseOmitted(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:     "item",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"available": false,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Available", Slug: "available", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, "<dt>Available</dt>")
	assert.NotContains(t, body, "No")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsDateFormatting(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "event").Return(&contentdomain.Content{
		Slug:     "event",
		Title:    "Event",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"event_date": "2026-05-04T00:00:00Z",
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{}, assert.AnError).Once()
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name: "Event",
		Slug: "event",
		Fields: []customfield.FieldSchema{
			{Name: "Event Date", Slug: "event_date", Type: customfield.FieldTypeDate},
		},
	}, nil).Once()

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/event", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>May 4, 2026</dd>")
	assert.NotContains(t, body, "2026-05-04T00:00:00Z")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsSelectText(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:     "item",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"size": "Large",
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Size", Slug: "size", Type: customfield.FieldTypeSelect, Options: []string{"Small", "Medium", "Large"}},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>Large</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_NoCustomFieldsSectionWhenNoValues(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:         "item",
		Title:        "Item",
		Content:      tiptapJSON,
		PostType:     "menu-item",
		Tags:         []string{},
		CustomFields: map[string]any{},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name:   "Menu Item",
		Slug:   "menu-item",
		Fields: []customfield.FieldSchema{{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber}},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Custom Fields"`)
	assert.NotContains(t, body, "<dt>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_NoCustomFieldsWhenNoPostTypeResolver(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "plain-post").Return(&contentdomain.Content{
		Slug:         "plain-post",
		Title:        "Plain Post",
		Content:      tiptapJSON,
		Tags:         []string{},
		CustomFields: map[string]any{"price": float64(10)},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/plain-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Custom Fields"`)
	assert.Contains(t, body, "Plain Post")
	mockService.AssertExpectations(t)
}

func TestServeContent_NoCustomFieldsWhenPostTypeResolveFails(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:         "item",
		Title:        "Item",
		Content:      tiptapJSON,
		PostType:     "unknown-type",
		Tags:         []string{},
		CustomFields: map[string]any{"price": float64(10)},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "unknown-type").Return(posttype.PostType{}, assert.AnError)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Custom Fields"`)
	assert.Contains(t, body, "Item")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsNilCustomFields(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:         "item",
		Title:        "Item",
		Content:      tiptapJSON,
		PostType:     "menu-item",
		Tags:         []string{},
		CustomFields: nil,
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name:   "Menu Item",
		Slug:   "menu-item",
		Fields: []customfield.FieldSchema{{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber}},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Custom Fields"`)
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsExistingFunctionalityPreserved(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "full-item").Return(&contentdomain.Content{
		Slug:      "full-item",
		Title:     "Full Item",
		Content:   tiptapJSON,
		PostType:  "menu-item",
		Tags:      []string{"food"},
		Author:    "Jane Doe",
		Username:  "jane",
		CreatedAt: "2026-05-12T10:30:00Z",
		CustomFields: map[string]any{
			"price": float64(9.99),
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "full-item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/full-item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Full Item")
	assert.Contains(t, body, `<p class="content-wrapper">Hello</p>`)
	assert.Contains(t, body, "food")
	assert.Contains(t, body, "Jane Doe")
	assert.Contains(t, body, "May 12, 2026")
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.Contains(t, body, "<dd>9.99</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_CustomFieldsTextareaFormatting(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:     "item",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"description": "A rich chocolate cake",
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Description", Slug: "description", Type: customfield.FieldTypeTextarea},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dt>Description</dt>")
	assert.Contains(t, body, "<dd>A rich chocolate cake</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_NoCustomFieldsWhenAllValuesEmpty(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item").Return(&contentdomain.Content{
		Slug:     "item",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"price":    "",
			"notes":    nil,
			"featured": false,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeText},
			{Name: "Notes", Slug: "notes", Type: customfield.FieldTypeTextarea},
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Custom Fields"`)
	assert.NotContains(t, body, "<dt>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeLogin(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Login")
	assert.Contains(t, body, `id="login-form"`)
	assert.Contains(t, body, `id="username"`)
	assert.Contains(t, body, `id="password"`)
	assert.Contains(t, body, `href="/register"`)
	assert.Contains(t, body, `href="/forgot-password"`)
	mockService.AssertExpectations(t)
}

func TestServeRegister(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Register")
	assert.Contains(t, body, `id="register-form"`)
	assert.Contains(t, body, `id="username"`)
	assert.Contains(t, body, `id="name"`)
	assert.Contains(t, body, `id="email"`)
	assert.Contains(t, body, `id="password"`)
	assert.Contains(t, body, `href="/login"`)
	mockService.AssertExpectations(t)
}

func TestServeForgotPassword(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Forgot Password")
	assert.Contains(t, body, `id="forgot-form"`)
	assert.Contains(t, body, `id="email"`)
	assert.Contains(t, body, `href="/login"`)
	mockService.AssertExpectations(t)
}

func TestServeAuthPages_IncludeHeaderNavigation(t *testing.T) {
	for _, path := range []string{"/login", "/register", "/forgot-password"} {
		t.Run(path, func(t *testing.T) {
			mockService := new(mocks.MockContentService)
			setupNavMocks(mockService)

			handler := setupHandler(t, mockService)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			body := w.Body.String()
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, body, `class="site-header"`)
			assert.Contains(t, body, `href="/"`)
			assert.Contains(t, body, "Lesstruct")
			mockService.AssertExpectations(t)
		})
	}
}

func TestServeAuthPages_ExistingPagesStillWork(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
	mockService.AssertExpectations(t)
}

func TestServeLogin_DoesNotConflictWithContentSlugs(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `id="login-form"`)
	mockService.AssertExpectations(t)
}

func TestServeRegister_DoesNotConflictWithContentSlugs(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `id="register-form"`)
	mockService.AssertExpectations(t)
}

func TestServeForgotPassword_DoesNotConflictWithContentSlugs(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `id="forgot-form"`)
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentsRendered(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "my-post").Return(&contentdomain.Content{
		ID: 1, Slug: "my-post", Title: "My Post", Content: tiptapJSON,
		Tags: []string{}, AllowComments: true,
	}, nil)
	mockService.On("GetCommentsForContent", mock.Anything, 1).Return([]*contentdomain.Comment{
		{Author: "Jane Doe", Comment: "Great article!", CreatedAt: "2026-01-15T10:30:00Z"},
		{Author: "John Smith", Comment: "Thanks for sharing", CreatedAt: "2026-02-20T14:00:00Z"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/my-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `aria-label="ui.comments"`)
	assert.Contains(t, body, "Jane Doe")
	assert.Contains(t, body, "Great article!")
	assert.Contains(t, body, "January 15, 2026")
	assert.Contains(t, body, "John Smith")
	assert.Contains(t, body, "Thanks for sharing")
	assert.Contains(t, body, "February 20, 2026")
	assert.True(t, strings.Index(body, "Jane Doe") < strings.Index(body, "John Smith"),
		"comments should be ordered oldest-first")
	assert.True(t, strings.Index(body, `class="custom-fields"`) < strings.Index(body, `aria-label="ui.comments"`),
		"comments section should appear after custom fields")
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentsHiddenWhenDisabled(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "no-comments-post").Return(&contentdomain.Content{
		ID: 2, Slug: "no-comments-post", Title: "No Comments Post", Content: tiptapJSON,
		Tags: []string{}, AllowComments: false,
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/no-comments-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Comments"`)
	assert.NotContains(t, body, "comment-")
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentsEmptyState(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "empty-comments").Return(&contentdomain.Content{
		ID: 3, Slug: "empty-comments", Title: "Empty Comments", Content: tiptapJSON,
		Tags: []string{}, AllowComments: true,
	}, nil)
	mockService.On("GetCommentsForContent", mock.Anything, 3).Return([]*contentdomain.Comment{}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/empty-comments", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "ui.no_comments")
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentFormPresent(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "commentable-post").Return(&contentdomain.Content{
		ID: 4, Slug: "commentable-post", Title: "Commentable", Content: tiptapJSON,
		Tags: []string{}, AllowComments: true,
	}, nil)
	mockService.On("GetCommentsForContent", mock.Anything, 4).Return([]*contentdomain.Comment{}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/commentable-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `id="comment-form"`)
	assert.Contains(t, body, `id="comment-login-link"`)
	assert.Contains(t, body, `id="comment-error"`)
	assert.Contains(t, body, `id="comment-success"`)
	assert.Contains(t, body, `data-slug="commentable-post"`)
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentsJSIncluded(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "js-post").Return(&contentdomain.Content{
		ID: 5, Slug: "js-post", Title: "JS Post", Content: tiptapJSON,
		Tags: []string{}, AllowComments: true,
	}, nil)
		mockService.On("GetCommentsForContent", mock.Anything, 5).Return([]*contentdomain.Comment{}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/js-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `<script src="/static/comments.js"></script>`)
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentsGetCommentsError_GracefulEmpty(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "error-post").Return(&contentdomain.Content{
		ID: 6, Slug: "error-post", Title: "Error Post", Content: tiptapJSON,
		Tags: []string{}, AllowComments: true,
	}, nil)
	mockService.On("GetCommentsForContent", mock.Anything, 6).Return([]*contentdomain.Comment(nil), assert.AnError)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/error-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "ui.no_comments")
	mockService.AssertExpectations(t)
}

func TestServeContent_CommentsExistingPagesUnchanged(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)

	// Index page
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello", Title: "Hello", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello")

	// 404 page
	req = httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w = httptest.NewRecorder()
	mockService.On("GetPublishedBySlugAny", mock.Anything, "nonexistent").Return(nil, assert.AnError)
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "404")

	// Author page
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	req = httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Admin")

	// Content without AllowComments still works
	mockService.On("GetPublishedBySlugAny", mock.Anything, "old-post").Return(&contentdomain.Content{
		Slug: "old-post", Title: "Old Post", Content: tiptapJSON, Tags: []string{},
	}, nil)
	req = httptest.NewRequest(http.MethodGet, "/old-post", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Old Post")

	mockService.AssertExpectations(t)
}

func TestStaticCSSIsServed(t *testing.T) {
	handler := template.StaticFiles(nil)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/css")
}

func TestStaticCSSContainsBrandColors(t *testing.T) {
	handler := template.StaticFiles(nil)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	css := w.Body.String()
	assert.Contains(t, css, "#22d3ee", "should contain primary brand color")
	assert.Contains(t, css, "#2536eb", "should contain secondary brand color")
	assert.Contains(t, css, "#8b5cf6", "should contain accent brand color")
	assert.Contains(t, css, "#06b6d4", "should contain primary hover color")
}

func TestStaticCSSContainsInterFont(t *testing.T) {
	handler := template.StaticFiles(nil)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	css := w.Body.String()
	assert.Contains(t, css, "Inter", "should contain Inter font family")
	assert.Contains(t, css, "fonts.googleapis.com/css2?family=Inter", "should import Inter from Google Fonts")
}

func TestAllPagesRenderWithCSS(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	tests := []struct {
		name string
		path string
		want int
		body string
	}{
		{name: "homepage", path: "/", want: http.StatusOK, body: "Hello World"},
		{name: "login", path: "/login", want: http.StatusOK, body: "Login"},
		{name: "register", path: "/register", want: http.StatusOK, body: "Register"},
		{name: "forgot-password", path: "/forgot-password", want: http.StatusOK, body: "Forgot Password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.want, w.Code)
			assert.Contains(t, w.Body.String(), tt.body)
			assert.Contains(t, w.Body.String(), `/static/style.css`, "should include CSS link")
		})
	}
	mockService.AssertExpectations(t)
}

func TestAllPagesRenderWithCSS_AuthorAnd404(t *testing.T) {
	mockService := new(mocks.MockContentService)
	setupNavMocks(mockService)

	// Content page
	mockService.On("GetPublishedBySlugAny", mock.Anything, "my-post").Return(&contentdomain.Content{
		Slug: "my-post", Title: "My Post",
		Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
		Tags:    []string{},
	}, nil)

	// Author page
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin User", PostType: "post", Language: "en"},
	}, nil)

	// 404 page
	mockService.On("GetPublishedBySlugAny", mock.Anything, "nonexistent").Return(nil, assert.AnError)

	handler := setupHandler(t, mockService)

	tests := []struct {
		name string
		path string
		want int
		body string
	}{
		{name: "content-page", path: "/my-post", want: http.StatusOK, body: "My Post"},
		{name: "author-page", path: "/authors/admin", want: http.StatusOK, body: "Admin User"},
		{name: "404-page", path: "/nonexistent", want: http.StatusNotFound, body: "404"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.want, w.Code)
			assert.Contains(t, w.Body.String(), tt.body)
			assert.Contains(t, w.Body.String(), `/static/style.css`, "should include CSS link")
		})
	}
	mockService.AssertExpectations(t)
}

func TestPageRendersContainLesstructBranding(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
		{Slug: "hello-world", Title: "Hello World", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Lesstruct", "should contain Lesstruct branding")
	assert.Contains(t, body, `class="site-logo"`, "should have site logo")
	assert.Contains(t, body, `class="site-header"`, "should have site header")
	assert.Contains(t, body, `class="site-footer"`, "should have site footer")
	mockService.AssertExpectations(t)
}

func TestServeContent_SystemFieldsRenderedAlongsideCustomFields(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "event-1").Return(&contentdomain.Content{
		Slug:     "event-1",
		Title:    "Event One",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"price":     float64(25),
			"location":  "Main Hall",
			"is_online": true,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "event-1").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name: "Event",
		Slug: "event",
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
			{Name: "Location", Slug: "location", Type: customfield.FieldTypeText},
		},
		SystemFields: []customfield.FieldSchema{
			{Name: "Is Online", Slug: "is_online", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/event-1", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `aria-label="Custom Fields"`)
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.Contains(t, body, "<dd>25</dd>")
	assert.Contains(t, body, "<dt>Location</dt>")
	assert.Contains(t, body, "<dd>Main Hall</dd>")
	assert.Contains(t, body, "<dt>Is Online</dt>")
	assert.Contains(t, body, "<dd>Yes</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldsWithoutCustomFields(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "event-2").Return(&contentdomain.Content{
		Slug:     "event-2",
		Title:    "Event Two",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"is_online": true,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "event-2").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Is Online", Slug: "is_online", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/event-2", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `aria-label="Custom Fields"`)
	assert.Contains(t, body, "<dt>Is Online</dt>")
	assert.Contains(t, body, "<dd>Yes</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldsEmptyValuesOmitted(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "event-3").Return(&contentdomain.Content{
		Slug:     "event-3",
		Title:    "Event Three",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"price":    float64(10),
			"notes":    "",
			"featured": false,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "event-3").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
			{Name: "Notes", Slug: "notes", Type: customfield.FieldTypeText},
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/event-3", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.NotContains(t, body, "<dt>Notes</dt>")
	assert.NotContains(t, body, "<dt>Featured</dt>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_NoSystemFieldsWhenNoneDefined(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "croissant-2").Return(&contentdomain.Content{
		Slug:     "croissant-2",
		Title:    "Croissant",
		Content:  tiptapJSON,
		PostType: "menu-item",
		Tags:     []string{},
		CustomFields: map[string]any{
			"price": float64(4.5),
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "croissant-2").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name:         "Menu Item",
		Slug:         "menu-item",
		Fields:       []customfield.FieldSchema{{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber}},
		SystemFields: []customfield.FieldSchema{},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/croissant-2", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.Contains(t, body, "<dd>4.5</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldsExistingCustomFieldsPreserved(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "classic-item").Return(&contentdomain.Content{
		Slug:      "classic-item",
		Title:     "Classic Item",
		Content:   tiptapJSON,
		PostType:  "menu-item",
		Tags:      []string{"food"},
		Author:    "Jane Doe",
		Username:  "jane",
		CreatedAt: "2026-05-12T10:30:00Z",
		CustomFields: map[string]any{
			"price": float64(9.99),
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "classic-item").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "menu-item").Return(posttype.PostType{
		Name: "Menu Item",
		Slug: "menu-item",
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/classic-item", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Classic Item")
	assert.Contains(t, body, `<p class="content-wrapper">Hello</p>`)
	assert.Contains(t, body, "food")
	assert.Contains(t, body, "Jane Doe")
	assert.Contains(t, body, "May 12, 2026")
	assert.Contains(t, body, "<dt>Price</dt>")
	assert.Contains(t, body, "<dd>9.99</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldCheckboxFormatting(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item-sf").Return(&contentdomain.Content{
		Slug:     "item-sf",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"featured": true,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item-sf").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item-sf", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>Yes</dd>")
	assert.NotContains(t, body, "true")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldDateFormatting(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "event-d").Return(&contentdomain.Content{
		Slug:     "event-d",
		Title:    "Event D",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"event_date": "2026-05-04T00:00:00Z",
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "event-d").Return(posttype.PostType{}, assert.AnError).Once()
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Event Date", Slug: "event_date", Type: customfield.FieldTypeDate},
		},
	}, nil).Once()

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/event-d", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>May 4, 2026</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldCheckboxFalseOmitted(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item-cf").Return(&contentdomain.Content{
		Slug:     "item-cf",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"featured": false,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item-cf").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item-cf", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, "<dt>Featured</dt>")
	assert.NotContains(t, body, "No")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldsAllEmptyNoSection(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item-ae").Return(&contentdomain.Content{
		Slug:     "item-ae",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"notes":    "",
			"featured": false,
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item-ae").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Notes", Slug: "notes", Type: customfield.FieldTypeText},
			{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item-ae", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="Custom Fields"`)
	assert.NotContains(t, body, "<dt>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldsOnlyNoCustomFields(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "only-sys").Return(&contentdomain.Content{
		Slug:     "only-sys",
		Title:    "Only System Fields",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"priority": float64(1),
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "only-sys").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:         "Event",
		Slug:         "event",
		Fields:       nil,
		SystemFields: []customfield.FieldSchema{{Name: "Priority", Slug: "priority", Type: customfield.FieldTypeNumber}},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/only-sys", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `aria-label="Custom Fields"`)
	assert.Contains(t, body, "<dt>Priority</dt>")
	assert.Contains(t, body, "<dd>1</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func TestServeContent_SystemFieldSelectFormatting(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "item-sel").Return(&contentdomain.Content{
		Slug:     "item-sel",
		Title:    "Item",
		Content:  tiptapJSON,
		PostType: "event",
		Tags:     []string{},
		CustomFields: map[string]any{
			"tier": "Gold",
		},
	}, nil)
	setupNavMocks(mockService)

	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", "item-sel").Return(posttype.PostType{}, assert.AnError)
	mockResolver.On("GetBySlug", "event").Return(posttype.PostType{
		Name:   "Event",
		Slug:   "event",
		Fields: []customfield.FieldSchema{},
		SystemFields: []customfield.FieldSchema{
			{Name: "Tier", Slug: "tier", Type: customfield.FieldTypeSelect, Options: []string{"Silver", "Gold", "Platinum"}},
		},
	}, nil)

	handler := setupHandlerWithResolver(t, mockService, mockResolver)

	req := httptest.NewRequest(http.MethodGet, "/item-sel", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>Gold</dd>")
	mockService.AssertExpectations(t)
	mockResolver.AssertExpectations(t)
}

func setupAuthorHandler(t *testing.T, mockService *mocks.MockContentService, mockUserProvider *mocks.MockUserProvider, mockUserFieldResolver *mocks.MockUserFieldResolver) *contentpage.ContentPageHandler {
	t.Helper()
	mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]*contentdomain.Content{}, nil).Maybe()
	renderer := tiptap.NewRenderer(nil)
	mockResolver := new(mocks.MockPostTypeResolver)
	mockResolver.On("GetBySlug", mock.AnythingOfType("string")).Return(posttype.PostType{}, assert.AnError)
	return contentpage.NewContentPageHandler(mockService, mockResolver, mockUserFieldResolver, mockUserProvider, newTemplates(t), renderer, nil, []string{"en"})
}

func TestServeAuthor_CustomFieldsDisplayed(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin User", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "admin").Return(&contentpage.UserBasicInfo{
		Name:         "Admin User",
		Username:     "admin",
		CustomFields: map[string]any{"job_title": "Engineer", "company": "Acme Inc"},
	}, nil)

	mockUserFieldResolver := new(mocks.MockUserFieldResolver)
	mockUserFieldResolver.On("GetUserFields").Return([]customfield.FieldSchema{
		{Name: "Job Title", Slug: "job_title", Type: customfield.FieldTypeText},
		{Name: "Company", Slug: "company", Type: customfield.FieldTypeText},
		{Name: "Website", Slug: "website", Type: customfield.FieldTypeText},
	})

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `aria-label="About the Author"`)
	assert.Contains(t, body, "<dt>Job Title</dt>")
	assert.Contains(t, body, "<dd>Engineer</dd>")
	assert.Contains(t, body, "<dt>Company</dt>")
	assert.Contains(t, body, "<dd>Acme Inc</dd>")
	assert.NotContains(t, body, "<dt>Website</dt>")
	assert.Contains(t, body, "Post One")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_SystemFieldsExcluded(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "admin").Return(&contentpage.UserBasicInfo{
		Name:         "Admin",
		Username:     "admin",
		CustomFields: map[string]any{"job_title": "Dev", "internal_rating": "gold"},
	}, nil)

	mockUserFieldResolver := new(mocks.MockUserFieldResolver)
	mockUserFieldResolver.On("GetUserFields").Return([]customfield.FieldSchema{
		{Name: "Job Title", Slug: "job_title", Type: customfield.FieldTypeText},
	})

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dt>Job Title</dt>")
	assert.Contains(t, body, "<dd>Dev</dd>")
	assert.NotContains(t, body, "internal_rating")
	assert.NotContains(t, body, "gold")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_EmptyCustomFieldsOmitted(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "admin").Return(&contentpage.UserBasicInfo{
		Name:         "Admin",
		Username:     "admin",
		CustomFields: map[string]any{"job_title": "", "company": nil, "featured": false},
	}, nil)

	mockUserFieldResolver := new(mocks.MockUserFieldResolver)
	mockUserFieldResolver.On("GetUserFields").Return([]customfield.FieldSchema{
		{Name: "Job Title", Slug: "job_title", Type: customfield.FieldTypeText},
		{Name: "Company", Slug: "company", Type: customfield.FieldTypeText},
		{Name: "Featured", Slug: "featured", Type: customfield.FieldTypeCheckbox},
	})

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="About the Author"`)
	assert.NotContains(t, body, "<dt>")
	assert.Contains(t, body, "Post One")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_NoUserFieldsConfigured(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "admin").Return(&contentpage.UserBasicInfo{
		Name:     "Admin",
		Username: "admin",
	}, nil)

	mockUserFieldResolver := new(mocks.MockUserFieldResolver)
	mockUserFieldResolver.On("GetUserFields").Return([]customfield.FieldSchema{})

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="About the Author"`)
	assert.NotContains(t, body, "<dt>")
	assert.Contains(t, body, "Post One")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_UserHasNoCustomFieldValues(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "admin").Return(&contentpage.UserBasicInfo{
		Name:         "Admin",
		Username:     "admin",
		CustomFields: map[string]any{},
	}, nil)

	mockUserFieldResolver := new(mocks.MockUserFieldResolver)
	mockUserFieldResolver.On("GetUserFields").Return([]customfield.FieldSchema{
		{Name: "Job Title", Slug: "job_title", Type: customfield.FieldTypeText},
	})

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="About the Author"`)
	assert.NotContains(t, body, "<dt>")
	assert.Contains(t, body, "Post One")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_UserNotFoundStillRendersPosts(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "admin").Return(nil, assert.AnError)

	// userFieldResolver.GetUserFields is not called because authorUser is nil when GetUserByUsername fails
	mockUserFieldResolver := new(mocks.MockUserFieldResolver)

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Admin")
	assert.Contains(t, body, "Post One")
	assert.NotContains(t, body, `aria-label="About the Author"`)
	assert.NotContains(t, body, "<dt>")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_UserProviderNil_NoCustomFields(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `aria-label="About the Author"`)
	assert.Contains(t, body, "Post One")
	mockService.AssertExpectations(t)
}

func TestServeAuthor_CheckboxAndDateFormatting(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "jane").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "jane", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Jane Doe", PostType: "post", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	mockUserProvider := new(mocks.MockUserProvider)
	mockUserProvider.On("GetUserByUsername", mock.Anything, "jane").Return(&contentpage.UserBasicInfo{
		Name:         "Jane Doe",
		Username:     "jane",
		CustomFields: map[string]any{"available": true, "joined": "2026-01-15T00:00:00Z"},
	}, nil)

	mockUserFieldResolver := new(mocks.MockUserFieldResolver)
	mockUserFieldResolver.On("GetUserFields").Return([]customfield.FieldSchema{
		{Name: "Available", Slug: "available", Type: customfield.FieldTypeCheckbox},
		{Name: "Joined", Slug: "joined", Type: customfield.FieldTypeDate},
	})

	handler := setupAuthorHandler(t, mockService, mockUserProvider, mockUserFieldResolver)

	req := httptest.NewRequest(http.MethodGet, "/authors/jane", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "<dd>Yes</dd>")
	assert.NotContains(t, body, "true")
	assert.Contains(t, body, "<dd>January 15, 2026</dd>")
	assert.NotContains(t, body, "2026-01-15T00:00:00Z")
	mockService.AssertExpectations(t)
	mockUserProvider.AssertExpectations(t)
	mockUserFieldResolver.AssertExpectations(t)
}

func TestServeAuthor_ExistingFunctionalityPreserved(t *testing.T) {
	mockService := new(mocks.MockContentService)
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", 50, 0).Return([]*contentdomain.Content{
		{Slug: "post-1", Title: "Post One", Author: "Admin User", PostType: "post", CreatedAt: "2026-05-12T10:30:00Z", Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/authors/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, "Admin User")
	assert.Contains(t, body, "Post One")
	assert.Contains(t, body, "May 12, 2026")
	assert.Contains(t, body, `href="/post-1"`)
	assert.NotContains(t, body, `aria-label="About the Author"`)
	mockService.AssertExpectations(t)
}


func TestExtractHashFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal media URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "thumb variant URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "medium variant URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a_medium.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "large variant URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a_large.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "external URL extracts filename stem",
			input:    "https://example.com/photo.jpg",
			expected: "photo",
		},
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "malformed URL returns empty",
			input:    "://bad-url",
			expected: "",
		},
		{
			name:     "URL ending with directory",
			input:    "http://localhost:8080/uploads/media/",
			expected: "media",
		},
		{
			name:     "percent-encoded path",
			input:    "http://localhost:8080/uploads/media/abc%20def.webp",
			expected: "abc def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contentpage.ExtractHashFromURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolvePostImage(t *testing.T) {
	tests := []struct {
		name            string
		imageURL        string
		setupMock       func(*mediaMocks.MockRepository)
		expectThumbURL  string
		expectSrcset    string
		expectSizes     string
	}{
		{
			name:           "empty URL returns empty",
			imageURL:       "",
			expectThumbURL: "",
			expectSrcset:   "",
			expectSizes:    "",
		},
		{
			name:           "URL with no hash returns empty",
			imageURL:       "http://localhost:8080/uploads/media/.webp",
			expectThumbURL: "http://localhost:8080/uploads/media/.webp",
			expectSrcset:   "",
			expectSizes:    "",
		},
		{
			name:     "FindByHashPrefix error returns empty",
			imageURL: "http://localhost:8080/uploads/media/abc123.webp",
			setupMock: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					(*mediadomain.Media)(nil),
					assert.AnError,
				)
			},
			expectThumbURL: "http://localhost:8080/uploads/media/abc123.webp",
			expectSrcset:   "",
			expectSizes:    "",
		},
		{
			name:     "media not found returns empty",
			imageURL: "http://localhost:8080/uploads/media/abc123.webp",
			setupMock: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					(*mediadomain.Media)(nil),
					nil,
				)
			},
			expectThumbURL: "http://localhost:8080/uploads/media/abc123.webp",
			expectSrcset:   "",
			expectSizes:    "",
		},
		{
			name:     "empty variants returns empty",
			imageURL: "http://localhost:8080/uploads/media/abc123.webp",
			setupMock: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					&mediadomain.Media{Variants: map[string]mediadomain.MediaVariant{}},
					nil,
				)
			},
			expectThumbURL: "http://localhost:8080/uploads/media/abc123.webp",
			expectSrcset:   "",
			expectSizes:    "",
		},
		{
			name:     "variants present returns srcset and sizes",
			imageURL: "http://localhost:8080/uploads/media/abc123.webp",
			setupMock: func(m *mediaMocks.MockRepository) {
				m.On("FindByHashPrefix", mock.Anything, "abc123").Return(
					&mediadomain.Media{
						Variants: map[string]mediadomain.MediaVariant{
							"_thumb":  {URL: "http://localhost:8080/uploads/media/abc123_thumb.webp", Width: 370},
							"_medium": {URL: "http://localhost:8080/uploads/media/abc123_medium.webp", Width: 800},
						},
					},
					nil,
				)
			},
			expectThumbURL: "http://localhost:8080/uploads/media/abc123_thumb.webp",
			expectSrcset:   "http://localhost:8080/uploads/media/abc123_thumb.webp 370w, http://localhost:8080/uploads/media/abc123_medium.webp 800w",
			expectSizes:    "(min-width: 1200px) 370px, (min-width: 768px) calc(50vw - 3rem), calc(100vw - 3rem)",
		},
	}

	encodeImageContent := func(imageURL string) string {
		return `{"type":"doc","content":[{"type":"image","attrs":{"src":"` + imageURL + `"}}]}`
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(mocks.MockContentService)
			contentBody := encodeImageContent(tt.imageURL)
			mockService.On("GetPublished", mock.Anything, 50, 0).Return([]*contentdomain.Content{
				{
					Slug:     "test-post",
					Title:    "Test Post",
					PostType: "post",
					Content:  contentBody,
					Language: "en",
				},
			}, nil)
			setupNavMocks(mockService)
			mockResolver := new(mocks.MockPostTypeResolver)
			mockResolver.On("GetBySlug", mock.AnythingOfType("string")).Return(posttype.PostType{}, assert.AnError)

			var mediaRepo mediadomain.Repository
			if tt.setupMock != nil {
				mockMedia := mediaMocks.NewMockRepository(t)
				tt.setupMock(mockMedia)
				mediaRepo = mockMedia
			}

			handler := contentpage.NewContentPageHandler(
				mockService,
				mockResolver,
				nil,
				nil,
				newTemplates(t),
				tiptap.NewRenderer(nil),
				mediaRepo,
				[]string{"en"},
			)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			body := w.Body.String()
			assert.Contains(t, body, "Test Post")
			if tt.expectThumbURL != "" {
				assert.Contains(t, body, `src="`+tt.expectThumbURL+`"`)
			}
			if tt.expectSrcset != "" {
				assert.Contains(t, body, `srcset="`+tt.expectSrcset+`"`)
				assert.Contains(t, body, `sizes="`+tt.expectSizes+`"`)
			} else {
				assert.NotContains(t, body, "srcset")
				assert.NotContains(t, body, "sizes")
			}
		})
	}
}

func TestServeContent_RelatedPosts(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "source-post").Return(&contentdomain.Content{
		ID:       7,
		Slug:     "source-post",
		Title:    "Source Post",
		Content:  tiptapJSON,
		Tags:     []string{"go", "web"},
		PostType: "post",
		Language: "en",
	}, nil)
	mockService.On("GetRelated", mock.Anything, 7, 5).Return([]*contentdomain.Content{
		{Slug: "related-one", Title: "Related One", Content: tiptapJSON, Language: "en"},
		{Slug: "related-two", Title: "Related Two", Content: tiptapJSON, Language: "en"},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/source-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, body, `class="related-posts`)
	assert.Contains(t, body, `aria-label="ui.related_posts"`)
	assert.Contains(t, body, ">ui.related_posts<")
	assert.Contains(t, body, `href="/related-one"`)
	assert.Contains(t, body, ">Related One<")
	assert.Contains(t, body, `href="/related-two"`)
	assert.Contains(t, body, ">Related Two<")
	mockService.AssertExpectations(t)
}

func TestServeContent_RelatedPostsEmpty(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`
	mockService := new(mocks.MockContentService)
	mockService.On("GetPublishedBySlugAny", mock.Anything, "lonely-post").Return(&contentdomain.Content{
		ID:      8,
		Slug:    "lonely-post",
		Title:   "Lonely Post",
		Content: tiptapJSON,
		Tags:    []string{},
	}, nil)
	setupNavMocks(mockService)

	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/lonely-post", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, body, `class="related-posts`)
	assert.NotContains(t, body, "ui.related_posts")
	mockService.AssertExpectations(t)
}
