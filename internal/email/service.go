package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// EmailService defines the interface for email operations
type EmailService interface {
	SendVerificationEmail(ctx context.Context, to, username, token string) error
	SendAccountLockoutEmail(ctx context.Context, to, username string) error
	SendUserApprovedEmail(ctx context.Context, to, username string) error
	SendUserRejectedEmail(ctx context.Context, to, username string) error
	SendUserSuspendedEmail(ctx context.Context, to, username, reason string) error
	SendUserUnsuspendedEmail(ctx context.Context, to, username string) error
	SendUserSoftDeletedEmail(ctx context.Context, to, username, reason string) error
	SendEmailUpdateVerificationEmail(ctx context.Context, to, username, token string) error
	SendDataExportNotificationEmail(ctx context.Context, to, username string) error
	SendAccountDeletedNotification(ctx context.Context, to, username string) error
	SendAccountCreatedEmail(ctx context.Context, to, username, plainPassword string) error
	SendPasswordResetEmail(ctx context.Context, to, username, token string) error
}

// SMTPConfig contains SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Service handles email sending operations
type Service struct {
	config  SMTPConfig
	timeout time.Duration
	enabled bool
	logger  *slog.Logger
	baseURL string // Base URL for site links (e.g., "http://localhost:8080")
}

// emailBody holds both plain text and HTML versions of an email
type emailBody struct {
	text string
	html string
}

// sendEmail sends an email using SMTP
func (s *Service) sendEmail(ctx context.Context, to, subject string, body emailBody) error {
	// Create SMTP address
	smtpAddr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Create authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Build email message
	from := s.config.From
	if from == "" {
		from = s.config.Username
	}

	msg := s.buildEmailMessage(from, to, subject, body)

	// Send email with timeout
	done := make(chan error, 1)
	go func() {
		// Try TLS first
		err := s.sendWithTLS(smtpAddr, auth, from, to, msg)
		if err != nil {
			// Fall back to plain SMTP if TLS fails
			done <- smtp.SendMail(smtpAddr, auth, from, []string{to}, msg)
			return
		}
		done <- err
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("email sending timed out after %v", s.timeout)
	case err := <-done:
		return err
	}
}

// sendWithTLS attempts to send email using TLS
func (s *Service) sendWithTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	// Connect to SMTP server
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	// Start TLS if available
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{
			ServerName: s.config.Host,
		}
		if err := client.StartTLS(config); err != nil {
			return err
		}
	}

	// Authenticate if credentials provided
	if s.config.Username != "" && s.config.Password != "" {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	// Set sender and recipient
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	// Send message body
	writer, err := client.Data()
	if err != nil {
		return err
	}
	defer func() {
		_ = writer.Close()
	}()

	_, err = writer.Write(msg)
	return err
}

// buildEmailMessage creates a properly formatted multipart/alternative email message
func (s *Service) buildEmailMessage(from, to, subject string, body emailBody) []byte {
	boundary := "LesstructBoundary_" + fmt.Sprintf("%d", time.Now().UnixNano())

	var msg strings.Builder

	// Add headers
	fmt.Fprintf(&msg, "From: %s\r\n", from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	msg.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: multipart/alternative; boundary=%s\r\n", boundary)
	msg.WriteString("\r\n")

	// Plain text part
	fmt.Fprintf(&msg, "--%s\r\n", boundary)
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body.text)
	msg.WriteString("\r\n\r\n")

	// HTML part
	fmt.Fprintf(&msg, "--%s\r\n", boundary)
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body.html)
	msg.WriteString("\r\n\r\n")

	// End boundary
	fmt.Fprintf(&msg, "--%s--\r\n", boundary)

	return []byte(msg.String())
}

func htmlWrap(body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; color: #333; line-height: 1.6; margin: 0; padding: 0; }
.container { max-width: 600px; margin: 40px auto; padding: 20px; }
.button { display: inline-block; padding: 12px 24px; background-color: #4F46E5; color: #fff !important; text-decoration: none; border-radius: 6px; font-weight: 600; }
.button:hover { background-color: #4338CA; }
.footer { margin-top: 32px; padding-top: 16px; border-top: 1px solid #E5E7EB; font-size: 14px; color: #6B7280; }
</style>
</head>
<body>
<div class="container">
%s
<div class="footer">Best regards,<br>The Lesstruct Team</div>
</div>
</body>
</html>`, body)
}

// generateVerificationEmailBody generates the email body for verification
func (s *Service) generateVerificationEmailBody(username, verificationURL string) emailBody {
	text := fmt.Sprintf(`Hello %s,

Thank you for registering with Lesstruct!

Please click the link below to verify your email address:
%s

This link will expire in 24 hours.

If you did not create an account, please ignore this email.

Best regards,
The Lesstruct Team`, username, verificationURL)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Thank you for registering with Lesstruct!</p>

<p>Please click the button below to verify your email address:</p>

<p><a href="%s" class="button">Verify Email Address</a></p>

<p>This link will expire in 24 hours.</p>

<p>If you did not create an account, please ignore this email.</p>`, username, verificationURL))

	return emailBody{text: text, html: html}
}

// generateAccountLockoutEmailBody generates the email body for account lockout notification
func (s *Service) generateAccountLockoutEmailBody(username string) emailBody {
	text := fmt.Sprintf(`Hello %s,

Your account has been temporarily locked due to multiple failed login attempts.

This is a security measure to protect your account. Your account will be locked for 15 minutes.

If this was not you, please ensure your password is secure and consider changing it.

If you continue to experience issues, please contact our support team.

Best regards,
The Lesstruct Team`, username)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Your account has been temporarily locked due to multiple failed login attempts.</p>

<p>This is a security measure to protect your account. Your account will be locked for 15 minutes.</p>

<p>If this was not you, please ensure your password is secure and consider changing it.</p>

<p>If you continue to experience issues, please contact our support team.</p>`, username))

	return emailBody{text: text, html: html}
}

// generateUserApprovedEmailBody generates the email body for user approval
func (s *Service) generateUserApprovedEmailBody(username string) emailBody {
	loginURL := fmt.Sprintf("%s/login", s.baseURL)
	text := fmt.Sprintf(`Hello %s,

Good news! Your account has been approved by the Site Administrator.

You can now log in to your account at:
%s

If you have any questions, please contact us.

Best regards,
The Lesstruct Team`, username, loginURL)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Good news! Your account has been approved by the Site Administrator.</p>

<p>You can now log in to your account at: <a href="%s">%s</a></p>

<p>If you have any questions, please contact us.</p>`, username, loginURL, loginURL))

	return emailBody{text: text, html: html}
}

// generateUserRejectedEmailBody generates the email body for user rejection
func (s *Service) generateUserRejectedEmailBody(username string) emailBody {
	text := fmt.Sprintf(`Hello %s,

Your registration for Lesstruct has been rejected by the Site Administrator.

You may register again if you wish to do so.

If you have any questions, please contact us.

Best regards,
The Lesstruct Team`, username)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Your registration for Lesstruct has been rejected by the Site Administrator.</p>

<p>You may register again if you wish to do so.</p>

<p>If you have any questions, please contact us.</p>`, username))

	return emailBody{text: text, html: html}
}

// generateUserSuspendedEmailBody generates the email body for user suspension
func (s *Service) generateUserSuspendedEmailBody(username, reason string) emailBody {
	textBody := fmt.Sprintf(`Hello %s,

Your Lesstruct account has been suspended by the Site Administrator.
`, username)

	if reason != "" {
		textBody += fmt.Sprintf("\nReason: %s\n", reason)
	}

	textBody += `
If you believe this is an error, please contact us.

Best regards,
The Lesstruct Team`

	htmlBody := fmt.Sprintf(`<p>Hello %s,</p>

<p>Your Lesstruct account has been suspended by the Site Administrator.</p>`, username)

	if reason != "" {
		htmlBody += fmt.Sprintf(`<p><strong>Reason:</strong> %s</p>`, reason)
	}

	htmlBody += `<p>If you believe this is an error, please contact us.</p>`

	return emailBody{text: textBody, html: htmlWrap(htmlBody)}
}

// generateUserUnsuspendedEmailBody generates the email body for user unsuspension
func (s *Service) generateUserUnsuspendedEmailBody(username string) emailBody {
	text := fmt.Sprintf(`Hello %s,

Good news! Your Lesstruct account has been unsuspended and you can now log in again.

If you have any questions, please contact us.

Best regards,
The Lesstruct Team`, username)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Good news! Your Lesstruct account has been unsuspended and you can now log in again.</p>

<p>If you have any questions, please contact us.</p>`, username))

	return emailBody{text: text, html: html}
}

// generateUserSoftDeletedEmailBody generates the email body for user soft deletion
func (s *Service) generateUserSoftDeletedEmailBody(username, reason string) emailBody {
	textBody := fmt.Sprintf(`Hello %s,

Your Lesstruct account has been soft deleted by the Site Administrator.
`, username)

	if reason != "" {
		textBody += fmt.Sprintf("\nReason: %s\n", reason)
	}

	textBody += `
Your content is no longer visible on the site. If you believe this is an error, please contact us.

Best regards,
The Lesstruct Team`

	htmlBody := fmt.Sprintf(`<p>Hello %s,</p>

<p>Your Lesstruct account has been soft deleted by the Site Administrator.</p>`, username)

	if reason != "" {
		htmlBody += fmt.Sprintf(`<p><strong>Reason:</strong> %s</p>`, reason)
	}

	htmlBody += `<p>Your content is no longer visible on the site. If you believe this is an error, please contact us.</p>`

	return emailBody{text: textBody, html: htmlWrap(htmlBody)}
}

// generateEmailUpdateVerificationEmailBody generates the email body for email update verification
func (s *Service) generateEmailUpdateVerificationEmailBody(username, verificationURL string) emailBody {
	text := fmt.Sprintf(`Hello %s,

You requested to update your email address for your Lesstruct account.

Please click the link below to verify your new email address:
%s

This link will expire in 24 hours.

If you did not request this change, please ignore this email.

Best regards,
The Lesstruct Team`, username, verificationURL)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>You requested to update your email address for your Lesstruct account.</p>

<p>Please click the button below to verify your new email address:</p>

<p><a href="%s" class="button">Verify New Email</a></p>

<p>This link will expire in 24 hours.</p>

<p>If you did not request this change, please ignore this email.</p>`, username, verificationURL))

	return emailBody{text: text, html: html}
}

// generateDataExportNotificationEmailBody generates the email body for data export notification
func (s *Service) generateDataExportNotificationEmailBody(username string) emailBody {
	text := fmt.Sprintf(`Hello %s,

Your personal data export is ready.

You can download your data from the Profile section in the admin panel.

Your export includes all your posts, pages, comments, media files, and account information.

Best regards,
The Lesstruct Team`, username)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Your personal data export is ready.</p>

<p>You can download your data from the Profile section in the admin panel.</p>

<p>Your export includes all your posts, pages, comments, media files, and account information.</p>`, username))

	return emailBody{text: text, html: html}
}

// generateAccountDeletedNotificationEmailBody generates the email body for account deletion notification
func (s *Service) generateAccountDeletedNotificationEmailBody(username string) emailBody {
	text := fmt.Sprintf(`Hello %s,

Your Lesstruct account has been permanently deleted.

All your personal data, including:
- Posts and pages
- Comments
- Media files
- Account information

has been permanently removed from our systems.

If you did not request this deletion, please contact us immediately.

Best regards,
The Lesstruct Team`, username)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>Your Lesstruct account has been permanently deleted.</p>

<p>All your personal data, including:</p>
<ul>
<li>Posts and pages</li>
<li>Comments</li>
<li>Media files</li>
<li>Account information</li>
</ul>
<p>has been permanently removed from our systems.</p>

<p>If you did not request this deletion, please contact us immediately.</p>`, username))

	return emailBody{text: text, html: html}
}

// generateAccountCreatedEmailBody generates the email body for admin-created account
func (s *Service) generateAccountCreatedEmailBody(username, plainPassword string) emailBody {
	loginURL := fmt.Sprintf("%s/login", s.baseURL)
	text := fmt.Sprintf(`Hello %s,

An administrator has created a Lesstruct account for you.

Here are your login credentials:
Username: %s
Password: %s

Please change your password after your first login.

You can log in at: %s

Best regards,
The Lesstruct Team`, username, username, plainPassword, loginURL)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>An administrator has created a Lesstruct account for you.</p>

<p>Here are your login credentials:</p>
<p><strong>Username:</strong> %s</p>
<p><strong>Password:</strong> %s</p>

<p>Please change your password after your first login.</p>

<p>You can log in at: <a href="%s">%s</a></p>`, username, username, plainPassword, loginURL, loginURL))

	return emailBody{text: text, html: html}
}

// SendVerificationEmail sends a verification email to a user
func (s *Service) SendVerificationEmail(ctx context.Context, to, username, token string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, token)

	subject := "Verify Your Email Address for Lesstruct"
	body := s.generateVerificationEmailBody(username, verificationURL)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send verification email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	s.logger.Info("Verification email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// SendAccountLockoutEmail sends an account lockout notification email to a user
func (s *Service) SendAccountLockoutEmail(ctx context.Context, to, username string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Account Locked - Lesstruct"
	body := s.generateAccountLockoutEmailBody(username)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send account lockout email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send account lockout email: %w", err)
	}

	s.logger.Info("Account lockout email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// IsEnabled returns whether the email service is enabled
func (s *Service) IsEnabled() bool {
	return s.enabled
}

// SetBaseURL sets the base URL for verification links
func (s *Service) SetBaseURL(baseURL string) {
	s.baseURL = baseURL
}

// SendUserApprovedEmail sends an email when a user's registration is approved
func (s *Service) SendUserApprovedEmail(ctx context.Context, to, username string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Account Has Been Approved - Lesstruct"
	body := s.generateUserApprovedEmailBody(username)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send user approved email",
			"recipient", to,
			"username", username,
			"error", err)
		return err
	}

	s.logger.Info("User approved email sent successfully",
		"recipient", to,
		"username", username)
	return nil
}

// SendUserRejectedEmail sends an email when a user's registration is rejected
func (s *Service) SendUserRejectedEmail(ctx context.Context, to, username string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Registration Was Rejected - Lesstruct"
	body := s.generateUserRejectedEmailBody(username)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send user rejected email",
			"recipient", to,
			"username", username,
			"error", err)
		return err
	}

	s.logger.Info("User rejected email sent successfully",
		"recipient", to,
		"username", username)
	return nil
}

// SendUserSuspendedEmail sends an email when a user's account is suspended
func (s *Service) SendUserSuspendedEmail(ctx context.Context, to, username, reason string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Account Has Been Suspended - Lesstruct"
	body := s.generateUserSuspendedEmailBody(username, reason)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send user suspended email",
			"recipient", to,
			"username", username,
			"error", err)
		return err
	}

	s.logger.Info("User suspended email sent successfully",
		"recipient", to,
		"username", username)
	return nil
}

// SendUserUnsuspendedEmail sends an email when a user's account is unsuspended
func (s *Service) SendUserUnsuspendedEmail(ctx context.Context, to, username string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Account Has Been Unsuspended - Lesstruct"
	body := s.generateUserUnsuspendedEmailBody(username)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send user unsuspended email",
			"recipient", to,
			"username", username,
			"error", err)
		return err
	}

	s.logger.Info("User unsuspended email sent successfully",
		"recipient", to,
		"username", username)
	return nil
}

// SendUserSoftDeletedEmail sends an email when a user's account is soft deleted
func (s *Service) SendUserSoftDeletedEmail(ctx context.Context, to, username, reason string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Account Has Been Deleted - Lesstruct"
	body := s.generateUserSoftDeletedEmailBody(username, reason)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send user soft deleted email",
			"recipient", to,
			"username", username,
			"error", err)
		return err
	}

	s.logger.Info("User soft deleted email sent successfully",
		"recipient", to,
		"username", username)
	return nil
}

// SendEmailUpdateVerificationEmail sends an email update verification email to a user
func (s *Service) SendEmailUpdateVerificationEmail(ctx context.Context, to, username, token string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	verificationURL := fmt.Sprintf("%s/api/profile/verify-email?token=%s", s.baseURL, token)

	subject := "Verify Your New Email Address"
	body := s.generateEmailUpdateVerificationEmailBody(username, verificationURL)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send email update verification email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send email update verification email: %w", err)
	}

	s.logger.Info("Email update verification email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// SendDataExportNotificationEmail sends a data export notification email to a user
func (s *Service) SendDataExportNotificationEmail(ctx context.Context, to, username string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Data Export is Ready"
	body := s.generateDataExportNotificationEmailBody(username)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send data export notification email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send data export notification email: %w", err)
	}

	s.logger.Info("Data export notification email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// SendAccountDeletedNotification sends an account deletion notification email to a user
func (s *Service) SendAccountDeletedNotification(ctx context.Context, to, username string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Lesstruct Account Has Been Deleted"
	body := s.generateAccountDeletedNotificationEmailBody(username)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send account deleted notification email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send account deleted notification email: %w", err)
	}

	s.logger.Info("Account deleted notification email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// SendAccountCreatedEmail sends a welcome email to a newly admin-created user with credentials
func (s *Service) SendAccountCreatedEmail(ctx context.Context, to, username, plainPassword string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	subject := "Your Lesstruct Account Has Been Created"
	body := s.generateAccountCreatedEmailBody(username, plainPassword)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send account created email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send account created email: %w", err)
	}

	s.logger.Info("Account created email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// generatePasswordResetEmailBody generates the email body for password reset
func (s *Service) generatePasswordResetEmailBody(username, resetURL string) emailBody {
	text := fmt.Sprintf(`Hello %s,

You requested a password reset for your Lesstruct account.

Please click the link below to set a new password:
%s

This link will expire in 1 hour.

If you did not request this password reset, please ignore this email.

Best regards,
The Lesstruct Team`, username, resetURL)

	html := htmlWrap(fmt.Sprintf(`
<p>Hello %s,</p>

<p>You requested a password reset for your Lesstruct account.</p>

<p>Please click the button below to set a new password:</p>

<p><a href="%s" class="button">Reset Password</a></p>

<p>This link will expire in 1 hour.</p>

<p>If you did not request this password reset, please ignore this email.</p>`, username, resetURL))

	return emailBody{text: text, html: html}
}

// SendPasswordResetEmail sends a password reset email to a user
func (s *Service) SendPasswordResetEmail(ctx context.Context, to, username, token string) error {
	if !s.enabled {
		s.logger.Warn("Email service is disabled",
			"reason", "SMTP_HOST and SMTP_PORT must be configured",
			"recipient", to)
		return fmt.Errorf("email service is disabled: SMTP_HOST and SMTP_PORT must be configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, token)

	subject := "Reset Your Password - Lesstruct"
	body := s.generatePasswordResetEmailBody(username, resetURL)

	err := s.sendEmail(ctx, to, subject, body)
	if err != nil {
		s.logger.Error("Failed to send password reset email",
			"recipient", to,
			"username", username,
			"error", err)
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	s.logger.Info("Password reset email sent successfully",
		"recipient", to,
		"username", username)

	return nil
}

// NewService creates a new email service
func NewService(config SMTPConfig, logger *slog.Logger, baseURL string) *Service {
	enabled := config.Host != "" && config.Port > 0
	return &Service{
		config:  config,
		timeout: 30 * time.Second,
		enabled: enabled,
		logger:  logger,
		baseURL: baseURL,
	}
}
