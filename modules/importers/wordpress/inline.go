package wordpress

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// parseInline converts an HTML fragment into TipTap inline nodes (text nodes
// carrying marks). Unknown tags are ignored; their text content is preserved.
// <br> becomes a hardBreak node.
func parseInline(htmlStr string) []ttNode {
	var nodes []ttNode
	var marks []ttMark

	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return nodes
		case html.TextToken:
			text := string(tokenizer.Text())
			if text == "" {
				continue
			}
			node := ttNode{Type: "text", Text: text}
			if len(marks) > 0 {
				node.Marks = append([]ttMark(nil), marks...)
			}
			nodes = append(nodes, node)
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokenizer.TagName()
			tag := string(name)
			switch tag {
			case "br":
				nodes = append(nodes, ttNode{Type: "hardBreak"})
			case "strong", "b":
				marks = append(marks, ttMark{Type: "bold"})
			case "em", "i":
				marks = append(marks, ttMark{Type: "italic"})
			case "u":
				marks = append(marks, ttMark{Type: "underline"})
			case "s", "del", "strike":
				marks = append(marks, ttMark{Type: "strike"})
			case "code":
				marks = append(marks, ttMark{Type: "code"})
			case "a":
				if hasAttr {
					href := readAttr(tokenizer, "href")
					if href != "" {
						marks = append(marks, ttMark{Type: "link", Attrs: map[string]any{"href": href}})
					}
				}
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			popMark(&marks, string(name))
		}
	}
}

// popMark removes the most-recently-pushed mark matching the closing tag's
// formatting type. Tolerant of unbalanced tags.
func popMark(marks *[]ttMark, tag string) {
	switch tag {
	case "strong", "b":
		removeMarkType(marks, "bold")
	case "em", "i":
		removeMarkType(marks, "italic")
	case "u":
		removeMarkType(marks, "underline")
	case "s", "del", "strike":
		removeMarkType(marks, "strike")
	case "code":
		removeMarkType(marks, "code")
	case "a":
		removeMarkType(marks, "link")
	}
}

func removeMarkType(marks *[]ttMark, markType string) {
	for i := len(*marks) - 1; i >= 0; i-- {
		if (*marks)[i].Type == markType {
			*marks = append((*marks)[:i], (*marks)[i+1:]...)
			return
		}
	}
}

// readAttr reads the value of a named attribute from the tokenizer's current tag.
func readAttr(tokenizer *html.Tokenizer, name string) string {
	for {
		key, val, more := tokenizer.TagAttr()
		if string(key) == name {
			return string(val)
		}
		if !more {
			return ""
		}
	}
}

// findTagAttr scans an HTML fragment for the first occurrence of tag and returns
// the value of its attribute. Returns ok=false if the tag is not found.
func findTagAttr(htmlStr, tag, attr string) (string, bool) {
	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return "", false
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		name, _ := tokenizer.TagName()
		if string(name) != tag {
			continue
		}
		val := readAttr(tokenizer, attr)
		if val == "" {
			continue
		}
		return val, true
	}
}

// findTagText scans an HTML fragment for the first occurrence of tag and returns
// its trimmed text content.
func findTagText(htmlStr, tag string) (string, bool) {
	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	depth := 0
	var text strings.Builder
	found := false
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return strings.TrimSpace(text.String()), found
		}
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == tag && depth == 0 {
				depth = 1
				found = true
			} else if depth > 0 {
				depth++
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == tag && depth == 1 {
				return strings.TrimSpace(text.String()), found
			}
			if depth > 0 {
				depth--
			}
		case html.TextToken:
			if depth > 0 {
				text.Write(tokenizer.Text())
			}
		}
	}
}

// extractTagText extracts the trimmed inner text of the first occurrence of tag.
func extractTagText(htmlStr, tag string) string {
	text, _ := findTagText(htmlStr, tag)
	return text
}

// collapseText returns all visible text from an HTML fragment with tags removed.
func collapseText(htmlStr string) string {
	var text strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.TextToken {
			text.Write(tokenizer.Text())
		}
	}
	return strings.TrimSpace(text.String())
}

var headingTagRe = regexp.MustCompile(`(?i)<h([1-6])\b`)

// detectHeadingLevel infers the heading level from the first <hN> tag in the
// fragment when the block attributes don't specify one.
func detectHeadingLevel(htmlStr string) (int, bool) {
	m := headingTagRe.FindStringSubmatch(htmlStr)
	if m == nil {
		return 0, false
	}
	if len(m[1]) != 1 || m[1][0] < '1' || m[1][0] > '6' {
		return 0, false
	}
	return int(m[1][0] - '0'), true
}

// extractListItems scans a raw <ul>/<ol> fragment for <li> cell text. Used when
// a list block has no nested wp:list-item children (older WordPress exports).
func extractListItems(htmlStr string) []string {
	var items []string
	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	depth := 0
	var cell strings.Builder
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return items
		}
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "li" {
				depth = 1
				cell.Reset()
			} else if depth > 0 {
				depth++
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "li" {
				items = append(items, strings.TrimSpace(cell.String()))
				depth = 0
			} else if depth > 0 {
				depth--
			}
		case html.TextToken:
			if depth > 0 {
				cell.Write(tokenizer.Text())
			}
		}
	}
}

// extractAnnotationLatex pulls the LaTeX source out of a MathML
// <annotation encoding="application/x-tex"> element.
func extractAnnotationLatex(htmlStr string) string {
	var latex string
	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	inAnnotation := false
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return strings.TrimSpace(latex)
		}
		switch tt {
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "annotation" {
				inAnnotation = true
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "annotation" {
				inAnnotation = false
			}
		case html.TextToken:
			if inAnnotation {
				latex += string(tokenizer.Text())
			}
		}
	}
}

// youtubeEmbedURL converts a YouTube watch/shorts/share URL into an embed URL.
// Returns "" if the URL is not a YouTube URL. The provider hint is optional.
func youtubeEmbedURL(rawURL, provider string) string {
	if provider != "" && provider != "youtube" {
		return ""
	}
	lower := strings.ToLower(rawURL)
	if !strings.Contains(lower, "youtube.com") && !strings.Contains(lower, "youtu.be") {
		return ""
	}
	if id := extractYouTubeID(rawURL); id != "" {
		return "https://www.youtube.com/embed/" + id
	}
	return ""
}

// extractYouTubeID extracts the 11-character video ID from common YouTube URLs.
func extractYouTubeID(rawURL string) string {
	// youtu.be/<id>
	if _, rest, ok := strings.Cut(rawURL, "youtu.be/"); ok {
		return takeVideoID(rest)
	}
	// youtube.com/embed/<id> (case-insensitive search, preserve ID case)
	if idx := strings.Index(strings.ToLower(rawURL), "embed/"); idx >= 0 {
		return takeVideoID(rawURL[idx+len("embed/"):])
	}
	// youtube.com/watch?v=<id> or shorts
	if _, rest, ok := strings.Cut(rawURL, "v="); ok {
		return takeVideoID(rest)
	}
	return ""
}

// takeVideoID reads up to 11 alphanumeric/dash/underscore characters.
func takeVideoID(s string) string {
	var id strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			id.WriteRune(r)
			if id.Len() == 11 {
				break
			}
		} else {
			break
		}
	}
	return id.String()
}
