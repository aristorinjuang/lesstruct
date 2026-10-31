package commerce

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCatalogConfigExtra(t *testing.T) {
	tests := []struct {
		name   string
		config config.CatalogConfig
		want   map[string]any
	}{
		{
			name:   "success - display currencies project with codes and rates",
			config: config.CatalogConfig{PostType: "product", Currency: "IDR", DisplayCurrencies: []config.DisplayCurrency{{Code: "USD", Rate: 0.000062}}},
			want: map[string]any{
				"postType": "product",
				"currency": "IDR",
				"displayCurrencies": []any{
					map[string]any{"code": "USD", "rate": 0.000062},
				},
			},
		},
		{
			name:   "success - no display currencies projects empty list",
			config: config.CatalogConfig{PostType: "product", Currency: "IDR"},
			want: map[string]any{
				"postType":          "product",
				"currency":          "IDR",
				"displayCurrencies": []any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, catalogConfigExtra(tt.config))
		})
	}
}

func TestManualPayeeAdopted(t *testing.T) {
	full := config.ManualPayee{Bank: "BCA", Account: "1234567890", Name: "PT Demo"}

	tests := []struct {
		name    string
		methods []string
		payee   config.ManualPayee
		want    bool
	}{
		{
			name:    "success - complete block with manual offered adopts",
			methods: []string{"manual"},
			payee:   full,
			want:    true,
		},
		{
			name:    "success - complete block with manual among gateways adopts",
			methods: []string{"midtrans", "manual"},
			payee:   full,
			want:    true,
		},
		{
			name:    "complete block without manual offered stays silent",
			methods: []string{"midtrans"},
			payee:   full,
			want:    false,
		},
		{
			name:    "no methods offered stays silent",
			methods: nil,
			payee:   full,
			want:    false,
		},
		{
			name:    "manual offered without a block stays silent",
			methods: []string{"manual"},
			payee:   config.ManualPayee{},
			want:    false,
		},
		{
			name:    "manual offered with a partial block stays silent",
			methods: []string{"manual"},
			payee:   config.ManualPayee{Bank: "BCA"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, manualPayeeAdopted(tt.methods, tt.payee))
		})
	}
}
