package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// catalogGetArgs carries the `catalog_get` content id.
type catalogGetArgs struct {
	ContentID int `json:"contentId" jsonschema:"content item id of the product"`
}

// catalogUpsertArgs carries the `catalog_upsert` payload. It merges like the
// CLI: the current record is read first and only the provided fields are
// overlaid, so omitted fields keep their server values. On a product with no
// record yet, price is required. Prices are integer minor units.
type catalogUpsertArgs struct {
	ContentID         int                 `json:"contentId" jsonschema:"content item id of the product"`
	Price             *int64              `json:"price,omitempty" jsonschema:"price in integer minor units (required when no record exists yet)"`
	SalePrice         *int64              `json:"salePrice,omitempty" jsonschema:"sale price in integer minor units (must not exceed price)"`
	SKU               *string             `json:"sku,omitempty" jsonschema:"product-level SKU"`
	TrackInventory    *bool               `json:"trackInventory,omitempty" jsonschema:"track inventory for the product"`
	Stock             *int                `json:"stock,omitempty" jsonschema:"product-level stock (used when no variants exist)"`
	WeightGrams       *int                `json:"weightGrams,omitempty" jsonschema:"product-level weight in grams"`
	LowStockThreshold *int                `json:"lowStockThreshold,omitempty" jsonschema:"low-stock threshold that triggers the notice email"`
	Options           map[string][]string `json:"options,omitempty" jsonschema:"variant axes, e.g. {size:[S,M]} (replaces the matrix axes wholesale)"`
	Variants          []any               `json:"variants,omitempty" jsonschema:"variant rows (replaces the matrix rows wholesale)"`
}

// ordersListArgs carries the `orders_list` filters.
type ordersListArgs struct {
	Status string `json:"status,omitempty" jsonschema:"filter by one lifecycle status"`
	Cursor string `json:"cursor,omitempty" jsonschema:"opaque cursor from a previous call"`
}

// orderGetArgs carries the `orders_get` admin id (numeric, not the ORD-000042
// display number).
type orderGetArgs struct {
	ID int64 `json:"id" jsonschema:"order admin id (numeric, not the ORD- display number)"`
}

// orderTransitionArgs carries the `orders_transition` payload. Illegal moves
// read 409; "paid" is refused outright — payment confirmation is the only path
// into paid.
type orderTransitionArgs struct {
	ID     int64  `json:"id" jsonschema:"order admin id (numeric)"`
	Status string `json:"status" jsonschema:"target lifecycle status"`
}

// orderShippingArgs carries the `orders_set_shipping` payload. Only pending
// orders carry shipping; settled orders refuse with 409.
type orderShippingArgs struct {
	ID     int64 `json:"id" jsonschema:"order admin id (numeric)"`
	Amount int64 `json:"amount" jsonschema:"shipping line in integer minor units"`
}

// paymentsListArgs carries the `payments_list` filters.
type paymentsListArgs struct {
	Status string `json:"status,omitempty" jsonschema:"filter by one payment status"`
	Cursor string `json:"cursor,omitempty" jsonschema:"opaque cursor from a previous call"`
}

// paymentGetArgs carries the `payments_get` id.
type paymentGetArgs struct {
	ID int64 `json:"id" jsonschema:"payment id (numeric)"`
}

// invoiceSettingsSetArgs carries the `invoice_settings_set` payload: the
// seller identity block plus the PPN rate. The save applies to the next
// rendered invoice without a restart.
type invoiceSettingsSetArgs struct {
	Settings map[string]any `json:"settings,omitempty" jsonschema:"invoice settings block (seller identity plus taxRate)"`
}

// registerCommerceTools adds the store operator tools to ms (eCommerce
// builds). All handlers close over the shared /api/v1 client.
func registerCommerceTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "catalog_get",
		Description: "Get the commerce data of one product by content id. Items without a catalog record answer 404 NOT_FOUND.",
	}, makeCatalogGetHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "catalog_upsert",
		Description: "Create or replace a product's commerce data (price, sale, SKU, inventory, variant matrix). Merges: omitted fields keep server values.",
	}, makeCatalogUpsertHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "orders_list",
		Description: "List orders, newest first (paginated). Filter by one lifecycle status; page with the cursor from a previous call.",
	}, makeOrdersListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "orders_get",
		Description: "Get a single order with its item snapshot by numeric admin id.",
	}, makeOrdersGetHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "orders_transition",
		Description: "Move an order to a new lifecycle status. Illegal moves read 409.",
	}, makeOrdersTransitionHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "orders_set_shipping",
		Description: "Set the shipping line of a pending order in integer minor units.",
	}, makeOrdersSetShippingHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "payments_list",
		Description: "List manual transfer payment proofs, newest first (paginated).",
	}, makePaymentsListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "payments_get",
		Description: "Get a single payment proof with its order context by id.",
	}, makePaymentsGetHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "payments_verify",
		Description: "Verify (accept) a payment proof. Decided proofs read 409.",
	}, makePaymentsVerifyHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "payments_reject",
		Description: "Reject a payment proof. The order stays pending. Decided proofs read 409.",
	}, makePaymentsRejectHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "invoice_settings_get",
		Description: "Get the live invoice settings block (seller identity plus PPN rate).",
	}, makeInvoiceSettingsGetHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "invoice_settings_set",
		Description: "Replace the invoice settings block. Applies to the next rendered invoice without a restart.",
	}, makeInvoiceSettingsSetHandler(c))
}

// makeCatalogGetHandler returns the `catalog_get` handler.
func makeCatalogGetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, catalogGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args catalogGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ContentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ContentID)), nil, nil
		}
		data, meta, err := c.CatalogGetProduct(ctx, args.ContentID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no product")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeCatalogUpsertHandler returns the `catalog_upsert` handler.
func makeCatalogUpsertHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, catalogUpsertArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args catalogUpsertArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ContentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ContentID)), nil, nil
		}
		if args.Price == nil && args.SalePrice == nil && args.SKU == nil &&
			args.TrackInventory == nil && args.Stock == nil && args.WeightGrams == nil &&
			args.LowStockThreshold == nil && args.Options == nil && args.Variants == nil {
			return toolError(fmt.Errorf("VALIDATION_ERROR: nothing to set (provide at least one field)")), nil, nil
		}
		body := map[string]any{}
		if current, _, err := c.CatalogGetProduct(ctx, args.ContentID); err == nil && len(current) > 0 {
			if uerr := json.Unmarshal(current, &body); uerr != nil {
				return toolError(fmt.Errorf("decode current product: %w", uerr)), nil, nil
			}
		} else if err != nil && !isMCPNotFound(err) {
			return toolError(err), nil, nil
		}
		if len(body) == 0 && args.Price == nil {
			return toolError(fmt.Errorf("VALIDATION_ERROR: price is required when the product has no catalog record yet")), nil, nil
		}
		overlayMCPInt64(body, "price", args.Price)
		overlayMCPInt64(body, "salePrice", args.SalePrice)
		overlayMCPString(body, "sku", args.SKU)
		overlayMCPBool(body, "trackInventory", args.TrackInventory)
		overlayMCPInt(body, "stock", args.Stock)
		overlayMCPInt(body, "weightGrams", args.WeightGrams)
		overlayMCPInt(body, "lowStockThreshold", args.LowStockThreshold)
		if args.Options != nil {
			body["options"] = args.Options
		}
		if args.Variants != nil {
			body["variants"] = args.Variants
		}
		data, meta, err := c.CatalogUpsertProduct(ctx, args.ContentID, body)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no product")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeOrdersListHandler returns the `orders_list` handler.
func makeOrdersListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, ordersListArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args ordersListArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.OrdersList(ctx, args.Status, args.Cursor)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeOrdersGetHandler returns the `orders_get` handler.
func makeOrdersGetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, orderGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args orderGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid order id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.OrdersGet(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no order")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeOrdersTransitionHandler returns the `orders_transition` handler.
func makeOrdersTransitionHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, orderTransitionArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args orderTransitionArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid order id: %d", args.ID)), nil, nil
		}
		if args.Status == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: status is required")), nil, nil
		}
		data, meta, err := c.OrdersTransition(ctx, args.ID, args.Status)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no order")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeOrdersSetShippingHandler returns the `orders_set_shipping` handler.
func makeOrdersSetShippingHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, orderShippingArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args orderShippingArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid order id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.OrdersSetShipping(ctx, args.ID, args.Amount)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no order")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePaymentsListHandler returns the `payments_list` handler.
func makePaymentsListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, paymentsListArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args paymentsListArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.PaymentsList(ctx, args.Status, args.Cursor)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePaymentsGetHandler returns the `payments_get` handler.
func makePaymentsGetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, paymentGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args paymentGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid payment id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.PaymentsGet(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no payment")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePaymentsVerifyHandler returns the `payments_verify` handler.
func makePaymentsVerifyHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, paymentGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args paymentGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid payment id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.PaymentsVerify(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no payment")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePaymentsRejectHandler returns the `payments_reject` handler.
func makePaymentsRejectHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, paymentGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args paymentGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid payment id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.PaymentsReject(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no payment")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeInvoiceSettingsGetHandler returns the `invoice_settings_get` handler.
func makeInvoiceSettingsGetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.InvoiceSettingsGet(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no settings")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeInvoiceSettingsSetHandler returns the `invoice_settings_set` handler.
func makeInvoiceSettingsSetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, invoiceSettingsSetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args invoiceSettingsSetArgs,
	) (*mcp.CallToolResult, any, error) {
		if len(args.Settings) == 0 {
			return toolError(fmt.Errorf("VALIDATION_ERROR: settings are required")), nil, nil
		}
		data, meta, err := c.InvoiceSettingsSet(ctx, args.Settings)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no settings")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// overlayMCPInt64 sets body[key] when the pointer is non-nil (an explicit
// field the caller wants stored). A nil pointer keeps the server value.
func overlayMCPInt64(body map[string]any, key string, v *int64) {
	if v != nil {
		body[key] = *v
	}
}

// overlayMCPString sets body[key] when the pointer is non-nil.
func overlayMCPString(body map[string]any, key string, v *string) {
	if v != nil {
		body[key] = *v
	}
}

// overlayMCPBool sets body[key] when the pointer is non-nil.
func overlayMCPBool(body map[string]any, key string, v *bool) {
	if v != nil {
		body[key] = *v
	}
}

// overlayMCPInt sets body[key] when the pointer is non-nil (int variant for
// stock, weight, and threshold fields).
func overlayMCPInt(body map[string]any, key string, v *int) {
	if v != nil {
		body[key] = *v
	}
}

// isMCPNotFound reports whether err is a 404 *client.APIError (a product with
// no catalog record yet reads 404, which starts the merge from empty).
func isMCPNotFound(err error) bool {
	apiErr, ok := err.(*client.APIError)
	return ok && apiErr.StatusCode == http.StatusNotFound
}
