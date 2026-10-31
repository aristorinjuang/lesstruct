// Package api hosts the catalog module's HTTP surfaces: the admin
// product-commerce endpoints (protected realm) and the public product
// payload (anonymous realm). Both write through kernel/httpx — this is a
// new surface with no legacy envelope contract.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/capability"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
)

// parseContentID extracts and validates the {contentID} path value.
func parseContentID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("contentID"))
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return 0, false
	}
	return id, true
}

// mapServiceError maps domain sentinels to the agent-style envelope codes
// (this is a new surface; kernel/httpx is the contract).
func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, catalogdomain.ErrProductNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Product not found", nil)
	case errors.Is(err, catalogdomain.ErrInvalidPrice),
		errors.Is(err, catalogdomain.ErrInvalidSalePrice),
		errors.Is(err, catalogdomain.ErrInvalidSKU),
		errors.Is(err, catalogdomain.ErrInvalidOptions),
		errors.Is(err, catalogdomain.ErrInvalidVariant),
		errors.Is(err, catalogdomain.ErrVariantSKUDuplicate),
		errors.Is(err, catalogdomain.ErrInvalidStock),
		errors.Is(err, catalogdomain.ErrInvalidImageURL),
		errors.Is(err, catalogdomain.ErrTooManyVariants):
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// adminVariantView is the admin wire shape for one variant: camelCase keys
// mirroring the upsert payload plus the stored IDs.
type adminVariantView struct {
	ID               int64             `json:"id"`
	ProductContentID int               `json:"productContentId"`
	Options          map[string]string `json:"options"`
	SKU              string            `json:"sku,omitempty"`
	Price            *int64            `json:"price,omitempty"`
	Stock            int               `json:"stock"`
	WeightGrams      *int              `json:"weightGrams,omitempty"`
	ImageURL         string            `json:"imageUrl,omitempty"`
}

// adminProductView is the admin wire shape for one product: camelCase keys
// mirroring upsertProductBody plus the stored IDs and timestamp. The domain
// Product carries no JSON tags, so the handler maps through this view instead
// of dumping the raw struct (which would serialize as PascalCase).
type adminProductView struct {
	ContentID         int                 `json:"contentId"`
	Price             int64               `json:"price"`
	SalePrice         *int64              `json:"salePrice,omitempty"`
	SKU               string              `json:"sku,omitempty"`
	TrackInventory    bool                `json:"trackInventory"`
	Stock             int                 `json:"stock"`
	WeightGrams       *int                `json:"weightGrams,omitempty"`
	LowStockThreshold *int                `json:"lowStockThreshold,omitempty"`
	Options           map[string][]string `json:"options"`
	Variants          []adminVariantView  `json:"variants"`
	UpdatedAt         string              `json:"updatedAt,omitempty"`
}

// mapAdminVariant projects one domain variant onto the admin wire shape.
func mapAdminVariant(v catalogdomain.Variant) adminVariantView {
	return adminVariantView{
		ID:               v.ID,
		ProductContentID: v.ProductContentID,
		Options:          v.Options,
		SKU:              v.SKU,
		Price:            v.Price,
		Stock:            v.Stock,
		WeightGrams:      v.WeightGrams,
		ImageURL:         v.ImageURL,
	}
}

// mapAdminProduct projects one domain product onto the admin wire shape.
func mapAdminProduct(product *catalogdomain.Product) adminProductView {
	view := adminProductView{
		ContentID:         product.ContentID,
		Price:             product.Price,
		SalePrice:         product.SalePrice,
		SKU:               product.SKU,
		TrackInventory:    product.TrackInventory,
		Stock:             product.Stock,
		WeightGrams:       product.WeightGrams,
		LowStockThreshold: product.LowStockThreshold,
		Options:           product.Options,
		Variants:          make([]adminVariantView, 0, len(product.Variants)),
	}
	for _, v := range product.Variants {
		view.Variants = append(view.Variants, mapAdminVariant(v))
	}
	if !product.UpdatedAt.IsZero() {
		view.UpdatedAt = product.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return view
}

// upsertProductBody is the admin PUT body (the GET returns the stored
// product with its variant IDs).
type upsertProductBody struct {
	Price             int64                        `json:"price"`
	SalePrice         *int64                       `json:"salePrice,omitempty"`
	SKU               string                       `json:"sku,omitempty"`
	TrackInventory    bool                         `json:"trackInventory"`
	Stock             int                          `json:"stock"`
	WeightGrams       *int                         `json:"weightGrams,omitempty"`
	LowStockThreshold *int                         `json:"lowStockThreshold,omitempty"`
	Options           map[string][]string          `json:"options"`
	Variants          []catalogdomain.VariantInput `json:"variants"`
}

// Handler serves both catalog realms; New builds it once so the admin and
// public surfaces share the adopted service.
type Handler struct {
	service   *catalogdomain.Service
	config    Config
	postTypes PostTypeChecker
	slug      ProductSlugResolver
	resolve   kernel.CapabilityResolver
}

// GetProduct handles GET /catalog/products/{contentID}: the stored commerce
// data for one product, or 404 when the item carries none.
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	contentID, ok := parseContentID(w, r)
	if !ok {
		return
	}

	product, err := h.service.GetProduct(r.Context(), contentID)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	httpx.Success(w, mapAdminProduct(product))
}

// UpsertProduct handles PUT /catalog/products/{contentID}: validates and
// writes the commerce data, replacing the variant matrix. Gated to the
// admin capability for now — the catalog.manage capability is contributed
// via RoleProvider so a role-registry grants mechanism can widen access
// later without changing this surface's wire behavior.
func (h *Handler) UpsertProduct(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || h.resolve == nil || !h.resolve(principal, string(capability.CapabilityAdmin)) {
		httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to manage the catalog", nil)
		return
	}

	contentID, ok := parseContentID(w, r)
	if !ok {
		return
	}

	var body upsertProductBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	product, err := h.service.UpsertProduct(r.Context(), contentID, catalogdomain.UpsertProductInput{
		Price:             body.Price,
		SalePrice:         body.SalePrice,
		SKU:               strings.TrimSpace(body.SKU),
		TrackInventory:    body.TrackInventory,
		Stock:             body.Stock,
		WeightGrams:       body.WeightGrams,
		LowStockThreshold: body.LowStockThreshold,
		Options:           body.Options,
		Variants:          body.Variants,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}

	httpx.Success(w, mapAdminProduct(product))
}

// GetPublicProduct handles GET /catalog/products/{slug} on the public
// router: the commerce payload for a published product, or 404 when the
// slug is unknown, unpublished, or carries no catalog data.
func (h *Handler) GetPublicProduct(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Product slug is required", nil)
		return
	}

	contentID, found := h.slug(r.Context(), slug)
	if !found {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Product not found", nil)
		return
	}

	product, err := h.service.GetProduct(r.Context(), contentID)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	httpx.Success(w, projectPublicProduct(product, h.config.Currency))
}

// New builds the handler. The capability resolver gates admin mutations
// (nil resolver denies every mutation — fail closed); the post-type checker
// and slug resolver come from the composition root.
func New(
	service *catalogdomain.Service,
	config Config,
	postTypes PostTypeChecker,
	slug ProductSlugResolver,
	resolve kernel.CapabilityResolver,
) *Handler {
	return &Handler{
		service:   service,
		config:    config,
		postTypes: postTypes,
		slug:      slug,
		resolve:   resolve,
	}
}
