package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
)

// legalConfigFile mirrors the TOML shape of the [legal] block. It keeps the
// parse shape separate from the public LegalConfig so unknown keys in
// config.toml are simply ignored by the TOML decoder.
type legalConfigFile struct {
	Legal legalConfigParse `toml:"legal"`
}

type legalConfigParse struct {
	CheckoutConsent     *bool  `toml:"checkout_consent"`
	RegistrationConsent *bool  `toml:"registration_consent"`
	LegalNav            string `toml:"legal_nav"`
	CookiePolicyURL     string `toml:"cookie_policy_url"`
	PrivacyPolicyURL    string `toml:"privacy_policy_url"`
	TermsOfServiceURL   string `toml:"terms_of_service_url"`
	ReturnsPolicyURL    string `toml:"returns_refunds_url"`
}

// loadLegalURL validates one explicit override. Empty stays empty; every
// other value must be a site-relative path or an absolute http(s) URL, so a
// mistyped scheme never renders as a link the browser refuses to follow.
func loadLegalURL(key, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return "", nil
	case strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//"):
		return trimmed, nil
	case strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://"):
		return trimmed, nil
	default:
		return "", fmt.Errorf("failed to parse legal config: %s must be a site-relative path or an http(s) URL", key)
	}
}

// LegalConfig is the optional [legal] block from config.toml. Legal pages stay
// ordinary bilingual page content: the platform resolves them by conventional
// slug and this block carries only behavior overrides.
//
// Absent block behavior (backward compatible):
//   - CheckoutConsent / RegistrationConsent: required when both the terms and
//     privacy documents resolve; the checkout and registration flows are
//     unchanged when they do not.
//   - LegalNav: legal pages are listed in the footer only.
//   - URL overrides: empty, so conventional slug resolution applies.
type LegalConfig struct {
	CheckoutConsent     *bool
	RegistrationConsent *bool
	LegalNav            legal.NavPlacement
	CookiePolicyURL     string
	PrivacyPolicyURL    string
	TermsOfServiceURL   string
	ReturnsPolicyURL    string
}

// Nav returns the configured placement, defaulting to footer for the
// zero-value config a missing block produces.
func (c LegalConfig) Nav() legal.NavPlacement {
	if c.LegalNav == "" {
		return legal.NavFooter
	}
	return c.LegalNav
}

// CheckoutConsentEnabled reports whether checkout must demand an affirmative
// consent act. An explicit key wins; otherwise consent is required exactly
// when the terms and privacy documents both resolve.
func (c LegalConfig) CheckoutConsentEnabled(documentsPresent bool) bool {
	if c.CheckoutConsent != nil {
		return *c.CheckoutConsent
	}
	return documentsPresent
}

// RegistrationConsentEnabled reports whether registration must demand an
// affirmative consent act. An explicit key wins; otherwise consent is
// required exactly when the terms and privacy documents both resolve.
func (c LegalConfig) RegistrationConsentEnabled(documentsPresent bool) bool {
	if c.RegistrationConsent != nil {
		return *c.RegistrationConsent
	}
	return documentsPresent
}

// Overrides maps the configured explicit URL overrides onto their
// conventional pages. Empty entries are dropped: they mean "resolve the
// conventional slug".
func (c LegalConfig) Overrides() map[legal.Page]string {
	overrides := make(map[legal.Page]string, 4)
	for page, raw := range map[legal.Page]string{
		legal.PageCookiePolicy:   c.CookiePolicyURL,
		legal.PagePrivacyPolicy:  c.PrivacyPolicyURL,
		legal.PageTermsOfService: c.TermsOfServiceURL,
		legal.PageReturnsPolicy:  c.ReturnsPolicyURL,
	} {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			overrides[page] = trimmed
		}
	}
	return overrides
}

// LoadLegal reads the optional [legal] block from the same config.toml that
// supplies the site config. A missing config directory or file yields a
// zero-value LegalConfig: conventional resolution applies and consent follows
// whether the documents exist.
func LoadLegal(cfg *Config) (LegalConfig, error) {
	if cfg == nil {
		return LegalConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return LegalConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return LegalConfig{}, nil
		}
		return LegalConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — conventional resolution applies.
		return LegalConfig{}, nil
	}

	var lcf legalConfigFile
	if err := toml.Unmarshal(data, &lcf); err != nil {
		return LegalConfig{}, fmt.Errorf("failed to parse legal config: %w", err)
	}

	placement := legal.NavPlacement(strings.TrimSpace(lcf.Legal.LegalNav))
	switch placement {
	case "":
		placement = legal.NavFooter
	case legal.NavFooter, legal.NavHeader, legal.NavBoth:
	default:
		return LegalConfig{}, fmt.Errorf("failed to parse legal config: legal_nav must be %q, %q, or %q", legal.NavFooter, legal.NavHeader, legal.NavBoth)
	}

	cookiePolicyURL, err := loadLegalURL("cookie_policy_url", lcf.Legal.CookiePolicyURL)
	if err != nil {
		return LegalConfig{}, err
	}
	privacyPolicyURL, err := loadLegalURL("privacy_policy_url", lcf.Legal.PrivacyPolicyURL)
	if err != nil {
		return LegalConfig{}, err
	}
	termsOfServiceURL, err := loadLegalURL("terms_of_service_url", lcf.Legal.TermsOfServiceURL)
	if err != nil {
		return LegalConfig{}, err
	}
	returnsPolicyURL, err := loadLegalURL("returns_refunds_url", lcf.Legal.ReturnsPolicyURL)
	if err != nil {
		return LegalConfig{}, err
	}

	return LegalConfig{
		CheckoutConsent:     lcf.Legal.CheckoutConsent,
		RegistrationConsent: lcf.Legal.RegistrationConsent,
		LegalNav:            placement,
		CookiePolicyURL:     cookiePolicyURL,
		PrivacyPolicyURL:    privacyPolicyURL,
		TermsOfServiceURL:   termsOfServiceURL,
		ReturnsPolicyURL:    returnsPolicyURL,
	}, nil
}
