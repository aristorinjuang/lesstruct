package commerce_test

import (
	"context"
	"errors"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/editions/commerce"
	kernelcontent "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func publishedProductContent() *kernelcontent.Content {
	return &kernelcontent.Content{
		ID:            7,
		Title:         "Heavyweight Tee",
		Slug:          "heavyweight-tee",
		Status:        kernelcontent.StatusPublished,
		FeaturedImage: "/media/tee.jpg",
	}
}

func TestNewProductIdentity(t *testing.T) {
	tests := []struct {
		name     string
		content  *kernelcontent.Content
		repoErr  error
		expected commerce.ProductIdentity
		found    bool
	}{
		{
			name:    "published content projects title slug and featured image",
			content: publishedProductContent(),
			expected: commerce.ProductIdentity{
				Title:    "Heavyweight Tee",
				Slug:     "heavyweight-tee",
				ImageURL: "/media/tee.jpg",
			},
			found: true,
		},
		{
			name: "html body image is the fallback",
			content: func() *kernelcontent.Content {
				content := publishedProductContent()
				content.FeaturedImage = ""
				content.Format = kernelcontent.FormatHTML
				content.Content = `<p><img src="/media/body.jpg" alt=""></p>`
				return content
			}(),
			expected: commerce.ProductIdentity{
				Title:    "Heavyweight Tee",
				Slug:     "heavyweight-tee",
				ImageURL: "/media/body.jpg",
			},
			found: true,
		},
		{
			name: "draft content is not found",
			content: func() *kernelcontent.Content {
				content := publishedProductContent()
				content.Status = kernelcontent.StatusDraft
				return content
			}(),
			found: false,
		},
		{
			name:    "repository failure is not found",
			repoErr: errors.New("database unavailable"),
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			repo.On("GetByID", mock.Anything, 7).Return(tt.content, tt.repoErr)

			identity := commerce.NewProductIdentity(contentdomain.NewService(repo, nil, nil))
			got, ok := identity(context.Background(), 7)

			require.Equal(t, tt.found, ok)
			assert.Equal(t, tt.expected, got)
		})
	}
}
