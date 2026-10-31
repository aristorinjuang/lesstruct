// Package legal carries the storefront's conventional legal-document
// vocabulary: which documents a compliant store publishes, how those
// documents resolve against the published page set in a given language, and
// the consent evidence recorded when a visitor affirms them. Orders, auth,
// and the SSR engine share this package so a single resolution drives both
// rendering (the banner and the consent checkboxes) and enforcement (the
// checkout and registration gates).
package legal

import (
	"slices"
	"time"
)

const (
	// PlaceholderCookiePolicy is the token the cookie-notice translation
	// carries where the policy link belongs. Translations stay plain text;
	// the renderer splits the notice on this token and escapes every piece,
	// so no translation can inject markup.
	PlaceholderCookiePolicy = "{cookie_policy}"
)

var (
	// StoreNoticeCopy is the storefront cookie-notice wording: it names the
	// language and currency preferences and the cart because the commerce
	// edition actually stores those. The commerce edition adopts it; every
	// other edition renders the neutral pair (no currency, no cart).
	StoreNoticeCopy = NoticeCopy{
		PlainKey:      "ui.cookie_notice_store",
		PlainDefault:  "We use cookies to remember your language and currency preferences. The cart uses strictly necessary storage only and always works.",
		PolicyKey:     "ui.cookie_notice_store_policy",
		PolicyDefault: "We use cookies to remember your language and currency preferences. See our {cookie_policy} for details. The cart uses strictly necessary storage only and always works.",
	}
)

// groupKey identifies the translation group a candidate belongs to.
func groupKey(candidate Candidate) int {
	if candidate.TranslationGroupID != nil {
		return *candidate.TranslationGroupID
	}
	return candidate.PageID
}

// best picks the candidate whose language ranks first in the chain, breaking
// ties by page ID so the resolution is deterministic across reads.
func best(candidates []Candidate, languages []string) Candidate {
	chosen := candidates[0]
	for _, candidate := range candidates[1:] {
		if languageRank(languages, candidate.Language) < languageRank(languages, chosen.Language) ||
			(languageRank(languages, candidate.Language) == languageRank(languages, chosen.Language) &&
				candidate.PageID < chosen.PageID) {
			chosen = candidate
		}
	}
	return chosen
}

// languageRank places a language in the preference chain. Languages outside
// the chain rank last and still resolve when nothing else exists.
func languageRank(languages []string, language string) int {
	if index := slices.Index(languages, language); index >= 0 {
		return index
	}
	return len(languages)
}

// Page names one conventionally resolved legal document. The values are the
// canonical (English) slugs: a group is anchored by the page published under
// this slug and its siblings are found through the translation group, so
// localized slugs (e.g. kebijakan-privasi) resolve without a slug table.
type Page string

const (
	// PageCookiePolicy is the cookie notice the consent banner links.
	PageCookiePolicy Page = "cookie-policy"
	// PagePrivacyPolicy is the data-processing notice consent refers to.
	PagePrivacyPolicy Page = "privacy-policy"
	// PageTermsOfService is the contract consent refers to.
	PageTermsOfService Page = "terms-of-service"
	// PageReturnsPolicy is the returns and refunds policy. It carries no
	// consent gate; it is resolved so navigation can place it.
	PageReturnsPolicy Page = "returns-refunds"
)

// NavPlacement controls where conventional legal pages appear in the site
// chrome. The footer always carries the full site map; a placement other
// than footer additionally lists legal pages in the header.
type NavPlacement string

const (
	// NavFooter lists legal pages in the footer only (the default).
	NavFooter NavPlacement = "footer"
	// NavHeader lists legal pages in the header as well as the footer.
	NavHeader NavPlacement = "header"
	// NavBoth is NavHeader under the current chrome contract, where the
	// footer always lists every published page.
	NavBoth NavPlacement = "both"
)

// Pages lists the conventional documents in resolution order.
func Pages() []Page {
	return []Page{
		PageCookiePolicy,
		PagePrivacyPolicy,
		PageTermsOfService,
		PageReturnsPolicy,
	}
}

// NoticeCopy carries one edition's cookie-notice wording: the i18n keys to
// look up plus the English defaults used when no UI catalog is installed.
// The zero value selects the neutral pair (no currency, no cart); the
// commerce edition adopts StoreNoticeCopy instead.
type NoticeCopy struct {
	PlainKey      string
	PlainDefault  string
	PolicyKey     string
	PolicyDefault string
}

// Candidate is one published page row projected onto the resolver's
// vocabulary. TranslationGroupID links a translated page to its group
// anchor; the anchor itself carries nil and is keyed by its own PageID.
type Candidate struct {
	PageID             int
	Slug               string
	Language           string
	TranslationGroupID *int
	UpdatedAt          time.Time
}

// Document is one resolved legal document: the page that answers for it at
// the moment it was resolved. PageID plus UpdatedAt is the document version
// recorded as consent evidence; Slug and Language keep the record readable.
type Document struct {
	Page      Page      `json:"page"`
	PageID    int       `json:"pageId"`
	Slug      string    `json:"slug"`
	Language  string    `json:"language"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// URL returns the document's public path.
func (d Document) URL() string {
	return "/" + d.Slug
}

// Resolution is the outcome of resolving the conventional documents against
// the published page set.
type Resolution struct {
	Documents map[Page]Document
	// Groups holds the translation-group keys of every conventional page
	// row, so a page in a language the resolver did not pick still reads as
	// legal for navigation placement.
	Groups map[int]bool
}

// Document returns the resolved document for page.
func (r Resolution) Document(page Page) (Document, bool) {
	doc, ok := r.Documents[page]
	return doc, ok
}

// Has reports whether the document resolved.
func (r Resolution) Has(page Page) bool {
	_, ok := r.Documents[page]
	return ok
}

// IsLegalGroup reports whether the group key belongs to a conventional
// document.
func (r Resolution) IsLegalGroup(key int) bool {
	return r.Groups[key]
}

// Resolve resolves each conventional document against the published page
// set. languages is the preference chain: the first language a document
// exists in wins, which makes the visitor's negotiated chain produce the
// localized page and a primary-first chain produce the canonical revision.
//
// A document is present only when a row with the canonical slug exists: the
// anchor is what names the group. Once anchored, the best-ranked translation
// in the group answers for it, so a visitor reading Indonesian gets the
// kebijakan-cookie sibling of cookie-policy.
func Resolve(candidates []Candidate, languages []string) Resolution {
	resolution := Resolution{
		Documents: make(map[Page]Document, len(Pages())),
		Groups:    make(map[int]bool),
	}
	if len(candidates) == 0 {
		return resolution
	}

	bySlug := make(map[string][]Candidate, len(candidates))
	byGroup := make(map[int][]Candidate, len(candidates))
	for _, candidate := range candidates {
		bySlug[candidate.Slug] = append(bySlug[candidate.Slug], candidate)
		key := groupKey(candidate)
		byGroup[key] = append(byGroup[key], candidate)
	}

	for _, page := range Pages() {
		seeds := bySlug[string(page)]
		if len(seeds) == 0 {
			continue
		}
		anchor := best(seeds, languages)
		key := groupKey(anchor)
		resolution.Groups[key] = true

		chosen := best(byGroup[key], languages)
		resolution.Documents[page] = Document{
			Page:      page,
			PageID:    chosen.PageID,
			Slug:      chosen.Slug,
			Language:  chosen.Language,
			UpdatedAt: chosen.UpdatedAt,
		}
	}

	return resolution
}

// Consent is the evidence recorded when a visitor affirms the terms: when the
// acceptance happened and the document revisions it referred to.
type Consent struct {
	AcceptedAt time.Time `json:"acceptedAt"`
	Terms      Document  `json:"terms"`
	Privacy    Document  `json:"privacy"`
}

// NewConsent stamps an acceptance of the given documents at now.
func NewConsent(terms, privacy Document, now time.Time) Consent {
	return Consent{
		AcceptedAt: now.UTC(),
		Terms:      terms,
		Privacy:    privacy,
	}
}
