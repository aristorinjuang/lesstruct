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

func TestGenerateNewOrderAlertEmailBody(t *testing.T) {
	items := []OrderEmailItem{{SKU: "TSHIRT", Title: "T-Shirt", Quantity: 2, LineTotal: "IDR 598,000"}}

	tests := []struct {
		name          string
		customerPhone string
		wantPhone     bool
	}{
		{
			name:          "a set phone rides the notice",
			customerPhone: "+6281234567890",
			wantPhone:     true,
		},
		{
			name:          "an empty phone stays silent",
			customerPhone: "",
			wantPhone:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := generateNewOrderAlertEmailBody(
				"ORD-000042",
				"Annisa Putri",
				"shopper@example.com",
				tt.customerPhone,
				"Jl. Kemang Raya 21\nJakarta Selatan",
				items,
				"IDR 598,000",
				"IDR 0",
				"IDR 598,000",
			)

			assert.Contains(t, body.text, "ORD-000042")
			assert.Contains(t, body.text, "Annisa Putri")
			assert.Contains(t, body.text, "shopper@example.com")
			assert.Contains(t, body.text, "Jl. Kemang Raya 21")
			assert.Contains(t, body.text, "TSHIRT")
			assert.Contains(t, body.text, "IDR 598,000")
			assert.Contains(t, body.html, "ORD-000042")
			assert.Contains(t, body.html, "Annisa Putri")
			if tt.wantPhone {
				assert.Contains(t, body.text, "+6281234567890")
				assert.Contains(t, body.html, "+6281234567890")
			} else {
				assert.NotContains(t, body.text, "Phone:")
				assert.NotContains(t, body.html, "Phone:")
			}
		})
	}
}
