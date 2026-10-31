package legal_test

import (
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/stretchr/testify/assert"
)

func TestStoreNoticeCopy(t *testing.T) {
	copy := legal.StoreNoticeCopy

	assert.NotEmpty(t, copy.PlainKey, "the store copy names its plain key")
	assert.NotEmpty(t, copy.PolicyKey, "the store copy names its policy key")
	assert.NotEqual(t, copy.PlainKey, copy.PolicyKey, "plain and policy are distinct keys")
	assert.NotContains(t, copy.PlainDefault, legal.PlaceholderCookiePolicy, "the plain default carries no link placeholder")
	assert.Contains(t, copy.PolicyDefault, legal.PlaceholderCookiePolicy, "the policy default marks where the link belongs")
	assert.Contains(t, copy.PlainDefault, "cart", "the store copy names the cart the neutral pair avoids")
}

func candidate(pageID int, slug, language string, groupID *int) legal.Candidate {
	return legal.Candidate{
		PageID:             pageID,
		Slug:               slug,
		Language:           language,
		TranslationGroupID: groupID,
		UpdatedAt:          time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC),
	}
}

func TestResolve(t *testing.T) {
	group := func(id int) *int { return &id }

	tests := []struct {
		name       string
		candidates []legal.Candidate
		languages  []string
		wantSlugs  map[legal.Page]string
		wantGroups map[int]bool
	}{
		{
			name:       "no pages resolves nothing",
			candidates: nil,
			languages:  []string{"en"},
			wantSlugs:  map[legal.Page]string{},
			wantGroups: map[int]bool{},
		},
		{
			name: "a monolingual cookie policy resolves",
			candidates: []legal.Candidate{
				candidate(10, "cookie-policy", "en", nil),
			},
			languages: []string{"en"},
			wantSlugs: map[legal.Page]string{
				legal.PageCookiePolicy: "cookie-policy",
			},
			wantGroups: map[int]bool{10: true},
		},
		{
			name: "the visitor language picks the sibling",
			candidates: []legal.Candidate{
				candidate(10, "cookie-policy", "en", nil),
				candidate(11, "kebijakan-cookie", "id", group(10)),
			},
			languages: []string{"id", "en"},
			wantSlugs: map[legal.Page]string{
				legal.PageCookiePolicy: "kebijakan-cookie",
			},
			wantGroups: map[int]bool{10: true},
		},
		{
			name: "the primary language picks the anchor",
			candidates: []legal.Candidate{
				candidate(10, "cookie-policy", "en", nil),
				candidate(11, "kebijakan-cookie", "id", group(10)),
			},
			languages: []string{"en", "id"},
			wantSlugs: map[legal.Page]string{
				legal.PageCookiePolicy: "cookie-policy",
			},
			wantGroups: map[int]bool{10: true},
		},
		{
			name: "an unconfigured language still resolves when nothing else exists",
			candidates: []legal.Candidate{
				candidate(10, "privacy-policy", "fr", nil),
			},
			languages: []string{"en"},
			wantSlugs: map[legal.Page]string{
				legal.PagePrivacyPolicy: "privacy-policy",
			},
			wantGroups: map[int]bool{10: true},
		},
		{
			name: "only conventional documents resolve",
			candidates: []legal.Candidate{
				candidate(10, "about", "en", nil),
				candidate(20, "returns-refunds", "en", nil),
			},
			languages: []string{"en"},
			wantSlugs: map[legal.Page]string{
				legal.PageReturnsPolicy: "returns-refunds",
			},
			wantGroups: map[int]bool{20: true},
		},
		{
			name: "the lowest page ID breaks a language tie",
			candidates: []legal.Candidate{
				candidate(12, "terms-of-service", "en", nil),
				candidate(9, "terms-of-service", "en", nil),
			},
			languages: []string{"en"},
			wantSlugs: map[legal.Page]string{
				legal.PageTermsOfService: "terms-of-service",
			},
			wantGroups: map[int]bool{9: true},
		},
		{
			name: "a localized anchor names its own group",
			candidates: []legal.Candidate{
				candidate(30, "privacy-policy", "en", nil),
				candidate(31, "kebijakan-privasi", "id", group(30)),
				candidate(40, "cookie-policy", "en", nil),
			},
			languages: []string{"id", "en"},
			wantSlugs: map[legal.Page]string{
				legal.PagePrivacyPolicy: "kebijakan-privasi",
				legal.PageCookiePolicy:  "cookie-policy",
			},
			wantGroups: map[int]bool{30: true, 40: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution := legal.Resolve(tt.candidates, tt.languages)

			gotSlugs := make(map[legal.Page]string, len(tt.wantSlugs))
			for page, document := range resolution.Documents {
				gotSlugs[page] = document.Slug
			}
			assert.Equal(t, tt.wantSlugs, gotSlugs)
			assert.Equal(t, tt.wantGroups, resolution.Groups)
		})
	}
}

func TestResolution_Document(t *testing.T) {
	resolution := legal.Resolve([]legal.Candidate{
		candidate(10, "privacy-policy", "en", nil),
	}, []string{"en"})

	document, ok := resolution.Document(legal.PagePrivacyPolicy)
	assert.True(t, ok)
	assert.Equal(t, "/privacy-policy", document.URL())
	assert.Equal(t, 10, document.PageID)
	assert.Equal(t, "en", document.Language)
	assert.True(t, resolution.Has(legal.PagePrivacyPolicy))

	_, ok = resolution.Document(legal.PageCookiePolicy)
	assert.False(t, ok)
	assert.False(t, resolution.Has(legal.PageCookiePolicy))
	assert.True(t, resolution.IsLegalGroup(10))
	assert.False(t, resolution.IsLegalGroup(11))
}

func TestNewConsent(t *testing.T) {
	terms := legal.Document{Page: legal.PageTermsOfService, PageID: 3, Slug: "terms-of-service", Language: "en"}
	privacy := legal.Document{Page: legal.PagePrivacyPolicy, PageID: 4, Slug: "privacy-policy", Language: "en"}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.FixedZone("UTC+7", 7*60*60))

	consent := legal.NewConsent(terms, privacy, now)

	assert.Equal(t, now.UTC(), consent.AcceptedAt)
	assert.Equal(t, terms, consent.Terms)
	assert.Equal(t, privacy, consent.Privacy)
}
