package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateComment_Success(t *testing.T) {
	h := &recordingHandler{
		status: http.StatusOK,
		body:   `{"data":{"comment":{"id":9,"comment":"Nice!","status":"pending","createdAt":"2026-06-23T12:00:00Z"}}}`,
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
	require.NoError(t, err)

	data, meta, err := c.CreateComment(context.Background(), 5, client.CreateCommentRequest{Comment: "Nice!"})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, h.gotMethod)
	assert.Equal(t, "/api/v1/content/5/comments", h.gotPath)
	assert.Equal(t, "Bearer lesstruct_a1b2c3d4e5f6_<secret>", h.gotAuth)
	assert.Equal(t, "Nice!", h.gotPayload["comment"])

	var resp struct {
		Comment struct {
			ID int `json:"id"`
		} `json:"comment"`
	}
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, 9, resp.Comment.ID)
	assert.Nil(t, meta)
}

func TestClient_ListComments_Success(t *testing.T) {
	h := &recordingHandler{
		status: http.StatusOK,
		body:   `{"data":[{"id":1,"comment":"ok","status":"approved"}]}`,
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	data, _, err := c.ListComments(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, h.gotMethod)
	assert.Equal(t, "/api/v1/content/5/comments", h.gotPath)

	var items []map[string]any
	require.NoError(t, json.Unmarshal(data, &items))
	require.Len(t, items, 1)
	assert.Equal(t, float64(1), items[0]["id"])
}

func TestClient_DeleteComment_Success(t *testing.T) {
	h := &recordingHandler{
		status: http.StatusNoContent,
		body:   "",
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	data, meta, err := c.DeleteComment(context.Background(), 5, 9)
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, h.gotMethod)
	assert.Equal(t, "/api/v1/content/5/comments/9", h.gotPath)
	assert.Nil(t, data)
	assert.Nil(t, meta)
}

func TestClient_UpdateCommentStatus_Success(t *testing.T) {
	h := &recordingHandler{
		status: http.StatusOK,
		body:   `{"data":{"comment":{"id":9,"comment":"ok","status":"approved"}}}`,
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	data, _, err := c.UpdateCommentStatus(context.Background(), 5, 9, "approved")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPut, h.gotMethod)
	assert.Equal(t, "/api/v1/content/5/comments/9/status", h.gotPath)
	assert.Equal(t, "approved", h.gotPayload["status"])

	var resp struct {
		Comment struct {
			Status string `json:"status"`
		} `json:"comment"`
	}
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, "approved", resp.Comment.Status)
}
