package tiptap

import (
	"fmt"
	"strings"
)

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

	var sb strings.Builder
	fmt.Fprintf(&sb, `<a href="%s"`, escapeAttr(href))

	if target, ok := attrs["target"].(string); ok && target != "" {
		fmt.Fprintf(&sb, ` target="%s"`, escapeAttr(target))
	}
	if rel, ok := attrs["rel"].(string); ok && rel != "" {
		fmt.Fprintf(&sb, ` rel="%s"`, escapeAttr(rel))
	}
	if title, ok := attrs["title"].(string); ok && title != "" {
		fmt.Fprintf(&sb, ` title="%s"`, escapeAttr(title))
	}

	sb.WriteString(">")
	sb.WriteString(content)
	sb.WriteString("</a>")
	return sb.String()
}
