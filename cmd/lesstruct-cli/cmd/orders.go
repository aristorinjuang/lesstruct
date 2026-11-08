package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// orderSummary is the cmd-layer projection of an admin order row — the
// fields the text output prints. Decoding a server envelope into it drops
// unknown fields silently.
type orderSummary struct {
	ID            int64  `json:"id"`
	OrderNumber   string `json:"orderNumber"`
	Status        string `json:"status"`
	CustomerEmail string `json:"customerEmail"`
	CustomerName  string `json:"customerName"`
	Currency      string `json:"currency"`
	Subtotal      int64  `json:"subtotal"`
	ShippingMinor int64  `json:"shippingMinor"`
	TotalMinor    int64  `json:"totalMinor"`
	ItemCount     int    `json:"itemCount"`
	CreatedAt     string `json:"createdAt"`
}

// newOrdersCmd builds the `orders` command tree: list, read, transition,
// and set shipping on orders over /api/v1/orders.
func newOrdersCmd(apiKey, baseURL, output *string) *cobra.Command {
	orders := &cobra.Command{
		Use:           "orders",
		Short:         "Manage orders over /api/v1/orders",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var listStatus, listCursor string
	var shipAmount int64

	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List orders, newest first (paginated)",
		Long:          "List orders via GET /api/v1/orders. Filter with --status (one lifecycle status); page with --cursor from a previous call.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrdersList(cmd, args, ordersListOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				status:  listStatus,
				cursor:  listCursor,
			})
		},
	}
	listCmd.Flags().StringVar(&listStatus, "status", "", "lifecycle status to filter by (e.g. pending, paid)")
	listCmd.Flags().StringVar(&listCursor, "cursor", "", "opaque cursor returned by a previous list call")

	getCmd := &cobra.Command{
		Use:           "get <order-id>",
		Short:         "Get a single order with its item snapshot",
		Long:          "Get one order via GET /api/v1/orders/{orderID}. The id is the numeric admin id, not the ORD-000042 display number.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrdersGet(cmd, args, ordersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	transitionCmd := &cobra.Command{
		Use:           "transition <order-id> <status>",
		Short:         "Move an order to a new lifecycle status",
		Long:          "Move an order via POST /api/v1/orders/{orderID}/status. Illegal moves read 409; \"paid\" is refused outright — payment confirmation is the only path into paid.",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrdersTransition(cmd, args, ordersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	shippingCmd := &cobra.Command{
		Use:           "shipping <order-id>",
		Short:         "Set the shipping line of a pending order",
		Long:          "Set the admin shipping line in integer minor units via POST /api/v1/orders/{orderID}/shipping (pass --amount; a positional amount cannot start with a dash). Only pending orders carry shipping; settled orders refuse with 409.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrdersShipping(cmd, args, ordersShippingOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				amount:  shipAmount,
			})
		},
	}
	shippingCmd.Flags().Int64Var(&shipAmount, "amount", -1, "shipping line in integer minor units (>= 0, required)")

	orders.AddCommand(listCmd)
	orders.AddCommand(getCmd)
	orders.AddCommand(transitionCmd)
	orders.AddCommand(shippingCmd)
	return orders
}

// ordersListOptions bundles the resolved flag values for runOrdersList.
type ordersListOptions struct {
	apiKey  string
	baseURL string
	output  string
	status  string
	cursor  string
}

// ordersReadOptions bundles the resolved flag values for the single-order
// commands (get, transition, shipping).
type ordersReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// runOrdersList implements `lesstruct-cli orders list`.
func runOrdersList(cmd *cobra.Command, args []string, opts ordersListOptions) error {
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

	data, meta, err := cl.OrdersList(cmd.Context(), opts.status, opts.cursor)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printOrdersList(cmd.OutOrStdout(), opts.output, data, meta)
}

// runOrdersGet implements `lesstruct-cli orders get <order-id>`.
func runOrdersGet(cmd *cobra.Command, args []string, opts ordersReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseOrderID(args[0])
	if err != nil {
		return err
	}

	return runSingleOrder(cmd, opts, func(cl *client.Client) (json.RawMessage, json.RawMessage, error) {
		return cl.OrdersGet(cmd.Context(), id)
	}, "Order")
}

// runOrdersTransition implements `lesstruct-cli orders transition <order-id> <status>`.
func runOrdersTransition(cmd *cobra.Command, args []string, opts ordersReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseOrderID(args[0])
	if err != nil {
		return err
	}

	return runSingleOrder(cmd, opts, func(cl *client.Client) (json.RawMessage, json.RawMessage, error) {
		return cl.OrdersTransition(cmd.Context(), id, args[1])
	}, "Order")
}

// ordersShippingOptions bundles the resolved flag values for
// runOrdersShipping.
type ordersShippingOptions struct {
	apiKey  string
	baseURL string
	output  string
	amount  int64
}

// runOrdersShipping implements `lesstruct-cli orders shipping <order-id> --amount N`.
func runOrdersShipping(cmd *cobra.Command, args []string, opts ordersShippingOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseOrderID(args[0])
	if err != nil {
		return err
	}

	if !cmd.Flags().Changed("amount") {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --amount is required",
		}
	}
	if opts.amount < 0 {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: invalid shipping amount: %d (want integer minor units >= 0)", opts.amount),
		}
	}

	readOpts := ordersReadOptions{apiKey: opts.apiKey, baseURL: opts.baseURL, output: opts.output}
	return runSingleOrder(cmd, readOpts, func(cl *client.Client) (json.RawMessage, json.RawMessage, error) {
		return cl.OrdersSetShipping(cmd.Context(), id, opts.amount)
	}, "Order")
}

// runSingleOrder resolves credentials, runs fetch against a client, and
// prints one order. It keeps get/transition/shipping to one code path:
// the verb differs only in the status line.
func runSingleOrder(cmd *cobra.Command, opts ordersReadOptions, fetch func(*client.Client) (json.RawMessage, json.RawMessage, error), verb string) error {
	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := fetch(cl)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no order",
		}
	}

	return printOrder(cmd.OutOrStdout(), opts.output, data, meta, verb)
}

// parseOrderID validates a positional numeric admin order id. The display
// number (ORD-000042) is not accepted — the admin surface keys on the row
// id, and silently misreading a display number would operate the wrong
// order.
func parseOrderID(arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || id <= 0 {
		return 0, &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: invalid order id: %q (want the numeric admin id, not ORD-000042)", arg),
		}
	}
	return id, nil
}

// printOrdersList writes an order list to w in the requested mode.
func printOrdersList(w io.Writer, mode string, data, meta json.RawMessage) error {
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

	var orders []orderSummary
	if err := json.Unmarshal(data, &orders); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	for _, o := range orders {
		if _, err := fmt.Fprintf(w, "%s %s %s total %d %s (%d items)\n",
			o.OrderNumber, o.Status, o.CustomerEmail, o.TotalMinor, o.Currency, o.ItemCount); err != nil {
			return err
		}
	}
	return nil
}

// printOrder writes a single order to w in the requested mode.
func printOrder(w io.Writer, mode string, data, meta json.RawMessage, verb string) error {
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

	var o orderSummary
	if err := json.Unmarshal(data, &o); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if o.ID == 0 {
		return fmt.Errorf("decode response: missing order projection")
	}

	_, err := fmt.Fprintf(w, "%s %s %s %s total %d %s (%d items)\n",
		verb, o.OrderNumber, o.Status, o.CustomerEmail, o.TotalMinor, o.Currency, o.ItemCount)
	return err
}
