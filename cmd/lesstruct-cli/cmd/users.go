package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// userSummary is the cmd-layer projection of an admin user row — the fields
// the text output prints. Decoding a server envelope into it drops unknown
// fields silently.
type userSummary struct {
	ID            int    `json:"id"`
	Username      string `json:"username"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email"`
	Status        string `json:"status,omitempty"`
	Role          string `json:"role,omitempty"`
	EmailVerified bool   `json:"emailVerified,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
}

// newUsersCmd builds the `users` command tree: list, registration-queue
// triage, and account moderation over /api/v1/users. Account creation and
// role changes stay in the admin panel; the destructive decisions require
// explicit confirmation.
func newUsersCmd(apiKey, baseURL, output *string) *cobra.Command {
	users := &cobra.Command{
		Use:           "users",
		Short:         "Triage registrations and moderate accounts over /api/v1/users",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var listStatus string
	var listLimit, listOffset int
	var reason string

	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List users, newest first (paginated)",
		Long:          "List users via GET /api/v1/users. Filter with --status (pending, verified, suspended, soft_deleted); page with --limit/--offset.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersList(cmd, args, usersListOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				status:  listStatus,
				limit:   listLimit,
				offset:  listOffset,
			})
		},
	}
	listCmd.Flags().StringVar(&listStatus, "status", "", "user status to filter by (pending, verified, suspended, soft_deleted)")
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "page size (0 = server default)")
	listCmd.Flags().IntVar(&listOffset, "offset", 0, "page offset")

	pendingCmd := &cobra.Command{
		Use:           "pending",
		Short:         "List the registration queue awaiting a decision",
		Long:          "List pending registrations via GET /api/v1/users/pending, including each registrant's email-verification state. Approve only verified rows (the server refuses the rest with 409 EMAIL_NOT_VERIFIED).",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersPending(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	approveCmd := &cobra.Command{
		Use:           "approve <user-id>",
		Short:         "Approve a pending registration",
		Long:          "Approve a pending registration via POST /api/v1/users/{id}/approve. Non-pending users read 400 INVALID_USER_STATUS. When the store requires admin approval ([registration] admin_approval), the registrant must also have verified their email first (409 EMAIL_NOT_VERIFIED otherwise); with the default open registration, approval activates the account directly.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersDecide(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "approve")
		},
	}

	rejectCmd := &cobra.Command{
		Use:           "reject <user-id>",
		Short:         "Reject a pending registration (requires --confirm)",
		Long:          "Reject a pending registration via POST /api/v1/users/{id}/reject. Destructive and final: the user row is deleted (there is no rejected status to audit later), so pass --confirm to send the required confirmation. A rejection email goes out best-effort.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersDecide(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "reject")
		},
	}

	spamCmd := &cobra.Command{
		Use:           "mark-spam <user-id>",
		Short:         "Mark a registration as spam (requires --confirm)",
		Long:          "Mark a registration as spam via POST /api/v1/users/{id}/mark-spam. Destructive and final: the email is added to the blocklist and the user row is deleted (no record remains to audit later), so pass --confirm to send the required confirmation.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersDecide(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "mark-spam")
		},
	}

	suspendCmd := &cobra.Command{
		Use:           "suspend <user-id>",
		Short:         "Suspend an account (optional --reason)",
		Long:          "Suspend an account via POST /api/v1/users/{id}/suspend. A --reason is emailed to the user best-effort. Settled states refuse with 409.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersDecide(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "suspend")
		},
	}
	suspendCmd.Flags().StringVar(&reason, "reason", "", "suspension reason, emailed to the user")

	unsuspendCmd := &cobra.Command{
		Use:           "unsuspend <user-id>",
		Short:         "Lift an account suspension",
		Long:          "Lift an account suspension via POST /api/v1/users/{id}/unsuspend.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersDecide(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "unsuspend")
		},
	}

	deleteCmd := &cobra.Command{
		Use:           "delete <user-id>",
		Short:         "Soft-delete an account (requires --confirm)",
		Long:          "Soft-delete an account via POST /api/v1/users/{id}/soft-delete. Destructive: pass --confirm to send the required confirmation; --reason is recorded alongside.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsersDecide(cmd, args, usersReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, "delete")
		},
	}
	deleteCmd.Flags().StringVar(&reason, "reason", "", "reason recorded with the soft-delete")

	// --confirm gates the destructive decisions, mirroring the server's
	// confirmation-body contract on reject/mark-spam/soft-delete. Wiring it
	// as a persistent-style flag on the parent keeps every decision behind
	// the same explicit gate.
	for _, c := range []*cobra.Command{rejectCmd, spamCmd, suspendCmd, deleteCmd} {
		c.Flags().Bool("confirm", false, "confirm this destructive decision (required)")
	}

	users.AddCommand(listCmd)
	users.AddCommand(pendingCmd)
	users.AddCommand(approveCmd)
	users.AddCommand(rejectCmd)
	users.AddCommand(spamCmd)
	users.AddCommand(suspendCmd)
	users.AddCommand(unsuspendCmd)
	users.AddCommand(deleteCmd)
	return users
}

// usersListOptions bundles the resolved flag values for runUsersList.
type usersListOptions struct {
	apiKey  string
	baseURL string
	output  string
	status  string
	limit   int
	offset  int
}

// usersReadOptions bundles the resolved flag values for the single-user
// commands (pending, approve, reject, mark-spam, suspend, unsuspend,
// delete).
type usersReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// runUsersList implements `lesstruct-cli users list`.
func runUsersList(cmd *cobra.Command, args []string, opts usersListOptions) error {
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

	data, meta, err := cl.UsersList(cmd.Context(), opts.status, opts.limit, opts.offset)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}
	var users []userSummary
	if err := json.Unmarshal(data, &users); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	for _, u := range users {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "#%d %s <%s> %s %s\n",
			u.ID, u.Username, u.Email, u.Role, u.Status); err != nil {
			return err
		}
	}
	return nil
}

// runUsersPending implements `lesstruct-cli users pending`.
func runUsersPending(cmd *cobra.Command, args []string, opts usersReadOptions) error {
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

	data, meta, err := cl.UsersPending(cmd.Context())
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}
	var users []userSummary
	if err := json.Unmarshal(data, &users); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	for _, u := range users {
		verified := "unverified"
		if u.EmailVerified {
			verified = "verified"
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "#%d %s <%s> %s\n",
			u.ID, u.Username, u.Email, verified); err != nil {
			return err
		}
	}
	return nil
}

// runUsersDecide implements the single-user decision commands: approve,
// reject, mark-spam, suspend, unsuspend, and delete.
func runUsersDecide(cmd *cobra.Command, args []string, opts usersReadOptions, action string) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseUserID(args[0])
	if err != nil {
		return err
	}

	if action == "reject" || action == "mark-spam" || action == "delete" {
		confirmed, cerr := cmd.Flags().GetBool("confirm")
		if cerr != nil || !confirmed {
			return &exitError{
				code: client.ExitUsage,
				msg:  "lesstruct-cli: --confirm is required for this destructive decision",
			}
		}
	}
	var reason string
	if action == "suspend" || action == "delete" {
		reason, _ = cmd.Flags().GetString("reason")
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	var data, meta json.RawMessage
	switch action {
	case "approve":
		data, meta, err = cl.UsersApprove(cmd.Context(), id)
	case "reject":
		data, meta, err = cl.UsersReject(cmd.Context(), id)
	case "mark-spam":
		data, meta, err = cl.UsersMarkSpam(cmd.Context(), id)
	case "suspend":
		data, meta, err = cl.UsersSuspend(cmd.Context(), id, reason)
	case "unsuspend":
		data, meta, err = cl.UsersUnsuspend(cmd.Context(), id)
	case "delete":
		data, meta, err = cl.UsersSoftDelete(cmd.Context(), id, reason)
	default:
		return &exitError{code: client.ExitGeneric, msg: "lesstruct-cli: unknown user decision"}
	}
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printUserDecision(cmd.OutOrStdout(), opts.output, data, meta, action, pastTense(action))
}

func pastTense(action string) string {
	switch action {
	case "approve":
		return "approved"
	case "reject":
		return "rejected"
	case "mark-spam":
		return "marked as spam"
	case "suspend":
		return "suspended"
	case "unsuspend":
		return "unsuspended"
	case "delete":
		return "deleted"
	default:
		return action + "d"
	}
}

// parseUserID validates a positional user id.
func parseUserID(arg string) (int64, error) {
	id, err := parseIntID(arg, "user")
	if err != nil {
		return 0, err
	}
	return int64(id), nil
}

// printUserDecision writes a decision response to w in the requested mode.
func printUserDecision(w io.Writer, mode string, data, meta json.RawMessage, action, verb string) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	_, err := fmt.Fprintf(w, "user %s\n", verb)
	return err
}
