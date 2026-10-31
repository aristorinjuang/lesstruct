package cart

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// randSource is the entropy source for cart tokens. It defaults to
// crypto/rand; tests may swap it for a failing reader to cover the mint
// failure branch.
var (
	randSource io.Reader = rand.Reader
)

// hashToken renders the storage key for a presented token: hex of its
// SHA-256 digest. Raw tokens never touch the database (the API-key
// service's hashSecret is the precedent).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newToken mints an opaque cart token with TokenBytes of entropy, encoded
// base64url without padding for header- and JSON-safe transport.
func newToken() (string, error) {
	entropy := make([]byte, TokenBytes)
	if _, err := io.ReadFull(randSource, entropy); err != nil {
		return "", fmt.Errorf("failed to mint cart token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(entropy), nil
}

// validateQuantity enforces the per-line quantity range.
func validateQuantity(quantity int) error {
	if quantity < MinQuantity || quantity > MaxQuantityPerLine {
		return ErrInvalidQuantity
	}
	return nil
}

// findStoredLine locates a stored line by (product, variant) key, treating
// nil and non-nil variant IDs as distinct keys.
func findStoredLine(stored *StoredCart, productContentID int, variantID *int64) *StoredLine {
	for i := range stored.Lines {
		line := &stored.Lines[i]
		if line.ProductContentID != productContentID {
			continue
		}
		if (line.VariantID == nil) != (variantID == nil) {
			continue
		}
		if variantID != nil && *line.VariantID != *variantID {
			continue
		}
		return line
	}
	return nil
}

// findStoredLineByID locates a stored line by its ID.
func findStoredLineByID(stored *StoredCart, itemID int64) *StoredLine {
	for i := range stored.Lines {
		if stored.Lines[i].ID == itemID {
			return &stored.Lines[i]
		}
	}
	return nil
}

// isUnknown reports the catalog lookup misses the read path tolerates.
func isUnknown(err error) bool {
	return errors.Is(err, ErrUnknownProduct) || errors.Is(err, ErrUnknownVariant)
}

// Service is the cart domain service: token-keyed carts with live catalog
// pricing over the persistence and catalog seams.
type Service struct {
	store   CartStore
	catalog CatalogSource
}

// GetCart resolves the token to a cart with live-priced lines, or
// ErrCartNotFound for unknown, empty, or malformed tokens.
func (s *Service) GetCart(ctx context.Context, token string) (*Cart, error) {
	stored, err := s.findCart(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.resolve(ctx, stored)
}

// CreateCart mints a token, persists an empty cart under its hash, and
// returns both. The raw token is shown exactly once — callers must present
// it on every subsequent call.
func (s *Service) CreateCart(ctx context.Context) (string, *Cart, error) {
	token, err := newToken()
	if err != nil {
		return "", nil, err
	}
	id, err := s.store.CreateCart(ctx, hashToken(token))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create cart: %w", err)
	}
	return token, &Cart{ID: id}, nil
}

// AddItem resolves the sellable line, refuses out-of-stock demand, then
// merges the quantity into the cart (same (product, variant) lines add up,
// capped at MaxQuantityPerLine). It returns the refreshed cart.
func (s *Service) AddItem(ctx context.Context, token string, productContentID int, variantID *int64, quantity int) (*Cart, error) {
	if err := validateQuantity(quantity); err != nil {
		return nil, err
	}
	stored, err := s.findCart(ctx, token)
	if err != nil {
		return nil, err
	}
	item, err := s.resolveLine(ctx, productContentID, variantID)
	if err != nil {
		return nil, err
	}
	if !item.InStock {
		return nil, ErrOutOfStock
	}
	if existing := findStoredLine(stored, productContentID, variantID); existing != nil {
		if existing.Quantity+quantity > MaxQuantityPerLine {
			return nil, ErrInvalidQuantity
		}
		if err := s.store.SetLineQuantity(ctx, stored.ID, existing.ID, existing.Quantity+quantity); err != nil {
			return nil, fmt.Errorf("failed to merge cart line: %w", err)
		}
	} else {
		if _, err := s.store.AddLine(ctx, stored.ID, productContentID, variantID, quantity); err != nil {
			return nil, fmt.Errorf("failed to add cart line: %w", err)
		}
	}
	return s.GetCart(ctx, token)
}

// UpdateItem replaces a line's quantity within the valid range, re-checking
// the catalog posture: unknown products or variants and out-of-stock
// demand fail the update.
func (s *Service) UpdateItem(ctx context.Context, token string, itemID int64, quantity int) (*Cart, error) {
	if err := validateQuantity(quantity); err != nil {
		return nil, err
	}
	stored, err := s.findCart(ctx, token)
	if err != nil {
		return nil, err
	}
	line := findStoredLineByID(stored, itemID)
	if line == nil {
		return nil, ErrLineNotFound
	}
	item, err := s.resolveLine(ctx, line.ProductContentID, line.VariantID)
	if err != nil {
		return nil, err
	}
	// Out-of-stock demand is refused only when it grows: reducing a line
	// (or holding it steady) is always allowed, so shoppers can correct a
	// cart after stock ran out.
	if !item.InStock && quantity > line.Quantity {
		return nil, ErrOutOfStock
	}
	if err := s.store.SetLineQuantity(ctx, stored.ID, itemID, quantity); err != nil {
		return nil, fmt.Errorf("failed to update cart line: %w", err)
	}
	return s.GetCart(ctx, token)
}

// RemoveItem deletes one line of the cart; unknown lines are a no-op. It
// returns the refreshed cart.
func (s *Service) RemoveItem(ctx context.Context, token string, itemID int64) (*Cart, error) {
	stored, err := s.findCart(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := s.store.RemoveLine(ctx, stored.ID, itemID); err != nil {
		return nil, fmt.Errorf("failed to remove cart line: %w", err)
	}
	return s.GetCart(ctx, token)
}

// DeleteCart removes the cart and every line in it; unknown tokens are
// refused with ErrCartNotFound. The store-level delete is also claim-atomic:
// a cart that vanishes between the read and the delete (a parallel checkout
// won the claim) reports ErrCartNotFound, so a loser can never proceed as if
// it still held the cart.
func (s *Service) DeleteCart(ctx context.Context, token string) error {
	stored, err := s.findCart(ctx, token)
	if err != nil {
		return err
	}
	if err := s.store.DeleteCart(ctx, stored.ID); err != nil {
		return fmt.Errorf("failed to delete cart: %w", err)
	}
	return nil
}

// findCart resolves a presented token to its stored cart. Empty tokens fail
// closed without touching the store.
func (s *Service) findCart(ctx context.Context, token string) (*StoredCart, error) {
	if token == "" {
		return nil, ErrCartNotFound
	}
	stored, err := s.store.FindCart(ctx, hashToken(token))
	if err != nil {
		return nil, err
	}
	return stored, nil
}

// resolveLine validates the (product, variant) key shape before delegating
// to the catalog seam: non-positive content IDs and variants are unknown by
// construction.
func (s *Service) resolveLine(ctx context.Context, productContentID int, variantID *int64) (CatalogItem, error) {
	if productContentID <= 0 {
		return CatalogItem{}, ErrUnknownProduct
	}
	if variantID != nil && *variantID <= 0 {
		return CatalogItem{}, ErrUnknownVariant
	}
	return s.catalog.ResolveLine(ctx, productContentID, variantID)
}

// resolve enriches stored lines with live catalog projections. Lines whose
// product or variant no longer resolves stay in the cart marked
// unavailable (excluded from totals) so the cart stays readable; any other
// catalog failure aborts the read.
func (s *Service) resolve(ctx context.Context, stored *StoredCart) (*Cart, error) {
	cart := &Cart{ID: stored.ID}
	cart.Lines = make([]CartLine, 0, len(stored.Lines))
	for _, storedLine := range stored.Lines {
		line := CartLine{
			ID:               storedLine.ID,
			ProductContentID: storedLine.ProductContentID,
			VariantID:        storedLine.VariantID,
			Quantity:         storedLine.Quantity,
		}
		item, err := s.resolveLine(ctx, storedLine.ProductContentID, storedLine.VariantID)
		if err != nil {
			if isUnknown(err) {
				cart.Lines = append(cart.Lines, line)
				continue
			}
			return nil, fmt.Errorf("failed to resolve cart line: %w", err)
		}
		line.SKU = item.SKU
		line.Title = item.Title
		line.Slug = item.Slug
		line.ImageURL = item.ImageURL
		line.UnitPrice = item.UnitPrice
		line.InStock = item.InStock
		line.Available = true
		cart.Lines = append(cart.Lines, line)
	}
	return cart, nil
}

// NewService builds the service over the persistence and catalog seams.
func NewService(store CartStore, catalog CatalogSource) *Service {
	return &Service{store: store, catalog: catalog}
}
