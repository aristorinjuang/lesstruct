package sanitize

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

type Sanitizer struct {
	strictPolicy   *bluemonday.Policy
	richTextPolicy *bluemonday.Policy
}

func (s Sanitizer) SanitizePlainText(value string) string {
	return s.strictPolicy.Sanitize(value)
}

func (s Sanitizer) SanitizeRichHTML(value string) string {
	return s.richTextPolicy.Sanitize(value)
}

func (s Sanitizer) ContainsHTML(value string) bool {
	sanitized := s.strictPolicy.Sanitize(value)
	return strings.TrimSpace(sanitized) != strings.TrimSpace(value)
}

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

var anchorPattern = regexp.MustCompile(`(?si)<a\s+href\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]*))[^>]*>(.*?)</a>`)

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
