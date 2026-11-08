package cmd

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// usersListArgs carries the `users_list` filters.
type usersListArgs struct {
	Status string `json:"status,omitempty" jsonschema:"filter by status: pending, verified, suspended, soft_deleted"`
	Limit  int    `json:"limit,omitempty" jsonschema:"page size"`
	Offset int    `json:"offset,omitempty" jsonschema:"page offset"`
}

// userDecideArgs carries the user decision tools' payload. Destructive
// decisions (reject, mark_spam, soft_delete) require confirm:true — the call
// is refused otherwise, mirroring the CLI's --confirm gate.
type userDecideArgs struct {
	ID      int64  `json:"id" jsonschema:"user id (numeric)"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"required true for reject, mark_spam, and soft_delete"`
	Reason  string `json:"reason,omitempty" jsonschema:"reason recorded with suspend and soft-delete"`
}

// registerUserTools adds the registration-triage and moderation tools to ms.
// All handlers close over the shared /api/v1 client.
func registerUserTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_list",
		Description: "List users, newest first (paginated). Filter by status; page with limit/offset.",
	}, makeUsersListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_pending",
		Description: "List the registration queue awaiting a decision, with each registrant's email-verification state. Approve only verified rows.",
	}, makeUsersPendingHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_approve",
		Description: "Approve a pending registration. Non-pending users read 400; unverified emails read 409 when admin approval is required.",
	}, makeUsersApproveHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_reject",
		Description: "Reject a pending registration (requires confirm:true). Destructive and final: the user row is deleted.",
	}, makeUsersRejectHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_mark_spam",
		Description: "Mark a registration as spam (requires confirm:true). The email is blocklisted and the user row is deleted.",
	}, makeUsersMarkSpamHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_suspend",
		Description: "Suspend an account, with an optional recorded reason.",
	}, makeUsersSuspendHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_unsuspend",
		Description: "Unsuspend an account.",
	}, makeUsersUnsuspendHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "users_soft_delete",
		Description: "Soft-delete an account (requires confirm:true), with an optional recorded reason.",
	}, makeUsersSoftDeleteHandler(c))
}

// makeUsersListHandler returns the `users_list` handler.
func makeUsersListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, usersListArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args usersListArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.UsersList(ctx, args.Status, args.Limit, args.Offset)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersPendingHandler returns the `users_pending` handler.
func makeUsersPendingHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.UsersPending(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersApproveHandler returns the `users_approve` handler.
func makeUsersApproveHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, userDecideArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args userDecideArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid user id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.UsersApprove(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no decision")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersRejectHandler returns the `users_reject` handler.
func makeUsersRejectHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, userDecideArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args userDecideArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid user id: %d", args.ID)), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.UsersReject(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no decision")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersMarkSpamHandler returns the `users_mark_spam` handler.
func makeUsersMarkSpamHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, userDecideArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args userDecideArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid user id: %d", args.ID)), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.UsersMarkSpam(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no decision")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersSuspendHandler returns the `users_suspend` handler.
func makeUsersSuspendHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, userDecideArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args userDecideArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid user id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.UsersSuspend(ctx, args.ID, args.Reason)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no decision")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersUnsuspendHandler returns the `users_unsuspend` handler.
func makeUsersUnsuspendHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, userDecideArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args userDecideArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid user id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.UsersUnsuspend(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no decision")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeUsersSoftDeleteHandler returns the `users_soft_delete` handler.
func makeUsersSoftDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, userDecideArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args userDecideArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid user id: %d", args.ID)), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.UsersSoftDelete(ctx, args.ID, args.Reason)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no decision")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}
