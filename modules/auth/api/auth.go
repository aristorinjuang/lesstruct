package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

const (
	// registrationAcceptedMessage is the single reply every accepted-shaped
	// registration gets, success or duplicate alike.
	registrationAcceptedMessage = "Registration successful. Please check your email to verify your account."
	// logoutMessage is the single reply every successful logout gets.
	logoutMessage = "Logged out successfully."
)

// FirstLoginRequiredResponse tells the client to open the first-login setup
// page instead of completing a session. The bootstrap admin has no email
// address until setup finishes, so no code can be delivered yet.
type FirstLoginRequiredResponse struct {
	FirstLoginRequired bool `json:"firstLoginRequired"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	// Username carries the login identifier: a username or an email address.
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the successful login response. The session
// travels in the HttpOnly cookie, never in the body.
type LoginResponse struct {
	User UserInfoRef `json:"user"`
}

// UserInfoRef represents user information returned in login response
type UserInfoRef struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// RegisterUserRequest represents the registration request body
type RegisterUserRequest struct {
	Username        string `json:"username"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConsentAccepted bool   `json:"consentAccepted,omitempty"`
}

// RegisterUserResponse is the registration response. Deliberately minimal:
// a duplicate username or email must be indistinguishable from success, so
// the body never includes the new account's ID (a conflict has none) and
// both outcomes share one message and status.
type RegisterUserResponse struct {
	Message string `json:"message"`
}

// VerifyEmailResponse represents the successful email verification response
type VerifyEmailResponse struct {
	Message string `json:"message"`
	// AwaitingApproval reports that the email address was proven but the account
	// stays pending because the site requires admin approval for activation.
	AwaitingApproval bool   `json:"awaitingApproval"`
	Redirect         string `json:"redirect"`
}

// ResendVerificationRequest represents the resend verification request body
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// ResendVerificationResponse represents the successful resend response
type ResendVerificationResponse struct {
	Message string `json:"message"`
}

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest represents the reset password request body
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// LogoutResponse represents the successful logout response. Sessions are
// revoked server-side and the session cookie is cleared.
type LogoutResponse struct {
	Message string `json:"message"`
}

// AuthHandler handles authentication HTTP requests
// AuthHandlerOption customizes an AuthHandler at construction time
type AuthHandlerOption func(*AuthHandler)

// WithAdminApprovalRequired configures the handler for sites where registrants
// must be approved by an administrator after verifying their email. A pending
// registrant whose email is already verified is not issued another verification
// link (the reply stays generic for anti-enumeration); without this option the
// resend endpoint always re-issues a link for pending registrants (legacy).
func WithAdminApprovalRequired() AuthHandlerOption {
	return func(h *AuthHandler) {
		h.adminApprovalRequired = true
	}
}

type AuthHandler struct {
	authService           *authdomain.AuthService
	jwtManager            TokenIssuer
	logger                kernel.Logger
	firstLoginService     *authdomain.FirstLoginService
	registrationService   *authdomain.RegistrationService
	verificationService   *authdomain.VerificationService
	loginService          *authdomain.LoginService
	mfaService            *authdomain.MFAService
	passwordResetService  *authdomain.PasswordResetService
	userRepo              UserStore
	failedLoginRepo       authdomain.FailedLoginAttemptRepo
	notificationRepo      NotificationRecorder
	emailService          Mailer
	mfaDeliverer          MFACodeDeliverer
	blockedEmailRepo      BlockedEmailChecker
	adminApprovalRequired bool
}

// findUser resolves a login identifier to a user. Identifiers containing an
// "@" are email addresses; usernames are validated without "@", so the
// distinction is unambiguous.
func (h *AuthHandler) findUser(ctx context.Context, identifier string) (*userdomain.User, error) {
	if authdomain.IsEmailIdentifier(identifier) {
		return h.userRepo.GetUserByEmail(ctx, identifier)
	}
	return h.userRepo.GetUserByUsername(ctx, identifier)
}

// sendAccountLockoutEmail sends an email notification about account lockout
func (h *AuthHandler) sendAccountLockoutEmail(ctx context.Context, identifier string) {
	// Validate username before processing
	if identifier == "" {
		h.logger.Error("Empty username provided for lockout email")
		return
	}

	// Get user details
	user, err := h.findUser(ctx, identifier)
	if err != nil || user == nil {
		h.logger.Error("Failed to get user for lockout email: %v", err)
		return
	}

	// Validate user data before sending email
	if user.Email == "" || user.Username == "" {
		h.logger.Error("Invalid user data for lockout email: email=%s, username=%s", user.Email, user.Username)
		return
	}

	// Check if we should send the email (prevent email spam - minimum 5 minutes between emails)
	const minEmailInterval = 5 * time.Minute
	shouldSend, err := h.failedLoginRepo.ShouldSendLockoutEmail(ctx, user.ID, minEmailInterval)
	if err != nil {
		h.logger.Error("Failed to check if lockout email should be sent: %v", err)
		return
	}
	if !shouldSend {
		h.logger.Info("Lockout email recently sent, skipping for user: %s", user.Username)
		return
	}

	// Update the last email sent timestamp
	if err := h.failedLoginRepo.UpdateLastEmailSent(ctx, user.ID); err != nil {
		h.logger.Error("Failed to update last email sent timestamp: %v", err)
		// Continue anyway - sending the email is more important
	}

	// Send lockout email asynchronously with rate limiting
	// Use a semaphore to limit concurrent goroutines
	const maxConcurrentEmails = 10
	emailSemaphore := make(chan struct{}, maxConcurrentEmails)
	emailSemaphore <- struct{}{} // Acquire semaphore

	go func() {
		defer func() { <-emailSemaphore }() // Release semaphore

		// Use background context with timeout to avoid request cancellation
		emailCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Add panic recovery to prevent process crash
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in lockout email sending: %v", r)
			}
		}()

		if err := h.emailService.SendAccountLockoutEmail(emailCtx, user.Email, user.Username); err != nil {
			h.logger.Error("Failed to send account lockout email: %v", err)
		} else {
			h.logger.Info("Account lockout email sent to: %s", user.Email)
		}
	}()
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// Limit request body to 1MB to prevent DoS attacks (must be before reading body)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate input before authentication
	if req.Username == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_CREDENTIALS", "Username and password are required", nil)
		return
	}

	// Validate username length and content to prevent abuse
	maxUsernameLength := 255
	if len(req.Username) > maxUsernameLength {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USERNAME", "Username exceeds maximum length", nil)
		return
	}

	// Check for NULL bytes or other potentially malicious characters
	if strings.Contains(req.Username, "\x00") {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USERNAME", "Username contains invalid characters", nil)
		return
	}

	// Check if default credentials are being used after first-login setup is complete
	// Only check DB when someone actually tries the default credentials (performance optimization)
	bootstrapSetup := false
	if req.Username == authdomain.DefaultUsername && req.Password == authdomain.DefaultPassword {
		admin, err := h.userRepo.GetAdminUser(r.Context())
		switch {
		case err != nil && !errors.Is(err, userdomain.ErrAdminNotFound):
			// A missing admin is a normal invalid-credentials case. Any other
			// lookup failure must fail closed: falling through would let the
			// default credentials start an MFA flow without the setup step.
			h.logger.Error("Failed to check default admin credentials: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
			return
		case err == nil && admin != nil && h.firstLoginService.IsSetupComplete(admin.PasswordHash):
			h.logger.Info("Attempted login with default credentials after first-login setup")
			httpx.Error(w, http.StatusUnauthorized, "DEFAULT_CREDENTIALS_INVALID", "Default credentials are no longer valid. Please use your new password.", nil)
			return
		case err == nil && admin != nil:
			bootstrapSetup = true
		}
	}

	// Validate credentials using LoginService (includes account lockout and status check)
	loginResult, err := h.loginService.Login(r.Context(), authdomain.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})

	// Handle specific error cases
	if err != nil {
		switch err {
		case authdomain.ErrEmailNotVerified:
			h.logger.Info("Login attempt with unverified email: %s", req.Username)
			httpx.Error(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Please verify your email address before logging in.", nil)
			return

		case authdomain.ErrAccountLocked:
			h.logger.Info("Login attempt on locked account: %s", req.Username)

			// Send email notification about account lockout
			h.sendAccountLockoutEmail(r.Context(), req.Username)

			httpx.Error(w, http.StatusForbidden, "ACCOUNT_LOCKED", "Account locked due to too many failed login attempts. Please try again later.", nil)
			return

		case authdomain.ErrAccountSuspended:
			h.logger.Info("Login attempt on suspended account: %s", req.Username)
			httpx.Error(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended. Please contact an administrator.", nil)
			return

		case authdomain.ErrAccountDeleted:
			h.logger.Info("Login attempt on deleted account: %s", req.Username)
			httpx.Error(w, http.StatusForbidden, "ACCOUNT_DELETED", "Your account has been deleted.", nil)
			return

		case authdomain.ErrInvalidCredentials:
			h.logger.Info("Failed login attempt for username: %s", req.Username)
			httpx.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", nil)
			return

		default:
			h.logger.Error("Login error for username %s: %v", req.Username, err)
			httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
			return
		}
	}

	// Validate loginResult is not nil (defensive check)
	if loginResult == nil {
		h.logger.Error("Login returned nil result for username: %s", req.Username)
		httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
		return
	}

	// Validate loginResult fields are not zero/empty
	if loginResult.UserID == "" || loginResult.Username == "" || loginResult.Role == "" {
		h.logger.Error("Login returned invalid result for username: %s - UserID=%s, Username=%s, Role=%s",
			req.Username, loginResult.UserID, loginResult.Username, loginResult.Role)
		httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
		return
	}

	// The bootstrap admin has no email address until setup finishes, so it
	// cannot receive a code. Send the client to the setup page without issuing
	// a session; every later login, including this account's, uses MFA.
	if bootstrapSetup {
		h.logger.Info("Default admin requires first-login setup")
		httpx.Success(w, FirstLoginRequiredResponse{FirstLoginRequired: true})
		return
	}

	// Credentials are correct: start the second factor. A JWT is only issued
	// after the emailed code is verified, and a missing MFA wiring fails
	// closed rather than degrading to password-only login.
	if h.mfaService == nil || h.mfaDeliverer == nil {
		h.logger.Error("Login MFA is not configured")
		httpx.Error(w, http.StatusServiceUnavailable, "MFA_UNAVAILABLE", "Login verification is unavailable. Please try again later.", nil)
		return
	}

	challenge, err := h.mfaService.CreateChallenge(r.Context(), loginResult.ID)
	if err != nil {
		h.logger.Error("Failed to create login challenge for user: %s", loginResult.Username)
		httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
		return
	}

	if err := h.mfaDeliverer.DeliverMFACode(r.Context(), loginResult.Email, loginResult.Username, challenge.Code); err != nil {
		h.logger.Error("Failed to deliver login MFA code for user: %s", loginResult.Username)
		httpx.Error(w, http.StatusServiceUnavailable, "MFA_UNAVAILABLE", "We could not send your verification code. Please try again later.", nil)
		return
	}

	h.logger.Info("Login password accepted for user: %s; MFA code sent", loginResult.Username)

	httpx.Success(w, MFARequiredResponse{
		MFARequired:    true,
		ChallengeToken: challenge.Token,
		EmailHint:      MaskEmail(loginResult.Email),
		ExpiresIn:      int(time.Until(challenge.ExpiresAt).Seconds()),
	})
}

// RegisterUser handles POST /api/auth/register
func (h *AuthHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// Limit request body to 1MB to prevent DoS attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req RegisterUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Username, email, and password are required", nil)
		return
	}

	// Validate input lengths
	if len(req.Username) > 50 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Username must be 50 characters or less", nil)
		return
	}
	if len(req.Email) > 255 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email must be 255 characters or less", nil)
		return
	}
	if len(req.Password) > 128 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Password must be 128 characters or less", nil)
		return
	}

	// Check if registration is enabled before any repository work: the
	// blocked-email lookup below would otherwise run (and leak whether the
	// submitted email is blocked) even on instances that disable registration.
	if !h.registrationService.RegistrationEnabled() {
		httpx.Error(w, http.StatusForbidden, "REGISTRATION_DISABLED", "Self-registration is disabled on this instance", nil)
		return
	}

	// Check if email is blocked (Task 1.5: User Registration Management)
	blocked, err := h.blockedEmailRepo.IsEmailBlocked(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to check if email is blocked: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to process registration", nil)
		return
	}
	if blocked {
		httpx.Error(w, http.StatusForbidden, "EMAIL_BLOCKED", "This email address has been blocked", nil)
		return
	}

	// Register user with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := h.registrationService.RegisterUser(ctx, authdomain.RegisterRequest{
		Username:        req.Username,
		Name:            req.Name,
		Email:           req.Email,
		Password:        req.Password,
		ConsentAccepted: req.ConsentAccepted,
	})

	// Handle specific errors
	if err != nil {
		switch err {
		case authdomain.ErrRegistrationDisabled:
			httpx.Error(w, http.StatusForbidden, "REGISTRATION_DISABLED", "Self-registration is disabled on this instance", nil)
		case authdomain.ErrConsentRequired:
			httpx.Error(w, http.StatusBadRequest, "CONSENT_REQUIRED", "Consent to the terms and privacy policy is required", nil)
		case authdomain.ErrUsernameInvalid:
			httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid username format", nil)
		case authdomain.ErrUsernameExists, authdomain.ErrEmailExists:
			// Anti-enumeration: a duplicate username or email reads exactly
			// like a fresh registration. The account holder — never the
			// requester — is the one who gets told, by email.
			h.notifyRegistrationConflict(r.Context(), req.Username, req.Email)
			httpx.SendJSON(w, http.StatusOK, httpx.Response{
				Data: RegisterUserResponse{Message: registrationAcceptedMessage},
			})
		case authdomain.ErrInvalidPassword:
			httpx.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", authdomain.ErrInvalidPassword.Error(), nil)
		case authdomain.ErrInvalidEmail:
			httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid email format", nil)
		default:
			h.logger.Error("Registration failed: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to register user", nil)
		}
		return
	}

	h.logger.Info("New user registered: %s (ID: %d, Status: pending)", req.Username, result.UserID)

	// Create notification for admin
	if err := h.notificationRepo.RecordUserRegistered(r.Context(), req.Username, req.Email); err != nil {
		h.logger.Error("Failed to create notification: %v", err)
		// Don't fail registration if notification fails
	}

	// Create verification token BEFORE returning response
	tokenResult, err := h.verificationService.CreateVerificationToken(r.Context(), result.UserID)
	if err != nil {
		// Token creation failed - log but don't fail registration
		h.logger.Error("Failed to create verification token: %v", err)
		tokenResult = nil
	}

	if tokenResult != nil {
		// Send verification email asynchronously
		go func() {
			// Use background context with timeout to avoid request cancellation
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Add panic recovery to prevent process crash
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("Panic in email sending: %v", r)
				}
			}()

			if err := h.emailService.SendVerificationEmail(ctx, req.Email, req.Username, tokenResult.Token); err != nil {
				h.logger.Error("Failed to send verification email: %v", err)
				// Email sending failure is not critical for registration
			}
		}()
	}

	httpx.SendJSON(w, http.StatusOK, httpx.Response{
		Data: RegisterUserResponse{Message: registrationAcceptedMessage},
	})
}

// notifyRegistrationConflict resolves the account a registration attempt
// collided with and emails its holder. Lookup by email first (the address is
// provably theirs), falling back to the username's account when the submitted
// email is new. The send is asynchronous and best-effort — the response is
// already on the wire and must not depend on it — and the notice carries no
// links, so it can never be turned into a phishing vector.
func (h *AuthHandler) notifyRegistrationConflict(ctx context.Context, username, email string) {
	existing, err := h.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		h.logger.Error("Failed to look up conflicting account by email: %v", err)
		return
	}
	if existing == nil {
		existing, err = h.userRepo.GetUserByUsername(ctx, username)
		if err != nil {
			h.logger.Error("Failed to look up conflicting account by username: %v", err)
			return
		}
	}
	if existing == nil || existing.Email == "" {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in registration-conflict email sending: %v", r)
			}
		}()

		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := h.emailService.SendRegistrationAttemptEmail(sendCtx, existing.Email, existing.Username); err != nil {
			h.logger.Error("Failed to send registration attempt email: %v", err)
		}
	}()
}

// VerifyEmail handles GET /api/auth/verify-email
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// Get token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid verification link", nil)
		return
	}

	// Validate token length (valid tokens are ~70 chars max)
	if len(token) > 512 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid verification link", nil)
		return
	}

	// Verify email
	result, err := h.verificationService.VerifyEmail(r.Context(), token)

	// Handle specific errors
	if err != nil {
		switch err {
		case authdomain.ErrTokenInvalid:
			httpx.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid verification link", nil)
		case authdomain.ErrTokenExpired:
			httpx.Error(w, http.StatusBadRequest, "TOKEN_EXPIRED", "Verification link has expired. Please request a new verification link", nil)
		default:
			h.logger.Error("Email verification failed: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "VERIFICATION_FAILED", "Failed to verify email", nil)
		}
		return
	}

	h.logger.Info("Email verified successfully")

	// Get user details for notification
	user, err := h.userRepo.GetUserByID(r.Context(), result.UserID)
	if err != nil {
		h.logger.Error("Failed to fetch user for notification: %v", err)
		// Don't fail verification if user lookup fails - notification is optional
		// Continue with success response
	} else if user != nil {
		// Create notification for admin
		if err := h.notificationRepo.RecordUserVerified(r.Context(), user.Username, user.Email); err != nil {
			h.logger.Error("Failed to create notification: %v", err)
			// Don't fail verification if notification fails
		}
	}

	httpx.Success(w, VerifyEmailResponse{
		Message:          result.Message,
		AwaitingApproval: result.AwaitingApproval,
		Redirect:         "/admin/login",
	})
}

// ResendVerificationEmail handles POST /api/auth/resend-verification
func (h *AuthHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// Limit request body to 1MB to prevent DoS attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate email
	if req.Email == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_EMAIL", "Email address is required", nil)
		return
	}

	// Find user by email
	user, err := h.userRepo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to lookup user by email: %v", err)
		// Return generic success to prevent enumeration (consistent with other error handling)
		httpx.Success(w, ResendVerificationResponse{
			Message: "If your email is registered and pending verification, you will receive a verification link.",
		})
		return
	}

	// Don't reveal if email exists or not (security best practice)
	if user == nil {
		// Return success even if user doesn't exist to prevent email enumeration
		h.logger.Info("Resend verification requested for non-existent email: %s", req.Email)
		httpx.Success(w, ResendVerificationResponse{
			Message: "If your email is registered and pending verification, you will receive a verification link.",
		})
		return
	}

	// Check if user is still pending
	if user.Status != "pending" {
		// User is already verified, suspended, or in another state
		// Log unexpected status values for monitoring
		if user.Status != "verified" && user.Status != "suspended" {
			h.logger.Info("Resend verification requested for user with unexpected status: %s (status: %s)", req.Email, user.Status)
		} else {
			h.logger.Info("Resend verification requested for non-pending user: %s (status: %s)", req.Email, user.Status)
		}

		// Return generic message to prevent enumeration
		httpx.Success(w, ResendVerificationResponse{
			Message: "If your email is registered and pending verification, you will receive a verification link.",
		})
		return
	}

	// In admin-approval mode, don't re-issue a token for a pending user whose
	// email is already verified: the proof step is done and the account is only
	// waiting on admin approval, so another link would be pointless. Same
	// generic reply (anti-enumeration).
	//
	// Without admin-approval mode this gate must NOT apply: a pending user with
	// a verified email can only exist there through a partial failure or a
	// config flip, and refusing a new link would leave the account permanently
	// stuck — resend must keep working (legacy behavior).
	if h.adminApprovalRequired && user.EmailVerified {
		h.logger.Info("Resend verification requested for pending user with verified email: %s", req.Email)
		httpx.Success(w, ResendVerificationResponse{
			Message: "If your email is registered and pending verification, you will receive a verification link.",
		})
		return
	}

	// Generate new verification token
	tokenResult, err := h.verificationService.CreateVerificationToken(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("Failed to create verification token for user %d: %v", user.ID, err)
		// Return generic success to prevent enumeration
		// (Consistent with non-existent user handling)
		httpx.Success(w, ResendVerificationResponse{
			Message: "If your email is registered and pending verification, you will receive a verification link.",
		})
		return
	}

	// Send verification email asynchronously
	go func() {
		// Use background context with timeout to avoid request cancellation
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Add panic recovery to prevent process crash
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in email sending: %v", r)
			}
		}()

		if err := h.emailService.SendVerificationEmail(ctx, req.Email, user.Username, tokenResult.Token); err != nil {
			h.logger.Error("Failed to send verification email: %v", err)
			// Email sending failure is not critical for resend
		}
	}()

	h.logger.Info("Verification email resent successfully to: %s", req.Email)

	httpx.Success(w, ResendVerificationResponse{
		Message: "If your email is registered and pending verification, you will receive a verification link.",
	})
}

// ForgotPassword handles POST /api/auth/forgot-password
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	if req.Email == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Email is required", nil)
		return
	}

	if len(req.Email) > 255 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email must be 255 characters or less", nil)
		return
	}

	result, err := h.passwordResetService.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to process forgot password request: %v", err)
	}

	if result != nil {
		go func() {
			emailCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("Panic in password reset email sending: %v", r)
				}
			}()

			if err := h.emailService.SendPasswordResetEmail(emailCtx, result.Email, result.Name, result.Token); err != nil {
				h.logger.Error("Failed to send password reset email: %v", err)
			}
		}()
	}

	httpx.Success(w, ResendVerificationResponse{
		Message: "If an account exists with this email, a password reset link has been sent.",
	})
}

// ResetPassword handles POST /api/auth/reset-password
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Token and new password are required", nil)
		return
	}

	if len(req.Token) > 512 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid token", nil)
		return
	}

	if len(req.NewPassword) > 128 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Password must be 128 characters or less", nil)
		return
	}

	result, err := h.passwordResetService.ResetPassword(r.Context(), req.Token, req.NewPassword)
	if err != nil {
		switch err {
		case authdomain.ErrResetTokenInvalid:
			httpx.Error(w, http.StatusBadRequest, "TOKEN_INVALID", "This password reset link is invalid.", nil)
		case authdomain.ErrResetTokenExpired:
			httpx.Error(w, http.StatusBadRequest, "TOKEN_EXPIRED", "This password reset link has expired. Please request a new one.", nil)
		case authdomain.ErrInvalidPassword:
			httpx.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", "Password must be at least 12 characters with uppercase, lowercase, numbers, and special characters.", nil)
		default:
			h.logger.Error("Password reset failed: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "RESET_FAILED", "Failed to reset password", nil)
		}
		return
	}

	h.logger.Info("Password reset successfully for user ID: %d", result.UserID)

	httpx.Success(w, ResendVerificationResponse{
		Message: "Password reset successfully",
	})
}

// Logout handles POST /api/auth/logout. It revokes every session for the
// caller by bumping the stored token version — previously issued Bearer
// tokens and session cookies stop validating on the next request — and
// clears the session cookie with matching attributes. It runs behind
// RequireAuth, so a missing credential never reaches it; the defensive
// rejection below only covers a forged context.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || principal.ID == "" {
		httpx.Error(w, http.StatusUnauthorized, "MISSING_TOKEN", "Missing authorization token", nil)
		return
	}

	userID, err := strconv.Atoi(principal.ID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid authorization token", nil)
		return
	}

	if err := h.userRepo.RevokeSessions(r.Context(), userID); err != nil {
		h.logger.Error("Failed to revoke sessions for user %s: %v", principal.ID, err)
		httpx.Error(w, http.StatusInternalServerError, "LOGOUT_FAILED", "Failed to log out", nil)
		return
	}

	kernel.ClearSessionCookie(w, r)

	h.logger.Info("Successful logout for user: %s", principal.Username)

	httpx.Success(w, LogoutResponse{
		Message: logoutMessage,
	})
}

// NewAuthHandler creates a new auth handler. A nil logger degrades to a
// discard sink so the handler is safe to construct in any context.
func NewAuthHandler(
	authService *authdomain.AuthService,
	jwtManager TokenIssuer,
	logger kernel.Logger,
	firstLoginService *authdomain.FirstLoginService,
	registrationService *authdomain.RegistrationService,
	verificationService *authdomain.VerificationService,
	loginService *authdomain.LoginService,
	mfaService *authdomain.MFAService,
	passwordResetService *authdomain.PasswordResetService,
	userRepo UserStore,
	failedLoginRepo authdomain.FailedLoginAttemptRepo,
	notificationRepo NotificationRecorder,
	emailService Mailer,
	mfaDeliverer MFACodeDeliverer,
	blockedEmailRepo BlockedEmailChecker,
	opts ...AuthHandlerOption,
) *AuthHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	h := &AuthHandler{
		authService:          authService,
		jwtManager:           jwtManager,
		logger:               logger,
		firstLoginService:    firstLoginService,
		registrationService:  registrationService,
		verificationService:  verificationService,
		loginService:         loginService,
		mfaService:           mfaService,
		passwordResetService: passwordResetService,
		userRepo:             userRepo,
		failedLoginRepo:      failedLoginRepo,
		notificationRepo:     notificationRepo,
		emailService:         emailService,
		mfaDeliverer:         mfaDeliverer,
		blockedEmailRepo:     blockedEmailRepo,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
