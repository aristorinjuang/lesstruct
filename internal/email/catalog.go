// Catalog stock notifications: the commerce sends the platform offers out
// of the box. The service stays module-agnostic — the bucket details
// arrive as display-ready values composed by the caller.
package email

import (
	"context"
	"fmt"
)

// generateLowStockEmailBody renders the low-stock notice for one tracked
// bucket that crossed its threshold downward: which bucket, its new
// stock, and the threshold it fell through.
func generateLowStockEmailBody(contentID int, sku string, stock int, threshold int) emailBody {
	bucket := fmt.Sprintf("product #%d", contentID)
	if sku != "" {
		bucket += fmt.Sprintf(" (SKU %s)", sku)
	}

	text := fmt.Sprintf(`Hello,

Stock is running low for %s.

Current stock: %d
Low-stock threshold: %d

Consider restocking soon to avoid turning customers away at checkout.

Best regards,
The Lesstruct Team`, bucket, stock, threshold)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>Stock is running low for <strong>%s</strong>.</p>

<p><strong>Current stock:</strong> %d</p>

<p><strong>Low-stock threshold:</strong> %d</p>

<p>Consider restocking soon to avoid turning customers away at checkout.</p>`,
		bucket, stock, threshold))

	return emailBody{text: text, html: html}
}

// SendLowStockAlert tells the operator one tracked bucket crossed its
// threshold downward.
func (s *Service) SendLowStockAlert(ctx context.Context, to string, contentID int, sku string, stock int, threshold int) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := fmt.Sprintf("Low stock: product #%d", contentID)
	body := generateLowStockEmailBody(contentID, sku, stock, threshold)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send low stock email",
			"recipient", to,
			"contentID", contentID,
			"error", err)
		return fmt.Errorf("failed to send low stock email: %w", err)
	}

	s.logger.Info("Low stock email sent successfully",
		"recipient", to,
		"contentID", contentID)

	return nil
}
