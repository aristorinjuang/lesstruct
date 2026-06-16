package content_test

import (
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/stretchr/testify/assert"
)

func TestContent_AuthorField(t *testing.T) {
	content := contentdomain.Content{
		Author: "Jane Doe",
	}

	assert.Equal(t, "Jane Doe", content.Author, "Author should be set to 'Jane Doe'")
}

func TestContent_AuthorFieldEmpty(t *testing.T) {
	content := contentdomain.Content{}

	assert.Empty(t, content.Author, "Author should be empty by default")
}

func TestContent_UsernameField(t *testing.T) {
	content := contentdomain.Content{
		Author:   "Jane Doe",
		Username: "janedoe",
	}

	assert.Equal(t, "janedoe", content.Username, "Username should be set to 'janedoe'")
	assert.Equal(t, "Jane Doe", content.Author, "Author should be set to 'Jane Doe'")
}
