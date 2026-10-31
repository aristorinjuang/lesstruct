package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
)

// mapProductToDomain converts the product row plus its variant rows into the
// domain shape, decoding the options JSON payloads.
func mapProductToDomain(row productRow, variants []variantRow, contentID int) (*catalogdomain.Product, error) {
	options := map[string][]string{}
	if err := json.Unmarshal([]byte(row.Options), &options); err != nil {
		return nil, fmt.Errorf("failed to decode catalog options: %w", err)
	}

	product := &catalogdomain.Product{
		ContentID:      contentID,
		Price:          row.Price,
		SKU:            row.SKU,
		Stock:          row.Stock,
		Options:        options,
		UpdatedAt:      row.UpdatedAt,
		TrackInventory: row.TrackInventory,
	}
	if row.SalePrice.Valid {
		v := row.SalePrice.Int64
		product.SalePrice = &v
	}
	if row.WeightGrams.Valid {
		v := int(row.WeightGrams.Int64)
		product.WeightGrams = &v
	}
	if row.LowStockThreshold.Valid {
		v := int(row.LowStockThreshold.Int64)
		product.LowStockThreshold = &v
	}

	product.Variants = make([]catalogdomain.Variant, 0, len(variants))
	for _, v := range variants {
		variantOptions := map[string]string{}
		if err := json.Unmarshal([]byte(v.Options), &variantOptions); err != nil {
			return nil, fmt.Errorf("failed to decode catalog variant options: %w", err)
		}
		variant := catalogdomain.Variant{
			ID:               v.ID,
			ProductContentID: contentID,
			Options:          variantOptions,
			SKU:              v.SKU,
			Stock:            v.Stock,
			ImageURL:         v.ImageURL,
		}
		if v.Price.Valid {
			price := v.Price.Int64
			variant.Price = &price
		}
		if v.WeightGrams.Valid {
			w := int(v.WeightGrams.Int64)
			variant.WeightGrams = &w
		}
		product.Variants = append(product.Variants, variant)
	}

	return product, nil
}

// nullInt64 converts an optional int64 for nullable-column scans.
func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// nullInt64FromInt converts an optional int for nullable-column scans.
func nullInt64FromInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}
