package middleware

import (
	"context"

	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// stubUserLookup resolves every token subject to an active account. Tests
// override the fields to simulate revocation or a deleted account.
type stubUserLookup struct {
	user *userdomain.User
	err  error
}

func (s stubUserLookup) GetUserByID(_ context.Context, userID int) (*userdomain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.user != nil {
		return s.user, nil
	}
	return &userdomain.User{
		ID:       userID,
		Username: "testuser",
		Role:     "Contributor",
		Status:   "verified",
	}, nil
}
