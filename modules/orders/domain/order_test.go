package orders_test

import (
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/stretchr/testify/assert"
)

func TestOrder_DisplayNumber(t *testing.T) {
	tests := []struct {
		name     string
		id       int64
		expected string
	}{
		{
			name:     "low IDs zero-pad to the width",
			id:       7,
			expected: "ORD-000007",
		},
		{
			name:     "mid IDs pad fully",
			id:       42,
			expected: "ORD-000042",
		},
		{
			name:     "IDs beyond the width grow naturally",
			id:       1000000,
			expected: "ORD-1000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &domain.Order{ID: tt.id}
			assert.Equal(t, tt.expected, order.DisplayNumber())
		})
	}
}

func TestParseOrderNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected int64
		wantOK   bool
	}{
		{
			name:     "the rendered shape parses",
			number:   "ORD-000042",
			expected: 42,
			wantOK:   true,
		},
		{
			name:     "numbers beyond the width parse",
			number:   "ORD-1000000",
			expected: 1000000,
			wantOK:   true,
		},
		{
			name:   "missing prefix rejects",
			number: "000042",
		},
		{
			name:   "wrong prefix rejects",
			number: "INV-000042",
		},
		{
			name:   "prefix case is exact",
			number: "ord-000042",
		},
		{
			name:   "non-numeric digits reject",
			number: "ORD-0000xy",
		},
		{
			name:   "zero rejects",
			number: "ORD-000000",
		},
		{
			name:   "negative rejects",
			number: "ORD--1",
		},
		{
			name:   "empty rejects",
			number: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := domain.ParseOrderNumber(tt.number)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.expected, id)
		})
	}
}

func TestOrderItem_LineTotal(t *testing.T) {
	tests := []struct {
		name     string
		item     domain.OrderItem
		expected int64
	}{
		{
			name:     "quantity times unit price",
			item:     domain.OrderItem{Quantity: 3, UnitPrice: 299000},
			expected: 897000,
		},
		{
			name:     "zero quantity totals zero",
			item:     domain.OrderItem{Quantity: 0, UnitPrice: 299000},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.item.LineTotal())
		})
	}
}

func TestSentinelMessages(t *testing.T) {
	assert.Equal(t, "order not found", domain.ErrOrderNotFound.Error())
	assert.Equal(t, "order transition is not allowed", domain.ErrInvalidTransition.Error())
	assert.Equal(t, "status is invalid", domain.ErrInvalidStatus.Error())
	assert.Equal(t, "email is required and must be valid", domain.ErrInvalidEmail.Error())
	assert.Equal(t, "name is required", domain.ErrInvalidName.Error())
	assert.Equal(t, "shipping address is incomplete", domain.ErrInvalidAddress.Error())
	assert.Equal(t, "cart is empty", domain.ErrEmptyCart.Error())
	assert.Equal(t, "cart is gone", domain.ErrCartGone.Error())
	assert.Equal(t, "item is out of stock", domain.ErrOutOfStock.Error())
	assert.Equal(t, "cart line is no longer available", domain.ErrLineUnavailable.Error())
	assert.Equal(t, "shipping amount must be non-negative", domain.ErrInvalidShipping.Error())
}

func TestOrder_TotalMinor(t *testing.T) {
	tests := []struct {
		name     string
		subtotal int64
		shipping int64
		expected int64
	}{
		{
			name:     "a fresh checkout totals its subtotal",
			subtotal: 598000,
			shipping: 0,
			expected: 598000,
		},
		{
			name:     "the shipping line joins the total",
			subtotal: 598000,
			shipping: 15000,
			expected: 613000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &domain.Order{Subtotal: tt.subtotal, ShippingMinor: tt.shipping}
			assert.Equal(t, tt.expected, order.TotalMinor())
		})
	}
}
