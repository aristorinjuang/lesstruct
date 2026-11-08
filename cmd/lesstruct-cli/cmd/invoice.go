package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// invoiceSettings is the cmd-layer projection of the live [invoice] block —
// seller identity plus the PPN rate. Decoding a server envelope into it
// drops unknown fields silently.
type invoiceSettings struct {
	SellerName    string  `json:"sellerName"`
	SellerAddress string  `json:"sellerAddress"`
	SellerTaxID   string  `json:"sellerTaxId"`
	TaxRate       float64 `json:"taxRate"`
}

// newInvoiceSettingsCmd builds the `invoice-settings` command tree: read and
// save the live invoice block over /api/v1/orders/invoice-settings.
func newInvoiceSettingsCmd(apiKey, baseURL, output *string) *cobra.Command {
	invoice := &cobra.Command{
		Use:           "invoice-settings",
		Short:         "Manage invoice seller identity and PPN rate",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var sellerName, sellerAddress, sellerTaxID string
	var taxRate float64

	getCmd := &cobra.Command{
		Use:           "get",
		Short:         "Get the live invoice settings",
		Long:          "Get the live [invoice] block via GET /api/v1/orders/invoice-settings: the seller identity and PPN rate the next rendered invoice uses.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvoiceSettingsGet(cmd, args, invoiceSettingsReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Save the invoice settings",
		Long: "Save the [invoice] block via PUT /api/v1/orders/invoice-settings. " +
			"Unset flags keep their server values (the block is read first and only changed flags are overlaid); " +
			"the save applies to the next rendered invoice with no restart. " +
			"taxRate is a percent between 0 and 100 — 0 for non-PKP sellers charging no PPN.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvoiceSettingsSet(cmd, args, invoiceSettingsSetOptions{
				apiKey:         *apiKey,
				baseURL:        *baseURL,
				output:         *output,
				sellerName:     sellerName,
				nameChanged:    cmd.Flags().Changed("seller-name"),
				sellerAddress:  sellerAddress,
				addressChanged: cmd.Flags().Changed("seller-address"),
				sellerTaxID:    sellerTaxID,
				taxIDChanged:   cmd.Flags().Changed("seller-tax-id"),
				taxRate:        taxRate,
				rateChanged:    cmd.Flags().Changed("tax-rate"),
			})
		},
	}
	setCmd.Flags().StringVar(&sellerName, "seller-name", "", "seller name stamped on invoices")
	setCmd.Flags().StringVar(&sellerAddress, "seller-address", "", "seller address stamped on invoices")
	setCmd.Flags().StringVar(&sellerTaxID, "seller-tax-id", "", "seller tax ID (NPWP) stamped on invoices")
	setCmd.Flags().Float64Var(&taxRate, "tax-rate", 0, "PPN rate percent between 0 and 100")

	invoice.AddCommand(getCmd)
	invoice.AddCommand(setCmd)
	return invoice
}

// invoiceSettingsReadOptions bundles the resolved flag values for
// runInvoiceSettingsGet.
type invoiceSettingsReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// invoiceSettingsSetOptions bundles the resolved flag values for
// runInvoiceSettingsSet, plus per-flag Changed markers so omitted flags
// keep their server values.
type invoiceSettingsSetOptions struct {
	apiKey         string
	baseURL        string
	output         string
	sellerName     string
	nameChanged    bool
	sellerAddress  string
	addressChanged bool
	sellerTaxID    string
	taxIDChanged   bool
	taxRate        float64
	rateChanged    bool
}

// runInvoiceSettingsGet implements `lesstruct-cli invoice-settings get`.
func runInvoiceSettingsGet(cmd *cobra.Command, args []string, opts invoiceSettingsReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
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

	data, meta, err := cl.InvoiceSettingsGet(cmd.Context())
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no invoice settings",
		}
	}

	return printInvoiceSettings(cmd.OutOrStdout(), opts.output, data, meta)
}

// runInvoiceSettingsSet implements `lesstruct-cli invoice-settings set`.
// Omitted flags keep their server values: the live block is read first and
// only changed flags are overlaid before the PUT.
func runInvoiceSettingsSet(cmd *cobra.Command, args []string, opts invoiceSettingsSetOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	if !opts.nameChanged && !opts.addressChanged && !opts.taxIDChanged && !opts.rateChanged {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: nothing to set (pass at least one field flag)",
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
	if current, _, err := cl.InvoiceSettingsGet(cmd.Context()); err == nil && len(current) > 0 {
		if err := json.Unmarshal(current, &body); err != nil {
			return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("lesstruct-cli: decode current settings: %v", err)}
		}
	} else if err != nil && !isNotFound(err) {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	overlayString(body, "sellerName", opts.sellerName, opts.nameChanged)
	overlayString(body, "sellerAddress", opts.sellerAddress, opts.addressChanged)
	overlayString(body, "sellerTaxId", opts.sellerTaxID, opts.taxIDChanged)
	if opts.rateChanged {
		body["taxRate"] = opts.taxRate
	}

	data, meta, err := cl.InvoiceSettingsSet(cmd.Context(), body)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no invoice settings",
		}
	}

	return printInvoiceSettings(cmd.OutOrStdout(), opts.output, data, meta)
}

// printInvoiceSettings writes the invoice block to w in the requested mode.
func printInvoiceSettings(w io.Writer, mode string, data, meta json.RawMessage) error {
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

	var s invoiceSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	_, err := fmt.Fprintf(w, "Invoice settings: seller %q tax id %q rate %.2f%%\n  %s\n",
		s.SellerName, s.SellerTaxID, s.TaxRate, s.SellerAddress)
	return err
}
