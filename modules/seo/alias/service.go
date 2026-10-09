package alias

import "context"

type Service struct {
	repo Repository
}

func (s *Service) Create(ctx context.Context, contentID int, aliasStr string) error {
	a := &Alias{
		ContentID: contentID,
		Alias:     aliasStr,
	}
	return s.repo.Create(ctx, a)
}

func (s *Service) FindByAlias(ctx context.Context, aliasStr string) (*Alias, error) {
	return s.repo.FindByAlias(ctx, aliasStr)
}

func (s *Service) FindByContentID(ctx context.Context, contentID int) ([]*Alias, error) {
	return s.repo.FindByContentID(ctx, contentID)
}

func (s *Service) FindAll(ctx context.Context) ([]*Alias, error) {
	return s.repo.FindAll(ctx)
}

func (s *Service) DeleteByContentID(ctx context.Context, contentID int) error {
	return s.repo.DeleteByContentID(ctx, contentID)
}

// Repoint re-points an existing alias to a new content id, but only when it
// still points at the expected fromContentID (concurrent changes fail with
// ErrAliasNotFound instead of being overwritten). Used by importers to adopt
// an alias whose previous target no longer exists.
func (s *Service) Repoint(ctx context.Context, aliasStr string, fromContentID, toContentID int) error {
	return s.repo.Repoint(ctx, aliasStr, fromContentID, toContentID)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}
