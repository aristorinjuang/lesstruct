package modules

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
)

// ProfilePictureUserRepo adapts repository.UserRepo to the users module's
// profilepicture.UserRepo seam. It lives in core — the only layer that may
// import both sides.
type ProfilePictureUserRepo struct {
	repo repository.UserRepo
}

// GetUserByID returns the profile picture filename for a user.
func (r *ProfilePictureUserRepo) GetUserByID(ctx context.Context, userID int) (string, error) {
	user, err := r.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.ProfilePicture, nil
}

// UpdateProfilePicture updates a user's profile picture filename.
func (r *ProfilePictureUserRepo) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	return r.repo.UpdateProfilePicture(ctx, userID, profilePicture)
}

// DeleteProfilePicture clears a user's profile picture.
func (r *ProfilePictureUserRepo) DeleteProfilePicture(ctx context.Context, userID int) error {
	return r.repo.DeleteProfilePicture(ctx, userID)
}

// NewProfilePictureUserRepo creates a ProfilePictureUserRepo.
func NewProfilePictureUserRepo(repo repository.UserRepo) *ProfilePictureUserRepo {
	return &ProfilePictureUserRepo{repo: repo}
}

var _ profilepicture.UserRepo = (*ProfilePictureUserRepo)(nil)
