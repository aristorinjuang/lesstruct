package orders_test

import (
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/stretchr/testify/assert"
)

func TestMoney_Format(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		amount   int64
		expected string
	}{
		{
			name:     "zero-decimal currency groups the whole amount",
			currency: "IDR",
			amount:   598000,
			expected: "IDR 598,000",
		},
		{
			name:     "zero-decimal currency keeps small amounts whole",
			currency: "IDR",
			amount:   999,
			expected: "IDR 999",
		},
		{
			name:     "zero-decimal currency renders zero without decimals",
			currency: "IDR",
			amount:   0,
			expected: "IDR 0",
		},
		{
			name:     "yen joins the zero-decimal set",
			currency: "JPY",
			amount:   1234567,
			expected: "JPY 1,234,567",
		},
		{
			name:     "minor-unit currency renders two decimals",
			currency: "USD",
			amount:   5980,
			expected: "USD 59.80",
		},
		{
			name:     "minor-unit currency pads sub-unit amounts",
			currency: "USD",
			amount:   5,
			expected: "USD 0.05",
		},
		{
			name:     "minor-unit currency groups the whole part",
			currency: "EUR",
			amount:   123456789,
			expected: "EUR 1,234,567.89",
		},
		{
			name:     "negative zero-decimal amounts carry the sign",
			currency: "IDR",
			amount:   -25000,
			expected: "IDR -25,000",
		},
		{
			name:     "negative minor-unit amounts carry the sign",
			currency: "USD",
			amount:   -5980,
			expected: "USD -59.80",
		},
		{
			name:     "the zero-decimal check ignores code casing",
			currency: "idr",
			amount:   25000,
			expected: "idr 25,000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, domain.NewMoney(tt.amount, tt.currency).Format())
		})
	}
}
