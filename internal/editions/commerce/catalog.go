package commerce

import (
	"context"

	"github.com/aristorinjuang/lesstruct/internal/email"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
)

// catalogMailer adapts the platform email service onto the catalog
// module's Mailer seam. The notice recipient rides the [catalog]
// notify_email setting, so an unset address degrades to silence without
// the domain ever knowing. Failures log adapter-side and never fail a
// stock write.
type catalogMailer struct {
	email *email.Service
	to    string
}

// SendLowStockAlert reports one tracked bucket crossing its threshold
// downward, naming the bucket by content ID and SKU.
func (m catalogMailer) SendLowStockAlert(
	ctx context.Context,
	product *catalogdomain.Product,
	variantID *int64,
	stock int,
) error {
	if m.to == "" {
		return nil
	}
	sku := product.SKU
	if variantID != nil {
		for i := range product.Variants {
			if product.Variants[i].ID == *variantID {
				sku = product.Variants[i].SKU
				break
			}
		}
	}
	threshold := 0
	if product.LowStockThreshold != nil {
		threshold = *product.LowStockThreshold
	}
	return m.email.SendLowStockAlert(ctx, m.to, product.ContentID, sku, stock, threshold)
}

// NewCatalogMailer builds the Mailer seam adapter over the platform email
// service and the [catalog] notify_email recipient (empty disables).
func NewCatalogMailer(service *email.Service, notifyEmail string) catalogdomain.Mailer {
	return catalogMailer{email: service, to: notifyEmail}
}
