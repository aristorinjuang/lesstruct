// Package catalog is the official commerce catalog module: native pricing,
// variant matrices, stock, and SKUs for a configured product post type.
// Products are content items; this module owns the commerce structure keyed
// to them. Prices are integer minor units — never floats.
package catalog

import (
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/capability"
)

// MaxVariants caps the variant matrix size: the product of the option-axis
// sizes must not exceed this, so a runaway cross-product cannot flood the
// table or the admin editor. MaxSKUTrackRunes bounds SKUs (product and
// variant). CapabilityManage gates catalog admin mutations; it is declared
// module-local (a plain kernel capability string) so the kernel vocabulary
// stays frozen and the module contributes it through RoleProvider.
const (
	MaxVariants = 250

	MaxSKUTrackRunes = 100

	CapabilityManage = capability.Capability("catalog.manage")
)

// Sentinels surfaced by the service and mapped to HTTP responses by the
// handlers.
var (
	// ErrProductNotFound is returned when a content item carries no catalog
	// product data.
	ErrProductNotFound = errCatalog("product not found")
	// ErrInvalidPrice is returned when a price is negative.
	ErrInvalidPrice = errCatalog("price must be zero or greater")
	// ErrInvalidSalePrice is returned when a sale price is negative or
	// exceeds the regular price.
	ErrInvalidSalePrice = errCatalog("sale price must be zero or greater and must not exceed the price")
	// ErrInvalidSKU is returned when a SKU exceeds the length bound.
	ErrInvalidSKU = errCatalog("SKU must be at most 100 characters")
	// ErrInvalidOptions is returned when the variant axes are malformed:
	// blank axis names, no axes, or axes without values.
	ErrInvalidOptions = errCatalog("variant options must define at least one axis with non-blank values")
	// ErrInvalidVariant is returned when a variant's options do not match the
	// product's variant axes (unknown axis or value not in the axis list).
	ErrInvalidVariant = errCatalog("variant options must match the product's variant axes")
	// ErrVariantSKUDuplicate is returned when two variants of one product
	// share a non-empty SKU.
	ErrVariantSKUDuplicate = errCatalog("variant SKUs must be unique within a product")
	// ErrInvalidStock is returned when a stock value is negative.
	ErrInvalidStock = errCatalog("stock must be zero or greater")
	// ErrInsufficientStock is returned when an adjustment would drive a
	// tracked stock bucket below zero.
	ErrInsufficientStock = errCatalog("insufficient stock")
	// ErrTooManyVariants is returned when the variant matrix exceeds
	// MaxVariants.
	ErrTooManyVariants = errCatalog("too many variants")
	// ErrInvalidImageURL is returned when a variant image URL is neither an
	// http(s) URL nor a root-relative media path. Empty stays valid (no
	// image), mirroring the gallery rule.
	ErrInvalidImageURL = errCatalog("variant imageUrl must be an http(s) URL or a root-relative path")
)

// errCatalog keeps sentinel construction tidy.
type errCatalog string

func (e errCatalog) Error() string { return string(e) }

// Variant is one member of a product's variant matrix: a concrete option
// combination with its own SKU, optional price override, stock, weight, and
// image.
type Variant struct {
	ID               int64
	ProductContentID int
	Options          map[string]string
	SKU              string
	Price            *int64 // nil inherits the product price
	Stock            int
	WeightGrams      *int
	ImageURL         string
}

// Product is the commerce data for one content item of the configured
// product post type.
type Product struct {
	ContentID         int
	Price             int64 // minor units
	SalePrice         *int64
	SKU               string
	TrackInventory    bool
	Stock             int
	WeightGrams       *int
	LowStockThreshold *int
	Options           map[string][]string // variant axes: name → allowed values
	Variants          []Variant
	UpdatedAt         time.Time
}

// InStock reports whether the product can be sold: inventory-untracked
// products are always in stock; variant products need any variant with
// positive stock; variant-less tracked products need positive stock.
func (p *Product) InStock() bool {
	if !p.TrackInventory {
		return true
	}
	if len(p.Variants) > 0 {
		for i := range p.Variants {
			if p.Variants[i].Stock > 0 {
				return true
			}
		}
		return false
	}
	return p.Stock > 0
}

// LowStock reports whether tracked inventory has reached the configured
// threshold. Untracked products and products without a threshold are never
// low-stock. Variant products sum the variant buckets (and must be in stock);
// variant-less products use the primary stock.
func (p *Product) LowStock() bool {
	if !p.TrackInventory || p.LowStockThreshold == nil {
		return false
	}
	if len(p.Variants) > 0 {
		if !p.InStock() {
			return false
		}
		total := 0
		for i := range p.Variants {
			total += p.Variants[i].Stock
		}
		return total <= *p.LowStockThreshold
	}
	return p.Stock <= *p.LowStockThreshold
}

// EffectivePrice resolves the sellable price for a variant: the variant's
// price override, else the product's sale price, else the product price.
// A nil variant resolves the product-level effective price.
func (p *Product) EffectivePrice(v *Variant) int64 {
	if v != nil && v.Price != nil {
		return *v.Price
	}
	if p.SalePrice != nil {
		return *p.SalePrice
	}
	return p.Price
}

// VariantInStock reports sellability for one variant: the variant's own
// stock when the product tracks inventory, otherwise always in stock.
func (p *Product) VariantInStock(v *Variant) bool {
	if !p.TrackInventory {
		return true
	}
	return v.Stock > 0
}
