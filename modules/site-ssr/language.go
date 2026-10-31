package sitessr

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	// LangCookieName persists the visitor's preferred content language.
	// Set on a valid ?lang request, read when the query param is absent.
	// Exported so the NoCookie allowlist can reference the same value.
	LangCookieName = "lesstruct_lang"

	// ConsentCookieName records the visitor's GDPR choice (accepted/declined).
	// Set client-side by the cookie banner; exempt from NoCookie stripping.
	ConsentCookieName = "lesstruct_consent"

	// ConsentAccepted marks an accepted banner choice.
	ConsentAccepted = "accepted"

	// ConsentDeclined marks a declined banner choice. When present, the SSR
	// engine renders the negotiated language but skips Set-Cookie persistence.
	ConsentDeclined = "declined"

	// langCookieMaxAge is one year in seconds.
	langCookieMaxAge = 31536000
)

// matchConfigured returns the configured entry matching code
// case-insensitively, preserving the configured casing.
func matchConfigured(configured []string, code string) (string, bool) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return "", false
	}
	for _, lang := range configured {
		if strings.EqualFold(lang, trimmed) {
			return lang, true
		}
	}
	return "", false
}

// reorderLanguages returns the configured chain with the preferred language
// first, preserving the configured order for the rest.
func reorderLanguages(configured []string, preferred string) []string {
	ordered := make([]string, 0, len(configured))
	ordered = append(ordered, preferred)
	for _, lang := range configured {
		if !strings.EqualFold(lang, preferred) {
			ordered = append(ordered, lang)
		}
	}
	return ordered
}

// parseAcceptLanguage returns the configured language best matching the
// Accept-Language header, respecting q-values. Tags are matched by primary
// subtag (id-ID matches id). An empty string means no match.
func parseAcceptLanguage(header string, configured []string) string {
	if strings.TrimSpace(header) == "" || len(configured) == 0 {
		return ""
	}
	type candidate struct {
		tag string
		q   float64
		idx int
	}
	var candidates []candidate
	idx := 0
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			idx++
			continue
		}
		tag := part
		q := 1.0
		if before, after, ok := strings.Cut(part, ";"); ok {
			tag = strings.TrimSpace(before)
			for param := range strings.SplitSeq(after, ";") {
				param = strings.TrimSpace(param)
				if value, ok := strings.CutPrefix(param, "q="); ok {
					if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
						q = parsed
					}
				}
			}
		}
		if tag == "" || tag == "*" {
			idx++
			continue
		}
		primary := strings.ToLower(strings.TrimSpace(strings.Split(tag, "-")[0]))
		primary = strings.Split(primary, "_")[0]
		if primary == "" {
			idx++
			continue
		}
		candidates = append(candidates, candidate{tag: primary, q: q, idx: idx})
		idx++
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].q == candidates[j].q {
			return candidates[i].idx < candidates[j].idx
		}
		return candidates[i].q > candidates[j].q
	})
	for _, c := range candidates {
		if matched, ok := matchConfigured(configured, c.tag); ok {
			return matched
		}
	}
	return ""
}

// negotiateLanguage resolves the request's language chain: ?lang query first,
// then the lesstruct_lang cookie, then Accept-Language, then the configured
// primary. A bogus ?lang never errors — it falls through to the next source.
// It returns the ordered chain (preferred first), the display language for
// <html lang>, and whether the response should persist the choice in a cookie.
func negotiateLanguage(r *http.Request, configured []string) (ordered []string, display string, setCookie bool) {
	if len(configured) == 0 {
		return []string{"en"}, "en", false
	}
	if lang := strings.TrimSpace(r.URL.Query().Get("lang")); lang != "" {
		if matched, ok := matchConfigured(configured, lang); ok {
			return reorderLanguages(configured, matched), matched, true
		}
	}
	if cookie, err := r.Cookie(LangCookieName); err == nil {
		if matched, ok := matchConfigured(configured, cookie.Value); ok {
			return reorderLanguages(configured, matched), matched, false
		}
	}
	if matched := parseAcceptLanguage(r.Header.Get("Accept-Language"), configured); matched != "" {
		return reorderLanguages(configured, matched), matched, false
	}
	return append([]string{}, configured...), primaryLanguage(configured), false
}

// isConsentDeclined reports whether the visitor declined non-essential storage.
// When true, the SSR engine must render the negotiated language without
// persisting it in a Set-Cookie response.
func isConsentDeclined(r *http.Request) bool {
	cookie, err := r.Cookie(ConsentCookieName)
	if err != nil {
		return false
	}
	return strings.TrimSpace(cookie.Value) == ConsentDeclined
}

// redirectTarget preserves the request's non-lang query parameters on a
// language redirect so filters and pagination survive the hop.
func redirectTarget(r *http.Request, path string) string {
	query := r.URL.Query()
	if len(query) == 0 {
		return path
	}
	query.Del("lang")
	if encoded := query.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

// requestIsHTTPS reports whether the visitor reached the site over TLS.
// X-Forwarded-Proto wins when present so deployments behind a
// TLS-terminating reverse proxy see the client-facing scheme (mirroring the
// CORS same-origin check); otherwise the connection state decides. Only the
// first forwarded value is used.
func requestIsHTTPS(r *http.Request) bool {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		if first, _, _ := strings.Cut(proto, ","); strings.TrimSpace(first) != "" {
			return strings.EqualFold(strings.TrimSpace(first), "https")
		}
	}
	return r.TLS != nil
}

// setLangCookie persists the negotiated display language for returning
// visitors. Secure follows the request scheme so HTTPS origins keep the
// preference cookie off plain HTTP while local plain-HTTP development
// keeps working. HttpOnly is deliberately absent: the storefront clears
// this cookie from JS when consent is declined, which HttpOnly would
// silently break.
func setLangCookie(w http.ResponseWriter, r *http.Request, display string) {
	http.SetCookie(w, &http.Cookie{
		Name:     LangCookieName,
		Value:    display,
		Path:     "/",
		MaxAge:   langCookieMaxAge,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}
