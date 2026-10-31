package auth

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"time"
)

const (
	// mfaCodeDigits is the length of the emailed one-time login code.
	mfaCodeDigits = 6
	// mfaCodeModulus bounds the random draw to the code space (10^6).
	mfaCodeModulus = 1000000

	// MFACodeTTL is how long an emailed login code stays valid.
	MFACodeTTL time.Duration = 5 * time.Minute

	// MFAResendCooldown is the minimum wait between code re-issues.
	MFAResendCooldown time.Duration = 60 * time.Second

	// MFAMaxAttempts is the wrong-code budget per challenge. The final
	// mismatch discards the challenge.
	MFAMaxAttempts = 5
)

var (
	// ErrMFAChallengeInvalid is returned when a login challenge cannot be
	// found (unknown token, consumed token, or cleared by a newer login).
	ErrMFAChallengeInvalid = errors.New("login verification challenge is invalid")
	// ErrMFAChallengeExpired is returned when a login challenge passed its
	// expiration. The challenge is discarded so the user must sign in again.
	ErrMFAChallengeExpired = errors.New("login verification challenge has expired")

	// ErrMFACodeInvalid is returned when the submitted code does not match.
	ErrMFACodeInvalid = errors.New("invalid login verification code")

	// ErrMFACodeMaxAttempts is returned when the challenge burned through its
	// wrong-code budget. The challenge is discarded, forcing a fresh sign-in.
	ErrMFACodeMaxAttempts = errors.New("too many invalid login verification attempts")

	// ErrMFAResendTooSoon is returned when a resend arrives inside the
	// cooldown window.
	ErrMFAResendTooSoon = errors.New("login verification code was requested too recently")
)

// generateMFACode returns a cryptographically random, zero-padded one-time
// login code.
func generateMFACode() (string, error) {
	n, err := cryptorand.Int(randReader, big.NewInt(mfaCodeModulus))
	if err != nil {
		return "", fmt.Errorf("failed to generate login verification code: %w", err)
	}
	return fmt.Sprintf("%0*d", mfaCodeDigits, n.Int64()), nil
}

// MFAChallenge is a pending second-factor challenge: the hashed proof token
// issued after a correct password, the hashed emailed code, and its budget.
type MFAChallenge struct {
	ID        int
	UserID    int
	TokenHash string
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

// MFAChallengeCode carries the plaintext challenge token and code returned by
// the service for delivery. Neither value is ever stored in plaintext.
type MFAChallengeCode struct {
	Token     string
	Code      string
	UserID    int
	ExpiresAt time.Time
}

// MFAService issues and verifies the emailed one-time codes that complete a
// login after the password step.
type MFAService struct {
	repo           MFAChallengeRepo
	ttl            time.Duration
	resendCooldown time.Duration
	maxAttempts    int
}

// CreateChallenge discards any pending challenge for the user and issues a
// fresh token/code pair.
func (s *MFAService) CreateChallenge(ctx context.Context, userID int) (*MFAChallengeCode, error) {
	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	code, err := generateMFACode()
	if err != nil {
		return nil, err
	}

	// Best-effort sweep of abandoned challenges; a cleanup failure must not
	// block the new login attempt.
	_ = s.repo.DeleteExpired(ctx)

	if err := s.repo.DeleteUserChallenges(ctx, userID); err != nil {
		return nil, fmt.Errorf("failed to clear previous login challenges: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(s.ttl)
	challenge := &MFAChallenge{
		UserID:    userID,
		TokenHash: HashToken(token),
		CodeHash:  HashToken(code),
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := s.repo.Create(ctx, challenge); err != nil {
		return nil, fmt.Errorf("failed to create login challenge: %w", err)
	}

	return &MFAChallengeCode{
		Token:     token,
		Code:      code,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

// ResendChallenge replaces a pending challenge with a fresh one, enforcing the
// resend cooldown against the original issue time.
func (s *MFAService) ResendChallenge(ctx context.Context, token string) (*MFAChallengeCode, error) {
	challenge, err := s.repo.FindByTokenHash(ctx, HashToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to look up login challenge: %w", err)
	}

	if challenge == nil {
		return nil, ErrMFAChallengeInvalid
	}

	if time.Now().After(challenge.ExpiresAt) {
		_ = s.repo.Delete(ctx, challenge.ID)
		return nil, ErrMFAChallengeExpired
	}

	if time.Since(challenge.CreatedAt) < s.resendCooldown {
		return nil, ErrMFAResendTooSoon
	}

	return s.CreateChallenge(ctx, challenge.UserID)
}

// ChallengeUserID resolves the user behind a pending challenge without
// consuming or rotating it. The resend path uses it to re-check the account
// before spending the cooldown and clearing the challenge.
func (s *MFAService) ChallengeUserID(ctx context.Context, token string) (int, error) {
	challenge, err := s.repo.FindByTokenHash(ctx, HashToken(token))
	if err != nil {
		return 0, fmt.Errorf("failed to look up login challenge: %w", err)
	}

	if challenge == nil {
		return 0, ErrMFAChallengeInvalid
	}

	if time.Now().After(challenge.ExpiresAt) {
		_ = s.repo.Delete(ctx, challenge.ID)
		return 0, ErrMFAChallengeExpired
	}

	return challenge.UserID, nil
}

// VerifyChallenge consumes a challenge when the code matches and returns the
// challenged user ID. Each verify spends one attempt atomically; the final
// mismatch discards the challenge, and a concurrent verifier can consume the
// row only once.
func (s *MFAService) VerifyChallenge(ctx context.Context, token, code string) (int, error) {
	challenge, err := s.repo.FindByTokenHash(ctx, HashToken(token))
	if err != nil {
		return 0, fmt.Errorf("failed to look up login challenge: %w", err)
	}

	if challenge == nil {
		return 0, ErrMFAChallengeInvalid
	}

	if time.Now().After(challenge.ExpiresAt) {
		_ = s.repo.Delete(ctx, challenge.ID)
		return 0, ErrMFAChallengeExpired
	}

	spent, err := s.repo.IncrementAttempts(ctx, challenge.ID, s.maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("failed to record login verification attempt: %w", err)
	}

	if !spent {
		_ = s.repo.Delete(ctx, challenge.ID)
		return 0, ErrMFACodeMaxAttempts
	}

	expected := HashToken(code)
	if subtle.ConstantTimeCompare([]byte(challenge.CodeHash), []byte(expected)) != 1 {
		if challenge.Attempts+1 >= s.maxAttempts {
			_ = s.repo.Delete(ctx, challenge.ID)
			return 0, ErrMFACodeMaxAttempts
		}

		return 0, ErrMFACodeInvalid
	}

	consumed, err := s.repo.Consume(ctx, challenge.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to consume login challenge: %w", err)
	}

	if !consumed {
		return 0, ErrMFAChallengeInvalid
	}

	return challenge.UserID, nil
}

// NewMFAService creates the login MFA service. ttl bounds a challenge's life,
// resendCooldown throttles re-issues, and maxAttempts caps wrong codes before
// the challenge is discarded.
func NewMFAService(
	repo MFAChallengeRepo,
	ttl time.Duration,
	resendCooldown time.Duration,
	maxAttempts int,
) *MFAService {
	return &MFAService{
		repo:           repo,
		ttl:            ttl,
		resendCooldown: resendCooldown,
		maxAttempts:    maxAttempts,
	}
}
