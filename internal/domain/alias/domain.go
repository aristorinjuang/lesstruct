package alias

import (
	"context"
	"errors"
)

var ErrAliasNotFound = errors.New("alias not found")
var ErrAliasAlreadyExists = errors.New("alias already exists")

type Alias struct {
	ID        int    `json:"id"`
	ContentID int    `json:"contentId"`
	Alias     string `json:"alias"`
}

type Repository interface {
	Create(ctx context.Context, a *Alias) error
	DeleteByContentID(ctx context.Context, contentID int) error
	FindByAlias(ctx context.Context, alias string) (*Alias, error)
	FindByContentID(ctx context.Context, contentID int) ([]*Alias, error)
	FindAll(ctx context.Context) ([]*Alias, error)
	// Repoint re-points an alias from one content id to another. The WHERE
	// clause is guarded on the current content id so a concurrent change
	// cannot be silently overwritten; ErrAliasNotFound is returned when the
	// alias is absent or already points elsewhere.
	Repoint(ctx context.Context, aliasStr string, fromContentID, toContentID int) error
}
