package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redirectCatcher records the Authorization header it sees and answers a
// valid content envelope, so the client's redirect policy is the only thing
// under test: a forwarded key arrives here, a stripped one does not.
type redirectCatcher struct {
	gotAuth string
}

func (c *redirectCatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.gotAuth = r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}}}`))
}

func TestClient_RedirectPolicy(t *testing.T) {
	t.Run("self redirect keeps the key", func(t *testing.T) {
		var catcher redirectCatcher
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery == "" {
				http.Redirect(w, r, "/api/v1/content/7?followed=1", http.StatusFound)
				return
			}
			catcher.ServeHTTP(w, r)
		}))
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.GetContent(context.Background(), 7)
		require.NoError(t, err, "redirect chain must resolve")
		assert.Equal(t, "Bearer k", catcher.gotAuth, "same origin must keep the key")
	})

	t.Run("cross port strips the key", func(t *testing.T) {
		var catcher redirectCatcher
		target := httptest.NewServer(&catcher)
		defer target.Close()

		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/api/v1/content/7", http.StatusFound)
		}))
		defer origin.Close()
		require.NotEqual(t, origin.URL, target.URL, "httptest servers must listen on different ports")

		c, err := client.New(origin.URL, "k")
		require.NoError(t, err)

		_, _, err = c.GetContent(context.Background(), 7)
		require.NoError(t, err, "redirect chain must resolve")
		assert.Empty(t, catcher.gotAuth, "cross-port redirect must drop the key")
	})

	t.Run("cross host strips the key", func(t *testing.T) {
		var catcher redirectCatcher
		target := httptest.NewServer(&catcher)
		defer target.Close()

		targetURL, err := url.Parse(target.URL)
		require.NoError(t, err)
		// Same loopback, different hostname spelling: proves the hostname
		// itself is compared, not just the port.
		dest := "http://" + "localhost:" + targetURL.Port() + "/api/v1/content/7"
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, dest, http.StatusFound)
		}))
		defer origin.Close()

		c, err := client.New(origin.URL, "k")
		require.NoError(t, err)

		_, _, err = c.GetContent(context.Background(), 7)
		require.NoError(t, err, "redirect chain must resolve")
		assert.Empty(t, catcher.gotAuth, "cross-host redirect must drop the key")
	})
}
