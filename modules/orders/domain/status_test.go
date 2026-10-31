package orders_test

import (
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/stretchr/testify/assert"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.Status
		to       domain.Status
		expected bool
	}{
		{
			name:     "pending is paid",
			from:     domain.StatusPending,
			to:       domain.StatusPaid,
			expected: true,
		},
		{
			name:     "pending is expired",
			from:     domain.StatusPending,
			to:       domain.StatusExpired,
			expected: true,
		},
		{
			name:     "pending is not shipped",
			from:     domain.StatusPending,
			to:       domain.StatusShipped,
			expected: false,
		},
		{
			name:     "paid is packed",
			from:     domain.StatusPaid,
			to:       domain.StatusPacked,
			expected: true,
		},
		{
			name:     "paid is refunded",
			from:     domain.StatusPaid,
			to:       domain.StatusRefunded,
			expected: true,
		},
		{
			name:     "paid is not delivered",
			from:     domain.StatusPaid,
			to:       domain.StatusDelivered,
			expected: false,
		},
		{
			name:     "packed is shipped",
			from:     domain.StatusPacked,
			to:       domain.StatusShipped,
			expected: true,
		},
		{
			name:     "shipped is delivered",
			from:     domain.StatusShipped,
			to:       domain.StatusDelivered,
			expected: true,
		},
		{
			name:     "shipped is refunded",
			from:     domain.StatusShipped,
			to:       domain.StatusRefunded,
			expected: true,
		},
		{
			name:     "shipped is not packed backwards",
			from:     domain.StatusShipped,
			to:       domain.StatusPacked,
			expected: false,
		},
		{
			name:     "delivered is refunded",
			from:     domain.StatusDelivered,
			to:       domain.StatusRefunded,
			expected: true,
		},
		{
			name:     "delivered is terminal otherwise",
			from:     domain.StatusDelivered,
			to:       domain.StatusDelivered,
			expected: false,
		},
		{
			name:     "cancelled is terminal",
			from:     domain.StatusCancelled,
			to:       domain.StatusPaid,
			expected: false,
		},
		{
			name:     "refunded is terminal",
			from:     domain.StatusRefunded,
			to:       domain.StatusPending,
			expected: false,
		},
		{
			name:     "expired is terminal",
			from:     domain.StatusExpired,
			to:       domain.StatusPaid,
			expected: false,
		},
		{
			name:     "unknown states never transition",
			from:     domain.Status("superpaid"),
			to:       domain.StatusPaid,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, domain.CanTransition(tt.from, tt.to))
		})
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantState domain.Status
		wantOK    bool
	}{
		{
			name:      "pending",
			raw:       "pending",
			wantState: domain.StatusPending,
			wantOK:    true,
		},
		{
			name:      "paid",
			raw:       "paid",
			wantState: domain.StatusPaid,
			wantOK:    true,
		},
		{
			name:      "packed",
			raw:       "packed",
			wantState: domain.StatusPacked,
			wantOK:    true,
		},
		{
			name:      "shipped",
			raw:       "shipped",
			wantState: domain.StatusShipped,
			wantOK:    true,
		},
		{
			name:      "delivered",
			raw:       "delivered",
			wantState: domain.StatusDelivered,
			wantOK:    true,
		},
		{
			name:      "cancelled",
			raw:       "cancelled",
			wantState: domain.StatusCancelled,
			wantOK:    true,
		},
		{
			name:      "refunded",
			raw:       "refunded",
			wantState: domain.StatusRefunded,
			wantOK:    true,
		},
		{
			name:      "expired",
			raw:       "expired",
			wantState: domain.StatusExpired,
			wantOK:    true,
		},
		{
			name:      "unknown vocabulary",
			raw:       "superpaid",
			wantState: "",
			wantOK:    false,
		},
		{
			name:      "empty",
			raw:       "",
			wantState: "",
			wantOK:    false,
		},
		{
			name:      "case sensitive",
			raw:       "PAID",
			wantState: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, ok := domain.ParseStatus(tt.raw)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantState, state)
		})
	}
}
