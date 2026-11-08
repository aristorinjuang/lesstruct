package cmd_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserTools_ListPending(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		body     string
		status   int
		wantErr  bool
		wantText string
	}{
		{
			name:     "success - users_list filters by status",
			tool:     "users_list",
			args:     map[string]any{"status": "pending", "limit": 10},
			body:     `{"data":[{"id":1}],"meta":{}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name:     "success - users_pending returns queue",
			tool:     "users_pending",
			args:     map[string]any{},
			body:     `{"data":[{"id":2,"emailVerified":true}]}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"emailVerified":true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
		})
	}
}

func TestUserTools_Decide(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		args        map[string]any
		body        string
		status      int
		wantErr     bool
		wantText    string
		wantPayload map[string]any
	}{
		{
			name:     "success - users_approve activates account",
			tool:     "users_approve",
			args:     map[string]any{"id": 7},
			body:     `{"data":{"decision":"approved"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"approved"`,
		},
		{
			name:     "success - users_reject with confirm deletes",
			tool:     "users_reject",
			args:     map[string]any{"id": 7, "confirm": true},
			body:     `{"data":{"decision":"rejected"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"rejected"`,
		},
		{
			name:     "error - users_reject needs confirm",
			tool:     "users_reject",
			args:     map[string]any{"id": 7},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "confirm:true is required",
		},
		{
			name:     "success - users_mark_spam with confirm blocklists",
			tool:     "users_mark_spam",
			args:     map[string]any{"id": 7, "confirm": true},
			body:     `{"data":{"decision":"spam"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"spam"`,
		},
		{
			name:     "success - users_suspend records reason",
			tool:     "users_suspend",
			args:     map[string]any{"id": 7, "reason": "abuse"},
			body:     `{"data":{"decision":"suspended"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"suspended"`,
			wantPayload: map[string]any{
				"confirmed": true,
				"reason":    "abuse",
			},
		},
		{
			name:     "success - users_unsuspend reactivates",
			tool:     "users_unsuspend",
			args:     map[string]any{"id": 7},
			body:     `{"data":{"decision":"unsuspended"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"unsuspended"`,
		},
		{
			name:     "success - users_soft_delete with confirm removes",
			tool:     "users_soft_delete",
			args:     map[string]any{"id": 7, "confirm": true},
			body:     `{"data":{"decision":"deleted"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"deleted"`,
		},
		{
			name:     "error - users_approve invalid id",
			tool:     "users_approve",
			args:     map[string]any{"id": -2},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "INVALID_ID",
		},
		{
			name:     "error - users_suspend surfaces settled state",
			tool:     "users_suspend",
			args:     map[string]any{"id": 7},
			body:     `{"error":{"code":"INVALID_USER_STATUS","message":"settled"}}`,
			status:   http.StatusConflict,
			wantErr:  true,
			wantText: "INVALID_USER_STATUS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			if tt.wantPayload != nil {
				assert.Equal(t, tt.wantPayload, h.gotPayload)
			}
		})
	}
}

func TestCommentTools(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		args        map[string]any
		body        string
		status      int
		wantErr     bool
		wantText    string
		wantPayload map[string]any
	}{
		{
			name:     "success - comment_create posts comment",
			tool:     "comment_create",
			args:     map[string]any{"contentId": 5, "comment": "Nice post"},
			body:     `{"data":{"comment":{"id":1}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
			wantPayload: map[string]any{
				"comment": "Nice post",
			},
		},
		{
			name:        "error - comment_create needs text",
			tool:        "comment_create",
			args:        map[string]any{"contentId": 5, "comment": ""},
			body:        `{}`,
			status:      http.StatusOK,
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantPayload: nil,
		},
		{
			name:     "success - comment_list returns thread",
			tool:     "comment_list",
			args:     map[string]any{"contentId": 5},
			body:     `{"data":[{"id":1}],"meta":{}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name:     "success - comment_delete confirms",
			tool:     "comment_delete",
			args:     map[string]any{"contentId": 5, "commentId": 1},
			body:     ``,
			status:   http.StatusNoContent,
			wantErr:  false,
			wantText: `"deleted":true`,
		},
		{
			name:     "success - comment_moderate sets status",
			tool:     "comment_moderate",
			args:     map[string]any{"contentId": 5, "commentId": 1, "status": "spam"},
			body:     `{"data":{"comment":{"id":1,"status":"spam"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"spam"`,
			wantPayload: map[string]any{
				"status": "spam",
			},
		},
		{
			name:        "error - comment_moderate rejects unknown status",
			tool:        "comment_moderate",
			args:        map[string]any{"contentId": 5, "commentId": 1, "status": "maybe"},
			body:        `{}`,
			status:      http.StatusOK,
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantPayload: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			if tt.wantPayload != nil {
				assert.Equal(t, tt.wantPayload, h.gotPayload)
			}
		})
	}
}
