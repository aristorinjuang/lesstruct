package commerce

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/app"
	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	kernelcontent "github.com/aristorinjuang/lesstruct/kernel/content"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type resolverStubStore struct {
	products map[int]*catalogdomain.Product
	err      error
}

func (s *resolverStubStore) GetProduct(_ context.Context, contentID int) (*catalogdomain.Product, error) {
	if s.err != nil {
		return nil, s.err
	}
	if product, ok := s.products[contentID]; ok {
		return product, nil
	}
	return nil, catalogdomain.ErrProductNotFound
}

func (s *resolverStubStore) UpsertProduct(_ context.Context, _ int, _ *catalogdomain.Product) error {
	return nil
}

func (s *resolverStubStore) AdjustStock(_ context.Context, _ int, _ *int64, _ int) (bool, error) {
	return false, nil
}

type resolverStubMailer struct{}

func (resolverStubMailer) SendLowStockAlert(_ context.Context, _ *catalogdomain.Product, _ *int64, _ int) error {
	return nil
}

func TestPublishedProductResolver(t *testing.T) {
	rootGroup := 2

	tests := []struct {
		name       string
		slug       string
		content    *kernelcontent.Content
		contentErr error
		products   map[int]*catalogdomain.Product
		storeErr   error
		wantID     int
		wantFound  bool
	}{
		{
			name:    "success - translated without own record falls back to group root",
			slug:    "kaos",
			content: &kernelcontent.Content{ID: 11, Slug: "kaos", TranslationGroupID: &rootGroup},
			products: map[int]*catalogdomain.Product{
				2: {ContentID: 2, Price: 299000},
			},
			wantID:    2,
			wantFound: true,
		},
		{
			name:    "success - translated with own record keeps own ID",
			slug:    "kaos",
			content: &kernelcontent.Content{ID: 11, Slug: "kaos", TranslationGroupID: &rootGroup},
			products: map[int]*catalogdomain.Product{
				2:  {ContentID: 2, Price: 299000},
				11: {ContentID: 11, Price: 299000},
			},
			wantID:    11,
			wantFound: true,
		},
		{
			name:    "success - store error keeps own ID so the handler surfaces it",
			slug:    "kaos",
			content: &kernelcontent.Content{ID: 11, Slug: "kaos", TranslationGroupID: &rootGroup},
			products: map[int]*catalogdomain.Product{
				2: {ContentID: 2, Price: 299000},
			},
			storeErr:  errors.New("db error"),
			wantID:    11,
			wantFound: true,
		},
		{
			name:       "not found - missing slug",
			slug:       "unknown",
			contentErr: errors.New("not found"),
			wantID:     0,
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			repo.On("GetPublishedBySlugAny", mock.Anything, tt.slug).Return(tt.content, tt.contentErr)

			core := &app.Core{ContentService: contentservice.NewService(repo, nil, nil)}
			catalog := catalogdomain.NewService(&resolverStubStore{products: tt.products, err: tt.storeErr}, resolverStubMailer{})
			resolve := publishedProductResolver(core, catalog)

			gotID, gotFound := resolve(context.Background(), tt.slug)

			require.Equal(t, tt.wantFound, gotFound)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}
