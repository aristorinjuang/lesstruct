package cart_test

import (
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	"github.com/stretchr/testify/assert"
)

func TestCart_Subtotal(t *testing.T) {
	tests := []struct {
		name     string
		cart     domain.Cart
		expected int64
	}{
		{
			name:     "empty cart totals zero",
			cart:     domain.Cart{},
			expected: 0,
		},
		{
			name: "available lines sum their extended prices",
			cart: domain.Cart{Lines: []domain.CartLine{
				{Quantity: 2, UnitPrice: 299000, Available: true},
				{Quantity: 1, UnitPrice: 249000, Available: true},
			}},
			expected: 847000,
		},
		{
			name: "unavailable lines are excluded",
			cart: domain.Cart{Lines: []domain.CartLine{
				{Quantity: 2, UnitPrice: 299000, Available: true},
				{Quantity: 9, UnitPrice: 1000000, Available: false},
			}},
			expected: 598000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cart.Subtotal())
		})
	}
}

func TestCart_ItemCount(t *testing.T) {
	tests := []struct {
		name     string
		cart     domain.Cart
		expected int
	}{
		{
			name:     "empty cart counts zero",
			cart:     domain.Cart{},
			expected: 0,
		},
		{
			name: "available quantities sum",
			cart: domain.Cart{Lines: []domain.CartLine{
				{Quantity: 2, Available: true},
				{Quantity: 3, Available: true},
			}},
			expected: 5,
		},
		{
			name: "unavailable lines are excluded",
			cart: domain.Cart{Lines: []domain.CartLine{
				{Quantity: 2, Available: true},
				{Quantity: 9, Available: false},
			}},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cart.ItemCount())
		})
	}
}

func TestSentinelMessages(t *testing.T) {
	assert.Equal(t, "cart not found", domain.ErrCartNotFound.Error())
	assert.Equal(t, "cart line not found", domain.ErrLineNotFound.Error())
	assert.Equal(t, "quantity must be between 1 and 99", domain.ErrInvalidQuantity.Error())
	assert.Equal(t, "product not found", domain.ErrUnknownProduct.Error())
	assert.Equal(t, "variant not found", domain.ErrUnknownVariant.Error())
	assert.Equal(t, "item is out of stock", domain.ErrOutOfStock.Error())
}

func TestCartLine_LineTotal(t *testing.T) {
	tests := []struct {
		name     string
		line     domain.CartLine
		expected int64
	}{
		{
			name:     "quantity times unit price",
			line:     domain.CartLine{Quantity: 3, UnitPrice: 279000},
			expected: 837000,
		},
		{
			name:     "zero quantity totals zero",
			line:     domain.CartLine{Quantity: 0, UnitPrice: 279000},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.line.LineTotal())
		})
	}
}
