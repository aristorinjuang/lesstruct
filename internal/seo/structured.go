package seo

import (
	"encoding/json"
	"strings"
)

// JSONLDContext represents the JSON-LD context
type JSONLDContext string

const (
	// JSONLDContextSchemaOrg is the Schema.org context
	JSONLDContextSchemaOrg JSONLDContext = "https://schema.org"
)

// JSONLDType represents the JSON-LD type
type JSONLDType string

const (
	// JSONLDTypeArticle is the Article type
	JSONLDTypeArticle JSONLDType = "Article"
	// JSONLDTypeBlogPosting is the BlogPosting type
	JSONLDTypeBlogPosting JSONLDType = "BlogPosting"
	// JSONLDTypeNewsArticle is the NewsArticle type
	JSONLDTypeNewsArticle JSONLDType = "NewsArticle"
)

// Person represents a person in Schema.org
type Person struct {
	Type  string `json:"@type"`
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// NewPerson creates a new Person
func NewPerson(name string) *Person {
	return &Person{
		Type: "Person",
		Name: name,
	}
}

// ArticleStructuredData represents Schema.org Article structured data
type ArticleStructuredData struct {
	Context       JSONLDContext `json:"@context"`
	Type          JSONLDType    `json:"@type"`
	Headline      string        `json:"headline"`
	Description   string        `json:"description"`
	Image         string        `json:"image,omitempty"`
	DatePublished string        `json:"datePublished"`
	DateModified  string        `json:"dateModified"`
	Author        *Person       `json:"author"`
	Keywords      string        `json:"keywords,omitempty"`
}

// ToJSON converts the structured data to JSON string
func (a ArticleStructuredData) ToJSON() (string, error) {
	bytes, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToJSONLDScript renders the structured data as a JSON-LD script tag
func (a ArticleStructuredData) ToJSONLDScript() (string, error) {
	json, err := a.ToJSON()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(`<script type="application/ld+json">`)
	sb.WriteString(json)
	sb.WriteString(`</script>`)

	return sb.String(), nil
}

// GenerateArticleStructuredData generates Article structured data from content metadata
func GenerateArticleStructuredData(title, description, image, datePublished, dateModified, authorName string, tags []string) ArticleStructuredData {
	keywords := strings.Join(tags, ", ")

	data := ArticleStructuredData{
		Context:       JSONLDContextSchemaOrg,
		Type:          JSONLDTypeArticle,
		Headline:      title,
		Description:   description,
		Image:         image,
		DatePublished: datePublished,
		DateModified:  dateModified,
		Author:        NewPerson(authorName),
		Keywords:      keywords,
	}

	return data
}

// DefaultArticleStructuredData generates Article structured data with defaults
func DefaultArticleStructuredData(title, description, authorName string) ArticleStructuredData {
	return GenerateArticleStructuredData(
		title,
		description,
		"",
		"",
		"",
		authorName,
		[]string{},
	)
}
