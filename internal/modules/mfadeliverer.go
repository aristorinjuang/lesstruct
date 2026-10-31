package modules

import (
	"context"

	"github.com/aristorinjuang/lesstruct/kernel"
	authapi "github.com/aristorinjuang/lesstruct/modules/auth/api"
)

var (
	_ authapi.MFACodeDeliverer = (*EmailMFACodeDeliverer)(nil)
	_ authapi.MFACodeDeliverer = (*LogMFACodeDeliverer)(nil)
)

// MFACodeSender is the email-channel subset the deliverer needs. The core
// email service satisfies it structurally.
type MFACodeSender interface {
	SendMFACodeEmail(ctx context.Context, to, username, code string) error
}

// EmailMFACodeDeliverer delivers login verification codes through the account
// mailer.
type EmailMFACodeDeliverer struct {
	sender MFACodeSender
}

// DeliverMFACode sends the code to the user's email address.
func (d *EmailMFACodeDeliverer) DeliverMFACode(ctx context.Context, to, username, code string) error {
	return d.sender.SendMFACodeEmail(ctx, to, username, code)
}

// NewEmailMFACodeDeliverer creates an email-backed code deliverer.
func NewEmailMFACodeDeliverer(sender MFACodeSender) *EmailMFACodeDeliverer {
	return &EmailMFACodeDeliverer{
		sender: sender,
	}
}

// LogMFACodeDeliverer writes login verification codes to the application log.
// It is selected only when DEV_MODE is on and SMTP is unconfigured — the one
// combination where the code cannot be delivered any other way. Production
// never reaches it: SMTP configured wins, and missing SMTP without DEV_MODE
// uses the email deliverer, which fails closed.
type LogMFACodeDeliverer struct {
	logger kernel.Logger
}

// DeliverMFACode logs the code instead of sending it.
func (d *LogMFACodeDeliverer) DeliverMFACode(_ context.Context, to, username, code string) error {
	d.logger.Info(
		"DEV ONLY (DEV_MODE=true, SMTP unconfigured): login MFA code for %s (%s): %s",
		authapi.MaskEmail(to),
		username,
		code,
	)

	return nil
}

// NewLogMFACodeDeliverer creates a log-only code deliverer.
func NewLogMFACodeDeliverer(logger kernel.Logger) *LogMFACodeDeliverer {
	return &LogMFACodeDeliverer{
		logger: logger,
	}
}

// NewMFACodeDeliverer selects the delivery channel from configuration: email
// whenever SMTP is enabled, and the stdout sink only for the explicitly
// gated dev combination.
func NewMFACodeDeliverer(
	sender MFACodeSender,
	logger kernel.Logger,
	devMode bool,
	emailEnabled bool,
) authapi.MFACodeDeliverer {
	if devMode && !emailEnabled {
		logger.Error(
			"WARNING: DEV_MODE is on and SMTP is unconfigured, so login MFA codes will be written to stdout. Configure SMTP before exposing this instance.",
		)
		return NewLogMFACodeDeliverer(logger)
	}

	return NewEmailMFACodeDeliverer(sender)
}
