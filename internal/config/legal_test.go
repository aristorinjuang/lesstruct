package config_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadLegal(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantNav        legal.NavPlacement
		wantCheckout   *bool
		wantRegister   *bool
		wantCookieURL  string
		wantPrivacyURL string
		wantTermsURL   string
		wantReturnsURL string
		wantErr        string
	}{
		{
			name:    "absent block keeps footer placement and dependent defaults",
			content: "[site_config]\nname = \"x\"\n",
			wantNav: legal.NavFooter,
		},
		{
			name: "explicit behavior wins",
			content: `[legal]
checkout_consent = true
registration_consent = false
legal_nav = "header"
cookie_policy_url = "/legal/cookies"
privacy_policy_url = "https://example.com/privacy"
terms_of_service_url = "/legal/terms"
returns_refunds_url = "/legal/returns"
`,
			wantNav:        legal.NavHeader,
			wantCheckout:   new(true),
			wantRegister:   new(false),
			wantCookieURL:  "/legal/cookies",
			wantPrivacyURL: "https://example.com/privacy",
			wantTermsURL:   "/legal/terms",
			wantReturnsURL: "/legal/returns",
		},
		{
			name:    "both placement is accepted",
			content: "[legal]\nlegal_nav = \"both\"\n",
			wantNav: legal.NavBoth,
		},
		{
			name:    "unknown placement refuses",
			content: "[legal]\nlegal_nav = \"sidebar\"\n",
			wantErr: "legal_nav must be",
		},
		{
			name:    "a script URL refuses",
			content: "[legal]\ncookie_policy_url = \"javascript:alert(1)\"\n",
			wantErr: "cookie_policy_url must be a site-relative path or an http(s) URL",
		},
		{
			name:    "a protocol-relative URL refuses",
			content: "[legal]\nprivacy_policy_url = \"//evil.example/privacy\"\n",
			wantErr: "privacy_policy_url must be a site-relative path or an http(s) URL",
		},
		{
			name:    "malformed toml refuses",
			content: "[legal]\ncheckout_consent = \"yes\"\n",
			wantErr: "failed to parse legal config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writePaymentsConfig(t, tt.content)

			lc, err := config.LoadLegal(cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNav, lc.Nav())
			assert.Equal(t, tt.wantCheckout, lc.CheckoutConsent)
			assert.Equal(t, tt.wantRegister, lc.RegistrationConsent)
			assert.Equal(t, tt.wantCookieURL, lc.CookiePolicyURL)
			assert.Equal(t, tt.wantPrivacyURL, lc.PrivacyPolicyURL)
			assert.Equal(t, tt.wantTermsURL, lc.TermsOfServiceURL)
			assert.Equal(t, tt.wantReturnsURL, lc.ReturnsPolicyURL)
		})
	}
}

func TestLoadLegal_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	lc, err := config.LoadLegal(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Equal(t, legal.NavFooter, lc.Nav())
	assert.Nil(t, lc.CheckoutConsent)
}

func TestLoadLegal_NilConfig(t *testing.T) {
	_, err := config.LoadLegal(nil)
	require.Error(t, err)
}

func TestLegalConfig_ConsentDefaults(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name       string
		config     config.LegalConfig
		present    bool
		wantCheck  bool
		wantRegist bool
	}{
		{
			name:       "documents present enables both gates",
			config:     config.LegalConfig{},
			present:    true,
			wantCheck:  true,
			wantRegist: true,
		},
		{
			name:       "documents absent leaves both gates off",
			config:     config.LegalConfig{},
			present:    false,
			wantCheck:  false,
			wantRegist: false,
		},
		{
			name:       "explicit keys win over document presence",
			config:     config.LegalConfig{CheckoutConsent: &disabled, RegistrationConsent: &enabled},
			present:    false,
			wantCheck:  false,
			wantRegist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantCheck, tt.config.CheckoutConsentEnabled(tt.present))
			assert.Equal(t, tt.wantRegist, tt.config.RegistrationConsentEnabled(tt.present))
		})
	}
}

func TestLegalConfig_Overrides(t *testing.T) {
	lc := config.LegalConfig{
		CookiePolicyURL:   "/cookies",
		PrivacyPolicyURL:  "  ",
		TermsOfServiceURL: "https://example.com/terms",
	}

	overrides := lc.Overrides()

	assert.Equal(t, map[legal.Page]string{
		legal.PageCookiePolicy:   "/cookies",
		legal.PageTermsOfService: "https://example.com/terms",
	}, overrides)
}
