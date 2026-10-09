package sanitize

import (
	"net/url"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

var (
	anchorPattern = regexp.MustCompile(`(?si)<a\s+href\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]*))[^>]*>(.*?)</a>`)

	// htmlTagPattern matches the start of an HTML tag: "<" followed by a tag-name
	// letter (opening tag), "/" (closing tag), or "!" (comment / DOCTYPE). Used by
	// ContainsHTML. See ContainsHTML for why it is a tag check, not a "did the
	// sanitizer change the input" check.
	htmlTagPattern = regexp.MustCompile(`<[a-zA-Z!/]`)
)

func isSafeURLScheme(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "http", "https", "mailto", "":
		return true
	default:
		return false
	}
}

// Sanitizer applies the kernel HTML policies: strict plain-text stripping
// and rich-HTML sanitization with safe links.
type Sanitizer struct {
	strictPolicy   *bluemonday.Policy
	richTextPolicy *bluemonday.Policy
}

// SanitizePlainText strips all HTML, leaving plain text.
func (s Sanitizer) SanitizePlainText(value string) string {
	return s.strictPolicy.Sanitize(value)
}

// SanitizeRichHTML sanitizes rich HTML while allowing safe inline markup.
func (s Sanitizer) SanitizeRichHTML(value string) string {
	return s.richTextPolicy.Sanitize(value)
}

// ContainsHTML reports whether value contains an actual HTML tag (an opening "<"
// immediately followed by a tag-name letter, a "/" for closing tags, or a "!" for
// comments/DOCTYPE). It deliberately does NOT treat a bare "&" or a straight "'"
// as HTML: bluemonday's strict policy escapes those to "&amp;"/"&#39;", so a
// previous "did the sanitizer change the input?" heuristic false-positived on
// real titles like "Install & first content" or "roaster's". Plain text that
// merely contains an ampersand, an apostrophe, or a math "<" is left alone.
func (s Sanitizer) ContainsHTML(value string) bool {
	return htmlTagPattern.MatchString(value)
}

// SanitizeHTMLWithSafeLinks sanitizes rich HTML and keeps only safe links.
func (s Sanitizer) SanitizeHTMLWithSafeLinks(html string) string {
	sanitized := s.richTextPolicy.Sanitize(html)
	return anchorPattern.ReplaceAllStringFunc(sanitized, func(match string) string {
		submatch := anchorPattern.FindStringSubmatch(match)
		// Capture groups: 1=double-quoted, 2=single-quoted, 3=unquoted, 4=link text
		href := ""
		linkText := ""
		if len(submatch) >= 5 {
			linkText = submatch[4]
			for i := 1; i <= 3; i++ {
				if submatch[i] != "" {
					href = submatch[i]
					break
				}
			}
		}
		if href == "" {
			return match
		}
		if !isSafeURLScheme(href) {
			return linkText
		}
		return match
	})
}

// NewSanitizer builds a Sanitizer with the default policies.
func NewSanitizer() *Sanitizer {
	strict := bluemonday.StrictPolicy()

	rich := bluemonday.NewPolicy()
	rich.AllowElements(
		"p", "br", "strong", "em", "b", "i",
		"h1", "h2", "h3",
		"ul", "ol", "li",
		"blockquote", "code", "pre",
		"a", "img",
		"hr", "table", "thead", "tbody", "tr", "th", "td",
		"span", "div", "sup", "sub",
	)
	rich.AllowAttrs("href").OnElements("a")
	rich.AllowAttrs("src", "alt", "title").OnElements("img")
	rich.AllowAttrs("class").Globally()
	rich.AllowAttrs("id").Globally()

	return &Sanitizer{
		strictPolicy:   strict,
		richTextPolicy: rich,
	}
}
