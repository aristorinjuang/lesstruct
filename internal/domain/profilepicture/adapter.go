package profilepicture

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

// RepoAdapter adapts the repository.UserRepo to the profilepicture.UserRepo interface
type RepoAdapter struct {
	userRepo repository.UserRepo
}

// GetUserByID returns the profile picture filename for a user
func (a *RepoAdapter) GetUserByID(ctx context.Context, userID int) (string, error) {
	user, err := a.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.ProfilePicture, nil
}

// UpdateProfilePicture updates a user's profile picture filename
func (a *RepoAdapter) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	return a.userRepo.UpdateProfilePicture(ctx, userID, profilePicture)
}

// DeleteProfilePicture clears a user's profile picture
func (a *RepoAdapter) DeleteProfilePicture(ctx context.Context, userID int) error {
	return a.userRepo.DeleteProfilePicture(ctx, userID)
}

// NewRepoAdapter creates a new repository adapter
func NewRepoAdapter(userRepo repository.UserRepo) *RepoAdapter {
	return &RepoAdapter{userRepo: userRepo}
}
