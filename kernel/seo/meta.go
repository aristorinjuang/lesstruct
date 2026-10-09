package seo

import (
	"html/template"
	"strings"
)

// MetaTag represents an HTML meta tag
type MetaTag struct {
	Name      string
	Property  string
	Content   string
	httpEquiv string
	charset   string
}

// Render renders a meta tag as HTML
func (m MetaTag) Render() template.HTML {
	var sb strings.Builder

	sb.WriteString("<meta")

	if m.Name != "" {
		sb.WriteString(` name="`)
		sb.WriteString(template.HTMLEscapeString(m.Name))
		sb.WriteString(`"`)
	}

	if m.Property != "" {
		sb.WriteString(` property="`)
		sb.WriteString(template.HTMLEscapeString(m.Property))
		sb.WriteString(`"`)
	}

	if m.Content != "" {
		sb.WriteString(` content="`)
		sb.WriteString(template.HTMLEscapeString(m.Content))
		sb.WriteString(`"`)
	}

	if m.httpEquiv != "" {
		sb.WriteString(` http-equiv="`)
		sb.WriteString(template.HTMLEscapeString(m.httpEquiv))
		sb.WriteString(`"`)
	}

	if m.charset != "" {
		sb.WriteString(` charset="`)
		sb.WriteString(template.HTMLEscapeString(m.charset))
		sb.WriteString(`"`)
	}

	sb.WriteString(`>`)

	return template.HTML(sb.String())
}

// NewMetaTag creates a new meta tag with name attribute
func NewMetaTag(name, content string) MetaTag {
	return MetaTag{
		Name:    name,
		Content: content,
	}
}

// NewPropertyMetaTag creates a new meta tag with property attribute
func NewPropertyMetaTag(property, content string) MetaTag {
	return MetaTag{
		Property: property,
		Content:  content,
	}
}

// RenderMetaTags renders multiple meta tags as HTML
func RenderMetaTags(tags []MetaTag) template.HTML {
	var sb strings.Builder

	for _, tag := range tags {
		sb.WriteString(string(tag.Render()))
		sb.WriteString("\n")
	}

	return template.HTML(sb.String())
}
