package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/repository"
)

// allowedSuspendFromStatuses defines which statuses allow suspension
var allowedSuspendFromStatuses = map[string]bool{
	"verified": true,
}

// allowedSoftDeleteFromStatuses defines which statuses allow soft deletion
var allowedSoftDeleteFromStatuses = map[string]bool{
	"verified":  true,
	"suspended": true,
}

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailAlreadyBlocked = errors.New("email is already blocked")
	ErrInvalidStatus       = errors.New("user is not in the expected status")
	ErrEmailNotVerified    = errors.New("user email is not verified")
)

// UserRepo defines the interface for user repository operations
type UserRepo interface {
	GetPendingUsers(ctx context.Context, limit int, offset int) ([]*repository.User, error)
	UpdateUserStatus(ctx context.Context, userID int, status string) error
	UpdateUserStatusIfCurrentStatus(ctx context.Context, userID int, currentStatus string, newStatus string) error
	DeleteUser(ctx context.Context, userID int) error
	GetUserByID(ctx context.Context, userID int) (*repository.User, error)
	SuspendUser(ctx context.Context, userID int) error
	UnsuspendUser(ctx context.Context, userID int) error
	SoftDeleteUser(ctx context.Context, userID int) error
	GetAllUsers(ctx context.Context, status string, limit int, offset int) ([]*repository.User, error)
	CountUsers(ctx context.Context, status string) (int, error)
	GetUserStatus(ctx context.Context, userID int) (string, error)
	UpdateProfile(ctx context.Context, userID int, name string, email string, role string, customFields map[string]any) error
	SetEmailVerified(ctx context.Context, userID int, emailVerified bool) error
	CheckEmailExistsForOtherUser(ctx context.Context, userID int, email string) (bool, error)
}

// BlockedEmailRepo defines the interface for blocked email repository operations
type BlockedEmailRepo interface {
	BlockEmail(ctx context.Context, email string, reason string) error
	IsEmailBlocked(ctx context.Context, email string) (bool, error)
	UnblockEmail(ctx context.Context, email string) error
}

// UserManagementOption configures a UserManagementService. A role service makes
// the assignable-role allowlist config-driven; without one the legacy
// three-role allowlist applies.
type UserManagementOption func(*UserManagementService)

// WithManagementRoleService attaches the config-driven role registry so admins
// can assign every registered role (built-ins plus any [[role]] entries).
func WithManagementRoleService(rs *roledomain.Service) UserManagementOption {
	return func(s *UserManagementService) {
		s.roleService = rs
	}
}

// WithApprovalEmailRequired makes admin approval conditional on the registrant
// having already proven their email address: approving a user whose email is
// not yet verified returns ErrEmailNotVerified. Without it, approval ignores
// the email state (legacy behavior).
func WithApprovalEmailRequired() UserManagementOption {
	return func(s *UserManagementService) {
		s.approvalEmailRequired = true
	}
}

// UserManagementService handles user management operations
type UserManagementService struct {
	userRepo             UserRepo
	blockedEmailRepo     BlockedEmailRepo
	commentsEnabled      bool
	roleService          *roledomain.Service
	approvalEmailRequired bool
}

// isAssignableRole reports whether an admin may assign the role. With a role
// service the allowlist is registry-driven; the Commentator role additionally
// requires the comment system to be enabled.
func (s *UserManagementService) isAssignableRole(role string) bool {
	if s.roleService != nil {
		if !s.roleService.IsAssignable(role) {
			return false
		}
		if role == constants.RoleCommentator && !s.commentsEnabled {
			return false
		}
		return true
	}
	return isAllowedRole(role, s.commentsEnabled)
}

// GetPendingUsers retrieves users with pending status with pagination
// Default limit: 100, max limit: 1000
func (s *UserManagementService) GetPendingUsers(ctx context.Context, limit int, offset int) ([]*repository.User, error) {
	users, err := s.userRepo.GetPendingUsers(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending users: %w", err)
	}

	return users, nil
}

// ApproveUser approves a pending user by updating their status to verified.
// Uses an atomic status update to prevent TOCTOU race conditions. When the
// service was created with WithApprovalEmailRequired, the registrant must be
// pending AND have already proven their email address (email_verified) or the
// respective error is returned — status is checked before the email proof so
// that a non-pending user is never misreported as EMAIL_NOT_VERIFIED.
func (s *UserManagementService) ApproveUser(ctx context.Context, userID int) error {
	if s.approvalEmailRequired {
		user, err := s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			// The repository returns fmt.Errorf("user not found with ID %d", userID)
			// for not-found, which doesn't wrap our sentinel, so we wrap it here
			// for the handler to detect.
			return fmt.Errorf("user not found with ID %d: %w", userID, ErrUserNotFound)
		}
		if user.Status != "pending" {
			return ErrInvalidStatus
		}
		if !user.EmailVerified {
			return ErrEmailNotVerified
		}
	}

	// Atomically update user status from pending to verified
	// This prevents race conditions where concurrent requests could bypass the status check
	err := s.userRepo.UpdateUserStatusIfCurrentStatus(ctx, userID, "pending", "verified")
	if err != nil {
		// Check if the error indicates the user was not in pending status
		// (rows affected = 0 means either not found or wrong status)
		return fmt.Errorf("failed to approve user: %w", errors.Join(ErrInvalidStatus, err))
	}

	return nil
}

// RejectUser deletes a user account
func (s *UserManagementService) RejectUser(ctx context.Context, userID int) error {
	// Check if user exists
	_, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		// The repository returns fmt.Errorf("user not found with ID %d", userID) for not-found
		// which doesn't wrap our sentinel, so we wrap it here for the handler to detect
		return fmt.Errorf("user not found with ID %d: %w", userID, ErrUserNotFound)
	}

	// Delete user
	if err := s.userRepo.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// MarkUserAsSpam deletes a user and blocks their email address
func (s *UserManagementService) MarkUserAsSpam(ctx context.Context, userID int) error {
	// Check if user exists
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		// The repository returns fmt.Errorf("user not found with ID %d", userID) for not-found
		// which doesn't wrap our sentinel, so we wrap it here for the handler to detect
		return fmt.Errorf("user not found with ID %d: %w", userID, ErrUserNotFound)
	}

	// Check if email is already blocked
	blocked, err := s.blockedEmailRepo.IsEmailBlocked(ctx, user.Email)
	if err != nil {
		return fmt.Errorf("failed to check if email is blocked: %w", err)
	}
	if blocked {
		return ErrEmailAlreadyBlocked
	}

	// Block email FIRST (before deletion) to prevent re-registration if deletion succeeds but blocking fails
	if err := s.blockedEmailRepo.BlockEmail(ctx, user.Email, "marked_as_spam"); err != nil {
		return fmt.Errorf("failed to block email: %w", err)
	}

	// Delete user
	if err := s.userRepo.DeleteUser(ctx, userID); err != nil {
		// Attempt to compensate by unblocking the email
		if unblockErr := s.blockedEmailRepo.UnblockEmail(ctx, user.Email); unblockErr != nil {
			return fmt.Errorf("failed to delete user and failed to unblock email (compensation): delete: %v, unblock: %v", err, unblockErr)
		}
		return fmt.Errorf("failed to delete user (email unblocked as compensation): %w", err)
	}

	return nil
}

// IsEmailBlocked checks if an email address is blocked
func (s *UserManagementService) IsEmailBlocked(ctx context.Context, email string) (bool, error) {
	blocked, err := s.blockedEmailRepo.IsEmailBlocked(ctx, email)
	if err != nil {
		return false, fmt.Errorf("failed to check if email is blocked: %w", err)
	}

	return blocked, nil
}

// SuspendUser suspends a user account
// Only users with 'verified' status can be suspended
func (s *UserManagementService) SuspendUser(ctx context.Context, userID int) error {
	currentStatus, err := s.userRepo.GetUserStatus(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user status: %w", err)
	}

	if !allowedSuspendFromStatuses[currentStatus] {
		return fmt.Errorf("cannot suspend user with status '%s': %w", currentStatus, ErrInvalidStatus)
	}

	if err := s.userRepo.UpdateUserStatusIfCurrentStatus(ctx, userID, currentStatus, "suspended"); err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}
	return nil
}

// UnsuspendUser unsuspends a user account
// Only users with 'suspended' status can be unsuspended; status is set to 'verified'
func (s *UserManagementService) UnsuspendUser(ctx context.Context, userID int) error {
	currentStatus, err := s.userRepo.GetUserStatus(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user status: %w", err)
	}

	if currentStatus != "suspended" {
		return fmt.Errorf("cannot unsuspend user with status '%s': %w", currentStatus, ErrInvalidStatus)
	}

	if err := s.userRepo.UpdateUserStatusIfCurrentStatus(ctx, userID, "suspended", "verified"); err != nil {
		return fmt.Errorf("failed to unsuspend user: %w", err)
	}
	return nil
}

// SoftDeleteUser soft deletes a user account
// Only users with 'verified' or 'suspended' status can be soft deleted
func (s *UserManagementService) SoftDeleteUser(ctx context.Context, userID int) error {
	currentStatus, err := s.userRepo.GetUserStatus(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user status: %w", err)
	}

	if !allowedSoftDeleteFromStatuses[currentStatus] {
		return fmt.Errorf("cannot soft delete user with status '%s': %w", currentStatus, ErrInvalidStatus)
	}

	if err := s.userRepo.UpdateUserStatusIfCurrentStatus(ctx, userID, currentStatus, "soft_deleted"); err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}
	return nil
}

// GetAllUsers retrieves all users with optional status filtering and pagination
func (s *UserManagementService) GetAllUsers(ctx context.Context, status string, limit int, offset int) ([]*repository.User, error) {
	users, err := s.userRepo.GetAllUsers(ctx, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	return users, nil
}

// CountUsers returns the total number of users, optionally filtered by status — mirrors
// GetAllUsers' WHERE clause so a list's total always matches its rows.
func (s *UserManagementService) CountUsers(ctx context.Context, status string) (int, error) {
	total, err := s.userRepo.CountUsers(ctx, status)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return total, nil
}

// GetUserStatus retrieves the current status of a user
func (s *UserManagementService) GetUserStatus(ctx context.Context, userID int) (string, error) {
	status, err := s.userRepo.GetUserStatus(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user status: %w", err)
	}
	return status, nil
}

// UpdateUserProfile updates a user's profile fields (name, email, role, custom fields).
// All fields are optional — only non-empty fields are updated.
func (s *UserManagementService) UpdateUserProfile(
	ctx context.Context,
	userID int,
	name string,
	email string,
	role string,
	customFields map[string]any,
) (*repository.User, error) {
	existing, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found with ID %d: %w", userID, ErrUserNotFound)
	}

	// Resolve values: use provided values or fall back to existing
	finalName := name
	if finalName == "" {
		finalName = existing.Username
	}

	finalEmail := email
	if finalEmail == "" {
		finalEmail = existing.Email
	}

	finalRole := role
	if finalRole == "" {
		finalRole = existing.Role
	}

	// Validate email if provided
	if email != "" {
		if err := auth.ValidateEmail(email); err != nil {
			return nil, ErrEmailInvalid
		}
	}

	// Validate role if provided
	if role != "" {
		if !s.isAssignableRole(role) {
			return nil, ErrInvalidRole
		}
	}

	// Check email uniqueness if email is changing
	if email != "" && strings.EqualFold(email, existing.Email) {
		// same email, no conflict possible
	} else if email != "" {
		emailExists, err := s.userRepo.CheckEmailExistsForOtherUser(ctx, userID, email)
		if err != nil {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if emailExists {
			return nil, ErrEmailExists
		}
	}

	if err := s.userRepo.UpdateProfile(ctx, userID, finalName, finalEmail, finalRole, customFields); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// A changed email address invalidates the previous verification: the flag
	// certifies ownership of the CURRENT address, which the new one has not
	// proven yet. Reset it so the admin-approval gate cannot trust a stale
	// proof of the old address.
	if !strings.EqualFold(finalEmail, existing.Email) {
		if err := s.userRepo.SetEmailVerified(ctx, userID, false); err != nil {
			return nil, fmt.Errorf("failed to reset email verification: %w", err)
		}
	}

	updated, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated user: %w", err)
	}

	return updated, nil
}

// NewUserManagementService creates a new user management service. commentsEnabled
// gates the Commentator role on the update-user path: when false, admins cannot
// assign it (the role exists only for the comment system).
func NewUserManagementService(
	userRepo UserRepo,
	blockedEmailRepo BlockedEmailRepo,
	commentsEnabled bool,
	opts ...UserManagementOption,
) *UserManagementService {
	s := &UserManagementService{
		userRepo:         userRepo,
		blockedEmailRepo: blockedEmailRepo,
		commentsEnabled:  commentsEnabled,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
