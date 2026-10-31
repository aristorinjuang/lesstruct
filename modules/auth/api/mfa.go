package api

import (
	"encoding/json"
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
	// mfaCodeLength is the exact length of an emailed one-time login code.
	mfaCodeLength = 6
)

// isMFACode reports whether code is exactly six ASCII digits.
func isMFACode(code string) bool {
	if len(code) != mfaCodeLength {
		return false
	}

	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// rejectUnlessVerified writes the standard status rejection for accounts that
// may not complete a login and reports whether it rejected.
func rejectUnlessVerified(w http.ResponseWriter, user *userdomain.User) bool {
	switch user.Status {
	case "verified":
		return false
	case "pending":
		httpx.Error(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Please verify your email address before logging in.", nil)
	case "suspended":
		httpx.Error(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended. Please contact an administrator.", nil)
	case "soft_deleted":
		httpx.Error(w, http.StatusForbidden, "ACCOUNT_DELETED", "Your account has been deleted.", nil)
	default:
		httpx.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", nil)
	}

	return true
}

// MFARequiredResponse is the first login step's reply: credentials were
// accepted, and the emailed code must be presented to finish signing in.
type MFARequiredResponse struct {
	MFARequired    bool   `json:"mfaRequired"`
	ChallengeToken string `json:"challengeToken"`
	EmailHint      string `json:"emailHint"`
	ExpiresIn      int    `json:"expiresIn"`
}

// MFAVerifyRequest represents the second login step's request body.
type MFAVerifyRequest struct {
	ChallengeToken string `json:"challengeToken"`
	Code           string `json:"code"`
}

// MFAResendRequest represents the resend request body.
type MFAResendRequest struct {
	ChallengeToken string `json:"challengeToken"`
}

// MFAResendResponse represents the resend response body. A resend replaces the
// challenge, so the new token must travel back to the client.
type MFAResendResponse struct {
	Message        string `json:"message"`
	ChallengeToken string `json:"challengeToken"`
	EmailHint      string `json:"emailHint"`
	ExpiresIn      int    `json:"expiresIn"`
}

// MaskEmail obscures the local part of an address for display and dev-log
// hints. It never reveals enough to confirm an address to an unauthenticated
// caller.
func MaskEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || domain == "" {
		if email == "" {
			return ""
		}

		return "***"
	}

	runes := []rune(local)
	if len(runes) <= 1 {
		return "***@" + domain
	}

	return string(runes[0]) + "***@" + domain
}

// VerifyMFA handles POST /api/auth/mfa/verify and issues the session JWT only
// after the emailed code is accepted.
func (h *AuthHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

	if h.mfaService == nil {
		h.logger.Error("Login MFA is not configured")
		httpx.Error(w, http.StatusServiceUnavailable, "MFA_UNAVAILABLE", "Login verification is unavailable. Please try again later.", nil)
		return
	}

	var req MFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	if req.ChallengeToken == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_CHALLENGE", "A login verification challenge is required", nil)
		return
	}

	if !isMFACode(req.Code) {
		httpx.Error(w, http.StatusBadRequest, "INVALID_CODE_FORMAT", "Verification codes are six digits", nil)
		return
	}

	userID, err := h.mfaService.VerifyChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		switch err {
		case authdomain.ErrMFAChallengeInvalid:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "Your verification session is no longer valid. Please sign in again.", nil)
		case authdomain.ErrMFAChallengeExpired:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_EXPIRED", "Your verification code expired. Please sign in again.", nil)
		case authdomain.ErrMFACodeInvalid:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CODE_INVALID", "Invalid verification code", nil)
		case authdomain.ErrMFACodeMaxAttempts:
			httpx.Error(w, http.StatusUnauthorized, "MFA_MAX_ATTEMPTS", "Too many invalid verification attempts. Please sign in again.", nil)
		default:
			h.logger.Error("Login MFA verification failed: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "MFA_FAILED", "Failed to verify the code", nil)
		}

		return
	}

	user, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to load user after MFA verification: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to process login", nil)
		return
	}

	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", nil)
		return
	}

	if rejectUnlessVerified(w, user) {
		return
	}

	token, err := h.jwtManager.GenerateToken(
		strconv.Itoa(user.ID),
		user.Username,
		user.Role,
		user.TokenVersion,
	)
	if err != nil {
		h.logger.Error("Failed to generate token for user: %s", user.Username)
		httpx.Error(w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "Failed to generate authentication token", nil)
		return
	}

	// The session travels in the HttpOnly cookie, never in the body: read the
	// cookie lifetime from the issued token's own expiry instead of
	// duplicating the TTL.
	expiry, err := h.jwtManager.ValidateTokenExpiry(token)
	if err != nil {
		h.logger.Error("Failed to resolve session expiry for user: %s", user.Username)
		httpx.Error(w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "Failed to generate authentication token", nil)
		return
	}

	kernel.SetSessionCookie(w, r, token, expiry)

	h.logger.Info("Successful MFA login for user: %s (role: %s)", user.Username, user.Role)

	httpx.Success(w, LoginResponse{
		User: UserInfoRef{
			ID:       strconv.Itoa(user.ID),
			Username: user.Username,
			Role:     user.Role,
		},
	})
}

// ResendMFA handles POST /api/auth/mfa/resend, replacing the pending
// challenge with a fresh code subject to the resend cooldown.
func (h *AuthHandler) ResendMFA(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

	if h.mfaService == nil || h.mfaDeliverer == nil {
		h.logger.Error("Login MFA is not configured")
		httpx.Error(w, http.StatusServiceUnavailable, "MFA_UNAVAILABLE", "Login verification is unavailable. Please try again later.", nil)
		return
	}

	var req MFAResendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	if req.ChallengeToken == "" {
		httpx.Error(w, http.StatusBadRequest, "MISSING_CHALLENGE", "A login verification challenge is required", nil)
		return
	}

	// Re-check the account before spending the cooldown and clearing the
	// pending challenge: a suspended or deleted user must not leave an
	// undelivered challenge behind.
	userID, err := h.mfaService.ChallengeUserID(r.Context(), req.ChallengeToken)
	if err != nil {
		switch err {
		case authdomain.ErrMFAChallengeInvalid:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "Your verification session is no longer valid. Please sign in again.", nil)
		case authdomain.ErrMFAChallengeExpired:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_EXPIRED", "Your verification code expired. Please sign in again.", nil)
		default:
			h.logger.Error("Login MFA resend failed: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "MFA_FAILED", "Failed to resend the code", nil)
		}

		return
	}

	user, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to load user for MFA resend: %v", err)
		httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "Your verification session is no longer valid. Please sign in again.", nil)
		return
	}

	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "Your verification session is no longer valid. Please sign in again.", nil)
		return
	}

	if rejectUnlessVerified(w, user) {
		return
	}

	challenge, err := h.mfaService.ResendChallenge(r.Context(), req.ChallengeToken)
	if err != nil {
		switch err {
		case authdomain.ErrMFAChallengeInvalid:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "Your verification session is no longer valid. Please sign in again.", nil)
		case authdomain.ErrMFAResendTooSoon:
			httpx.Error(w, http.StatusTooManyRequests, "MFA_RESEND_COOLDOWN", "Please wait before requesting another code.", nil)
		case authdomain.ErrMFAChallengeExpired:
			httpx.Error(w, http.StatusUnauthorized, "MFA_CHALLENGE_EXPIRED", "Your verification code expired. Please sign in again.", nil)
		default:
			h.logger.Error("Login MFA resend failed: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "MFA_FAILED", "Failed to resend the code", nil)
		}

		return
	}

	if err := h.mfaDeliverer.DeliverMFACode(r.Context(), user.Email, user.Username, challenge.Code); err != nil {
		h.logger.Error("Failed to deliver login MFA code: %v", err)
		httpx.Error(w, http.StatusServiceUnavailable, "MFA_UNAVAILABLE", "We could not send a new code. Please sign in again.", nil)
		return
	}

	httpx.Success(w, MFAResendResponse{
		Message:        "A new verification code has been sent.",
		ChallengeToken: challenge.Token,
		EmailHint:      MaskEmail(user.Email),
		ExpiresIn:      int(time.Until(challenge.ExpiresAt).Seconds()),
	})
}
