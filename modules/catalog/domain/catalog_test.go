package catalog_test

import (
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidateProductInput(t *testing.T) {
	options := map[string][]string{"size": {"S", "M"}}
	validVariant := domain.VariantInput{Options: map[string]string{"size": "M"}, SKU: "TSHIRT-M"}

	tests := []struct {
		name    string
		input   domain.UpsertProductInput
		wantErr error
	}{
		{
			name: "valid input with variants",
			input: domain.UpsertProductInput{
				Price: 299000, Stock: 5, TrackInventory: true,
				Options: options, Variants: []domain.VariantInput{validVariant},
			},
			wantErr: nil,
		},
		{
			name:    "negative price rejected",
			input:   domain.UpsertProductInput{Price: -1},
			wantErr: domain.ErrInvalidPrice,
		},
		{
			name: "negative sale price rejected",
			input: domain.UpsertProductInput{
				Price: 100, SalePrice: new(int64(-5)),
			},
			wantErr: domain.ErrInvalidSalePrice,
		},
		{
			name: "sale price above price rejected",
			input: domain.UpsertProductInput{
				Price: 100, SalePrice: new(int64(200)),
			},
			wantErr: domain.ErrInvalidSalePrice,
		},
		{
			name: "oversized SKU rejected",
			input: domain.UpsertProductInput{
				Price: 100, SKU: string(make([]byte, 101)),
			},
			wantErr: domain.ErrInvalidSKU,
		},
		{
			name:    "negative stock rejected",
			input:   domain.UpsertProductInput{Price: 100, Stock: -1},
			wantErr: domain.ErrInvalidStock,
		},
		{
			name:    "no options rejected",
			input:   domain.UpsertProductInput{Price: 100},
			wantErr: domain.ErrInvalidOptions,
		},
		{
			name: "axis without values rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: map[string][]string{"size": {}},
			},
			wantErr: domain.ErrInvalidOptions,
		},
		{
			name: "blank axis name rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: map[string][]string{" ": {"M"}},
			},
			wantErr: domain.ErrInvalidOptions,
		},
		{
			name: "blank axis value rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: map[string][]string{"size": {"M", "  "}},
			},
			wantErr: domain.ErrInvalidOptions,
		},
		{
			name: "variant with unknown axis rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"color": "Red"}}},
			},
			wantErr: domain.ErrInvalidVariant,
		},
		{
			name: "variant with out-of-axis value rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "XXL"}}},
			},
			wantErr: domain.ErrInvalidVariant,
		},
		{
			name: "variant missing an axis rejected",
			input: domain.UpsertProductInput{
				Price:    100,
				Options:  map[string][]string{"size": {"S"}, "color": {"Red"}},
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}}},
			},
			wantErr: domain.ErrInvalidVariant,
		},
		{
			name: "duplicate variant SKUs rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{
					{Options: map[string]string{"size": "S"}, SKU: "DUP"},
					{Options: map[string]string{"size": "M"}, SKU: "DUP"},
				},
			},
			wantErr: domain.ErrVariantSKUDuplicate,
		},
		{
			name: "empty variant SKUs exempt from uniqueness",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{
					{Options: map[string]string{"size": "S"}},
					{Options: map[string]string{"size": "M"}},
				},
			},
			wantErr: nil,
		},
		{
			name: "negative variant price rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, Price: new(int64(-1))}},
			},
			wantErr: domain.ErrInvalidPrice,
		},
		{
			name: "negative variant stock rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, Stock: -1}},
			},
			wantErr: domain.ErrInvalidStock,
		},
		{
			name: "oversized variant SKU rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, SKU: string(make([]byte, 101))}},
			},
			wantErr: domain.ErrInvalidSKU,
		},
		{
			name: "variant matrix over the cap rejected",
			input: domain.UpsertProductInput{
				Price:    100,
				Options:  map[string][]string{"size": manyValues(domain.MaxVariants + 1)},
				Variants: manyVariantInputs(domain.MaxVariants + 1),
			},
			wantErr: domain.ErrTooManyVariants,
		},
		{
			name: "variant javascript imageUrl rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, ImageURL: "javascript:alert(1)"}},
			},
			wantErr: domain.ErrInvalidImageURL,
		},
		{
			name: "variant data imageUrl rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, ImageURL: "data:image/png;base64,AAA"}},
			},
			wantErr: domain.ErrInvalidImageURL,
		},
		{
			name: "variant ftp imageUrl rejected",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, ImageURL: "ftp://example.com/a.webp"}},
			},
			wantErr: domain.ErrInvalidImageURL,
		},
		{
			name: "variant https imageUrl accepted",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, ImageURL: "https://example.com/a.webp"}},
			},
			wantErr: nil,
		},
		{
			name: "variant root-relative imageUrl accepted",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, ImageURL: "/uploads/a.webp"}},
			},
			wantErr: nil,
		},
		{
			name: "variant empty imageUrl accepted",
			input: domain.UpsertProductInput{
				Price: 100, Options: options,
				Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, ImageURL: ""}},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateProductInput(tt.input)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestProduct_StockPosture(t *testing.T) {
	threshold := 3
	thresholdFive := 5
	tracked := domain.Product{TrackInventory: true, Stock: 5, LowStockThreshold: &threshold}

	tests := []struct {
		name         string
		product      domain.Product
		wantInStock  bool
		wantLowStock bool
	}{
		{
			name:         "tracked with stock is in stock and not low",
			product:      tracked,
			wantInStock:  true,
			wantLowStock: false,
		},
		{
			name:         "tracked at threshold is low",
			product:      domain.Product{TrackInventory: true, Stock: 3, LowStockThreshold: &threshold},
			wantInStock:  true,
			wantLowStock: true,
		},
		{
			name:         "tracked out of stock",
			product:      domain.Product{TrackInventory: true, Stock: 0, LowStockThreshold: &threshold},
			wantInStock:  false,
			wantLowStock: true,
		},
		{
			name:         "untracked is always in stock and never low",
			product:      domain.Product{TrackInventory: false, Stock: 0, LowStockThreshold: &threshold},
			wantInStock:  true,
			wantLowStock: false,
		},
		{
			name:         "tracked without threshold is never low",
			product:      domain.Product{TrackInventory: true, Stock: 0},
			wantInStock:  false,
			wantLowStock: false,
		},
		{
			name: "variant partial stock is in stock and low",
			product: domain.Product{
				TrackInventory:    true,
				Stock:             0,
				LowStockThreshold: &thresholdFive,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []domain.Variant{{Stock: 0}, {Stock: 3}},
			},
			wantInStock:  true,
			wantLowStock: true,
		},
		{
			name: "variant empty is out of stock and never low",
			product: domain.Product{
				TrackInventory:    true,
				Stock:             9,
				LowStockThreshold: &thresholdFive,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []domain.Variant{{Stock: 0}, {Stock: 0}},
			},
			wantInStock:  false,
			wantLowStock: false,
		},
		{
			name: "variant healthy stock is in stock and not low",
			product: domain.Product{
				TrackInventory:    true,
				Stock:             0,
				LowStockThreshold: &threshold,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []domain.Variant{{Stock: 5}, {Stock: 5}},
			},
			wantInStock:  true,
			wantLowStock: false,
		},
		{
			name: "untracked with variants is always in stock and never low",
			product: domain.Product{
				TrackInventory:    false,
				Stock:             0,
				LowStockThreshold: &threshold,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []domain.Variant{{Stock: 0}, {Stock: 0}},
			},
			wantInStock:  true,
			wantLowStock: false,
		},
		{
			name: "variant without threshold is never low",
			product: domain.Product{
				TrackInventory: true,
				Stock:          0,
				Options:        map[string][]string{"size": {"S"}},
				Variants:       []domain.Variant{{Stock: 2}},
			},
			wantInStock:  true,
			wantLowStock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantInStock, tt.product.InStock())
			assert.Equal(t, tt.wantLowStock, tt.product.LowStock())
		})
	}
}

func TestProduct_EffectivePrice(t *testing.T) {
	price := int64(299000)
	sale := int64(249000)
	variantOverride := int64(199000)

	tests := []struct {
		name      string
		product   domain.Product
		variant   *domain.Variant
		wantPrice int64
	}{
		{
			name:      "product price when no sale and no override",
			product:   domain.Product{Price: price},
			variant:   nil,
			wantPrice: price,
		},
		{
			name:      "sale price beats product price",
			product:   domain.Product{Price: price, SalePrice: &sale},
			variant:   nil,
			wantPrice: sale,
		},
		{
			name:      "variant override beats sale price",
			product:   domain.Product{Price: price, SalePrice: &sale},
			variant:   &domain.Variant{Price: &variantOverride},
			wantPrice: variantOverride,
		},
		{
			name:      "variant without override inherits product effective price",
			product:   domain.Product{Price: price, SalePrice: &sale},
			variant:   &domain.Variant{},
			wantPrice: sale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantPrice, tt.product.EffectivePrice(tt.variant))
		})
	}
}

func TestProduct_VariantInStock(t *testing.T) {
	tracked := domain.Product{TrackInventory: true}
	untracked := domain.Product{TrackInventory: false}

	tests := []struct {
		name      string
		product   domain.Product
		variant   domain.Variant
		wantStock bool
	}{
		{"tracked variant with stock", tracked, domain.Variant{Stock: 2}, true},
		{"tracked variant without stock", tracked, domain.Variant{Stock: 0}, false},
		{"untracked variant always in stock", untracked, domain.Variant{Stock: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantStock, tt.product.VariantInStock(&tt.variant))
		})
	}
}

func manyValues(n int) []string {
	values := make([]string, 0, n)
	for i := range n {
		values = append(values, "v"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	return values
}

func manyVariantInputs(n int) []domain.VariantInput {
	inputs := make([]domain.VariantInput, 0, n)
	for i := range n {
		inputs = append(inputs, domain.VariantInput{Options: map[string]string{"size": manyValues(n)[i]}})
	}
	return inputs
}
