package sitessr_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/i18n"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServeIndex_LegalChrome(t *testing.T) {
	cookiePolicy := sitessr.ContentRef{ID: 10, Slug: "cookie-policy", Title: "Cookie Policy", PostType: "page", Language: "en"}
	termsOfService := sitessr.ContentRef{ID: 11, Slug: "terms-of-service", Title: "Terms of Service", PostType: "page", Language: "en"}
	privacyPolicy := sitessr.ContentRef{ID: 12, Slug: "privacy-policy", Title: "Privacy Policy", PostType: "page", Language: "en"}

	tests := []struct {
		name               string
		pages              []sitessr.ContentRef
		nav                legal.NavPlacement
		wantHeaderLegal    bool
		wantBannerLink     string
		wantBannerPlain    string
		notice             legal.NoticeCopy
		wantBannerSentence string
	}{
		{
			name:           "footer placement keeps legal pages out of the header",
			pages:          []sitessr.ContentRef{cookiePolicy, termsOfService, privacyPolicy},
			nav:            legal.NavFooter,
			wantBannerLink: "/cookie-policy",
		},
		{
			name:            "header placement lists legal pages in both navs",
			pages:           []sitessr.ContentRef{cookiePolicy, termsOfService, privacyPolicy},
			nav:             legal.NavHeader,
			wantHeaderLegal: true,
			wantBannerLink:  "/cookie-policy",
		},
		{
			name:           "a missing cookie policy falls back to the privacy policy",
			pages:          []sitessr.ContentRef{termsOfService, privacyPolicy},
			nav:            legal.NavFooter,
			wantBannerLink: "/privacy-policy",
		},
		{
			name:            "no policy page keeps the neutral plain banner",
			pages:           []sitessr.ContentRef{termsOfService},
			nav:             legal.NavFooter,
			wantBannerPlain: "We use cookies to remember your preferences. Only strictly necessary storage is used, and everything keeps working.",
		},
		{
			name:               "store copy keeps the storefront wording",
			pages:              []sitessr.ContentRef{cookiePolicy, termsOfService, privacyPolicy},
			nav:                legal.NavFooter,
			notice:             legal.StoreNoticeCopy,
			wantBannerLink:     "/cookie-policy",
			wantBannerSentence: "The cart uses strictly necessary storage only and always works.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog, err := i18n.NewCatalog(i18n.Embedded(), []string{"en"})
			require.NoError(t, err)

			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "post", []string{"en"}, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			mockService.On("GetPublishedPages", mock.Anything).Return(tt.pages, nil)
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)
			mockService.On("GetPublishedTags", mock.Anything).Return([]string{}, nil).Maybe()

			handler := setupHandler(t, mockService)
			handler.Assembler().WithUICatalog(catalog).WithLegal(sitessr.LegalOptions{Nav: tt.nav, CookieNotice: tt.notice})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.NotContains(t, body, legal.PlaceholderCookiePolicy, "the banner never leaks the placeholder")

			footerIndex := strings.Index(body, "<footer")
			require.Positive(t, footerIndex, "the layout renders a footer")
			headerPart, footerPart := body[:footerIndex], body[footerIndex:]
			assert.Equal(t, tt.wantHeaderLegal, strings.Contains(headerPart, `href="/terms-of-service"`))
			for _, page := range tt.pages {
				assert.Contains(t, footerPart, `href="/`+page.Slug+`"`, "the footer lists every published page")
			}

			if tt.wantBannerLink != "" {
				assert.Contains(t, body, `<a href="`+tt.wantBannerLink+`">cookie policy</a>`)
			} else {
				assert.NotContains(t, body, `>cookie policy</a>`)
			}
			if tt.wantBannerPlain != "" {
				assert.Contains(t, body, tt.wantBannerPlain)
				assert.NotContains(t, body, "language and currency", "the neutral banner never mentions the cart")
			}
			if tt.wantBannerSentence != "" {
				assert.Contains(t, body, tt.wantBannerSentence)
			}
		})
	}
}

func TestServeIndex_LegalOverride(t *testing.T) {
	catalog, err := i18n.NewCatalog(i18n.Embedded(), []string{"en"})
	require.NoError(t, err)

	mockService := newMockReader()
	mockService.On("GetPublishedByPostType", mock.Anything, "post", []string{"en"}, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
	mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{
		{ID: 10, Slug: "cookie-policy", Title: "Cookie Policy", PostType: "page", Language: "en"},
	}, nil)
	mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)
	mockService.On("GetPublishedTags", mock.Anything).Return([]string{}, nil).Maybe()

	handler := setupHandler(t, mockService)
	handler.Assembler().WithUICatalog(catalog).WithLegal(sitessr.LegalOptions{
		Overrides: map[legal.Page]string{legal.PageCookiePolicy: "https://example.com/legal/cookies"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `<a href="https://example.com/legal/cookies">cookie policy</a>`)
	assert.NotContains(t, w.Body.String(), `href="/cookie-policy">cookie policy</a>`)
}

func TestServeRegister_ConsentGate(t *testing.T) {
	tests := []struct {
		name         string
		consent      bool
		wantCheckbox bool
	}{
		{
			name:         "consent required renders the checkbox with document links",
			consent:      true,
			wantCheckbox: true,
		},
		{
			name:         "consent off keeps the form unchanged",
			consent:      false,
			wantCheckbox: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedPages", mock.Anything).Return([]sitessr.ContentRef{
				{ID: 11, Slug: "terms-of-service", Title: "Terms of Service", PostType: "page", Language: "en"},
				{ID: 12, Slug: "privacy-policy", Title: "Privacy Policy", PostType: "page", Language: "en"},
			}, nil)
			mockService.On("GetPublishedCustomPostTypes", mock.Anything).Return([]string{}, nil)

			handler := setupHandler(t, mockService)
			handler.Assembler().WithLegal(sitessr.LegalOptions{RegistrationConsent: tt.consent})

			req := httptest.NewRequest(http.MethodGet, "/register", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.Equal(t, tt.wantCheckbox, strings.Contains(body, `id="register-consent"`))
			if tt.wantCheckbox {
				start := strings.Index(body, `id="register-consent"`)
				end := strings.Index(body[start:], "</label>")
				require.Positive(t, end, "the consent checkbox is wrapped in a label")
				consentBlock := body[start : start+end]
				assert.Contains(t, consentBlock, `href="/terms-of-service"`)
				assert.Contains(t, consentBlock, `href="/privacy-policy"`)
			}
		})
	}
}

func TestRenderIndex_CookieNoticeEscaping(t *testing.T) {
	templates := newTemplates(t)

	w := httptest.NewRecorder()
	data := sitessr.IndexData{
		LayoutData: sitessr.LayoutData{
			Title:     "Test Site",
			PageTitle: "Test Site",
			Legal: sitessr.LegalData{
				CookieNotice: sitessr.CookieNotice{
					Before:   `<script>alert("before")</script>`,
					LinkText: `<script>alert("link")</script>`,
					After:    `<script>alert("after")</script>`,
					URL:      "/cookie-policy",
				},
			},
		},
		Posts: []sitessr.PostItem{},
	}

	require.NoError(t, templates.RenderIndex(w, data))

	body := w.Body.String()
	assert.NotContains(t, body, `<script>alert("link")</script>`)
	assert.Contains(t, body, `&lt;script&gt;alert(&#34;link&#34;)&lt;/script&gt;`)
}
