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

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}
