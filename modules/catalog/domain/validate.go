package catalog

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
)

// validateOptions requires at least one axis, non-blank axis names, and
// non-blank values on every axis.
func validateOptions(options map[string][]string) error {
	if len(options) == 0 {
		return ErrInvalidOptions
	}
	for name, values := range options {
		if strings.TrimSpace(name) == "" {
			return ErrInvalidOptions
		}
		if len(values) == 0 {
			return fmt.Errorf("%w: axis %q has no values", ErrInvalidOptions, name)
		}
		for _, v := range values {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%w: axis %q has a blank value", ErrInvalidOptions, name)
			}
		}
	}
	return nil
}

// validateVariants checks the matrix against the axes: size cap, axis match
// (same keys, values within the axis lists), per-variant stock/SKU bounds,
// and SKU uniqueness within the product (empty SKUs are exempt — the admin
// generates them lazily).
func validateVariants(axes map[string][]string, variants []VariantInput) error {
	if len(variants) > MaxVariants {
		return ErrTooManyVariants
	}

	allowed := make(map[string]map[string]bool, len(axes))
	for name, values := range axes {
		allowed[name] = make(map[string]bool, len(values))
		for _, v := range values {
			allowed[name][v] = true
		}
	}

	seenSKUs := make(map[string]bool, len(variants))
	for i, variant := range variants {
		if len(variant.Options) != len(axes) {
			return fmt.Errorf("%w: variant %d does not define every axis", ErrInvalidVariant, i+1)
		}
		for name, value := range variant.Options {
			values, ok := allowed[name]
			if !ok {
				return fmt.Errorf("%w: variant %d has unknown axis %q", ErrInvalidVariant, i+1, name)
			}
			if !values[value] {
				return fmt.Errorf("%w: variant %d has value %q outside axis %q", ErrInvalidVariant, i+1, value, name)
			}
		}
		if variant.Price != nil && *variant.Price < 0 {
			return ErrInvalidPrice
		}
		if variant.Stock < 0 {
			return ErrInvalidStock
		}
		if utf8.RuneCountInString(variant.SKU) > MaxSKUTrackRunes {
			return ErrInvalidSKU
		}
		if variant.SKU != "" {
			if seenSKUs[variant.SKU] {
				return fmt.Errorf("%w: %s", ErrVariantSKUDuplicate, variant.SKU)
			}
			seenSKUs[variant.SKU] = true
		}
		if err := validateImageURL(variant.ImageURL); err != nil {
			return fmt.Errorf("variant %d: %w", i+1, err)
		}
	}
	return nil
}

// validateImageURL accepts empty, http(s) URLs, and root-relative media
// paths, using the shared gallery rule so the two surfaces cannot drift.
// Surrounding whitespace is trimmed before the check.
func validateImageURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	if err := customfield.ValidateGalleryURL(trimmed); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidImageURL, rawURL)
	}
	return nil
}

// UpsertProductInput carries the admin-editable commerce data for one
// product. The variant matrix is replaced wholesale on every upsert.
type UpsertProductInput struct {
	Price             int64
	SalePrice         *int64
	SKU               string
	TrackInventory    bool
	Stock             int
	WeightGrams       *int
	LowStockThreshold *int
	Options           map[string][]string
	Variants          []VariantInput
}

// VariantInput is one matrix entry in an upsert.
type VariantInput struct {
	Options     map[string]string
	SKU         string
	Price       *int64
	Stock       int
	WeightGrams *int
	ImageURL    string
}

// ValidateProductInput checks one upsert: prices, SKU bounds, stock, the
// variant-axes shape, and the whole variant matrix (axis match, duplicate
// SKUs, size cap).
func ValidateProductInput(input UpsertProductInput) error {
	if input.Price < 0 {
		return ErrInvalidPrice
	}
	if input.SalePrice != nil && (*input.SalePrice < 0 || *input.SalePrice > input.Price) {
		return ErrInvalidSalePrice
	}
	if utf8.RuneCountInString(input.SKU) > MaxSKUTrackRunes {
		return ErrInvalidSKU
	}
	if input.Stock < 0 {
		return ErrInvalidStock
	}
	if err := validateOptions(input.Options); err != nil {
		return err
	}
	return validateVariants(input.Options, input.Variants)
}
