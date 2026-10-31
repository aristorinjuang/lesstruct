package commerce

import (
	"context"
	"errors"
	"fmt"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	kernelcontent "github.com/aristorinjuang/lesstruct/kernel/content"
	kernelseo "github.com/aristorinjuang/lesstruct/kernel/seo"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
)

// contentImageURL resolves a content item's display image: the explicit
// featured image wins, then the first body image. The fallback mirrors the
// public content pages so cart lines and storefront cards agree.
func contentImageURL(content *kernelcontent.Content) string {
	if content.FeaturedImage != "" {
		return content.FeaturedImage
	}
	if content.Format == kernelcontent.FormatHTML {
		return kernelseo.ExtractImageURLFromHTML(content.Content)
	}
	return kernelseo.ExtractImageURL(content.Content)
}

// catalogSource adapts the catalog domain service onto the cart module's
// CatalogSource seam: sellable lines resolve from catalog products and
// variants, with unknown lookups wrapped in the cart sentinels the domain
// tolerates on read. The optional identity lookup labels each line with the
// published product's title, slug, and image for the cart page.
type catalogSource struct {
	service  *catalogdomain.Service
	identity ProductIdentityFunc
}

// lineIdentity resolves the display identity for one product, if available.
func (a catalogSource) lineIdentity(ctx context.Context, contentID int) ProductIdentity {
	if a.identity == nil {
		return ProductIdentity{}
	}
	identity, ok := a.identity(ctx, contentID)
	if !ok {
		return ProductIdentity{}
	}
	return identity
}

// ResolveLine resolves one sellable line: the base product when variantID
// is nil, else the matching matrix variant with its effective price.
func (a catalogSource) ResolveLine(ctx context.Context, productContentID int, variantID *int64) (cartdomain.CatalogItem, error) {
	product, err := a.service.GetProduct(ctx, productContentID)
	if err != nil {
		if errors.Is(err, catalogdomain.ErrProductNotFound) {
			return cartdomain.CatalogItem{}, fmt.Errorf("%w: %d", cartdomain.ErrUnknownProduct, productContentID)
		}
		return cartdomain.CatalogItem{}, err
	}

	item := cartdomain.CatalogItem{
		ProductContentID: product.ContentID,
		SKU:              product.SKU,
		UnitPrice:        product.EffectivePrice(nil),
		InStock:          product.InStock(),
	}
	if variantID != nil {
		found := false
		for i := range product.Variants {
			variant := &product.Variants[i]
			if variant.ID != *variantID {
				continue
			}
			found = true
			item.VariantID = variantID
			item.SKU = variant.SKU
			item.UnitPrice = product.EffectivePrice(variant)
			item.InStock = product.VariantInStock(variant)
			break
		}
		if !found {
			return cartdomain.CatalogItem{}, fmt.Errorf("%w: %d", cartdomain.ErrUnknownVariant, *variantID)
		}
	}

	identity := a.lineIdentity(ctx, product.ContentID)
	item.Title = identity.Title
	item.Slug = identity.Slug
	item.ImageURL = identity.ImageURL
	return item, nil
}

// ProductIdentity is the published display identity of one product: what the
// cart page needs to render a recognizable line.
type ProductIdentity struct {
	Title    string
	Slug     string
	ImageURL string
}

// ProductIdentityFunc resolves a published product's display identity. A
// miss returns ok=false: the line still resolves, just without labels.
type ProductIdentityFunc func(ctx context.Context, contentID int) (ProductIdentity, bool)

// NewProductIdentity adapts the content service onto ProductIdentityFunc.
// The image is the explicit featured image, falling back to the first image
// in the body — the same resolution the public content pages use, so cart
// lines match the storefront cards.
func NewProductIdentity(service *contentdomain.Service) ProductIdentityFunc {
	return func(ctx context.Context, contentID int) (ProductIdentity, bool) {
		content, err := service.GetPublishedByID(ctx, contentID)
		if err != nil || content == nil {
			return ProductIdentity{}, false
		}
		return ProductIdentity{
			Title:    content.Title,
			Slug:     content.Slug,
			ImageURL: contentImageURL(content),
		}, true
	}
}

// NewCatalogSource builds the seam adapter over the adopted catalog
// service. The identity lookup may be nil, in which case lines carry no
// display labels.
func NewCatalogSource(service *catalogdomain.Service, identity ProductIdentityFunc) cartdomain.CatalogSource {
	return catalogSource{service: service, identity: identity}
}
