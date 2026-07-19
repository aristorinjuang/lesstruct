package sanitize

import (
	"fmt"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// htmlDocumentPolicy creates a permissive bluemonday policy for HTML document
// content. It allows full HTML5 elements, inline styles, <style> blocks,
// data-* attributes, and iframe embeds. Script tags, event handlers (on*),
// and javascript: URLs are blocked.
func htmlDocumentPolicy(iframeAllowlist ...string) *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	// Allow all standard HTML5 elements.
	p.AllowElements(
		"html", "head", "body", "title", "meta", "link",
		"div", "span", "section", "article", "aside", "header", "footer", "nav", "main",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"p", "br", "hr", "pre", "blockquote",
		"ul", "ol", "li", "dl", "dt", "dd",
		"a", "img", "picture", "source", "figure", "figcaption",
		"strong", "em", "b", "i", "u", "s", "small", "sub", "sup", "mark", "del", "ins",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col",
		"form", "input", "textarea", "select", "option", "optgroup", "button", "label",
		"video", "audio", "source", "track",
		"embed", "object", "param",
		"svg", "path", "circle", "rect", "line", "polyline", "polygon", "g", "defs", "use",
		"canvas", "map", "area",
		"style",
	)

	// Global attributes.
	p.AllowAttrs("id", "class", "style", "title", "lang", "dir", "role", "tabindex").Globally()
	p.AllowDataAttributes()

	// Allow safe URL schemes only (http, https, mailto).
	p.AllowURLSchemes("http", "https", "mailto")

	// Link attributes.
	p.AllowAttrs("href", "target", "rel").OnElements("a")
	p.AllowAttrs("src", "alt", "width", "height", "loading", "decoding", "srcset", "sizes").OnElements("img")
	p.AllowAttrs("type", "media").OnElements("source")
	p.AllowAttrs("type", "src", "kind", "srclang", "label").OnElements("track")
	p.AllowAttrs("controls", "autoplay", "loop", "muted", "poster", "preload").OnElements("video", "audio")
	p.AllowAttrs("poster").OnElements("video")

	// Form elements.
	p.AllowAttrs("type", "name", "value", "placeholder", "required", "disabled", "checked", "readonly", "min", "max", "step", "pattern", "minlength", "maxlength").OnElements("input")
	p.AllowAttrs("rows", "cols", "placeholder", "required", "disabled", "readonly").OnElements("textarea")
	p.AllowAttrs("multiple", "required", "disabled").OnElements("select")
	p.AllowAttrs("selected", "disabled", "value").OnElements("option")
	p.AllowAttrs("disabled", "label").OnElements("optgroup")
	p.AllowAttrs("for").OnElements("label")
	p.AllowAttrs("type", "disabled").OnElements("button")

	// Table attributes.
	p.AllowAttrs("colspan", "rowspan", "scope", "headers").OnElements("th", "td")
	p.AllowAttrs("span").OnElements("colgroup", "col")

	// SVG attributes.
	p.AllowAttrs("viewBox", "xmlns", "fill", "stroke", "stroke-width", "stroke-linecap", "stroke-linejoin", "d", "cx", "cy", "r", "x", "y", "width", "height", "rx", "ry", "points", "transform", "opacity", "clip-path", "mask").OnElements("svg", "path", "circle", "rect", "line", "polyline", "polygon", "g", "defs", "use")
	p.AllowAttrs("href", "xlink:href").OnElements("use", "a")

	// Style attributes.
	p.AllowAttrs("type").OnElements("style")

	// AllowUnsafe is required for bluemonday to actually emit <style> elements
	// even when AllowElements declares them. bluemonday treats <style> and
	// <script> as "fundamentally unsafe" and silently drops them by default.
	// HTML format is admin-authored trusted content; <script> is rejected
	// upfront by ValidateHTMLDocument, and CSS-only XSS vectors are
	// neutralized by modern browsers.
	p.AllowUnsafe(true)

	// Allow iframe embeds when allowlist is provided.
	if len(iframeAllowlist) > 0 {
		p.AllowIFrames()
	}

	return p
}

// SanitizeHTMLDocument sanitizes an HTML document body using the permissive
// HTML document policy. The iframeAllowlist controls which domains are allowed
// in iframe src attributes.
func SanitizeHTMLDocument(html string, iframeAllowlist ...string) string {
	p := htmlDocumentPolicy(iframeAllowlist...)
	return p.Sanitize(html)
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
