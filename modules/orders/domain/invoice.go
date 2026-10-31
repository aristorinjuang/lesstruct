package orders

import (
	"context"
	"fmt"
	"time"
)

const (
	// InvoiceNumberPrefix precedes every rendered invoice number, mirroring
	// the derived order numbers (INV-000042 pays ORD-000042).
	InvoiceNumberPrefix = "INV-"
)

// invoiceNumber renders the derived invoice number for one order ID,
// zero-padded to the order-number width so the two numbers stay in step.
func invoiceNumber(id int64) string {
	return fmt.Sprintf("%s%0*d", InvoiceNumberPrefix, OrderNumberWidth, id)
}

// InvoiceSettings carries the [invoice] block into the document build:
// seller identity and the PPN rate. The rate is percent (12 = twelve
// percent); the computed tax is a display-only breakdown — the order
// total stays the money truth and never moves.
type InvoiceSettings struct {
	SellerName    string
	SellerAddress string
	SellerTaxID   string
	TaxRate       float64
}

// InvoiceLine is one billed row: the frozen snapshot values, never live
// pricing.
type InvoiceLine struct {
	Title     string
	SKU       string
	Quantity  int
	UnitPrice Money
	LineTotal Money
}

// InvoiceDocument is the render-ready invoice: everything the PDF needs,
// derived on demand from the stored order — nothing is persisted, so the
// document always reflects the current record.
type InvoiceDocument struct {
	Number        string
	OrderNumber   string
	IssuedAt      time.Time
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	ShipTo        Address
	SellerName    string
	SellerAddress string
	SellerTaxID   string
	Lines         []InvoiceLine
	Subtotal      Money
	Shipping      Money
	TaxRate       float64
	Tax           Money
	Total         Money
}

// InvoiceRenderer turns a document into PDF bytes. The maroto
// implementation lives in modules/orders/pdf; tests use fakes.
type InvoiceRenderer interface {
	// Render builds the invoice PDF for one document.
	Render(ctx context.Context, doc *InvoiceDocument) ([]byte, error)
}
