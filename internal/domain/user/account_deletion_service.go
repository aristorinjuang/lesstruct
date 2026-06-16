package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

var (
	// ErrLastAdminDeletionForbidden is returned when attempting to delete the last admin
	ErrLastAdminDeletionForbidden = errors.New("cannot delete the last administrator account")
	// ErrInvalidConfirmationString is returned when the confirmation string is invalid
	ErrInvalidConfirmationString = errors.New("invalid confirmation string")
)

// AccountDeletionServiceInterface defines the interface for account deletion operations
type AccountDeletionServiceInterface interface {
	DeleteAccount(ctx context.Context, userID int, username, userEmail string) error
	ValidateConfirmationString(confirmation string) error
}

// AccountDeletionService handles account deletion operations
type AccountDeletionService struct {
	userRepo     repository.UserRepo
	deletionRepo repository.UserDeletionRepo
	emailService email.EmailService
	logger       *util.Logger
}

// ValidateConfirmationString validates that the confirmation string is exactly "DELETE"
func (s *AccountDeletionService) ValidateConfirmationString(confirmation string) error {
	if confirmation != "DELETE" {
		return ErrInvalidConfirmationString
	}
	return nil
}

// isLastAdmin checks if the user is the last active administrator
func (s *AccountDeletionService) isLastAdmin(ctx context.Context, userID int) (bool, error) {
	// Get user to check if they're an admin
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	// If user is not an admin, they can be deleted
	if user.Role != "Admin" {
		return false, nil
	}

	// Count active admins
	adminCount, err := s.deletionRepo.CountUsersByRoleAndStatus(ctx, "Admin", "active")
	if err != nil {
		return false, fmt.Errorf("failed to count admins: %w", err)
	}

	return adminCount == 1, nil
}

// DeleteAccount performs a hard delete of a user account and all associated data
func (s *AccountDeletionService) DeleteAccount(ctx context.Context, userID int, username, userEmail string) error {
	// Check if user is the last admin
	isLastAdmin, err := s.isLastAdmin(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check last admin status: %w", err)
	}
	if isLastAdmin {
		return ErrLastAdminDeletionForbidden
	}

	// Delete all user data in a single transaction
	if err := s.deletionRepo.DeleteAllUserData(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user data: %w", err)
	}

	// Send deletion notification email (non-blocking)
	if err := s.emailService.SendAccountDeletedNotification(ctx, userEmail, username); err != nil {
		// Log error but don't fail the operation
		s.logger.Error("failed to send account deletion notification email: %v", err)
	}

	return nil
}

// NewAccountDeletionService creates a new account deletion service
func NewAccountDeletionService(
	userRepo repository.UserRepo,
	deletionRepo repository.UserDeletionRepo,
	emailService email.EmailService,
	logger *util.Logger,
) *AccountDeletionService {
	return &AccountDeletionService{
		userRepo:     userRepo,
		deletionRepo: deletionRepo,
		emailService: emailService,
		logger:       logger,
	}
}
