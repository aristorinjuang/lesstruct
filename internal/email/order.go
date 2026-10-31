// Order email notifications: the commerce sends the platform offers out of
// the box. The service stays module-agnostic — item lines arrive as
// display-ready strings composed by the caller.
package email

import (
	"context"
	"fmt"
	"strings"
)

// generateOrderConfirmationEmailBody renders the confirmation for a placed
// order: the money amounts arrive pre-formatted, so this stays a plain
// template over display strings. A set payee appends the How-to-pay block.
// A non-empty tracking token appends the Track-your-order link — the only
// place the per-order secret is ever delivered.
func generateOrderConfirmationEmailBody(orderNumber string, items []OrderEmailItem, subtotal string, shipping string, total string, payee ManualPayee, trackingURL string) emailBody {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := fmt.Sprintf("- %d x %s", item.Quantity, item.Title)
		if item.SKU != "" {
			line += fmt.Sprintf(" (SKU %s)", item.SKU)
		}
		line += fmt.Sprintf(" — %s", item.LineTotal)
		lines = append(lines, line)
	}
	itemList := strings.Join(lines, "\n")

	payeeText := ""
	payeeHTML := ""
	if payee.Bank != "" && payee.Account != "" && payee.Name != "" {
		payeeText = fmt.Sprintf("\n\nHow to pay (manual bank transfer):\nBank: %s\nAccount: %s\nName: %s\n\nInclude your order number %s in the transfer note, then submit the proof from the tracking page.",
			payee.Bank, payee.Account, payee.Name, orderNumber)
		payeeHTML = fmt.Sprintf(`<p><strong>How to pay (manual bank transfer):</strong></p><p>Bank: <strong>%s</strong><br/>Account: <strong>%s</strong><br/>Name: <strong>%s</strong></p><p>Include your order number %s in the transfer note, then submit the proof from the tracking page.</p>`,
			payee.Bank, payee.Account, payee.Name, orderNumber)
	}

	trackText := ""
	trackHTML := ""
	if trackingURL != "" {
		trackText = fmt.Sprintf("\n\nTrack your order (keep this link — it is your order's key):\n%s", trackingURL)
		trackHTML = fmt.Sprintf(`<p><strong>Track your order</strong> — keep this link, it is your order's key:<br/><a href="%s">%s</a></p>`, trackingURL, trackingURL)
	}

	text := fmt.Sprintf(`Hello,

Thank you for your order! It has been placed and is awaiting payment confirmation.

Order number: %s

Items:
%s

Subtotal: %s
Shipping: %s

Total due: %s%s%s

We will email you again once the payment is confirmed and the order is processed.

Best regards,
The Lesstruct Team`, orderNumber, itemList, subtotal, shipping, total, payeeText, trackText)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>Thank you for your order! It has been placed and is awaiting payment confirmation.</p>

<p>Order number: <strong>%s</strong></p>

<p><strong>Items:</strong></p>
<ul>%s</ul>

<p>Subtotal: <strong>%s</strong></p>

<p>Shipping: <strong>%s</strong></p>

<p>Total due: <strong>%s</strong></p>
%s%s
<p>We will email you again once the payment is confirmed and the order is processed.</p>`,
		orderNumber, htmlItemsList(items), subtotal, shipping, total, payeeHTML, trackHTML))

	return emailBody{text: text, html: html}
}

// htmlItemsList renders the confirmation item lines for the HTML variant.
func htmlItemsList(items []OrderEmailItem) string {
	rendered := make([]string, 0, len(items))
	for _, item := range items {
		line := fmt.Sprintf("<li>%d x %s — %s</li>", item.Quantity, item.Title, item.LineTotal)
		if item.SKU == "" {
			rendered = append(rendered, line)
			continue
		}
		rendered = append(rendered, fmt.Sprintf("<li>%d x %s (SKU %s) — %s</li>", item.Quantity, item.Title, item.SKU, item.LineTotal))
	}
	return strings.Join(rendered, "\n")
}

// generateOrderStatusEmailBody renders a status move for a placed order.
func generateOrderStatusEmailBody(orderNumber, status string) emailBody {
	text := fmt.Sprintf(`Hello,

Your order %s moved to %s.

We will keep you posted as it progresses.

Best regards,
The Lesstruct Team`, orderNumber, status)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>Your order <strong>%s</strong> moved to <strong>%s</strong>.</p>

<p>We will keep you posted as it progresses.</p>`, orderNumber, status))

	return emailBody{text: text, html: html}
}

// ManualPayee is the destination account for manual bank transfers,
// adopted from the [payments.manual] config block. Empty means no payee
// block is configured and the confirmation email carries no How-to-pay
// section.
type ManualPayee struct {
	Bank    string
	Account string
	Name    string
}

// OrderEmailItem is one purchase line of an order email. SKU and money
// amounts arrive as display-ready strings so the email service needs no
// commerce knowledge.
type OrderEmailItem struct {
	SKU       string
	Title     string
	Quantity  int
	UnitPrice string
	LineTotal string
}

// SendOrderConfirmationEmail sends the confirmation for a just-placed
// order. A set payee appends the How-to-pay block; an empty payee sends
// the legacy body unchanged. A non-empty trackingToken appends the
// Track-your-order link built from the site base URL — the token is the
// order's guest key, so the email is the only place it is delivered.
func (s *Service) SendOrderConfirmationEmail(ctx context.Context, to, orderNumber string, items []OrderEmailItem, subtotal string, shipping string, total string, payee ManualPayee, trackingToken string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	trackingURL := ""
	if trackingToken != "" {
		trackingURL = strings.TrimRight(s.baseURL, "/") + "/orders/track?order=" + orderNumber + "&t=" + trackingToken
	}

	subject := "Order confirmed: " + orderNumber
	body := generateOrderConfirmationEmailBody(orderNumber, items, subtotal, shipping, total, payee, trackingURL)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send order confirmation email",
			"recipient", to,
			"orderNumber", orderNumber,
			"error", err)
		return fmt.Errorf("failed to send order confirmation email: %w", err)
	}

	s.logger.Info("Order confirmation email sent successfully",
		"recipient", to,
		"orderNumber", orderNumber)

	return nil
}

// SendOrderStatusEmail tells the customer one lifecycle move of their
// placed order, carrying the new status as its display string.
func (s *Service) SendOrderStatusEmail(ctx context.Context, to, orderNumber, status string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Order update: " + orderNumber
	body := generateOrderStatusEmailBody(orderNumber, status)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send order status email",
			"recipient", to,
			"orderNumber", orderNumber,
			"status", status,
			"error", err)
		return fmt.Errorf("failed to send order status email: %w", err)
	}

	s.logger.Info("Order status email sent successfully",
		"recipient", to,
		"orderNumber", orderNumber,
		"status", status)

	return nil
}
