package api

import (
	"context"

	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
)

// projectPublicProduct maps the domain product onto the public payload.
func projectPublicProduct(product *catalogdomain.Product, currency string) PublicProduct {
	out := PublicProduct{
		ContentID: product.ContentID,
		SKU:       product.SKU,
		Currency:  currency,
		Price:     product.EffectivePrice(nil),
		InStock:   product.InStock(),
		LowStock:  product.LowStock(),
		Stock:     product.Stock,
		Options:   product.Options,
	}
	if product.SalePrice != nil {
		sale := *product.SalePrice
		out.SalePrice = &sale
	}
	out.Variants = make([]PublicVariant, 0, len(product.Variants))
	for i := range product.Variants {
		v := &product.Variants[i]
		out.Variants = append(out.Variants, PublicVariant{
			ID:       v.ID,
			Options:  v.Options,
			SKU:      v.SKU,
			Price:    product.EffectivePrice(v),
			InStock:  product.VariantInStock(v),
			ImageURL: v.ImageURL,
		})
	}
	return out
}

// PostTypeChecker reports whether a post-type slug is registered. Satisfied
// by the composition root over the post-type registry; the module fails
// startup closed when its configured product type is missing.
type PostTypeChecker interface {
	Exists(slug string) bool
}

// ProductSlugResolver resolves a published product slug to its content ID.
// Satisfied by the composition root over the content service's published
// lookup; unpublished or unknown slugs resolve to not-found.
type ProductSlugResolver func(ctx context.Context, slug string) (contentID int, found bool)

// Config carries the operator knobs the handlers need: which post type is
// the catalog product type, and the display currency for public payloads.
type Config struct {
	PostType string
	Currency string
}

// PublicProduct is the commerce payload themes and headless frontends
// consume: pricing (minor units), stock posture, and the variant matrix
// with resolved effective prices.
type PublicProduct struct {
	ContentID int                 `json:"contentId"`
	SKU       string              `json:"sku,omitempty"`
	Currency  string              `json:"currency"`
	Price     int64               `json:"price"`
	SalePrice *int64              `json:"salePrice,omitempty"`
	InStock   bool                `json:"inStock"`
	LowStock  bool                `json:"lowStock,omitempty"`
	Stock     int                 `json:"stock,omitempty"`
	Options   map[string][]string `json:"options,omitempty"`
	Variants  []PublicVariant     `json:"variants,omitempty"`
}

// PublicVariant is one matrix entry in the public payload.
type PublicVariant struct {
	ID       int64             `json:"id"`
	Options  map[string]string `json:"options"`
	SKU      string            `json:"sku,omitempty"`
	Price    int64             `json:"price"`
	InStock  bool              `json:"inStock"`
	ImageURL string            `json:"imageUrl,omitempty"`
}
