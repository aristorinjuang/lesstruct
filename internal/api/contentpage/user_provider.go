package contentpage

import (
	"context"
)

type userRepoAdapter struct {
	getUser func(ctx context.Context, username string) (*UserBasicInfo, error)
}

func (a *userRepoAdapter) GetUserByUsername(ctx context.Context, username string) (*UserBasicInfo, error) {
	return a.getUser(ctx, username)
}

func NewUserRepoAdapter(getUser func(ctx context.Context, username string) (*UserBasicInfo, error)) UserProvider {
	return &userRepoAdapter{getUser: getUser}
}
