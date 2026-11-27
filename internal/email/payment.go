// Payment email notifications: the commerce sends the platform offers out
// of the box. The service stays module-agnostic — the record details
// arrive as display-ready strings composed by the caller.
package email

import (
	"context"
	"fmt"
)

// generatePaymentReceivedEmailBody renders the proof-submission notice for
// a manually paid order. The money amounts arrive pre-formatted, so this
// stays a plain template over display strings.
func generatePaymentReceivedEmailBody(orderNumber, method, reference, amount string) emailBody {
	text := fmt.Sprintf(`Hello,

We received your payment proof for order %s via %s.

Reference: %s
Amount: %s

An administrator will verify the payment shortly, and we will email you
again once the order is confirmed and processed.

Best regards,
The Lesstruct Team`, orderNumber, method, reference, amount)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>We received your payment proof for order <strong>%s</strong> via <strong>%s</strong>.</p>

<p><strong>Reference:</strong> %s</p>

<p><strong>Amount:</strong> %s</p>

<p>An administrator will verify the payment shortly, and we will email you again once the order is confirmed and processed.</p>`,
		orderNumber, method, reference, amount))

	return emailBody{text: text, html: html}
}

// generatePaymentRejectedEmailBody renders the rejection notice: the
// reference could not be verified and the order stays open for a corrected
// resubmission.
func generatePaymentRejectedEmailBody(orderNumber, method, reference string) emailBody {
	text := fmt.Sprintf(`Hello,

Unfortunately we could not verify the payment reference you submitted for
order %s via %s:

Reference: %s

The order is still open and awaiting payment. If you believe this is a
mistake, please submit the payment again with the correct reference —
for example the full transfer receipt number or your bank's confirmation
code.

Best regards,
The Lesstruct Team`, orderNumber, method, reference)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>Unfortunately we could not verify the payment reference you submitted for order <strong>%s</strong> via <strong>%s</strong>:</p>

<p><strong>Reference:</strong> %s</p>

<p>The order is still open and awaiting payment. If you believe this is a mistake, please submit the payment again with the correct reference — for example the full transfer receipt number or your bank's confirmation code.</p>`,
		orderNumber, method, reference))

	return emailBody{text: text, html: html}
}

// generatePaymentProofAlertEmailBody renders the operator notice for a
// submitted manual payment proof: which order, which method, the
// reference to verify, and the expected amount. The money amount arrives
// pre-formatted, so this stays a plain template over display strings.
func generatePaymentProofAlertEmailBody(orderNumber, customerEmail, method, reference, amount string) emailBody {
	text := fmt.Sprintf(`Hello,

A payment proof was submitted for order %s and needs verification.

Customer: %s
Method: %s
Reference: %s
Amount: %s

Open the admin payments surface to verify or reject it.`, orderNumber, customerEmail, method, reference, amount)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>A payment proof was submitted for order <strong>%s</strong> and needs verification.</p>

<p><strong>Customer:</strong> %s<br/><strong>Method:</strong> %s<br/><strong>Reference:</strong> %s<br/><strong>Amount:</strong> %s</p>

<p>Open the admin payments surface to verify or reject it.</p>`,
		orderNumber, customerEmail, method, reference, amount))

	return emailBody{text: text, html: html}
}

// generatePaymentSettledAlertEmailBody renders the operator notice for a
// gateway-settled payment: the order is paid and ready to fulfill. The
// money amount arrives pre-formatted, so this stays a plain template
// over display strings.
func generatePaymentSettledAlertEmailBody(orderNumber, customerEmail, method, amount string) emailBody {
	text := fmt.Sprintf(`Hello,

The payment for order %s was confirmed via %s.

Customer: %s
Amount: %s

The order is paid and ready to fulfill.`, orderNumber, method, customerEmail, amount)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello,</p>

<p>The payment for order <strong>%s</strong> was confirmed via <strong>%s</strong>.</p>

<p><strong>Customer:</strong> %s<br/><strong>Amount:</strong> %s</p>

<p>The order is paid and ready to fulfill.</p>`,
		orderNumber, method, customerEmail, amount))

	return emailBody{text: text, html: html}
}

// SendPaymentReceivedEmail confirms a submitted payment proof for a
// placed order.
func (s *Service) SendPaymentReceivedEmail(ctx context.Context, to, orderNumber, method, reference, amount string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Payment received: " + orderNumber
	body := generatePaymentReceivedEmailBody(orderNumber, method, reference, amount)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send payment received email",
			"recipient", to,
			"orderNumber", orderNumber,
			"error", err)
		return fmt.Errorf("failed to send payment received email: %w", err)
	}

	s.logger.Info("Payment received email sent successfully",
		"recipient", to,
		"orderNumber", orderNumber)

	return nil
}

// SendPaymentRejectedEmail tells the customer their proof could not be
// verified and how to correct it.
func (s *Service) SendPaymentRejectedEmail(ctx context.Context, to, orderNumber, method, reference string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Payment rejected: " + orderNumber
	body := generatePaymentRejectedEmailBody(orderNumber, method, reference)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send payment rejected email",
			"recipient", to,
			"orderNumber", orderNumber,
			"error", err)
		return fmt.Errorf("failed to send payment rejected email: %w", err)
	}

	s.logger.Info("Payment rejected email sent successfully",
		"recipient", to,
		"orderNumber", orderNumber)

	return nil
}

// SendPaymentProofAlert tells the operator a manual payment proof was
// submitted and needs verification, carrying the reference to check and
// the expected amount. The amount arrives display-ready, so the email
// service needs no commerce knowledge.
func (s *Service) SendPaymentProofAlert(
	ctx context.Context,
	to string,
	orderNumber string,
	customerEmail string,
	method string,
	reference string,
	amount string,
) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Payment proof submitted: " + orderNumber
	body := generatePaymentProofAlertEmailBody(orderNumber, customerEmail, method, reference, amount)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send payment proof alert email",
			"recipient", to,
			"orderNumber", orderNumber,
			"error", err)
		return fmt.Errorf("failed to send payment proof alert email: %w", err)
	}

	s.logger.Info("Payment proof alert email sent successfully",
		"recipient", to,
		"orderNumber", orderNumber)

	return nil
}

// SendPaymentSettledAlert tells the operator a gateway payment settled,
// so the order is paid and ready to fulfill. The amount arrives
// display-ready, so the email service needs no commerce knowledge.
func (s *Service) SendPaymentSettledAlert(
	ctx context.Context,
	to string,
	orderNumber string,
	customerEmail string,
	method string,
	amount string,
) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Payment settled: " + orderNumber
	body := generatePaymentSettledAlertEmailBody(orderNumber, customerEmail, method, amount)

	if err := s.sendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error("Failed to send payment settled alert email",
			"recipient", to,
			"orderNumber", orderNumber,
			"error", err)
		return fmt.Errorf("failed to send payment settled alert email: %w", err)
	}

	s.logger.Info("Payment settled alert email sent successfully",
		"recipient", to,
		"orderNumber", orderNumber)

	return nil
}
