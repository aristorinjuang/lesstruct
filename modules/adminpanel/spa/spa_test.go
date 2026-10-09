package spa_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/adminpanel/spa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeAdmin_RootReturnsIndexHTML(t *testing.T) {
	h := spa.NewHandler(false, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.ServeAdmin(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lesstruct")
	assert.Contains(t, w.Body.String(), "<html")
}

func TestServeAdmin_UnknownPathFallsBackToIndex(t *testing.T) {
	h := spa.NewHandler(false, "")

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-page", nil)
	w := httptest.NewRecorder()

	h.ServeAdmin(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lesstruct")
}

func TestServeAdmin_ExistingFileServedDirectly(t *testing.T) {
	want, err := fs.ReadFile(spa.FS(), "favicon.ico")
	require.NoError(t, err)

	h := spa.NewHandler(false, "")

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	w := httptest.NewRecorder()

	h.ServeAdmin(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, want, w.Body.Bytes())
}

func TestServeAdmin_DevModeProxies(t *testing.T) {
	h := spa.NewHandler(true, "http://127.0.0.1:1")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	h.ServeAdmin(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestServeAdmin_DevModeBadURLReturns502(t *testing.T) {
	h := spa.NewHandler(true, "://invalid-url")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.ServeAdmin(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestEmbeddedFilesExist(t *testing.T) {
	_, err := fs.Stat(spa.FS(), "index.html")
	assert.NoError(t, err, "admin index.html should exist in embedded FS")
}
