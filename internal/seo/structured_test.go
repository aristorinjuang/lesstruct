package seo_test

import (
	"encoding/json"
	"strings"
	"testing"

	appseo "github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPerson(t *testing.T) {
	person := appseo.NewPerson("John Doe")

	assert.Equal(t, "John Doe", person.Name, "NewPerson() Name")
	assert.Equal(t, "Person", person.Type, "NewPerson() Type")
}

func TestArticleStructuredData_ToJSON(t *testing.T) {
	data := appseo.ArticleStructuredData{
		Context:      appseo.JSONLDContextSchemaOrg,
		Type:         appseo.JSONLDTypeArticle,
		Headline:     "Test Article",
		Description:  "Test description",
		Image:        "https://example.com/image.jpg",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified: "2026-04-10T12:00:00Z",
		Author:       appseo.NewPerson("John Doe"),
		Keywords:     "tag1, tag2",
	}

	jsonStr, err := data.ToJSON()
	require.NoError(t, err, "ArticleStructuredData.ToJSON() error")

	var parsed map[string]any
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, "Failed to parse JSON")

	assert.Equal(t, "https://schema.org", parsed["@context"], "ArticleStructuredData.ToJSON() @context")
	assert.Equal(t, "Article", parsed["@type"], "ArticleStructuredData.ToJSON() @type")
	assert.Equal(t, "Test Article", parsed["headline"], "ArticleStructuredData.ToJSON() headline")
	assert.Equal(t, "Test description", parsed["description"], "ArticleStructuredData.ToJSON() description")
}

func TestArticleStructuredData_ToJSONLDScript(t *testing.T) {
	data := appseo.DefaultArticleStructuredData("Test Article", "Test description", "John Doe")

	script, err := data.ToJSONLDScript()
	require.NoError(t, err, "ArticleStructuredData.ToJSONLDScript() error")

	assert.Contains(t, script, `<script type="application/ld+json">`, "ArticleStructuredData.ToJSONLDScript() missing script tag")
	assert.Contains(t, script, `</script>`, "ArticleStructuredData.ToJSONLDScript() missing closing script tag")
	assert.True(t,
		strings.Contains(script, `"@context":"https://schema.org"`) || strings.Contains(script, `"@context": "https://schema.org"`),
		"ArticleStructuredData.ToJSONLDScript() missing context",
	)
	assert.True(t,
		strings.Contains(script, `"@type":"Article"`) || strings.Contains(script, `"@type": "Article"`),
		"ArticleStructuredData.ToJSONLDScript() missing type",
	)
}

func TestGenerateArticleStructuredData(t *testing.T) {
	title := "Test Article"
	description := "Test description"
	image := "https://example.com/image.jpg"
	datePublished := "2026-04-10T00:00:00Z"
	dateModified := "2026-04-10T12:00:00Z"
	authorName := "John Doe"
	tags := []string{"tag1", "tag2"}

	data := appseo.GenerateArticleStructuredData(title, description, image, datePublished, dateModified, authorName, tags)

	assert.Equal(t, title, data.Headline, "GenerateArticleStructuredData() Headline")
	assert.Equal(t, description, data.Description, "GenerateArticleStructuredData() Description")
	assert.Equal(t, image, data.Image, "GenerateArticleStructuredData() Image")
	assert.Equal(t, datePublished, data.DatePublished, "GenerateArticleStructuredData() DatePublished")
	assert.Equal(t, dateModified, data.DateModified, "GenerateArticleStructuredData() DateModified")
	require.NotNil(t, data.Author, "GenerateArticleStructuredData() Author should not be nil")
	assert.Equal(t, authorName, data.Author.Name, "GenerateArticleStructuredData() Author.Name")
	assert.Equal(t, "tag1, tag2", data.Keywords, "GenerateArticleStructuredData() Keywords")
}

func TestDefaultArticleStructuredData(t *testing.T) {
	title := "Test Article"
	description := "Test description"
	authorName := "John Doe"

	data := appseo.DefaultArticleStructuredData(title, description, authorName)

	assert.Equal(t, title, data.Headline, "DefaultArticleStructuredData() Headline")
	assert.Equal(t, description, data.Description, "DefaultArticleStructuredData() Description")
	assert.Empty(t, data.Image, "DefaultArticleStructuredData() Image should be empty")
	assert.Empty(t, data.DatePublished, "DefaultArticleStructuredData() DatePublished should be empty")
	assert.Empty(t, data.DateModified, "DefaultArticleStructuredData() DateModified should be empty")
	require.NotNil(t, data.Author, "DefaultArticleStructuredData() Author should not be nil")
	assert.Equal(t, authorName, data.Author.Name, "DefaultArticleStructuredData() Author.Name")
	assert.Empty(t, data.Keywords, "DefaultArticleStructuredData() Keywords should be empty")
}

func TestArticleStructuredData_WithTags(t *testing.T) {
	tags := []string{"go", "cms", "web"}
	data := appseo.DefaultArticleStructuredData("Test", "Description", "Author")

	data.Keywords = strings.Join(tags, ", ")

	assert.Equal(t, "go, cms, web", data.Keywords, "ArticleStructuredData Keywords")

	jsonStr, _ := data.ToJSON()
	var parsed map[string]any
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, "Failed to parse JSON")

	assert.Equal(t, "go, cms, web", parsed["keywords"], "ArticleStructuredData JSON keywords")
}

func TestArticleStructuredData_EmptyFields(t *testing.T) {
	data := appseo.ArticleStructuredData{
		Context: appseo.JSONLDContextSchemaOrg,
		Type:    appseo.JSONLDTypeArticle,
	}

	jsonStr, err := data.ToJSON()
	require.NoError(t, err, "ArticleStructuredData.ToJSON() error")

	var parsed map[string]any
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, "Failed to parse JSON")

	assert.Empty(t, parsed["headline"], "ArticleStructuredData.ToJSON() headline should be empty")
	assert.Nil(t, parsed["image"], "ArticleStructuredData.ToJSON() image should be nil")
	assert.Nil(t, parsed["keywords"], "ArticleStructuredData.ToJSON() keywords should be nil")
}
