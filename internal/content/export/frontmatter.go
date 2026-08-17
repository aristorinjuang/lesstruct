package export

import (
	"fmt"
	"maps"
	"strings"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"gopkg.in/yaml.v3"
)

func BuildFrontmatter(c *contentdomain.Content, aliases []string) string {
	fm := make(map[string]any)

	fm["title"] = c.Title
	fm["date"] = c.CreatedAt.Format("2006-01-02T15:04:05Z07:00")

	if c.MetaDescription != "" {
		fm["description"] = c.MetaDescription
	}
	if len(c.Tags) > 0 {
		fm["tags"] = c.Tags
	}
	if c.Author != "" {
		fm["author"] = c.Author
	}

	urlPrefix := "posts"
	if c.PostType != "" && c.PostType != "post" {
		urlPrefix = c.PostType
	}
	fm["url"] = fmt.Sprintf("/%s/%s", urlPrefix, c.Slug)
	fm["language"] = c.Language
	if c.Language == "" {
		fm["language"] = "en"
	}

	if len(aliases) > 0 {
		fm["aliases"] = aliases
	}

	if c.Status == contentdomain.StatusDraft {
		fm["draft"] = true
	}

	if !c.UpdatedAt.IsZero() && !c.UpdatedAt.Equal(c.CreatedAt) {
		fm["lastmod"] = c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	maps.Copy(fm, c.CustomFields)

	out, err := yaml.Marshal(fm)
	if err != nil {
		return "---\n---\n"
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(string(out))
	sb.WriteString("---\n")

	return sb.String()
}
