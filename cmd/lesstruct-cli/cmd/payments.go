package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// paymentSummary is the cmd-layer projection of a payment record — the
// fields the text output prints. Decoding a server envelope into it drops
// unknown fields silently.
type paymentSummary struct {
	ID         int64  `json:"id"`
	OrderID    int64  `json:"orderId"`
	Method     string `json:"method"`
	Status     string `json:"status"`
	Reference  string `json:"reference"`
	Amount     int64  `json:"amount"`
	VerifiedBy string `json:"verifiedBy,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// newPaymentsCmd builds the `payments` command tree: list, read, and decide
// payment proofs over /api/v1/payments.
func newPaymentsCmd(apiKey, baseURL, output *string) *cobra.Command {
	payments := &cobra.Command{
		Use:           "payments",
		Short:         "Manage payment proofs over /api/v1/payments",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var listStatus, listCursor string

	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List payment proofs, newest first (paginated)",
		Long:          "List payment proofs via GET /api/v1/payments. Filter with --status (pending, verified, rejected); page with --cursor from a previous call.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaymentsList(cmd, args, paymentsListOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				status:  listStatus,
				cursor:  listCursor,
			})
		},
	}
	listCmd.Flags().StringVar(&listStatus, "status", "", "verification status to filter by (pending, verified, rejected)")
	listCmd.Flags().StringVar(&listCursor, "cursor", "", "opaque cursor returned by a previous list call")

	getCmd := &cobra.Command{
		Use:           "get <payment-id>",
		Short:         "Get a single payment proof with its order context",
		Long:          "Get one payment proof via GET /api/v1/payments/{paymentID}, including the order number, order status, and payable total the proof amount answers to.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaymentsGet(cmd, args, paymentsReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	verifyCmd := &cobra.Command{
		Use:           "verify <payment-id>",
		Short:         "Verify a pending payment proof (marks the order paid)",
		Long:          "Verify a pending proof via POST /api/v1/payments/{paymentID}/verify. The order flips to paid through the payment-gated path. Decided records and proofs whose order already left pending refuse with 409.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaymentsDecide(cmd, args, paymentsReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "verify")
		},
	}

	rejectCmd := &cobra.Command{
		Use:           "reject <payment-id>",
		Short:         "Reject a pending payment proof (order stays pending)",
		Long:          "Reject a pending proof via POST /api/v1/payments/{paymentID}/reject. The customer may correct and resubmit. Proofs whose order already left pending refuse with 409 and send no rejection email.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaymentsDecide(cmd, args, paymentsReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "reject")
		},
	}

	payments.AddCommand(listCmd)
	payments.AddCommand(getCmd)
	payments.AddCommand(verifyCmd)
	payments.AddCommand(rejectCmd)
	return payments
}

// paymentsListOptions bundles the resolved flag values for runPaymentsList.
type paymentsListOptions struct {
	apiKey  string
	baseURL string
	output  string
	status  string
	cursor  string
}

// paymentsReadOptions bundles the resolved flag values for the single-proof
// commands (get, verify, reject).
type paymentsReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// runPaymentsList implements `lesstruct-cli payments list`.
func runPaymentsList(cmd *cobra.Command, args []string, opts paymentsListOptions) error {
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

	data, meta, err := cl.PaymentsList(cmd.Context(), opts.status, opts.cursor)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}

	var records []paymentSummary
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	for _, p := range records {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "#%d %s %s ref %q amount %d by %s\n",
			p.ID, p.Method, p.Status, p.Reference, p.Amount, displayVerifier(p.VerifiedBy)); err != nil {
			return err
		}
	}
	return nil
}

// runPaymentsGet implements `lesstruct-cli payments get <payment-id>`.
func runPaymentsGet(cmd *cobra.Command, args []string, opts paymentsReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parsePaymentID(args[0])
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

	data, meta, err := cl.PaymentsGet(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printPayment(cmd.OutOrStdout(), opts.output, data, meta, "Payment")
}

// runPaymentsDecide implements `lesstruct-cli payments verify|reject <payment-id>`.
func runPaymentsDecide(cmd *cobra.Command, args []string, opts paymentsReadOptions, action string) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parsePaymentID(args[0])
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

	var (
		data json.RawMessage
		meta json.RawMessage
	)
	switch action {
	case "verify":
		data, meta, err = cl.PaymentsVerify(cmd.Context(), id)
	case "reject":
		data, meta, err = cl.PaymentsReject(cmd.Context(), id)
	default:
		return &exitError{code: client.ExitGeneric, msg: "lesstruct-cli: unknown payment decision"}
	}
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printPayment(cmd.OutOrStdout(), opts.output, data, meta, "Payment")
}

// parsePaymentID validates a positional payment record id.
func parsePaymentID(arg string) (int64, error) {
	id, err := parseIntID(arg, "payment")
	if err != nil {
		return 0, err
	}
	return int64(id), nil
}

// displayVerifier renders the deciding administrator, or a dash while the
// proof is still pending.
func displayVerifier(verifiedBy string) string {
	if verifiedBy == "" {
		return "-"
	}
	return verifiedBy
}

// printEnvelope writes the raw server envelope to w in JSON mode. Shared by
// commands whose text mode is a projection loop.
func printEnvelope(w io.Writer, data, meta json.RawMessage) error {
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

// printPayment writes a single payment record to w in the requested mode.
// Verify/reject return the bare record while get carries the order context
// alongside it; both shapes share the flat record fields, so one decode
// covers both and the order line prints only when present.
func printPayment(w io.Writer, mode string, data, meta json.RawMessage, verb string) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}

	var p paymentSummary
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if p.ID == 0 {
		return fmt.Errorf("decode response: missing payment projection")
	}

	if _, err := fmt.Fprintf(w, "%s #%d %s %s ref %q amount %d by %s\n",
		verb, p.ID, p.Method, p.Status, p.Reference, p.Amount, displayVerifier(p.VerifiedBy)); err != nil {
		return err
	}
	var ctx struct {
		Order struct {
			OrderNumber string `json:"orderNumber"`
			Status      string `json:"status"`
			TotalMinor  int64  `json:"totalMinor"`
		} `json:"order"`
	}
	if err := json.Unmarshal(data, &ctx); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if ctx.Order.OrderNumber != "" {
		_, err := fmt.Fprintf(w, "order %s %s total %d\n",
			ctx.Order.OrderNumber, ctx.Order.Status, ctx.Order.TotalMinor)
		return err
	}
	return nil
}
