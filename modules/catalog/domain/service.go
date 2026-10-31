package catalog

import (
	"context"
	"strings"
)

// bucketStock reads the tracked bucket a move touches: the base stock,
// or the variant's. The boolean reports whether the bucket exists — a
// vanished variant reads as not-found instead of a silent no-op.
func bucketStock(product *Product, variantID *int64) (int, bool) {
	if variantID == nil {
		return product.Stock, true
	}
	for i := range product.Variants {
		if product.Variants[i].ID == *variantID {
			return product.Variants[i].Stock, true
		}
	}
	return 0, false
}

// ProductStore is the persistence seam the service is built over. The
// single-source store satisfies it; tests use mocks.
type ProductStore interface {
	// GetProduct returns the product with its variant matrix, or
	// ErrProductNotFound when the content item carries no catalog data.
	GetProduct(ctx context.Context, contentID int) (*Product, error)
	// UpsertProduct writes the product row (insert or update) and replaces
	// the variant matrix wholesale, atomically.
	UpsertProduct(ctx context.Context, contentID int, product *Product) error
	// AdjustStock applies the signed delta to one tracked stock bucket —
	// the product when variantID is nil, else that variant — and reports
	// whether the row changed. Untracked and vanished buckets report
	// false: the caller classifies from its own read.
	AdjustStock(ctx context.Context, contentID int, variantID *int64, delta int) (bool, error)
}

// Mailer delivers the low-stock notice. Delivery is best-effort by
// contract: the stock write is authoritative, a failed send never fails
// an adjustment, and the adapter logs its own failures.
type Mailer interface {
	// SendLowStockAlert reports one tracked bucket crossing its
	// threshold downward, carrying the product, the moved bucket, and
	// the new stock for the notice.
	SendLowStockAlert(ctx context.Context, product *Product, variantID *int64, stock int) error
}

// Service is the catalog domain service: commerce data for products of the
// configured post type.
type Service struct {
	store ProductStore
	mail  Mailer
}

// GetProduct returns the commerce data for one content item, including the
// variant matrix. ErrProductNotFound when the item has none.
func (s *Service) GetProduct(ctx context.Context, contentID int) (*Product, error) {
	if contentID <= 0 {
		return nil, ErrProductNotFound
	}
	product, err := s.store.GetProduct(ctx, contentID)
	if err != nil {
		return nil, err
	}
	return product, nil
}

// UpsertProduct validates and writes the commerce data for one content item,
// replacing the variant matrix. It returns the stored product (re-read, so
// variant IDs and timestamps are populated).
func (s *Service) UpsertProduct(ctx context.Context, contentID int, input UpsertProductInput) (*Product, error) {
	if contentID <= 0 {
		return nil, ErrProductNotFound
	}
	if err := ValidateProductInput(input); err != nil {
		return nil, err
	}

	product := &Product{
		ContentID:         contentID,
		Price:             input.Price,
		SalePrice:         input.SalePrice,
		SKU:               input.SKU,
		TrackInventory:    input.TrackInventory,
		Stock:             input.Stock,
		WeightGrams:       input.WeightGrams,
		LowStockThreshold: input.LowStockThreshold,
		Options:           input.Options,
	}
	product.Variants = make([]Variant, 0, len(input.Variants))
	for _, v := range input.Variants {
		product.Variants = append(product.Variants, Variant{
			ProductContentID: contentID,
			Options:          v.Options,
			SKU:              v.SKU,
			Price:            v.Price,
			Stock:            v.Stock,
			WeightGrams:      v.WeightGrams,
			ImageURL:         strings.TrimSpace(v.ImageURL),
		})
	}

	if err := s.store.UpsertProduct(ctx, contentID, product); err != nil {
		return nil, err
	}
	return s.store.GetProduct(ctx, contentID)
}

// AdjustStock moves one tracked stock bucket by the signed delta — the
// payment-time inventory bridge: fulfillment decrements here, refunds
// restock here. Untracked products never move (and never fail), a zero
// delta is a no-op, and a move below zero is refused with
// ErrInsufficientStock. Unknown products and unknown variants read
// ErrProductNotFound.
func (s *Service) AdjustStock(ctx context.Context, contentID int, variantID *int64, delta int) error {
	if delta == 0 {
		return nil
	}
	product, err := s.GetProduct(ctx, contentID)
	if err != nil {
		return err
	}
	if !product.TrackInventory {
		return nil
	}
	before, ok := bucketStock(product, variantID)
	if !ok {
		return ErrProductNotFound
	}
	adjusted, err := s.store.AdjustStock(ctx, contentID, variantID, delta)
	if err != nil {
		return err
	}
	if !adjusted {
		return ErrInsufficientStock
	}
	// A decrement crossing the threshold downward alerts once:
	// restocks never fire, and a bucket already below stays quiet until
	// it climbs back out and falls again. The alert is best-effort.
	if delta < 0 && product.LowStockThreshold != nil &&
		before > *product.LowStockThreshold && before+delta <= *product.LowStockThreshold {
		_ = s.mail.SendLowStockAlert(ctx, product, variantID, before+delta)
	}
	return nil
}

// NewService builds the service over the store and the mail seam.
func NewService(store ProductStore, mail Mailer) *Service {
	return &Service{store: store, mail: mail}
}
