package middleware_test

import (
	"context"

	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// stubUserLookup resolves every token subject to an active account with the
// given session state. Tests override the fields to simulate a revoked token,
// a deleted account, or a lookup failure.
type stubUserLookup struct {
	user    *userdomain.User
	err     error
	missing bool
}

func (s stubUserLookup) GetUserByID(_ context.Context, userID int) (*userdomain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.missing {
		return nil, nil
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
