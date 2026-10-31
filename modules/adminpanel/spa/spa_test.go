package spa_test

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/adminpanel/spa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedBuild_IncludesTwoStepLogin(t *testing.T) {
	buildFS := spa.FS()

	var (
		foundMFARequired bool
		foundVerifyCall  bool
	)

	err := fs.WalkDir(buildFS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}

		content, err := fs.ReadFile(buildFS, path)
		if err != nil {
			return err
		}

		if bytes.Contains(content, []byte("mfaRequired")) {
			foundMFARequired = true
		}
		if bytes.Contains(content, []byte("/api/auth/mfa/verify")) {
			foundVerifyCall = true
		}

		return nil
	})
	require.NoError(t, err)
	assert.True(t, foundMFARequired, "embedded admin build should handle the MFA challenge; run make build-admin")
	assert.True(t, foundVerifyCall, "embedded admin build should call the MFA verify endpoint; run make build-admin")
}

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
