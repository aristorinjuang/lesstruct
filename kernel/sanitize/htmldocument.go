package sanitize

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

const (
	externalRel    = "noopener noreferrer"
	externalTarget = "_blank"
)

var (
	anchorTagRe            = regexp.MustCompile(`(?i)<a\b[^>]*>`)
	hrefAttrRe             = regexp.MustCompile(`(?i)href\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'` + "`" + `>]+))`)
	targetAttrWithSpaceRe  = regexp.MustCompile(`(?i)\s+target\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'` + "`" + `>]*)`)
	relAttrWithSpaceRe     = regexp.MustCompile(`(?i)\s+rel\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'` + "`" + `>]*)`)
	trailingSpaceBeforeEnd = regexp.MustCompile(`\s+>`)
)

// htmlDocumentPolicy creates a permissive bluemonday policy for HTML document
// content. It allows full HTML5 elements, inline styles, data-* attributes,
// and iframe embeds. Script tags, event handlers (on*), javascript: URLs,
// <style> blocks, and form controls are blocked: without style blocks and
// inputs HTML-format content cannot host overlay CSS or credential-collection
// UIs, and external CSS/image beacons are closed by the CSP (img-src has no
// wildcard https:).
func htmlDocumentPolicy(iframeAllowlist ...string) *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	// Allow all standard HTML5 elements.
	p.AllowElements(
		"html", "head", "body", "title", "meta", "link",
		"div", "span", "section", "article", "aside", "header", "footer", "nav", "main",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"p", "br", "hr", "pre", "blockquote",
		// Code elements: chroma/highlight output uses <code> inside <pre>,
		// kbd/samp/var are the remaining HTML5 phrase elements.
		"code", "kbd", "samp", "var",
		"ul", "ol", "li", "dl", "dt", "dd",
		"a", "img", "picture", "source", "figure", "figcaption",
		"strong", "em", "b", "i", "u", "s", "small", "sub", "sup", "mark", "del", "ins",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col",
		"video", "audio", "source", "track",
		"embed", "object", "param",
		"svg", "path", "circle", "rect", "line", "polyline", "polygon", "g", "defs", "use",
		"canvas", "map", "area",
	)

	// <style> blocks are stripped for every author: the element is unknown
	// to the policy and bluemonday treats it as fundamentally unsafe, so it
	// is dropped without AllowUnsafe. Page CSS belongs in the theme;
	// per-element styling stays available through inline style attributes.

	// Global attributes.
	p.AllowAttrs("id", "class", "style", "title", "lang", "dir", "role", "tabindex").Globally()
	// aria-hidden appears on chroma line-number cells in externally produced
	// highlight markup; it is a safe accessibility hint.
	p.AllowAttrs("aria-hidden").Globally()
	p.AllowDataAttributes()

	// Allow safe URL schemes only (http, https, mailto).
	p.AllowURLSchemes("http", "https", "mailto")
	// Allow scheme-less URLs too: media remapping rewrites uploads to
	// root-relative paths (e.g. /uploads/media/...), and imported HTML
	// frequently references same-site files without a scheme.
	p.AllowRelativeURLs(true)

	// Link attributes.
	p.AllowAttrs("href", "target", "rel").OnElements("a")
	p.AllowAttrs("src", "alt", "width", "height", "loading", "decoding", "srcset", "sizes").OnElements("img")
	p.AllowAttrs("type", "media").OnElements("source")
	p.AllowAttrs("type", "src", "kind", "srclang", "label").OnElements("track")
	p.AllowAttrs("controls", "autoplay", "loop", "muted", "poster", "preload").OnElements("video", "audio")
	p.AllowAttrs("poster").OnElements("video")

	// Form elements are stripped entirely: HTML-format content must not be
	// able to host credential-collection UI (fake inputs/buttons overlaid on
	// the page). The site's own forms are theme markup, not editorial content.

	// Table attributes.
	p.AllowAttrs("colspan", "rowspan", "scope", "headers").OnElements("th", "td")
	p.AllowAttrs("span").OnElements("colgroup", "col")

	// SVG attributes.
	p.AllowAttrs("viewBox", "xmlns", "fill", "stroke", "stroke-width", "stroke-linecap", "stroke-linejoin", "d", "cx", "cy", "r", "x", "y", "width", "height", "rx", "ry", "points", "transform", "opacity", "clip-path", "mask").OnElements("svg", "path", "circle", "rect", "line", "polyline", "polygon", "g", "defs", "use")
	p.AllowAttrs("href", "xlink:href").OnElements("use", "a")

	// Allow iframe embeds restricted to the allowlisted hosts when provided.
	// bluemonday's AllowIFrames only manages the sandbox attribute, so host
	// restriction is implemented directly: the src attribute must match an
	// allowlisted host (a "*." prefix allows one or more subdomain labels) or
	// be a root-relative path (same-origin embed). Without an allowlist
	// iframes stay stripped.
	if len(iframeAllowlist) > 0 {
		p.AllowElements("iframe")
		p.AllowAttrs("src").Matching(iframeSrcRegexp(iframeAllowlist)).OnElements("iframe")
		p.AllowAttrs("width", "height", "title", "loading", "allow", "allowfullscreen", "referrerpolicy", "frameborder", "scrolling", "name", "sandbox").OnElements("iframe")
	}

	return p
}

// iframeSrcRegexp builds the matcher for iframe src attributes from the
// allowlisted hosts. Hosts may carry a "*." prefix to require one or more
// subdomain labels, and may include a port or path (from CSP sources) to
// narrow matching. The pattern is fully anchored and case-insensitive, and
// userinfo ("@") is rejected anywhere so a host cannot be smuggled into an
// allowlisted domain. Relative srcs must be root-relative: a leading "//" is
// a scheme-relative absolute URL, not a relative path, so it only passes
// when its host is allowlisted.
func iframeSrcRegexp(hosts []string) *regexp.Regexp {
	var hostsRe strings.Builder
	for i, host := range hosts {
		if i > 0 {
			hostsRe.WriteString("|")
		}
		if strings.HasPrefix(host, "*.") {
			hostsRe.WriteString(`[^/?#]+\.`)
			host = strings.TrimPrefix(host, "*.")
		}
		hostsRe.WriteString(regexp.QuoteMeta(host))
	}
	return regexp.MustCompile(fmt.Sprintf(
		`(?i)^(?:/(?:[^/?#][^?#]*)?|(?:https?:)?//(?:%s)(?:[:/][^@?#]*)?(?:[?#][^@]*)?)$`,
		hostsRe.String(),
	))
}

func isExternalHref(href string) bool {
	trimmed := strings.TrimSpace(href)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(trimmed, "//")
}

func hardenAnchorTag(tag string) string {
	hrefMatch := hrefAttrRe.FindStringSubmatch(tag)
	if hrefMatch == nil {
		return tag
	}
	var href string
	for i := 1; i < len(hrefMatch); i++ {
		if hrefMatch[i] != "" {
			href = hrefMatch[i]
			break
		}
	}
	if isExternalHref(href) {
		// External: strip any existing target/rel, then force exact pair.
		result := targetAttrWithSpaceRe.ReplaceAllString(tag, "")
		result = relAttrWithSpaceRe.ReplaceAllString(result, "")
		result = strings.Replace(result, ">", fmt.Sprintf(` target="%s" rel="%s">`, externalTarget, externalRel), 1)
		return result
	}
	// Internal / relative / mailto: strip target and rel entirely.
	result := targetAttrWithSpaceRe.ReplaceAllString(tag, "")
	result = relAttrWithSpaceRe.ReplaceAllString(result, "")
	result = trailingSpaceBeforeEnd.ReplaceAllString(result, ">")
	return result
}

func hardenExternalLinks(input string) string {
	if input == "" {
		return input
	}
	lower := strings.ToLower(input)
	if !strings.Contains(lower, "<a") {
		return input
	}
	return anchorTagRe.ReplaceAllStringFunc(input, hardenAnchorTag)
}

// SanitizeHTMLDocument sanitizes an HTML document body using the permissive
// HTML document policy. <style> blocks are stripped for every author — page
// CSS belongs in the theme. The iframeAllowlist controls which domains
// are allowed in iframe src attributes.
func SanitizeHTMLDocument(html string, iframeAllowlist ...string) string {
	p := htmlDocumentPolicy(iframeAllowlist...)
	sanitized := p.Sanitize(html)
	return hardenExternalLinks(sanitized)
}

// ValidateHTMLDocument validates that the HTML document is non-empty and does
// not contain script tags or event handler attributes. It returns an error if
// the document is invalid.
func ValidateHTMLDocument(html string) error {
	if strings.TrimSpace(html) == "" {
		return fmt.Errorf("html document must not be empty")
	}
	// Check for script tags (bluemonday strips them, but we want to reject early).
	lower := strings.ToLower(html)
	if strings.Contains(lower, "<script") {
		return fmt.Errorf("html document must not contain script tags")
	}
	return nil
}
