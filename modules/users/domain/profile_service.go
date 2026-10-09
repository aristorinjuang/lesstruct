package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
)

var (
	ErrInvalidEmail        = errors.New("invalid email address")
	ErrEmailAlreadyInUse   = errors.New("email already in use")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrProfileUserNotFound = errors.New("user not found")
)

// systemFieldProvider returns slugs of user system fields that must not be written by non-admin users
type systemFieldProvider interface {
	GetUserSystemFieldSlugs() []string
}

// ProfileServiceInterface defines the interface for profile service operations
type ProfileServiceInterface interface {
	GetProfile(ctx context.Context, principal kernel.Principal) (*Profile, error)
	UpdateEmail(ctx context.Context, principal kernel.Principal, newEmail, currentPasswordHash string) error
	UpdateName(ctx context.Context, principal kernel.Principal, name string) error
	ChangePassword(ctx context.Context, principal kernel.Principal, currentPassword, newPassword string) error
	ExportUserData(ctx context.Context, principal kernel.Principal) (*UserDataExport, error)
	VerifyEmailUpdate(ctx context.Context, token string) (int, string, error)
	UpdateCustomFields(ctx context.Context, principal kernel.Principal, customFields map[string]any, isAdmin bool) error
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	// crypto/rand.Read reads from the OS's CSPRNG and essentially never fails
	_, _ = rand.Read(b)
	return hex.EncodeToString(b), nil
}

// hashToken hashes a token using SHA-256 for storage. It mirrors the token
// hashing previously shared with the auth domain; email-update tokens are
// created and verified inside this service, so the format is self-consistent.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// principalID parses the principal's decimal ID for repository calls,
// mirroring identity.IntID (fail-closed with the kernel sentinel).
func principalID(principal kernel.Principal) (int, error) {
	id, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", kernel.ErrInvalidPrincipalID, principal.ID)
	}
	return id, nil
}

// Profile represents user profile information
type Profile struct {
	ID             int
	Username       string
	Name           string
	Email          string
	Role           string
	ProfilePicture string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CustomFields   map[string]any
}

// EmailUpdateRequest represents a request to update email
type EmailUpdateRequest struct {
	NewEmail        string
	CurrentPassword string
}

// PasswordUpdateRequest represents a request to update password
type PasswordUpdateRequest struct {
	CurrentPassword string
	NewPassword     string
}

// profileUserStore is the user-repository subset the profile service needs.
type profileUserStore interface {
	GetUserByID(ctx context.Context, userID int) (*User, error)
	CheckEmailExists(ctx context.Context, email string) (bool, error)
	UpdateName(ctx context.Context, userID int, name string) error
	UpdateEmail(ctx context.Context, userID int, email string) error
	UpdatePassword(ctx context.Context, userID int, currentPasswordHash, newPasswordHash string) error
	UpdateCustomFields(ctx context.Context, userID int, customFields map[string]any) error
}

// emailTokenStore is the email-update-token subset the profile service needs.
type emailTokenStore interface {
	CreateToken(ctx context.Context, tokenHash string, userID int, newEmail string) error
	GetToken(ctx context.Context, tokenHash string) (*EmailUpdateToken, error)
	DeleteToken(ctx context.Context, tokenID int) error
}

// userExportStore is the data-export subset the profile service needs.
type userExportStore interface {
	GetUserDataForExport(ctx context.Context, userID int) (*UserDataExport, error)
}

// ProfileService handles profile management operations
type ProfileService struct {
	userRepo             profileUserStore
	emailUpdateTokenRepo emailTokenStore
	userDataExportRepo   userExportStore
	emailService         Mailer
	policy               CredentialPolicy
	systemFieldProvider  systemFieldProvider
}

// stripSystemFields removes system field slugs from the custom fields map
func (s *ProfileService) stripSystemFields(customFields map[string]any) map[string]any {
	if s.systemFieldProvider == nil {
		return customFields
	}

	systemSlugs := s.systemFieldProvider.GetUserSystemFieldSlugs()
	blocklist := make(map[string]bool, len(systemSlugs))
	for _, slug := range systemSlugs {
		blocklist[slug] = true
	}

	filtered := make(map[string]any, len(customFields))
	for k, v := range customFields {
		if !blocklist[k] {
			filtered[k] = v
		}
	}
	return filtered
}

// GetProfile retrieves user profile information
func (s *ProfileService) GetProfile(ctx context.Context, principal kernel.Principal) (*Profile, error) {
	userID, err := principalID(principal)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &Profile{
		ID:             user.ID,
		Username:       user.Username,
		Name:           user.Name,
		Email:          user.Email,
		Role:           user.Role,
		ProfilePicture: user.ProfilePicture,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		CustomFields:   user.CustomFields,
	}, nil
}

// UpdateCustomFields updates a user's custom fields. When isAdmin is false, system field slugs are stripped (defense-in-depth).
func (s *ProfileService) UpdateCustomFields(ctx context.Context, principal kernel.Principal, customFields map[string]any, isAdmin bool) error {
	userID, err := principalID(principal)
	if err != nil {
		return err
	}
	var filtered map[string]any
	if isAdmin {
		filtered = customFields
	} else {
		filtered = s.stripSystemFields(customFields)
	}

	return s.userRepo.UpdateCustomFields(ctx, userID, filtered)
}

// UpdateEmail initiates an email update by creating a verification token and sending an email
func (s *ProfileService) UpdateEmail(ctx context.Context, principal kernel.Principal, newEmail, currentPasswordHash string) error {
	userID, err := principalID(principal)
	if err != nil {
		return err
	}
	// Validate new email format
	if err := s.policy.ValidateEmail(newEmail); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEmail, err)
	}

	// Check if new email already exists
	exists, err := s.userRepo.CheckEmailExists(ctx, newEmail)
	if err != nil {
		return fmt.Errorf("failed to check if email exists: %w", err)
	}
	if exists {
		return ErrEmailAlreadyInUse
	}

	// Get user to verify current password
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if err := s.policy.VerifyPassword(user.PasswordHash, currentPasswordHash); err != nil {
		return fmt.Errorf("%w: current password incorrect", ErrInvalidPassword)
	}

	// Generate verification token
	token, _ := generateSecureToken()

	// Hash token for storage
	tokenHash := hashToken(token)

	// Create email update token
	if err := s.emailUpdateTokenRepo.CreateToken(ctx, tokenHash, userID, newEmail); err != nil {
		return fmt.Errorf("failed to create email update token: %w", err)
	}

	// Send verification email to NEW email address
	if err := s.emailService.SendEmailUpdateVerificationEmail(ctx, newEmail, user.Username, token); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}

// UpdateName updates a user's display name. Whitespace trimming is handled by the repository.
func (s *ProfileService) UpdateName(ctx context.Context, principal kernel.Principal, name string) error {
	userID, err := principalID(principal)
	if err != nil {
		return err
	}
	return s.userRepo.UpdateName(ctx, userID, name)
}

// VerifyEmailUpdate verifies an email update token and updates the user's email
func (s *ProfileService) VerifyEmailUpdate(ctx context.Context, token string) (int, string, error) {
	// Hash token to match stored value
	tokenHash := hashToken(token)

	// Get token from database
	emailUpdateToken, err := s.emailUpdateTokenRepo.GetToken(ctx, tokenHash)
	if err != nil {
		return 0, "", fmt.Errorf("invalid or expired token: %w", err)
	}

	// Update user email
	if err := s.userRepo.UpdateEmail(ctx, emailUpdateToken.UserID, emailUpdateToken.NewEmail); err != nil {
		return 0, "", fmt.Errorf("failed to update email: %w", err)
	}

	// Delete token after use
	if err := s.emailUpdateTokenRepo.DeleteToken(ctx, emailUpdateToken.ID); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("failed to delete email update token: %v\n", err)
	}

	return emailUpdateToken.UserID, emailUpdateToken.NewEmail, nil
}

// ChangePassword updates a user's password
func (s *ProfileService) ChangePassword(ctx context.Context, principal kernel.Principal, currentPassword, newPassword string) error {
	userID, err := principalID(principal)
	if err != nil {
		return err
	}
	// Validate new password
	if err := s.policy.ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPassword, err)
	}

	// Get user to verify current password
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if err := s.policy.VerifyPassword(user.PasswordHash, currentPassword); err != nil {
		return fmt.Errorf("%w: current password incorrect", ErrInvalidPassword)
	}

	// Hash new password
	newPasswordHash, _ := s.policy.HashPassword(newPassword)

	// Update password
	if err := s.userRepo.UpdatePassword(ctx, userID, user.PasswordHash, newPasswordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ExportUserData generates a JSON export of all user data
func (s *ProfileService) ExportUserData(ctx context.Context, principal kernel.Principal) (*UserDataExport, error) {
	userID, err := principalID(principal)
	if err != nil {
		return nil, err
	}
	// Get user data
	userData, err := s.userDataExportRepo.GetUserDataForExport(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data: %w", err)
	}

	// Send notification email
	if err := s.emailService.SendDataExportNotificationEmail(ctx, userData.User.Email, userData.User.Username); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("failed to send data export notification email: %v\n", err)
	}

	return userData, nil
}

// NewProfileService creates a new profile service
func NewProfileService(
	userRepo profileUserStore,
	emailUpdateTokenRepo emailTokenStore,
	userDataExportRepo userExportStore,
	emailService Mailer,
	policy CredentialPolicy,
	systemFieldProvider systemFieldProvider,
) *ProfileService {
	return &ProfileService{
		userRepo:             userRepo,
		emailUpdateTokenRepo: emailUpdateTokenRepo,
		userDataExportRepo:   userDataExportRepo,
		emailService:         emailService,
		policy:               policy,
		systemFieldProvider:  systemFieldProvider,
	}
}
