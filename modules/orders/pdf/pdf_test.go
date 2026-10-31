package pdf_test

import (
	"context"
	"testing"
	"time"

	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/aristorinjuang/lesstruct/modules/orders/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invoicedOrder is a shippable order rendered for paper: frozen lines,
// the shipping line, and the display-only tax.
func invoicedOrder() *ordersdomain.InvoiceDocument {
	return &ordersdomain.InvoiceDocument{
		Number:        "INV-000042",
		OrderNumber:   "ORD-000042",
		IssuedAt:      time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
		CustomerName:  "Annisa Putri",
		CustomerEmail: "shopper@example.com",
		ShipTo: ordersdomain.Address{
			Street:     "Jl. Kemang Raya 21",
			City:       "Jakarta Selatan",
			Province:   "DKI Jakarta",
			PostalCode: "12730",
		},
		SellerName:    "Lesstruct Demo",
		SellerAddress: "Jl. Example No. 1",
		SellerTaxID:   "01.234.567.8-901.234",
		Lines: []ordersdomain.InvoiceLine{{
			Title:     "Nainggolan T-Shirt",
			SKU:       "TSHIRT",
			Quantity:  2,
			UnitPrice: ordersdomain.NewMoney(299000, "IDR"),
			LineTotal: ordersdomain.NewMoney(598000, "IDR"),
		}},
		Subtotal: ordersdomain.NewMoney(598000, "IDR"),
		Shipping: ordersdomain.NewMoney(15000, "IDR"),
		TaxRate:  12,
		Tax:      ordersdomain.NewMoney(73560, "IDR"),
		Total:    ordersdomain.NewMoney(613000, "IDR"),
	}
}

func TestRenderer_Render(t *testing.T) {
	t.Run("renders a well-formed pdf", func(t *testing.T) {
		raw, err := pdf.Renderer{}.Render(context.Background(), invoicedOrder())

		require.NoError(t, err)
		require.NotEmpty(t, raw)
		assert.Equal(t, "%PDF", string(raw[:4]), "a real pdf header opens the bytes")
	})

	t.Run("renders without seller identity", func(t *testing.T) {
		bare := invoicedOrder()
		bare.SellerName = ""
		bare.SellerAddress = ""
		bare.SellerTaxID = ""

		raw, err := pdf.Renderer{}.Render(context.Background(), bare)

		require.NoError(t, err)
		assert.Equal(t, "%PDF", string(raw[:4]))
	})

	t.Run("renders a lineless document", func(t *testing.T) {
		empty := invoicedOrder()
		empty.Lines = nil
		empty.Subtotal = ordersdomain.NewMoney(0, "IDR")
		empty.Shipping = ordersdomain.NewMoney(0, "IDR")
		empty.Tax = ordersdomain.NewMoney(0, "IDR")
		empty.Total = ordersdomain.NewMoney(0, "IDR")

		raw, err := pdf.Renderer{}.Render(context.Background(), empty)

		require.NoError(t, err)
		assert.Equal(t, "%PDF", string(raw[:4]))
	})

	t.Run("zero rate renders a smaller document without the tax line", func(t *testing.T) {
		withTax := invoicedOrder()
		withoutTax := invoicedOrder()
		withoutTax.TaxRate = 0
		withoutTax.Tax = ordersdomain.NewMoney(0, "IDR")

		taxed, err := pdf.Renderer{}.Render(context.Background(), withTax)
		require.NoError(t, err)
		untaxed, err := pdf.Renderer{}.Render(context.Background(), withoutTax)
		require.NoError(t, err)

		require.NotEmpty(t, untaxed)
		assert.Equal(t, "%PDF", string(untaxed[:4]))
		assert.Less(t, len(untaxed), len(taxed), "skipping the tax row and footnote must shrink the document")
	})
}
