package static_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/static"
	"github.com/stretchr/testify/assert"
)

func mockContentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>mock content</html>"))
	})
}

func TestNewStaticServer(t *testing.T) {
	s := static.NewStaticServer(false, "http://localhost:5173", mockContentHandler())
	assert.NotNil(t, s)
}

func TestServeAdmin_RootReturnsIndexHTML(t *testing.T) {
	s := static.NewStaticServer(false, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.ServeAdmin(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lesstruct")
	assert.Contains(t, w.Body.String(), "<html")
}

func TestServeAdmin_UnknownPathFallsBackToIndex(t *testing.T) {
	s := static.NewStaticServer(false, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-page", nil)
	w := httptest.NewRecorder()

	s.ServeAdmin(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lesstruct")
}

func TestServeAdmin_ExistingFileServedDirectly(t *testing.T) {
	s := static.NewStaticServer(false, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/.gitkeep", nil)
	w := httptest.NewRecorder()

	s.ServeAdmin(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeContent_DelegatesToContentPage(t *testing.T) {
	s := static.NewStaticServer(false, "", mockContentHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.ServeContent(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mock content")
}

func TestServeContent_NoContentPageReturns404(t *testing.T) {
	s := static.NewStaticServer(false, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.ServeContent(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeAdmin_DevModeProxies(t *testing.T) {
	s := static.NewStaticServer(true, "http://127.0.0.1:1", nil)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	s.ServeAdmin(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestServeContent_AlwaysDelegatesToContentPage(t *testing.T) {
	s := static.NewStaticServer(true, "http://127.0.0.1:1", mockContentHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.ServeContent(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mock content")
}

func TestServeAdmin_DevModeBadURLReturns502(t *testing.T) {
	s := static.NewStaticServer(true, "://invalid-url", nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.ServeAdmin(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestNewStaticServer_AdminEmbeddedFilesExist(t *testing.T) {
	adminSub, _ := fs.Sub(static.AdminFS, "admin")

	_, adminErr := fs.Stat(adminSub, "index.html")
	assert.NoError(t, adminErr, "admin index.html should exist in embedded FS")
}
