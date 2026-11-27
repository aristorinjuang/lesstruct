package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePaymentProofAlertEmailBody(t *testing.T) {
	tests := []struct {
		name          string
		orderNumber   string
		customerEmail string
		method        string
		reference     string
		amount        string
	}{
		{
			name:          "the notice names the order and the reference",
			orderNumber:   "ORD-000042",
			customerEmail: "shopper@example.com",
			method:        "manual",
			reference:     "BR-12345",
			amount:        "IDR 522,500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := generatePaymentProofAlertEmailBody(tt.orderNumber, tt.customerEmail, tt.method, tt.reference, tt.amount)

			assert.Contains(t, body.text, tt.orderNumber)
			assert.Contains(t, body.text, tt.customerEmail)
			assert.Contains(t, body.text, tt.method)
			assert.Contains(t, body.text, tt.reference)
			assert.Contains(t, body.text, tt.amount)
			assert.Contains(t, body.html, tt.orderNumber)
			assert.Contains(t, body.html, tt.reference)
		})
	}
}

func TestGeneratePaymentSettledAlertEmailBody(t *testing.T) {
	tests := []struct {
		name          string
		orderNumber   string
		customerEmail string
		method        string
		amount        string
	}{
		{
			name:          "the notice names the order and the gateway",
			orderNumber:   "ORD-000042",
			customerEmail: "shopper@example.com",
			method:        "midtrans",
			amount:        "IDR 522,500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := generatePaymentSettledAlertEmailBody(tt.orderNumber, tt.customerEmail, tt.method, tt.amount)

			assert.Contains(t, body.text, tt.orderNumber)
			assert.Contains(t, body.text, tt.customerEmail)
			assert.Contains(t, body.text, tt.method)
			assert.Contains(t, body.text, tt.amount)
			assert.Contains(t, body.html, tt.orderNumber)
			assert.Contains(t, body.html, tt.method)
		})
	}
}
