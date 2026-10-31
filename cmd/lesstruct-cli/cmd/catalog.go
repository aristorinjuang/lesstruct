package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

// catalogProduct is the cmd-layer projection of the admin commerce record —
// camelCase keys mirroring the PUT body plus stored IDs. Decoding a server
// envelope into it drops unknown fields silently.
type catalogProduct struct {
	ContentID         int                 `json:"contentId"`
	Price             int64               `json:"price"`
	SalePrice         *int64              `json:"salePrice,omitempty"`
	SKU               string              `json:"sku,omitempty"`
	TrackInventory    bool                `json:"trackInventory"`
	Stock             int                 `json:"stock"`
	WeightGrams       *int                `json:"weightGrams,omitempty"`
	LowStockThreshold *int                `json:"lowStockThreshold,omitempty"`
	Options           map[string][]string `json:"options"`
	UpdatedAt         string              `json:"updatedAt,omitempty"`
}

// newCatalogCmd builds the `catalog` command tree: read and upsert the
// commerce data of one product over /api/v1/catalog/products/{contentID}.
func newCatalogCmd(apiKey, baseURL, output *string) *cobra.Command {
	catalog := &cobra.Command{
		Use:           "catalog",
		Short:         "Manage catalog commerce data over /api/v1/catalog/products",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var (
		price          int64
		salePrice      int64
		sku            string
		trackInventory bool
		stock          int
		weightGrams    int
		lowStock       int
		optionsJSON    string
		variantsJSON   string
	)

	getCmd := &cobra.Command{
		Use:           "get <content-id>",
		Short:         "Get the commerce data of one product",
		Long:          "Get the commerce data of the product content item via GET /api/v1/catalog/products/{contentID}. Items without a catalog record answer 404 NOT_FOUND.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCatalogGet(cmd, args, catalogReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	upsertCmd := &cobra.Command{
		Use:   "upsert <content-id>",
		Short: "Create or replace the commerce data of one product",
		Long: "Upsert the commerce data of the product content item via PUT /api/v1/catalog/products/{contentID}. " +
			"Upsert merges: the current record is read first and only changed flags are overlaid, so omitted flags keep their server values " +
			"(pass --options-json/--variants-json to replace the matrix wholesale). On a product with no record yet, --price is required. " +
			"The sale price cannot be cleared back to unset from the CLI; use the API directly for that.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCatalogUpsert(cmd, args, catalogUpsertOptions{
				apiKey:         *apiKey,
				baseURL:        *baseURL,
				output:         *output,
				price:          price,
				priceChanged:   cmd.Flags().Changed("price"),
				salePrice:      salePrice,
				saleChanged:    cmd.Flags().Changed("sale-price"),
				sku:            sku,
				skuChanged:     cmd.Flags().Changed("sku"),
				trackInventory: trackInventory,
				trackChanged:   cmd.Flags().Changed("track-inventory"),
				stock:          stock,
				stockChanged:   cmd.Flags().Changed("stock"),
				weightGrams:    weightGrams,
				weightChanged:  cmd.Flags().Changed("weight-grams"),
				lowStock:       lowStock,
				lowChanged:     cmd.Flags().Changed("low-stock"),
				optionsJSON:    optionsJSON,
				variantsJSON:   variantsJSON,
			})
		},
	}
	upsertCmd.Flags().Int64Var(&price, "price", 0, "price in integer minor units (required when the product has no record yet)")
	upsertCmd.Flags().Int64Var(&salePrice, "sale-price", 0, "sale price in integer minor units (must not exceed price)")
	upsertCmd.Flags().StringVar(&sku, "sku", "", "product-level SKU")
	upsertCmd.Flags().BoolVar(&trackInventory, "track-inventory", false, "track inventory for the product")
	upsertCmd.Flags().IntVar(&stock, "stock", 0, "product-level stock (used when no variants exist)")
	upsertCmd.Flags().IntVar(&weightGrams, "weight-grams", 0, "product-level weight in grams")
	upsertCmd.Flags().IntVar(&lowStock, "low-stock", 0, "low-stock threshold that triggers the notice email")
	upsertCmd.Flags().StringVar(&optionsJSON, "options-json", "", "variant axes as JSON, e.g. '{\"size\":[\"S\",\"M\"]}' (replaces the matrix axes)")
	upsertCmd.Flags().StringVar(&variantsJSON, "variants-json", "", "variant rows as JSON array (replaces the matrix rows)")

	catalog.AddCommand(getCmd)
	catalog.AddCommand(upsertCmd)
	return catalog
}

// catalogReadOptions bundles the resolved flag values for runCatalogGet.
type catalogReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// catalogUpsertOptions bundles the resolved flag values for
// runCatalogUpsert, plus per-flag Changed markers so omitted flags keep
// their server values instead of zeroing the record.
type catalogUpsertOptions struct {
	apiKey         string
	baseURL        string
	output         string
	price          int64
	priceChanged   bool
	salePrice      int64
	saleChanged    bool
	sku            string
	skuChanged     bool
	trackInventory bool
	trackChanged   bool
	stock          int
	stockChanged   bool
	weightGrams    int
	weightChanged  bool
	lowStock       int
	lowChanged     bool
	optionsJSON    string
	variantsJSON   string
}

// runCatalogGet implements `lesstruct-cli catalog get <content-id>`.
func runCatalogGet(cmd *cobra.Command, args []string, opts catalogReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := cl.CatalogGetProduct(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no product",
		}
	}

	return printCatalogProduct(cmd.OutOrStdout(), opts.output, data, meta)
}

// isNotFound reports whether err is a 404 API error (a missing catalog
// record on the merge read is a normal upsert-create path, not a failure).
func isNotFound(err error) bool {
	if apiErr, ok := errors.AsType[*client.APIError](err); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// runCatalogUpsert implements `lesstruct-cli catalog upsert <content-id>`.
// Omitted flags keep their server values: the current record is read first
// (a missing record starts from an empty one and requires --price) and only
// changed flags are overlaid before the PUT.
func runCatalogUpsert(cmd *cobra.Command, args []string, opts catalogUpsertOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	if !opts.priceChanged && !opts.saleChanged && !opts.skuChanged && !opts.trackChanged &&
		!opts.stockChanged && !opts.weightChanged && !opts.lowChanged &&
		opts.optionsJSON == "" && opts.variantsJSON == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: nothing to set (pass at least one field flag)",
		}
	}

	var options any
	if opts.optionsJSON != "" {
		if err := json.Unmarshal([]byte(opts.optionsJSON), &options); err != nil {
			return &exitError{code: client.ExitValidation, msg: fmt.Sprintf("lesstruct-cli: --options-json is not valid JSON: %v", err)}
		}
	}
	var variants any
	if opts.variantsJSON != "" {
		if err := json.Unmarshal([]byte(opts.variantsJSON), &variants); err != nil {
			return &exitError{code: client.ExitValidation, msg: fmt.Sprintf("lesstruct-cli: --variants-json is not valid JSON: %v", err)}
		}
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	body := map[string]any{}
	if current, _, err := cl.CatalogGetProduct(cmd.Context(), id); err == nil && len(current) > 0 {
		if err := json.Unmarshal(current, &body); err != nil {
			return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("lesstruct-cli: decode current product: %v", err)}
		}
	} else if err != nil && !isNotFound(err) {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if len(body) == 0 && !opts.priceChanged {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --price is required when the product has no catalog record yet",
		}
	}

	overlayInt(body, "price", opts.price, opts.priceChanged)
	overlayInt(body, "salePrice", opts.salePrice, opts.saleChanged)
	overlayString(body, "sku", opts.sku, opts.skuChanged)
	overlayBool(body, "trackInventory", opts.trackInventory, opts.trackChanged)
	overlayInt(body, "stock", int64(opts.stock), opts.stockChanged)
	overlayInt(body, "weightGrams", int64(opts.weightGrams), opts.weightChanged)
	overlayInt(body, "lowStockThreshold", int64(opts.lowStock), opts.lowChanged)
	if opts.optionsJSON != "" {
		body["options"] = options
	}
	if opts.variantsJSON != "" {
		body["variants"] = variants
	}

	data, meta, err := cl.CatalogUpsertProduct(cmd.Context(), id, body)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no product",
		}
	}

	return printCatalogProduct(cmd.OutOrStdout(), opts.output, data, meta)
}

// overlayInt sets key to value when changed, so omitted flags keep the
// merged server value instead of zeroing the record.
func overlayInt(body map[string]any, key string, value int64, changed bool) {
	if changed {
		body[key] = value
	}
}

// overlayString sets key to value when changed.
func overlayString(body map[string]any, key, value string, changed bool) {
	if changed {
		body[key] = value
	}
}

// overlayBool sets key to value when changed.
func overlayBool(body map[string]any, key string, value, changed bool) {
	if changed {
		body[key] = value
	}
}

// printCatalogProduct writes one commerce record to w in the requested mode.
func printCatalogProduct(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		env := struct {
			Data json.RawMessage `json:"data,omitempty"`
			Meta json.RawMessage `json:"meta,omitempty"`
		}{Data: data, Meta: meta}
		encoded, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("encode output: %w", err)
		}
		_, err = fmt.Fprintln(w, string(encoded))
		return err
	}

	var p catalogProduct
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if p.ContentID == 0 {
		return fmt.Errorf("decode response: missing product projection")
	}

	sale := "no sale"
	if p.SalePrice != nil {
		sale = fmt.Sprintf("sale %d", *p.SalePrice)
	}
	_, err := fmt.Fprintf(w, "Catalog product #%d: price %d %s sku %q stock %d (tracked=%v)\n",
		p.ContentID, p.Price, sale, p.SKU, p.Stock, p.TrackInventory)
	return err
}
