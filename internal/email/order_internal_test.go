package email

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateOrderConfirmationEmailBody_Payee(t *testing.T) {
	items := []OrderEmailItem{{SKU: "TSHIRT", Title: "T-Shirt", Quantity: 1, LineTotal: "IDR 299,000"}}
	payee := ManualPayee{Bank: "BCA", Account: "1234567890", Name: "PT Lesstruct Demo"}

	t.Run("set payee appends the how-to-pay block", func(t *testing.T) {
		body := generateOrderConfirmationEmailBody("ORD-000005", items, "IDR 299,000", "IDR 0", "IDR 299,000", payee, "")
		assert.Contains(t, body.text, "How to pay")
		assert.Contains(t, body.text, "BCA")
		assert.Contains(t, body.text, "1234567890")
		assert.Contains(t, body.text, "ORD-000005")
		assert.Contains(t, body.html, "How to pay")
		assert.Contains(t, body.html, "1234567890")
	})

	t.Run("empty payee keeps the legacy body", func(t *testing.T) {
		body := generateOrderConfirmationEmailBody("ORD-000005", items, "IDR 299,000", "IDR 0", "IDR 299,000", ManualPayee{}, "")
		assert.NotContains(t, body.text, "How to pay")
		assert.NotContains(t, body.html, "How to pay")
		assert.True(t, strings.Contains(body.text, "ORD-000005"))
	})
}

func TestGenerateOrderConfirmationEmailBody_TrackingLink(t *testing.T) {
	items := []OrderEmailItem{{SKU: "TSHIRT", Title: "T-Shirt", Quantity: 1, LineTotal: "IDR 299,000"}}

	t.Run("a tracking token appends the track link in both variants", func(t *testing.T) {
		body := generateOrderConfirmationEmailBody("ORD-000005", items, "IDR 299,000", "IDR 0", "IDR 299,000", ManualPayee{}, "http://localhost:8080/orders/track?order=ORD-000005&t=abc")
		assert.Contains(t, body.text, "Track your order")
		assert.Contains(t, body.text, "http://localhost:8080/orders/track?order=ORD-000005&t=abc")
		assert.Contains(t, body.html, `href="http://localhost:8080/orders/track?order=ORD-000005&t=abc"`)
	})

	t.Run("no token sends the legacy body without a link", func(t *testing.T) {
		body := generateOrderConfirmationEmailBody("ORD-000005", items, "IDR 299,000", "IDR 0", "IDR 299,000", ManualPayee{}, "")
		assert.NotContains(t, body.text, "Track your order")
		assert.NotContains(t, body.html, "orders/track")
	})
}
