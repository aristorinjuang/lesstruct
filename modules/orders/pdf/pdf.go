// Package pdf renders order invoices to PDF bytes over maroto. Money
// renders through the orders domain formatter, so the invoice, the order
// emails, and the storefront all read from one rule.
package pdf

import (
	"context"
	"fmt"

	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

var (
	// _ witnesses the renderer against the domain seam at compile time.
	_ ordersdomain.InvoiceRenderer = Renderer{}
)

// lineHeaderRow labels the billed-lines table.
func lineHeaderRow() core.Row {
	return row.New(8).Add(
		col.New(6).Add(text.New("Item", props.Text{Style: fontstyle.Bold, Size: 9})),
		col.New(2).Add(text.New("Qty", props.Text{Style: fontstyle.Bold, Size: 9, Align: align.Right})),
		col.New(2).Add(text.New("Unit", props.Text{Style: fontstyle.Bold, Size: 9, Align: align.Right})),
		col.New(2).Add(text.New("Line", props.Text{Style: fontstyle.Bold, Size: 9, Align: align.Right})),
	)
}

// lineRow renders one billed line: title plus SKU, quantity, and the
// frozen money.
func lineRow(line *ordersdomain.InvoiceLine) core.Row {
	title := line.Title
	if line.SKU != "" {
		title += " (" + line.SKU + ")"
	}
	return row.New(8).Add(
		col.New(6).Add(text.New(title, props.Text{Size: 9})),
		col.New(2).Add(text.New(fmt.Sprintf("%d", line.Quantity), props.Text{Size: 9, Align: align.Right})),
		col.New(2).Add(text.New(line.UnitPrice.Format(), props.Text{Size: 9, Align: align.Right})),
		col.New(2).Add(text.New(line.LineTotal.Format(), props.Text{Size: 9, Align: align.Right})),
	)
}

// totalRow renders one totals line; the grand total rides bold.
func totalRow(label string, amount string, bold bool) core.Row {
	style := fontstyle.Normal
	if bold {
		style = fontstyle.Bold
	}
	return row.New(8).Add(
		col.New(8).Add(text.New("", props.Text{Size: 9})),
		col.New(2).Add(text.New(label, props.Text{Style: style, Size: 9, Align: align.Right})),
		col.New(2).Add(text.New(amount, props.Text{Style: style, Size: 9, Align: align.Right})),
	)
}

// summaryRows renders the invoice totals: subtotal and shipping always,
// then the PPN row and its footnote only when the configured rate is
// positive. A zero rate (a non-PKP seller charging no PPN) carries no tax
// line, matching the settings UI copy. The grand total closes the section
// either way.
func summaryRows(doc *ordersdomain.InvoiceDocument) []core.Row {
	rows := []core.Row{
		totalRow("Subtotal", doc.Subtotal.Format(), false),
		totalRow("Shipping", doc.Shipping.Format(), false),
	}
	if doc.TaxRate > 0 {
		rows = append(rows,
			totalRow(fmt.Sprintf("Tax (PPN %.2f%%)", doc.TaxRate), doc.Tax.Format(), false),
			row.New(10).Add(col.New(12).Add(
				text.New("Tax is a display-only breakdown at the configured rate; the order total is the amount charged.", props.Text{Size: 8}),
			)),
		)
	}
	return append(rows, totalRow("Total", doc.Total.Format(), true))
}

// Renderer builds invoice PDFs.
type Renderer struct{}

// Render builds the invoice PDF for one document.
func (Renderer) Render(_ context.Context, doc *ordersdomain.InvoiceDocument) ([]byte, error) {
	m := maroto.New()
	m.AddRows(
		row.New(14).Add(col.New(12).Add(
			text.New("INVOICE", props.Text{Style: fontstyle.Bold, Size: 18}),
		)),
		row.New(8).Add(
			col.New(6).Add(text.New(doc.SellerName, props.Text{Style: fontstyle.Bold, Size: 11})),
			col.New(6).Add(text.New(doc.Number, props.Text{Align: align.Right, Size: 11})),
		),
	)
	if doc.SellerAddress != "" {
		m.AddRows(row.New(7).Add(col.New(12).Add(text.New(doc.SellerAddress, props.Text{Size: 9}))))
	}
	if doc.SellerTaxID != "" {
		m.AddRows(row.New(7).Add(col.New(12).Add(text.New("NPWP: "+doc.SellerTaxID, props.Text{Size: 9}))))
	}
	m.AddRows(
		row.New(8).Add(
			col.New(6).Add(text.New("Bill to: "+doc.CustomerName, props.Text{Size: 10})),
			col.New(6).Add(text.New("Order "+doc.OrderNumber, props.Text{Align: align.Right, Size: 10})),
		),
		row.New(7).Add(
			col.New(6).Add(text.New(doc.CustomerEmail, props.Text{Size: 9})),
			col.New(6).Add(text.New("Issued "+doc.IssuedAt.Format("2006-01-02"), props.Text{Align: align.Right, Size: 9})),
		),
	)
	m.AddRows(lineHeaderRow())
	for i := range doc.Lines {
		m.AddRows(lineRow(&doc.Lines[i]))
	}
	m.AddRows(summaryRows(doc)...)

	document, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice pdf: %w", err)
	}
	return document.GetBytes(), nil
}
