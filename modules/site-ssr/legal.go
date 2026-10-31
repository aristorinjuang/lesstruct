package sitessr

import (
	"context"
	"log"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
)

const (
	// defaultCookieNotice is the English cookie notice used when no UI
	// catalog is installed. It mirrors kernel/i18n/translations/en.toml and
	// carries no policy link: the banner renders exactly as before when no
	// policy page resolves.
	defaultCookieNotice = "We use cookies to remember your preferences. Only strictly necessary storage is used, and everything keeps working."
	// defaultCookieNoticePolicy is the linked variant used once a policy page
	// resolves. The placeholder marks where the link belongs.
	defaultCookieNoticePolicy = "We use cookies to remember your preferences. See our {cookie_policy} for details. Only strictly necessary storage is used, and everything keeps working."
	// defaultCookiePolicyLink labels the banner's policy link when no UI
	// catalog is installed.
	defaultCookiePolicyLink = "cookie policy"
)

// pageGroupKey identifies the translation group a page belongs to: its
// translation-group anchor, or its own ID when it is the anchor.
func pageGroupKey(page ContentRef) int {
	if page.TranslationGroupID != nil {
		return *page.TranslationGroupID
	}
	return page.ID
}

// legalCandidates projects the published page set onto the legal resolver's
// vocabulary.
func legalCandidates(pages []ContentRef) []legal.Candidate {
	candidates := make([]legal.Candidate, 0, len(pages))
	for _, page := range pages {
		candidates = append(candidates, legal.Candidate{
			PageID:             page.ID,
			Slug:               page.Slug,
			Language:           page.Language,
			TranslationGroupID: page.TranslationGroupID,
			UpdatedAt:          page.UpdatedAt,
		})
	}
	return candidates
}

// buildCookieNotice splits the notice translation around the policy
// placeholder. Every piece stays a plain string so html/template escapes it —
// a translation can never inject markup. A translation without the
// placeholder renders as a single unlinked sentence; a placeholder with no
// resolved policy keeps its plain-text label instead of a dead link.
func buildCookieNotice(notice, linkText, url string) CookieNotice {
	before, after, found := strings.Cut(notice, legal.PlaceholderCookiePolicy)
	if !found {
		return CookieNotice{Before: notice}
	}
	return CookieNotice{
		Before:   before,
		LinkText: linkText,
		After:    after,
		URL:      url,
	}
}

// LegalOptions carries the boot-time legal configuration the composition
// root maps from the [legal] block: explicit URL overrides, navigation
// placement, and the consent gates enforced by the orders and auth modules.
// Rendering reads the same options, so the theme never offers a checkbox the
// API would not enforce (or vice versa). CookieNotice selects the banner
// wording; the zero value renders the neutral pair, and the commerce
// edition adopts legal.StoreNoticeCopy to keep the storefront wording.
type LegalOptions struct {
	Overrides           map[legal.Page]string
	Nav                 legal.NavPlacement
	CheckoutConsent     bool
	RegistrationConsent bool
	CookieNotice        legal.NoticeCopy
}

// legalPlacement reports the configured navigation placement, defaulting to
// footer for the zero-value assembler.
func (a *DataAssembler) legalPlacement() legal.NavPlacement {
	if a.legalNav == "" {
		return legal.NavFooter
	}
	return a.legalNav
}

// legalURL prefers the operator's explicit override over the resolved page.
func (a *DataAssembler) legalURL(page legal.Page, resolution legal.Resolution) string {
	if override := strings.TrimSpace(a.legalOverrides[page]); override != "" {
		return override
	}
	if document, ok := resolution.Document(page); ok {
		return document.URL()
	}
	return ""
}

// buildLegal projects the resolution onto the template context. The banner
// prefers the cookie policy and falls back to the privacy policy; without
// either it degrades to plain text.
func (a *DataAssembler) buildLegal(resolution legal.Resolution) LegalData {
	lang := a.preferredLang()

	bannerURL := a.legalURL(legal.PageCookiePolicy, resolution)
	if bannerURL == "" {
		bannerURL = a.legalURL(legal.PagePrivacyPolicy, resolution)
	}

	// Without a policy the banner keeps today's plain notice; only a resolved
	// policy switches to the linked variant and its placeholder. The wording
	// follows the edition's copy (neutral unless the commerce edition
	// adopted the storefront pair); every other edition shares the neutral
	// sentence so a blog never mentions a cart it does not have.
	wording := a.cookieNotice
	if wording.PlainKey == "" {
		wording = legal.NoticeCopy{
			PlainKey:      "ui.cookie_notice",
			PlainDefault:  defaultCookieNotice,
			PolicyKey:     "ui.cookie_notice_policy",
			PolicyDefault: defaultCookieNoticePolicy,
		}
	}
	noticeKey, noticeDefault := wording.PlainKey, wording.PlainDefault
	if bannerURL != "" {
		noticeKey, noticeDefault = wording.PolicyKey, wording.PolicyDefault
	}
	notice := a.uiFormat(lang, noticeKey, noticeDefault)
	linkText := a.uiFormat(lang, "ui.cookie_policy_link", defaultCookiePolicyLink)

	return LegalData{
		CookiePolicyURL:     a.legalURL(legal.PageCookiePolicy, resolution),
		PrivacyPolicyURL:    a.legalURL(legal.PagePrivacyPolicy, resolution),
		TermsOfServiceURL:   a.legalURL(legal.PageTermsOfService, resolution),
		ReturnsPolicyURL:    a.legalURL(legal.PageReturnsPolicy, resolution),
		CookieNotice:        buildCookieNotice(notice, linkText, bannerURL),
		CheckoutConsent:     a.checkoutConsent,
		RegistrationConsent: a.registrationConsent,
	}
}

// buildNavigationItems renders the header and footer navigation from the
// published page set. Conventional legal pages are footer-only by default;
// any other placement also lists them in the header. The footer always
// carries the full site map, so `header` and `both` read the same today.
func (a *DataAssembler) buildNavigationItems(
	ctx context.Context,
	pages []ContentRef,
	currentPath string,
	resolution legal.Resolution,
) (header, footer []NavigationItem) {
	lang := a.preferredLang()
	include := func(item NavigationItem, isLegal bool) {
		if !isLegal || a.legalPlacement() != legal.NavFooter {
			header = append(header, item)
		}
		footer = append(footer, item)
	}

	include(NavigationItem{Title: a.uiFormat(lang, "ui.home", "Home"), URL: "/", IsActive: currentPath == "/"}, false)

	for _, page := range a.preferredPages(pages) {
		include(NavigationItem{
			Title:    page.Title,
			URL:      "/" + page.Slug,
			IsActive: currentPath == "/"+page.Slug,
		}, resolution.IsLegalGroup(pageGroupKey(page)))
	}

	postTypes, err := a.contentService.GetPublishedCustomPostTypes(ctx)
	if err != nil {
		log.Printf("failed to get published custom post types for navigation: %v", err)
		return header, footer
	}
	if a.postTypeResolver == nil {
		return header, footer
	}
	for _, pt := range postTypes {
		resolved, resolveErr := a.postTypeResolver.GetBySlug(pt)
		name := pt
		if resolveErr == nil {
			if localized := a.localizedPostTypeName(resolved, lang); strings.TrimSpace(localized) != "" {
				name = localized
			}
		}
		include(NavigationItem{
			Title:    name,
			URL:      "/" + pt,
			IsActive: currentPath == "/"+pt,
		}, false)
	}

	return header, footer
}

// buildChrome assembles the navigation and legal projection every rendered
// page shares. The published page set is read once per render and feeds both
// consumers.
func (a *DataAssembler) buildChrome(ctx context.Context, currentPath string) chrome {
	pages, err := a.contentService.GetPublishedPages(ctx)
	if err != nil {
		log.Printf("failed to get published pages for navigation: %v", err)
		pages = nil
	}
	resolution := legal.Resolve(legalCandidates(pages), a.languages)
	header, footer := a.buildNavigationItems(ctx, pages, currentPath, resolution)
	return chrome{
		Header: header,
		Footer: footer,
		Legal:  a.buildLegal(resolution),
	}
}

// WithLegal adopts the boot-time legal configuration for rendering. Passing a
// zero value keeps the defaults: conventional slug resolution, footer
// placement, and consent gates off (the composition root turns them on when
// the documents exist). Returns the receiver for chaining.
func (a *DataAssembler) WithLegal(opts LegalOptions) *DataAssembler {
	a.legalOverrides = opts.Overrides
	a.legalNav = opts.Nav
	a.checkoutConsent = opts.CheckoutConsent
	a.registrationConsent = opts.RegistrationConsent
	a.cookieNotice = opts.CookieNotice
	return a
}

// chrome is the navigation and legal projection shared by every page layout.
type chrome struct {
	Header []NavigationItem
	Footer []NavigationItem
	Legal  LegalData
}
