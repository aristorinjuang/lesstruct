package commerce_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/editions/commerce"
	"github.com/aristorinjuang/lesstruct/internal/email"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// disabledEmail is a platform email service with no SMTP configured: sends
// fail closed with the disabled error instead of touching the network.
func disabledEmail() *email.Service {
	return email.NewService(email.SMTPConfig{}, slog.Default(), "")
}

func stockedProduct() *catalogdomain.Product {
	threshold := 5
	return &catalogdomain.Product{
		ContentID:         7,
		TrackInventory:    true,
		Stock:             10,
		LowStockThreshold: &threshold,
		SKU:               "TSHIRT",
		Variants: []catalogdomain.Variant{
			{ID: 3, ProductContentID: 7, SKU: "TSHIRT-S", Stock: 4},
		},
	}
}

func TestCatalogMailer_SendLowStockAlert(t *testing.T) {
	t.Run("an unset recipient degrades to silence", func(t *testing.T) {
		mailer := commerce.NewCatalogMailer(disabledEmail(), "")

		err := mailer.SendLowStockAlert(context.Background(), stockedProduct(), nil, 4)

		require.NoError(t, err)
	})

	t.Run("a configured recipient routes the notice", func(t *testing.T) {
		mailer := commerce.NewCatalogMailer(disabledEmail(), "ops@example.com")

		err := mailer.SendLowStockAlert(context.Background(), stockedProduct(), nil, 4)

		// The disabled service fails closed: the assertion is the
		// routing itself — the notice reached the platform email
		// service instead of degrading to silence.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email service is disabled")
	})
}
