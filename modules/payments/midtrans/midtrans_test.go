package midtrans_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	"github.com/aristorinjuang/lesstruct/modules/payments/midtrans"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc intercepts any upstream request, so tests never leave the
// process while the driver still exercises its real request building.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func payableOrder() *paymentsdomain.OrderRef {
	return &paymentsdomain.OrderRef{
		ID:            42,
		OrderNumber:   "ORD-000042",
		Status:        "pending",
		TotalMinor:    522500,
		CustomerEmail: "shopper@example.com",
		Currency:      "IDR",
	}
}

func TestDriver_Mint(t *testing.T) {
	tests := []struct {
		name         string
		production   bool
		trip         roundTripFunc
		wantErr      string
		wantUpstream bool
		want         *paymentsdomain.GatewaySession
	}{
		{
			name: "mints a session over the sandbox with the server key",
			trip: func(r *http.Request) (*http.Response, error) {
				return respond(http.StatusOK, `{"token":"tok-1","redirect_url":"https://app.sandbox.midtrans.com/snap/v2/vtwindow?tok-1"}`), nil
			},
			want: &paymentsdomain.GatewaySession{
				Token:       "tok-1",
				RedirectURL: "https://app.sandbox.midtrans.com/snap/v2/vtwindow?tok-1",
			},
		},
		{
			name:       "the production base serves the session",
			production: true,
			trip: func(r *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(r.URL.String(), "https://app.midtrans.com/snap/v1/transactions") {
					return respond(http.StatusNotFound, `{"error":"wrong base"}`), nil
				}
				return respond(http.StatusOK, `{"token":"tok-2","redirect_url":"https://app.midtrans.com/snap/v2/vtwindow?tok-2"}`), nil
			},
			want: &paymentsdomain.GatewaySession{
				Token:       "tok-2",
				RedirectURL: "https://app.midtrans.com/snap/v2/vtwindow?tok-2",
			},
		},
		{
			name: "an upstream failure surfaces with the status",
			trip: func(r *http.Request) (*http.Response, error) {
				return respond(http.StatusUnauthorized, `401 Unauthorized`), nil
			},
			wantErr:      "midtrans mint failed (HTTP 401)",
			wantUpstream: true,
		},
		{
			name: "a tokenless reply refuses",
			trip: func(r *http.Request) (*http.Response, error) {
				return respond(http.StatusOK, `{}`), nil
			},
			wantErr:      "midtrans mint returned no token",
			wantUpstream: true,
		},
		{
			name: "a malformed reply refuses",
			trip: func(r *http.Request) (*http.Response, error) {
				return respond(http.StatusOK, `{not json`), nil
			},
			wantErr:      "failed to decode the midtrans mint response",
			wantUpstream: true,
		},
		{
			name: "a transport failure surfaces",
			trip: func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			wantErr:      "midtrans mint request failed",
			wantUpstream: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := midtrans.New("server-key", tt.production, midtrans.WithHTTPClient(&http.Client{Transport: tt.trip}))

			session, err := driver.Mint(context.Background(), payableOrder())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				if tt.wantUpstream {
					assert.True(t, errors.Is(err, paymentsdomain.ErrGatewayUpstream), "upstream failures must carry the gateway sentinel: %v", err)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, session)
		})
	}

	t.Run("carries the order and auth on the request", func(t *testing.T) {
		var sawAuth, sawOrderID, sawGross, sawEmail string
		trip := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			sawAuth = r.Header.Get("Authorization")
			raw, _ := io.ReadAll(r.Body)
			payload := string(raw)
			sawOrderID = `"order_id":"ORD-000042"`
			if !strings.Contains(payload, sawOrderID) {
				sawOrderID = payload
			}
			sawGross = `"gross_amount":522500`
			sawEmail = `"email":"shopper@example.com"`
			return respond(http.StatusOK, `{"token":"tok","redirect_url":"u"}`), nil
		})
		driver := midtrans.New("server-key", false, midtrans.WithHTTPClient(&http.Client{Transport: trip}))

		_, err := driver.Mint(context.Background(), payableOrder())
		require.NoError(t, err)

		user, _, _ := strings.Cut(sawAuth, " ")
		assert.Equal(t, "Basic", user)
		assert.Contains(t, sawAuth, "Basic", "the server key rides basic auth")
		assert.Equal(t, `"order_id":"ORD-000042"`, sawOrderID)
		assert.Equal(t, `"gross_amount":522500`, sawGross)
		assert.Equal(t, `"email":"shopper@example.com"`, sawEmail)
	})

	t.Run("a session TTL mirrors into the expiry payload", func(t *testing.T) {
		var payload string
		trip := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			payload = string(raw)
			return respond(http.StatusOK, `{"token":"tok","redirect_url":"u"}`), nil
		})
		driver := midtrans.New("server-key", false,
			midtrans.WithHTTPClient(&http.Client{Transport: trip}),
			midtrans.WithSessionTTL(90*time.Minute),
		)

		_, err := driver.Mint(context.Background(), payableOrder())

		require.NoError(t, err)
		assert.Contains(t, payload, `"expiry":{"duration":2,"unit":"hour"}`, "90 minutes rounds up to whole hours")
	})

	t.Run("a non-payable total refuses before the wire", func(t *testing.T) {
		driver := midtrans.New("server-key", false)
		broke := payableOrder()
		broke.TotalMinor = 0

		_, err := driver.Mint(context.Background(), broke)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not payable")
	})

	t.Run("finish callbacks steer the buyer back to tracking", func(t *testing.T) {
		var payload string
		trip := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			payload = string(raw)
			return respond(http.StatusOK, `{"token":"tok","redirect_url":"u"}`), nil
		})
		driver := midtrans.New("server-key", false,
			midtrans.WithHTTPClient(&http.Client{Transport: trip}),
			midtrans.WithFinishBase("https://shop.example"),
		)
		order := payableOrder()
		order.TrackingToken = "track-abc"

		_, err := driver.Mint(context.Background(), order)

		require.NoError(t, err)
		var decoded struct {
			Callbacks *struct {
				Finish string `json:"finish"`
			} `json:"callbacks"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &decoded))
		require.NotNil(t, decoded.Callbacks, "the finish base must send callbacks")
		target, err := url.Parse(decoded.Callbacks.Finish)
		require.NoError(t, err)
		assert.Equal(t, "https://shop.example/orders/track", target.Scheme+"://"+target.Host+target.Path)
		query := target.Query()
		assert.Equal(t, "ORD-000042", query.Get("order"))
		assert.Equal(t, "shopper@example.com", query.Get("email"))
		assert.Equal(t, "track-abc", query.Get("t"))
	})

	t.Run("no finish base sends no callbacks", func(t *testing.T) {
		var payload string
		trip := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			payload = string(raw)
			return respond(http.StatusOK, `{"token":"tok","redirect_url":"u"}`), nil
		})
		driver := midtrans.New("server-key", false, midtrans.WithHTTPClient(&http.Client{Transport: trip}))

		_, err := driver.Mint(context.Background(), payableOrder())

		require.NoError(t, err)
		assert.NotContains(t, payload, "callbacks")
	})

	t.Run("a missing tracking token omits the token parameter", func(t *testing.T) {
		var payload string
		trip := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			payload = string(raw)
			return respond(http.StatusOK, `{"token":"tok","redirect_url":"u"}`), nil
		})
		driver := midtrans.New("server-key", false,
			midtrans.WithHTTPClient(&http.Client{Transport: trip}),
			midtrans.WithFinishBase("https://shop.example/"),
		)

		_, err := driver.Mint(context.Background(), payableOrder())

		require.NoError(t, err)
		var decoded struct {
			Callbacks *struct {
				Finish string `json:"finish"`
			} `json:"callbacks"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &decoded))
		require.NotNil(t, decoded.Callbacks)
		target, err := url.Parse(decoded.Callbacks.Finish)
		require.NoError(t, err)
		assert.Equal(t, "https://shop.example/orders/track", target.Scheme+"://"+target.Host+target.Path)
		query := target.Query()
		assert.Equal(t, "ORD-000042", query.Get("order"))
		assert.Equal(t, "shopper@example.com", query.Get("email"))
		assert.Empty(t, query.Get("t"), "no token parameter without a tracking token")
	})
}

func TestWellFormedServerKey(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		production bool
		want       bool
	}{
		{name: "sandbox key on sandbox", key: "SB-Mid-server-abc123", production: false, want: true},
		{name: "production key on production", key: "Mid-server-abc123", production: true, want: true},
		{name: "sandbox key on production refuses", key: "SB-Mid-server-abc123", production: true, want: false},
		{name: "production key on sandbox refuses", key: "Mid-server-abc123", production: false, want: false},
		{name: "merchant id shape refuses", key: "M5935abcdefghij12345678901234567", production: false, want: false},
		{name: "empty key refuses", key: "", production: false, want: false},
		{name: "surrounding whitespace is tolerated", key: "  SB-Mid-server-abc123\n", production: false, want: true},
		{name: "quote-wrapped key refuses", key: `"SB-Mid-server-abc123"`, production: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, midtrans.WellFormedServerKey(tt.key, tt.production))
		})
	}
}

func TestDriver_Authenticate(t *testing.T) {
	driver := midtrans.New("server-key", false)
	mac := sha512.Sum512([]byte("ORD-000042" + "200" + "522500.00" + "server-key"))
	valid := hex.EncodeToString(mac[:])

	tests := []struct {
		name       string
		signature  string
		authorized bool
	}{
		{
			name:       "the server-key checksum passes",
			signature:  valid,
			authorized: true,
		},
		{
			name:       "a foreign checksum refuses",
			signature:  hex.EncodeToString([]byte("deadbeef")),
			authorized: false,
		},
		{
			name:       "an empty signature refuses",
			signature:  "",
			authorized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.authorized, driver.Authenticate("ORD-000042", "200", "522500.00", tt.signature))
		})
	}
}

func TestDriver_MinorUnits(t *testing.T) {
	tests := []struct {
		name    string
		gross   string
		want    int64
		wantErr bool
	}{
		{
			name:  "the standard decimal form converts to rupiah minor units",
			gross: "522500.00",
			want:  522500,
		},
		{
			name:  "an integer form converts unchanged",
			gross: "522500",
			want:  522500,
		},
		{
			name:  "whitespace trims",
			gross: " 522500.00 ",
			want:  522500,
		},
		{
			name:  "a trailing decimal point passes",
			gross: "522500.",
			want:  522500,
		},
		{
			name:    "fractional rupiah refuses",
			gross:   "522500.50",
			wantErr: true,
		},
		{
			name:    "a negative amount refuses",
			gross:   "-1",
			wantErr: true,
		},
		{
			name:    "an unparseable amount refuses",
			gross:   "five hundred",
			wantErr: true,
		},
		{
			name:    "an empty amount refuses",
			gross:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := midtrans.New("server-key", false)

			units, err := driver.MinorUnits(tt.gross)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, units)
		})
	}
}
