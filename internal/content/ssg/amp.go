package ssg

import (
	"log"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	tpl "github.com/aristorinjuang/lesstruct/internal/api/template"
)

const (
	ampMaxCustomCSSBytes = 75000
	ampIframeScript      = `<script async custom-element="amp-iframe" src="https://cdn.ampproject.org/v0/amp-iframe-0.1.js"></script>`
)

func addAMPAttr(n *html.Node) {
	var has bool
	for _, a := range n.Attr {
		if a.Key == "amp" || a.Key == "⚡" {
			has = true
			break
		}
	}
	if !has {
		n.Attr = append(n.Attr, html.Attribute{Key: "amp"})
	}
}

func appendBoilerplate(n *html.Node) {
	bpStyle := &html.Node{
		Type: html.ElementNode, DataAtom: atom.Style, Data: "style",
		Attr: []html.Attribute{{Key: "amp-boilerplate"}},
	}
	bpStyle.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: `body{-webkit-animation:-amp-start 8s steps(1,end) 0s 1 normal both;-moz-animation:-amp-start 8s steps(1,end) 0s 1 normal both;-ms-animation:-amp-start 8s steps(1,end) 0s 1 normal both;animation:-amp-start 8s steps(1,end) 0s 1 normal both}@-webkit-keyframes -amp-start{from{visibility:hidden}to{visibility:visible}}@-moz-keyframes -amp-start{from{visibility:hidden}to{visibility:visible}}@-ms-keyframes -amp-start{from{visibility:hidden}to{visibility:visible}}@-o-keyframes -amp-start{from{visibility:hidden}to{visibility:visible}}@keyframes -amp-start{from{visibility:hidden}to{visibility:visible}}`,
	})
	n.AppendChild(bpStyle)

	noscript := &html.Node{
		Type: html.ElementNode, DataAtom: atom.Noscript, Data: "noscript",
	}
	nsStyle := &html.Node{
		Type: html.ElementNode, DataAtom: atom.Style, Data: "style",
		Attr: []html.Attribute{{Key: "amp-boilerplate"}},
	}
	nsStyle.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: `body{-webkit-animation:none;-moz-animation:none;-ms-animation:none;animation:none}`,
	})
	noscript.AppendChild(nsStyle)
	n.AppendChild(noscript)
}

func appendScript(n *html.Node) {
	scriptNode := &html.Node{
		Type: html.ElementNode, DataAtom: atom.Script, Data: "script",
		Attr: []html.Attribute{
			{Key: "async"},
			{Key: "src", Val: "https://cdn.ampproject.org/v0.js"},
		},
	}
	n.AppendChild(scriptNode)
}

func rewriteHead(n *html.Node, theme *tpl.Theme) {
	var keep []*html.Node
	var oldChildren []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		oldChildren = append(oldChildren, c)
	}
	for _, c := range oldChildren {
		n.RemoveChild(c)
	}

	for _, c := range oldChildren {
		if c.Type == html.ElementNode && c.DataAtom == atom.Meta {
			keep = append(keep, c)
		}
	}

	hasCharset := false
	hasViewport := false
	for _, k := range keep {
		for _, a := range k.Attr {
			if a.Key == "charset" {
				hasCharset = true
			}
			if a.Key == "name" && a.Val == "viewport" {
				hasViewport = true
			}
		}
	}

	if !hasCharset {
		n.AppendChild(&html.Node{
			Type: html.ElementNode, DataAtom: atom.Meta, Data: "meta",
			Attr: []html.Attribute{{Key: "charset", Val: "utf-8"}},
		})
	}

	if !hasViewport {
		n.AppendChild(&html.Node{
			Type: html.ElementNode, DataAtom: atom.Meta, Data: "meta",
			Attr: []html.Attribute{{
				Key: "name", Val: "viewport",
			}, {
				Key: "content", Val: "width=device-width,minimum-scale=1,initial-scale=1",
			}},
		})
	}

	for _, k := range keep {
		n.AppendChild(k)
	}

	baseCSS, styleCSS := tpl.ReadThemeStyles(theme)
	combined := baseCSS + "\n" + styleCSS
	combined = minifyCSS(combined)
	if len(combined) > ampMaxCustomCSSBytes {
		log.Printf(
			"AMP inline CSS (%d bytes) exceeds %d byte limit; truncating.",
			len(combined), ampMaxCustomCSSBytes,
		)
		combined = combined[:ampMaxCustomCSSBytes]
	}
	if combined != "" {
		styleNode := &html.Node{
			Type: html.ElementNode, DataAtom: atom.Style, Data: "style",
			Attr: []html.Attribute{{Key: "amp-custom"}},
		}
		styleNode.AppendChild(&html.Node{
			Type: html.TextNode, Data: combined,
		})
		n.AppendChild(styleNode)
	}

	appendBoilerplate(n)
	appendScript(n)
}

func replaceImgWithAmpImg(n *html.Node) {
	if n.Parent == nil {
		return
	}

	attrs := n.Attr
	var src, width, height, alt, class, layout string
	for _, a := range attrs {
		switch a.Key {
		case "src":
			src = a.Val
		case "width":
			width = a.Val
		case "height":
			height = a.Val
		case "alt":
			alt = a.Val
		case "class":
			class = a.Val
		case "layout":
			layout = a.Val
		}
	}

	if layout == "" && width != "" && height != "" {
		layout = "responsive"
	}

	ampAttrs := make([]html.Attribute, 0, len(attrs)+1)
	ampAttrs = append(ampAttrs, html.Attribute{Key: "src", Val: src})
	if width != "" {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "width", Val: width})
	}
	if height != "" {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "height", Val: height})
	}
	if alt != "" {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "alt", Val: alt})
	}
	if class != "" {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "class", Val: class})
	}
	if layout != "" {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "layout", Val: layout})
	}

	ampImg := &html.Node{
		Type: html.ElementNode, Data: "amp-img",
		Attr: ampAttrs,
	}

	n.Parent.InsertBefore(ampImg, n)
	n.Parent.RemoveChild(n)
}

func replaceIframeWithAmpIframe(n *html.Node) {
	if n.Parent == nil {
		return
	}

	attrs := n.Attr
	ampAttrs := make([]html.Attribute, len(attrs))
	copy(ampAttrs, attrs)

	needsWidth := true
	needsHeight := true
	for _, a := range ampAttrs {
		switch a.Key {
		case "width":
			needsWidth = false
		case "height":
			needsHeight = false
		}
	}

	if needsWidth {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "width", Val: "16"})
	}
	if needsHeight {
		ampAttrs = append(ampAttrs, html.Attribute{Key: "height", Val: "9"})
	}
	ampAttrs = append(ampAttrs, html.Attribute{Key: "layout", Val: "responsive"})
	ampAttrs = append(ampAttrs, html.Attribute{Key: "sandbox", Val: "allow-scripts allow-popups allow-forms"})

	ampIframe := &html.Node{
		Type: html.ElementNode, Data: "amp-iframe",
		Attr: ampAttrs,
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		ampIframe.AppendChild(&html.Node{
			Type: c.Type, Data: c.Data, DataAtom: c.DataAtom, Attr: c.Attr,
		})
	}

	n.Parent.InsertBefore(ampIframe, n)
	n.Parent.RemoveChild(n)
}

func isAllowedScript(n *html.Node) bool {
	if n.Type != html.ElementNode || n.DataAtom != atom.Script {
		return false
	}

	for _, a := range n.Attr {
		if a.Key == "src" && strings.HasPrefix(a.Val, "https://cdn.ampproject.org/") {
			return true
		}
		if a.Key == "type" && a.Val == "application/ld+json" {
			return true
		}
	}
	return false
}

func addAMPIframeScript(htmlContent string) string {
	if strings.Contains(htmlContent, "<amp-iframe") && !strings.Contains(htmlContent, "amp-iframe-0.1") {
		htmlContent = strings.Replace(htmlContent, "</head>", "  "+ampIframeScript+"\n</head>", 1)
	}
	return htmlContent
}

func minifyCSS(css string) string {
	css = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(css, "")
	css = regexp.MustCompile(`\n\s*`).ReplaceAllString(css, "")
	css = regexp.MustCompile(`\s*([{}:;,])\s*`).ReplaceAllString(css, "$1")
	css = regexp.MustCompile(`;}`).ReplaceAllString(css, "}")
	return strings.TrimSpace(css)
}

func TransformToAMP(htmlContent string, theme *tpl.Theme) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent, err
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			walk(c)
			c = next
		}

		switch n.DataAtom {
		case atom.Html:
			addAMPAttr(n)

		case atom.Head:
			rewriteHead(n, theme)

		case atom.Img:
			replaceImgWithAmpImg(n)

		case atom.Iframe:
			replaceIframeWithAmpIframe(n)

		case atom.Script:
			if !isAllowedScript(n) {
				n.Parent.RemoveChild(n)
			}
		}
	}
	walk(doc)

	var sb strings.Builder
	if err := html.Render(&sb, doc); err != nil {
		return htmlContent, err
	}

	result := sb.String()
	result = addAMPIframeScript(result)

	return result, nil
}
