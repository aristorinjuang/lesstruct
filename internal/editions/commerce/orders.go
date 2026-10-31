// Orders wiring: the three seam adapters the orders module needs. The cart
// service feeds checkout, the content service snapshots published titles,
// and the platform email service carries the customer notifications. The
// wire step lives with the state in commerce.go.
package commerce

import (
	"context"
	"errors"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/kernel"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
)

// cartCheckout adapts the cart domain service onto the orders module's
// CartSource seam: the checkout consumes a live-priced cart read, and
// unknown or already-consumed tokens map to the order sentinel on both
// methods, failing the checkout closed (double submits and lost races
// read the same as an unknown token). Consume is the atomic claim —
// it deletes the cart, and whoever's delete lands first wins the
// checkout.
type cartCheckout struct {
	service *cartdomain.Service
}

// Snapshot resolves the presented cart: every line carries its product and
// variant keys, live posture, and live price. The cart read model already
// excludes unavailable lines' prices from suggestions — checkout sees the
// same lines the customer bought.
func (a cartCheckout) Snapshot(ctx context.Context, token string) (ordersdomain.CartSnapshot, error) {
	cart, err := a.service.GetCart(ctx, token)
	if err != nil {
		if errors.Is(err, cartdomain.ErrCartNotFound) {
			return ordersdomain.CartSnapshot{}, ordersdomain.ErrCartGone
		}
		return ordersdomain.CartSnapshot{}, err
	}

	snapshot := ordersdomain.CartSnapshot{Lines: make([]ordersdomain.CartSnapshotLine, 0, len(cart.Lines))}
	for i := range cart.Lines {
		snapshot.Lines = append(snapshot.Lines, ordersdomain.CartSnapshotLine{
			ProductContentID: cart.Lines[i].ProductContentID,
			VariantID:        cart.Lines[i].VariantID,
			SKU:              cart.Lines[i].SKU,
			Quantity:         cart.Lines[i].Quantity,
			UnitPrice:        cart.Lines[i].UnitPrice,
			InStock:          cart.Lines[i].InStock,
			Available:        cart.Lines[i].Available,
		})
	}
	return snapshot, nil
}

// Consume claims the presented cart by deleting it. Unknown and
// already-consumed tokens read as ErrCartGone so a lost race or a retry
// aborts before any order lands; a persistence failure passes through
// for the domain to abort on.
func (a cartCheckout) Consume(ctx context.Context, token string) error {
	err := a.service.DeleteCart(ctx, token)
	if errors.Is(err, cartdomain.ErrCartNotFound) {
		return ordersdomain.ErrCartGone
	}
	return err
}

// NewCartCheckout builds the seam adapter over the adopted cart service.
func NewCartCheckout(service *cartdomain.Service) ordersdomain.CartSource {
	return cartCheckout{service: service}
}

// contentTitles adapts the content service onto the orders module's
// ContentSource seam: published titles snapshot into order items. A miss —
// a deleted or unpublished product — reads as not-found and falls back to
// the module's generic title; lookup failures read the same so a broken
// title lookup can never abort a purchasable order.
type contentTitles struct {
	service *contentdomain.Service
}

// ProductTitle resolves one published product title.
func (t contentTitles) ProductTitle(ctx context.Context, contentID int) (string, bool) {
	product, err := t.service.GetPublishedByID(ctx, contentID)
	if err != nil || product == nil {
		return "", false
	}
	return product.Title, true
}

// NewContentTitles builds the seam adapter over the platform content
// service.
func NewContentTitles(service *contentdomain.Service) ordersdomain.ContentSource {
	return contentTitles{service: service}
}

// orderMailer adapts the platform email service onto the orders module's
// Mailer seam. It composes the display-ready item lines and money strings;
// the send failures the domain treats as best-effort already log here.
// A set payee appends the How-to-pay block to the confirmation email;
// an empty payee sends the legacy body unchanged.
type orderMailer struct {
	email *email.Service
	payee email.ManualPayee
}

// SendOrderConfirmationEmail renders the placed-order confirmation.
func (m orderMailer) SendOrderConfirmationEmail(ctx context.Context, order *ordersdomain.Order) error {
	items := make([]email.OrderEmailItem, 0, len(order.Items))
	for i := range order.Items {
		item := order.Items[i]
		items = append(items, email.OrderEmailItem{
			SKU:       item.SKU,
			Title:     item.Title,
			Quantity:  item.Quantity,
			UnitPrice: ordersdomain.NewMoney(item.UnitPrice, order.Currency).Format(),
			LineTotal: ordersdomain.NewMoney(item.LineTotal(), order.Currency).Format(),
		})
	}
	return m.email.SendOrderConfirmationEmail(
		ctx,
		order.CustomerEmail,
		order.DisplayNumber(),
		items,
		ordersdomain.NewMoney(order.Subtotal, order.Currency).Format(),
		ordersdomain.NewMoney(order.ShippingMinor, order.Currency).Format(),
		ordersdomain.NewMoney(order.TotalMinor(), order.Currency).Format(),
		m.payee,
		order.TrackingToken,
	)
}

// SendOrderStatusEmail renders one lifecycle move with the new status.
func (m orderMailer) SendOrderStatusEmail(ctx context.Context, order *ordersdomain.Order, from ordersdomain.Status) error {
	return m.email.SendOrderStatusEmail(
		ctx,
		order.CustomerEmail,
		order.DisplayNumber(),
		string(order.Status),
	)
}

// NewOrderMailer builds the seam adapter over the platform email service.
// A set payee appends the How-to-pay block to the confirmation email.
func NewOrderMailer(service *email.Service, payee email.ManualPayee) ordersdomain.Mailer {
	return orderMailer{email: service, payee: payee}
}

// catalogInventory adapts the catalog service onto the orders module's
// Inventory seam. Reserve lines adjust strictly and atomically per bucket;
// a shortfall after earlier lines moved compensates them before
// surfacing, so a refused confirm leaves no half-claim. The catalog's
// refused-move sentinel reads the orders vocabulary; a persisted-line
// restock failure logs rather than rewinding the transition that
// triggered it — the quantities are exactly what a strict Reserve took.
type catalogInventory struct {
	service *catalogdomain.Service
	log     kernel.Logger
}

// Reserve decrements every order line's tracked bucket, all-or-nothing.
func (i catalogInventory) Reserve(ctx context.Context, items []ordersdomain.OrderItem) error {
	for moved := range items {
		if err := i.adjust(ctx, items[moved], -items[moved].Quantity); err != nil {
			for rolled := range moved {
				_ = i.adjust(ctx, items[rolled], items[rolled].Quantity)
			}
			return err
		}
	}
	return nil
}

// Release restocks every line, mirroring a strict Reserve.
func (i catalogInventory) Release(ctx context.Context, items []ordersdomain.OrderItem) error {
	for line := range items {
		if err := i.adjust(ctx, items[line], items[line].Quantity); err != nil {
			if i.log != nil {
				i.log.Error("order stock release failed: %v", err)
			}
		}
	}
	return nil
}

// adjust moves one line's bucket, mapping the refused-move sentinel into
// the orders vocabulary.
func (i catalogInventory) adjust(ctx context.Context, item ordersdomain.OrderItem, delta int) error {
	err := i.service.AdjustStock(ctx, item.ProductContentID, item.VariantID, delta)
	if errors.Is(err, catalogdomain.ErrInsufficientStock) {
		return ordersdomain.ErrInsufficientStock
	}
	return err
}

// NewCatalogInventory builds the seam adapter over the adopted catalog
// service.
func NewCatalogInventory(service *catalogdomain.Service, log kernel.Logger) ordersdomain.Inventory {
	return catalogInventory{service: service, log: log}
}
