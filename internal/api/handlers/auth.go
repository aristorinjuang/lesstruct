package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the successful login response
type LoginResponse struct {
	Token string      `json:"token"`
	User  UserInfoRef `json:"user"`
}

// UserInfoRef represents user information returned in login response
type UserInfoRef struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// RegisterUserRequest represents the registration request body
type RegisterUserRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterUserResponse represents the successful registration response
type RegisterUserResponse struct {
	Message string `json:"message"`
	UserID  int    `json:"userId"`
}

// VerifyEmailResponse represents the successful email verification response
type VerifyEmailResponse struct {
	Message  string `json:"message"`
	Redirect string `json:"redirect"`
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

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	authService          *authdomain.AuthService
	jwtManager           *auth.JWTManager
	logger               *util.Logger
	firstLoginService    *authdomain.FirstLoginService
	registrationService  *authdomain.RegistrationService
	verificationService  *authdomain.VerificationService
	loginService         *authdomain.LoginService
	passwordResetService *authdomain.PasswordResetService
	userRepo             repository.UserRepo
	failedLoginRepo      repository.FailedLoginAttemptRepo
	notificationRepo     repository.NotificationRepo
	emailService         email.EmailService
	blockedEmailRepo     repository.BlockedEmailRepo
}

// sendAccountLockoutEmail sends an email notification about account lockout
func (h *AuthHandler) sendAccountLockoutEmail(ctx context.Context, username string) {
	// Validate username before processing
	if username == "" {
		h.logger.Error("Empty username provided for lockout email")
		return
	}

	// Get user details
	user, err := h.userRepo.GetUserByUsername(ctx, username)
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
		h.logger.Info("Lockout email recently sent, skipping for user: %s", username)
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
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate input before authentication
	if req.Username == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_CREDENTIALS", "Username and password are required", nil)
		return
	}

	// Validate username length and content to prevent abuse
	maxUsernameLength := 255
	if len(req.Username) > maxUsernameLength {
		response.Error(w, http.StatusBadRequest, "INVALID_USERNAME", "Username exceeds maximum length", nil)
		return
	}

	// Check for NULL bytes or other potentially malicious characters
	if strings.Contains(req.Username, "\x00") {
		response.Error(w, http.StatusBadRequest, "INVALID_USERNAME", "Username contains invalid characters", nil)
		return
	}

	// Check if default credentials are being used after first-login setup is complete
	// Only check DB when someone actually tries the default credentials (performance optimization)
	if req.Username == constants.DefaultUsername && req.Password == constants.DefaultPassword {
		admin, err := h.userRepo.GetAdminUser(r.Context())
		if err == nil && h.firstLoginService.IsSetupComplete(admin.PasswordHash) {
			h.logger.Info("Attempted login with default credentials after first-login setup")
			response.Error(w, http.StatusUnauthorized, "DEFAULT_CREDENTIALS_INVALID", "Default credentials are no longer valid. Please use your new password.", nil)
			return
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
			response.Error(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Please verify your email address before logging in.", nil)
			return

		case authdomain.ErrAccountLocked:
			h.logger.Info("Login attempt on locked account: %s", req.Username)

			// Send email notification about account lockout
			h.sendAccountLockoutEmail(r.Context(), req.Username)

			response.Error(w, http.StatusForbidden, "ACCOUNT_LOCKED", "Account locked due to too many failed login attempts. Please try again later.", nil)
			return

		case authdomain.ErrAccountSuspended:
			h.logger.Info("Login attempt on suspended account: %s", req.Username)
			response.Error(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended. Please contact an administrator.", nil)
			return

		case authdomain.ErrAccountDeleted:
			h.logger.Info("Login attempt on deleted account: %s", req.Username)
			response.Error(w, http.StatusForbidden, "ACCOUNT_DELETED", "Your account has been deleted.", nil)
			return

		case authdomain.ErrInvalidCredentials:
			h.logger.Info("Failed login attempt for username: %s", req.Username)
			response.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", nil)
			return

		default:
			h.logger.Error("Login error for username %s: %v", req.Username, err)
			response.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
			return
		}
	}

	// Validate loginResult is not nil (defensive check)
	if loginResult == nil {
		h.logger.Error("Login returned nil result for username: %s", req.Username)
		response.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
		return
	}

	// Validate loginResult fields are not zero/empty
	if loginResult.UserID == "" || loginResult.Username == "" || loginResult.Role == "" {
		h.logger.Error("Login returned invalid result for username: %s - UserID=%s, Username=%s, Role=%s",
			req.Username, loginResult.UserID, loginResult.Username, loginResult.Role)
		response.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(loginResult.UserID, loginResult.Username, loginResult.Role)
	if err != nil {
		h.logger.Error("Failed to generate token for user: %s", loginResult.Username)
		response.Error(w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "Failed to generate authentication token", nil)
		return
	}

	// Log successful login
	h.logger.Info("Successful login for user: %s (role: %s)", loginResult.Username, loginResult.Role)

	// Return success response with token and user info
	response.Success(w, LoginResponse{
		Token: token,
		User: UserInfoRef{
			ID:       loginResult.UserID,
			Username: loginResult.Username,
			Role:     loginResult.Role,
		},
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
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Username, email, and password are required", nil)
		return
	}

	// Validate input lengths
	if len(req.Username) > 50 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Username must be 50 characters or less", nil)
		return
	}
	if len(req.Email) > 255 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email must be 255 characters or less", nil)
		return
	}
	if len(req.Password) > 128 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Password must be 128 characters or less", nil)
		return
	}

	// Check if email is blocked (Task 1.5: User Registration Management)
	blocked, err := h.blockedEmailRepo.IsEmailBlocked(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to check if email is blocked: %v", err)
		response.Error(w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to process registration", nil)
		return
	}
	if blocked {
		response.Error(w, http.StatusForbidden, "EMAIL_BLOCKED", "This email address has been blocked", nil)
		return
	}

	// Register user with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := h.registrationService.RegisterUser(ctx, authdomain.RegisterRequest{
		Username: req.Username,
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})

	// Handle specific errors
	if err != nil {
		switch err {
		case authdomain.ErrUsernameInvalid:
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid username format", nil)
		case authdomain.ErrUsernameExists, authdomain.ErrEmailExists:
			// Use generic error to prevent email enumeration
			response.Error(w, http.StatusConflict, "ACCOUNT_EXISTS", "An account with this username or email already exists", nil)
		case auth.ErrPasswordInvalid, auth.ErrEmailInvalid:
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid email or password", nil)
		default:
			h.logger.Error("Registration failed: %v", err)
			response.Error(w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to register user", nil)
		}
		return
	}

	h.logger.Info("New user registered: %s (ID: %d, Status: pending)", req.Username, result.UserID)

	// Create notification for admin
	if err := h.notificationRepo.CreateNotification(r.Context(), repository.NotificationTypeUserRegistered,
		fmt.Sprintf("New user registration: %s (%s)", html.EscapeString(req.Username), html.EscapeString(req.Email))); err != nil {
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

	response.SendJSON(w, http.StatusCreated, response.Response{
		Data: RegisterUserResponse{
			Message: result.Message,
			UserID:  result.UserID,
		},
	})
}

// VerifyEmail handles GET /api/auth/verify-email
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// Get token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid verification link", nil)
		return
	}

	// Validate token length (valid tokens are ~70 chars max)
	if len(token) > 512 {
		response.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid verification link", nil)
		return
	}

	// Verify email
	result, err := h.verificationService.VerifyEmail(r.Context(), token)

	// Handle specific errors
	if err != nil {
		switch err {
		case authdomain.ErrTokenInvalid:
			response.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid verification link", nil)
		case authdomain.ErrTokenExpired:
			response.Error(w, http.StatusBadRequest, "TOKEN_EXPIRED", "Verification link has expired. Please request a new verification link", nil)
		default:
			h.logger.Error("Email verification failed: %v", err)
			response.Error(w, http.StatusInternalServerError, "VERIFICATION_FAILED", "Failed to verify email", nil)
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
		if err := h.notificationRepo.CreateNotification(r.Context(), repository.NotificationTypeUserVerified,
			fmt.Sprintf("User %s verified their email (%s)", html.EscapeString(user.Username), html.EscapeString(user.Email))); err != nil {
			h.logger.Error("Failed to create notification: %v", err)
			// Don't fail verification if notification fails
		}
	}

	response.Success(w, VerifyEmailResponse{
		Message:  result.Message,
		Redirect: "/admin/login",
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
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	// Validate email
	if req.Email == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_EMAIL", "Email address is required", nil)
		return
	}

	// Find user by email
	user, err := h.userRepo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to lookup user by email: %v", err)
		// Return generic success to prevent enumeration (consistent with other error handling)
		response.Success(w, ResendVerificationResponse{
			Message: "If your email is registered and pending verification, you will receive a verification link.",
		})
		return
	}

	// Don't reveal if email exists or not (security best practice)
	if user == nil {
		// Return success even if user doesn't exist to prevent email enumeration
		h.logger.Info("Resend verification requested for non-existent email: %s", req.Email)
		response.Success(w, ResendVerificationResponse{
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
		response.Success(w, ResendVerificationResponse{
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
		response.Success(w, ResendVerificationResponse{
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

	response.Success(w, ResendVerificationResponse{
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
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	if req.Email == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Email is required", nil)
		return
	}

	if len(req.Email) > 255 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email must be 255 characters or less", nil)
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

	response.Success(w, ResendVerificationResponse{
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
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_FIELDS", "Token and new password are required", nil)
		return
	}

	if len(req.Token) > 512 {
		response.Error(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid token", nil)
		return
	}

	if len(req.NewPassword) > 128 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Password must be 128 characters or less", nil)
		return
	}

	result, err := h.passwordResetService.ResetPassword(r.Context(), req.Token, req.NewPassword)
	if err != nil {
		switch err {
		case authdomain.ErrResetTokenInvalid:
			response.Error(w, http.StatusBadRequest, "TOKEN_INVALID", "This password reset link is invalid.", nil)
		case authdomain.ErrResetTokenExpired:
			response.Error(w, http.StatusBadRequest, "TOKEN_EXPIRED", "This password reset link has expired. Please request a new one.", nil)
		case auth.ErrPasswordInvalid:
			response.Error(w, http.StatusBadRequest, "INVALID_PASSWORD", "Password must be at least 12 characters with uppercase, lowercase, numbers, and special characters.", nil)
		default:
			h.logger.Error("Password reset failed: %v", err)
			response.Error(w, http.StatusInternalServerError, "RESET_FAILED", "Failed to reset password", nil)
		}
		return
	}

	h.logger.Info("Password reset successfully for user ID: %d", result.UserID)

	response.Success(w, ResendVerificationResponse{
		Message: "Password reset successfully",
	})
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	authService *authdomain.AuthService,
	jwtManager *auth.JWTManager,
	logger *util.Logger,
	firstLoginService *authdomain.FirstLoginService,
	registrationService *authdomain.RegistrationService,
	verificationService *authdomain.VerificationService,
	loginService *authdomain.LoginService,
	passwordResetService *authdomain.PasswordResetService,
	userRepo repository.UserRepo,
	failedLoginRepo repository.FailedLoginAttemptRepo,
	notificationRepo repository.NotificationRepo,
	emailService email.EmailService,
	blockedEmailRepo repository.BlockedEmailRepo,
) *AuthHandler {
	return &AuthHandler{
		authService:          authService,
		jwtManager:           jwtManager,
		logger:               logger,
		firstLoginService:    firstLoginService,
		registrationService:  registrationService,
		verificationService:  verificationService,
		loginService:         loginService,
		passwordResetService: passwordResetService,
		userRepo:             userRepo,
		failedLoginRepo:      failedLoginRepo,
		notificationRepo:     notificationRepo,
		emailService:         emailService,
		blockedEmailRepo:     blockedEmailRepo,
	}
}
