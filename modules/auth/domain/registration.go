package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

var (
	// ErrUsernameInvalid is returned when username format is invalid
	ErrUsernameInvalid = errors.New("username must be 1-50 characters and contain only letters, numbers, underscores, and hyphens")

	// ErrUsernameExists is returned when username already exists
	ErrUsernameExists = errors.New("username already exists")

	// ErrEmailExists is returned when email already exists
	ErrEmailExists = errors.New("email address already registered")

	// ErrRegistrationFailed is returned when registration fails
	ErrRegistrationFailed = errors.New("registration failed")

	// ErrRegistrationDisabled is returned when self-registration is disabled via
	// the [registration] config block (or, by legacy default, when the comment
	// system is off).
	ErrRegistrationDisabled = errors.New("self-registration is disabled")

	// ErrConsentRequired is returned when registration demands an affirmative
	// consent act and the request does not carry one.
	ErrConsentRequired = errors.New("consent is required to register")
)

// usernameRegex validates username format (alphanumeric, underscores, hyphens, 1-50 chars)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// ValidateUsername checks if a username has a valid format
func ValidateUsername(username string) error {
	if username == "" || !usernameRegex.MatchString(username) {
		return ErrUsernameInvalid
	}
	return nil
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username        string `json:"username"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConsentAccepted bool   `json:"consentAccepted,omitempty"`
}

// ConsentPolicy is the registration consent contract resolved once at boot:
// whether an affirmative act is required and which document revisions that
// act refers to. A zero policy leaves registration unchanged.
type ConsentPolicy struct {
	Required bool
	Terms    legal.Document
	Privacy  legal.Document
}

// RegisterResult contains the result of a successful registration
type RegisterResult struct {
	UserID  int
	Message string
}

// RegistrationOption configures a RegistrationService. The zero options
// reproduce the legacy behavior exactly: registration enabled iff the comment
// system is enabled, default role Commentator, pending status.
type RegistrationOption func(*registrationOptions)

type registrationOptions struct {
	enabled     *bool
	defaultRole string
}

// WithEnabled overrides the registration gate. When unset, registration is
// enabled iff comments are enabled (the legacy coupling).
func WithEnabled(enabled bool) RegistrationOption {
	return func(o *registrationOptions) {
		o.enabled = &enabled
	}
}

// WithDefaultRole sets the role assigned to new registrants. When unset, it
// defaults to the Commentator role.
func WithDefaultRole(roleName string) RegistrationOption {
	return func(o *registrationOptions) {
		o.defaultRole = roleName
	}
}

// RegistrationService handles user registration business logic
type RegistrationService struct {
	userRepo        userStore
	policy          CredentialPolicy
	commentsEnabled bool
	enabled         bool
	defaultRole     string
	consent         ConsentPolicy
}

// SetConsentPolicy adopts the boot-time registration consent contract. The
// composition root resolves the conventional legal documents after the auth
// module is built, so this seam lands before any request is served: enabling
// consent here also gates the API, not just the rendered checkbox.
func (s *RegistrationService) SetConsentPolicy(policy ConsentPolicy) {
	s.consent = policy
}

// consentEvidence stamps the acceptance evidence for a consent-gated
// registration. It returns nil evidence when the policy does not require
// consent, keeping the user row unchanged.
func (s *RegistrationService) consentEvidence(now time.Time) (*time.Time, string, error) {
	if !s.consent.Required {
		return nil, "", nil
	}
	consent := legal.NewConsent(s.consent.Terms, s.consent.Privacy, now)
	payload, err := json.Marshal(consent)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal consent evidence: %w", err)
	}
	acceptedAt := consent.AcceptedAt
	return &acceptedAt, string(payload), nil
}

// RegistrationEnabled reports whether self-registration is currently allowed.
// Handlers call this BEFORE any repository lookup (e.g. the blocked-email
// check) so a disabled instance short-circuits without a DB query and without
// leaking whether a submitted email is blocked.
func (s *RegistrationService) RegistrationEnabled() bool {
	return s.enabled
}

// RegisterUser registers a new user with validation. The registered user gets
// the configured default role (Commentator unless overridden) and is always
// created in "pending" status: email verification is mandatory, so the account
// only becomes usable after the registrant proves their email address (and, in
// admin-approval mode, after an administrator approves the account).
func (s *RegistrationService) RegisterUser(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	if !s.enabled {
		return nil, ErrRegistrationDisabled
	}

	// Consent is enforced before any repository work: an unconsented
	// registration must not create rows, send tokens, or leak existence.
	if s.consent.Required && !req.ConsentAccepted {
		return nil, ErrConsentRequired
	}

	// Validate username format
	if err := ValidateUsername(req.Username); err != nil {
		return nil, err
	}

	// Validate email format
	if err := s.policy.ValidateEmail(req.Email); err != nil {
		return nil, err
	}

	// Validate password strength
	if err := s.policy.ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	// Check username uniqueness (case-insensitive)
	usernameExists, err := s.userRepo.CheckUsernameExists(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}
	if usernameExists {
		return nil, ErrUsernameExists
	}

	// Check email uniqueness (case-insensitive)
	emailExists, err := s.userRepo.CheckEmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}
	if emailExists {
		return nil, ErrEmailExists
	}

	// Hash password
	passwordHash, _ := s.policy.HashPassword(req.Password)
	consentAt, consentEvidence, err := s.consentEvidence(time.Now())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}

	user := &userdomain.User{
		Username:        req.Username,
		PasswordHash:    passwordHash,
		Email:           req.Email,
		Name:            req.Name,
		Role:            s.defaultRole,
		Status:          "pending",
		ConsentAt:       consentAt,
		ConsentEvidence: consentEvidence,
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("%w: failed to create user", ErrRegistrationFailed)
	}

	return &RegisterResult{
		UserID:  user.ID,
		Message: "Registration successful. Please check your email to verify your account.",
	}, nil
}

// NewRegistrationService creates a new registration service. Legacy behavior —
// registration enabled iff comments enabled, default role Commentator, pending
// status — is preserved unless overridden via RegistrationOptions.
func NewRegistrationService(userRepo userStore, policy CredentialPolicy, commentsEnabled bool, opts ...RegistrationOption) *RegistrationService {
	o := registrationOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	enabled := commentsEnabled
	if o.enabled != nil {
		enabled = *o.enabled
	}
	defaultRole := o.defaultRole
	if defaultRole == "" {
		defaultRole = RoleCommentator
	}

	return &RegistrationService{
		userRepo:        userRepo,
		policy:          policy,
		commentsEnabled: commentsEnabled,
		enabled:         enabled,
		defaultRole:     defaultRole,
	}
}
