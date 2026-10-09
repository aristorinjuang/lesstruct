package tiptap

import (
	"fmt"
	"strings"
)

const (
	externalRel    = "noopener noreferrer"
	externalTarget = "_blank"
)

func isExternalHref(href string) bool {
	trimmed := strings.TrimSpace(href)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(trimmed, "//")
}

func renderMarks(text string, marks []mark) string {
	result := escapeHTML(text)
	for i := len(marks) - 1; i >= 0; i-- {
		result = renderMark(result, marks[i])
	}
	return result
}

func renderMark(content string, m mark) string {
	switch m.Type {
	case "bold":
		return fmt.Sprintf("<strong>%s</strong>", content)
	case "italic":
		return fmt.Sprintf("<em>%s</em>", content)
	case "underline":
		return fmt.Sprintf("<u>%s</u>", content)
	case "strike":
		return fmt.Sprintf("<s>%s</s>", content)
	case "code":
		return fmt.Sprintf("<code>%s</code>", content)
	case "link":
		return renderLinkMark(content, m.Attrs)
	default:
		return content
	}
}

func renderLinkMark(content string, attrs map[string]any) string {
	href, _ := attrs["href"].(string)
	if href == "" {
		return content
	}

	title, _ := attrs["title"].(string)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<a href="%s"`, escapeAttr(href))

	if isExternalHref(href) {
		fmt.Fprintf(&sb, ` target="%s" rel="%s"`, externalTarget, externalRel)
	}

	if title != "" {
		fmt.Fprintf(&sb, ` title="%s"`, escapeAttr(title))
	}

	sb.WriteString(">")
	sb.WriteString(content)
	sb.WriteString("</a>")
	return sb.String()
}
