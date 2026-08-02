package hugo

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	highlightRe      = regexp.MustCompile(`{{< highlight\s+"?(\w+)"?(?:\s+"([^"]*)")?\s*>}}`)
	highlightCloseRe = regexp.MustCompile(`{{< /highlight >}}`)
	iframeRe         = regexp.MustCompile(`{{<\s*iframe\s+(.*?)\s*>}}`)
)

func transformHighlight(body string) string {
	var result strings.Builder
	lastEnd := 0

	for {
		openMatch := highlightRe.FindStringSubmatchIndex(body[lastEnd:])
		if openMatch == nil {
			result.WriteString(body[lastEnd:])
			break
		}

		// Adjust indices relative to full body
		openStart := lastEnd + openMatch[0]
		openEnd := lastEnd + openMatch[1]

		lang := body[lastEnd+openMatch[2] : lastEnd+openMatch[3]]
		// lang might have quotes; strip them
		lang = strings.Trim(lang, "\"")

		// Find the closing shortcode
		closeMatch := highlightCloseRe.FindStringIndex(body[openEnd:])
		if closeMatch == nil {
			// No closing tag found; write the rest as-is and stop
			result.WriteString(body[lastEnd:])
			break
		}

		closeStart := openEnd + closeMatch[0]
		closeEnd := openEnd + closeMatch[1]

		// Extract the code content between open and close
		codeContent := body[openEnd:closeStart]
		codeContent = strings.TrimSpace(codeContent)

		// Write everything before this shortcode
		result.WriteString(body[lastEnd:openStart])

		// Write the transformed code block
		if lang != "" && lang != "text" && lang != "plaintext" {
			fmt.Fprintf(&result, `<pre><code class="language-%s">%s</code></pre>`, lang, codeContent)
		} else {
			fmt.Fprintf(&result, "<pre><code>%s</code></pre>", codeContent)
		}

		lastEnd = closeEnd
	}

	return result.String()
}

func transformIframe(body string) string {
	return iframeRe.ReplaceAllStringFunc(body, func(match string) string {
		// Extract the inner content between iframe and >
		inner := match
		inner = strings.TrimPrefix(inner, "{{<")
		inner = strings.TrimSuffix(inner, ">}}")
		inner = strings.TrimPrefix(inner, "iframe")
		inner = strings.TrimSpace(inner)

		// Parse key="value" pairs
		attrs := parseAttrs(inner)

		src := attrs["src"]
		title := attrs["title"]
		youtube := attrs["youtube"]

		if youtube != "" {
			if src == "" {
				src = "https://www.youtube.com/embed/" + youtube
			}
			if title == "" {
				title = "YouTube video player"
			}
			return fmt.Sprintf(
				`<iframe width="740" height="416" src="%s" title="%s" frameborder="0" allowfullscreen></iframe>`,
				src, title,
			)
		}

		if title == "" {
			title = "Embedded content"
		}
		return fmt.Sprintf(
			`<iframe width="740" height="416" src="%s" title="%s"></iframe>`,
			src, title,
		)
	})
}

func parseAttrs(s string) map[string]string {
	attrs := make(map[string]string)
	re := regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		attrs[m[1]] = m[2]
	}
	return attrs
}

func TransformShortcodes(body string) string {
	body = transformHighlight(body)
	body = transformIframe(body)
	return body
}
