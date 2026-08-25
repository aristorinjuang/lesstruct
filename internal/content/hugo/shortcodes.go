package hugo

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Whitespace-tolerant Hugo highlight shortcode matching. Both quoted and bare
// languages/options are accepted ({{< highlight go "linenos=table" >}},
// {{< highlight "go" >}}, {{< highlight >}}), and the closing tag tolerates
// Hugo's whitespace forms ({{< /highlight >}}, {{< / highlight >}}).
var (
	highlightRe      = regexp.MustCompile(`(?s){{<\s*highlight\s+(.*?)\s*>}}`)
	highlightCloseRe = regexp.MustCompile(`{{<\s*/\s*highlight\s*>}}`)
	iframeRe         = regexp.MustCompile(`{{<\s*iframe\s+(.*?)\s*>}}`)

	highlightArgRe = regexp.MustCompile(`"([^"]*)"|(\S+)`)

	// Languages rendered as plain code blocks without a language class,
	// mirroring the historical behavior for text-like languages.
	plainLanguages = map[string]bool{
		"text":      true,
		"plaintext": true,
		"plain":     true,
		"txt":       true,
	}
)

// parseHighlightArgs splits the highlight shortcode argument string into the
// language (first positional token) and its key=value / flag options.
func parseHighlightArgs(argString string) (string, map[string]string) {
	opts := make(map[string]string)
	lang := ""

	for _, match := range highlightArgRe.FindAllStringSubmatch(argString, -1) {
		arg := match[1]
		if arg == "" {
			arg = match[2]
		}

		if lang == "" && !strings.Contains(arg, "=") {
			lang = strings.Trim(arg, `"`)
			continue
		}

		key, value, found := strings.Cut(arg, "=")
		if !found {
			key, value = arg, "true"
		}
		opts[strings.ToLower(strings.Trim(key, `"`))] = strings.Trim(value, `"`)
	}

	return lang, opts
}

// linenosOption maps the Hugo linenos option onto chroma formatting:
// "table" uses the copy-paste friendly lntable layout, "true"/"inline" use
// inline gutter numbers, anything else disables numbering.
func linenosOption(opts map[string]string) (enabled, inTable bool) {
	value, ok := opts["linenos"]
	if !ok {
		return false, false
	}

	switch strings.ToLower(value) {
	case "table":
		return true, true
	case "true", "inline":
		return true, false
	default:
		return false, false
	}
}

// languagePreWrapper renders chroma's <pre>/<code> wrappers with the language
// class and the nohighlight marker (client-side highlighters must skip
// server-highlighted blocks instead of re-processing them).
type languagePreWrapper struct {
	lang string
}

func (w languagePreWrapper) Start(code bool, styleAttr string) string {
	if !code {
		return fmt.Sprintf("<pre%s>", styleAttr)
	}

	if w.lang != "" {
		return fmt.Sprintf(`<pre%s><code class="language-%s nohighlight" data-lang="%s">`, styleAttr, w.lang, w.lang)
	}
	return fmt.Sprintf(`<pre%s><code class="nohighlight">`, styleAttr)
}

func (w languagePreWrapper) End(code bool) string {
	if code {
		return "</code></pre>"
	}
	return "</pre>"
}

// renderHighlightedCode converts raw source into chroma's class-based HTML
// (Hugo-compatible: <div class="highlight"><pre class="chroma">…) honoring the
// linenos option. It falls back to a plain escaped code block whenever the
// language is unknown or rendering fails — importing must never fail on code.
func renderHighlightedCode(lang, code string, opts map[string]string) string {
	if lang == "" || plainLanguages[lang] {
		return plainCodeBlock(lang, code)
	}

	lexer := lexers.Get(lang)
	if lexer == nil {
		return plainCodeBlock(lang, code)
	}

	lineNumbers, lineNumbersInTable := linenosOption(opts)
	formatterOptions := []chromahtml.Option{
		chromahtml.WithClasses(true),
		chromahtml.WithPreWrapper(languagePreWrapper{lang: lang}),
	}
	if lineNumbers {
		formatterOptions = append(
			formatterOptions,
			chromahtml.WithLineNumbers(true),
			chromahtml.LineNumbersInTable(lineNumbersInTable),
		)
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return plainCodeBlock(lang, code)
	}

	var buf bytes.Buffer
	if err := chromahtml.New(formatterOptions...).Format(&buf, styles.Get("github"), iterator); err != nil {
		return plainCodeBlock(lang, code)
	}

	return `<div class="highlight">` + buf.String() + "</div>"
}

// plainCodeBlock emits an escaped code block without server-side highlighting.
// No nohighlight marker here: client-side highlighters are free to pick it up.
func plainCodeBlock(lang, code string) string {
	escaped := html.EscapeString(code)
	if lang != "" && !plainLanguages[lang] {
		return fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, lang, escaped)
	}
	return fmt.Sprintf("<pre><code>%s</code></pre>", escaped)
}

func transformHighlight(body string) string {
	var result strings.Builder
	lastEnd := 0

	for {
		openMatch := highlightRe.FindStringSubmatchIndex(body[lastEnd:])
		if openMatch == nil {
			// {{< highlight >}} without arguments (and unknown forms) is left
			// untouched.
			result.WriteString(body[lastEnd:])
			break
		}

		// Adjust indices relative to full body
		openStart := lastEnd + openMatch[0]
		openEnd := lastEnd + openMatch[1]

		lang, opts := parseHighlightArgs(body[lastEnd+openMatch[2] : lastEnd+openMatch[3]])

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
		codeContent := strings.TrimSpace(body[openEnd:closeStart])

		// Write everything before this shortcode
		result.WriteString(body[lastEnd:openStart])

		result.WriteString(renderHighlightedCode(lang, codeContent, opts))

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
