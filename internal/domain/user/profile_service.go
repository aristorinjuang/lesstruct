package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/repository"
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
	GetProfile(ctx context.Context, userID int) (*Profile, error)
	UpdateEmail(ctx context.Context, userID int, newEmail, currentPasswordHash string) error
	ChangePassword(ctx context.Context, userID int, currentPassword, newPassword string) error
	ExportUserData(ctx context.Context, userID int) (*repository.UserDataExport, error)
	VerifyEmailUpdate(ctx context.Context, token string) (int, string, error)
	UpdateCustomFields(ctx context.Context, userID int, customFields map[string]any, isAdmin bool) error
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	// crypto/rand.Read reads from the OS's CSPRNG and essentially never fails
	_, _ = rand.Read(b)
	return hex.EncodeToString(b), nil
}

// Profile represents user profile information
type Profile struct {
	ID             int
	Username       string
	Name           string
	Email          string
	Role           string
	ProfilePicture string
	CreatedAt      string
	UpdatedAt      string
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

// ProfileService handles profile management operations
type ProfileService struct {
	userRepo             repository.UserRepo
	emailUpdateTokenRepo repository.EmailUpdateTokenRepo
	userDataExportRepo   repository.UserDataExportRepo
	emailService         email.EmailService
	systemFieldProvider  systemFieldProvider
}

// NewProfileService creates a new profile service
func NewProfileService(
	userRepo repository.UserRepo,
	emailUpdateTokenRepo repository.EmailUpdateTokenRepo,
	userDataExportRepo repository.UserDataExportRepo,
	emailService email.EmailService,
	systemFieldProvider systemFieldProvider,
) *ProfileService {
	return &ProfileService{
		userRepo:             userRepo,
		emailUpdateTokenRepo: emailUpdateTokenRepo,
		userDataExportRepo:   userDataExportRepo,
		emailService:         emailService,
		systemFieldProvider:  systemFieldProvider,
	}
}

// GetProfile retrieves user profile information
func (s *ProfileService) GetProfile(ctx context.Context, userID int) (*Profile, error) {
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
func (s *ProfileService) UpdateCustomFields(ctx context.Context, userID int, customFields map[string]any, isAdmin bool) error {
	var filtered map[string]any
	if isAdmin {
		filtered = customFields
	} else {
		filtered = s.stripSystemFields(customFields)
	}

	return s.userRepo.UpdateCustomFields(ctx, userID, filtered)
}

// UpdateEmail initiates an email update by creating a verification token and sending an email
func (s *ProfileService) UpdateEmail(ctx context.Context, userID int, newEmail, currentPasswordHash string) error {
	// Validate new email format
	if err := auth.ValidateEmail(newEmail); err != nil {
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
	if err := auth.VerifyPassword(user.PasswordHash, currentPasswordHash); err != nil {
		return fmt.Errorf("%w: current password incorrect", ErrInvalidPassword)
	}

	// Generate verification token
	token, _ := generateSecureToken()

	// Hash token for storage
	tokenHash := repository.HashToken(token)

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

// VerifyEmailUpdate verifies an email update token and updates the user's email
func (s *ProfileService) VerifyEmailUpdate(ctx context.Context, token string) (int, string, error) {
	// Hash token to match stored value
	tokenHash := repository.HashToken(token)

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
func (s *ProfileService) ChangePassword(ctx context.Context, userID int, currentPassword, newPassword string) error {
	// Validate new password
	if err := auth.ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPassword, err)
	}

	// Get user to verify current password
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if err := auth.VerifyPassword(user.PasswordHash, currentPassword); err != nil {
		return fmt.Errorf("%w: current password incorrect", ErrInvalidPassword)
	}

	// Hash new password
	newPasswordHash, _ := auth.HashPassword(newPassword)

	// Update password
	if err := s.userRepo.UpdatePassword(ctx, userID, user.PasswordHash, newPasswordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ExportUserData generates a JSON export of all user data
func (s *ProfileService) ExportUserData(ctx context.Context, userID int) (*repository.UserDataExport, error) {
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
