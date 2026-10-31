// Package cart is the official commerce cart module: guest-first,
// server-side carts with live catalog pricing. Carts are keyed by opaque
// bearer tokens; only the token hash is stored. Prices are never stored —
// totals resolve live from the catalog at read time.
package cart

import "context"

const (
	// MaxQuantityPerLine caps the quantity of one cart line.
	MaxQuantityPerLine = 99
	// MinQuantity floors the quantity of one cart line.
	MinQuantity = 1
	// TokenBytes is the entropy of a minted cart token.
	TokenBytes = 32
)

// errCart keeps sentinel construction tidy.
type errCart string

func (e errCart) Error() string { return string(e) }

var (
	// ErrCartNotFound is returned when a cart token resolves to no cart. It
	// also covers empty or malformed tokens, so unusable tokens never
	// surface persistence errors.
	ErrCartNotFound = errCart("cart not found")
	// ErrLineNotFound is returned when a cart line does not belong to the
	// caller's cart.
	ErrLineNotFound = errCart("cart line not found")
	// ErrInvalidQuantity is returned when a quantity is outside the
	// MinQuantity..MaxQuantityPerLine range.
	ErrInvalidQuantity = errCart("quantity must be between 1 and 99")
	// ErrUnknownProduct is returned when the catalog has no such product.
	ErrUnknownProduct = errCart("product not found")
	// ErrUnknownVariant is returned when the product exists but the variant
	// does not belong to it.
	ErrUnknownVariant = errCart("variant not found")
	// ErrOutOfStock is returned when adding or updating a line whose catalog
	// posture is out of stock. Stock is a soft gate: quantities already in
	// the cart may exceed stock; only new demand is refused.
	ErrOutOfStock = errCart("item is out of stock")
)

// StoredCart is the persistence shape of a cart: identity and raw lines.
// Prices resolve live at read time, so storage carries none.
type StoredCart struct {
	ID        int64
	TokenHash string
	Lines     []StoredLine
}

// StoredLine is the persistence shape of one cart line.
type StoredLine struct {
	ID               int64
	CartID           int64
	ProductContentID int
	VariantID        *int64
	Quantity         int
}

// CatalogItem is the commerce projection of one sellable line, resolved by
// the CatalogSource seam. It is the narrowed catalog vocabulary the cart
// needs — never the whole catalog product.
type CatalogItem struct {
	ProductContentID int
	VariantID        *int64
	SKU              string
	Title            string
	Slug             string
	ImageURL         string
	UnitPrice        int64
	InStock          bool
}

// CatalogSource resolves sellable lines from the catalog module. The
// composition root adapts the catalog service onto it, so the cart module
// never imports the catalog packages. Unknown products and variants must
// wrap ErrUnknownProduct/ErrUnknownVariant; anything else is a persistence
// failure that aborts the read.
type CatalogSource interface {
	ResolveLine(ctx context.Context, productContentID int, variantID *int64) (CatalogItem, error)
}

// CartStore is the persistence seam the service is built over. The
// single-source store satisfies it; tests use mocks.
type CartStore interface {
	// CreateCart persists an empty cart keyed by token hash.
	CreateCart(ctx context.Context, tokenHash string) (int64, error)
	// FindCart returns the cart with its raw lines, or ErrCartNotFound.
	FindCart(ctx context.Context, tokenHash string) (*StoredCart, error)
	// AddLine appends a line, merging with an existing line on the same
	// (product, variant) key by incrementing its quantity.
	AddLine(ctx context.Context, cartID int64, productContentID int, variantID *int64, quantity int) (StoredLine, error)
	// SetLineQuantity replaces a line's quantity; the line must belong to
	// the cart or ErrLineNotFound results.
	SetLineQuantity(ctx context.Context, cartID int64, itemID int64, quantity int) error
	// RemoveLine deletes one line of the cart; unknown lines are a no-op.
	RemoveLine(ctx context.Context, cartID int64, itemID int64) error
	// DeleteCart removes the cart and every line in it; unknown carts are
	// a no-op.
	DeleteCart(ctx context.Context, cartID int64) error
}

// Cart is the read shape of a cart: resolved lines with live prices.
type Cart struct {
	ID    int64
	Lines []CartLine
}

// Subtotal sums the available lines' extended prices in minor units.
func (c *Cart) Subtotal() int64 {
	var total int64
	for i := range c.Lines {
		if c.Lines[i].Available {
			total += c.Lines[i].LineTotal()
		}
	}
	return total
}

// ItemCount sums the quantities of the available lines.
func (c *Cart) ItemCount() int {
	var count int
	for i := range c.Lines {
		if c.Lines[i].Available {
			count += c.Lines[i].Quantity
		}
	}
	return count
}

// CartLine is the read shape of one cart line: the stored quantity plus
// the live catalog projection. Available is false when the product or
// variant no longer resolves — such lines are excluded from totals and
// item counts so the cart stays readable.
type CartLine struct {
	ID               int64
	ProductContentID int
	VariantID        *int64
	SKU              string
	Title            string
	Slug             string
	ImageURL         string
	Quantity         int
	UnitPrice        int64
	InStock          bool
	Available        bool
}

// LineTotal is the extended price of the line in minor units.
func (l *CartLine) LineTotal() int64 {
	return l.UnitPrice * int64(l.Quantity)
}
