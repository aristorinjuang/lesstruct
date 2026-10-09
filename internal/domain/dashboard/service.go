package dashboard

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/identity"
	"github.com/aristorinjuang/lesstruct/kernel"
)

type Service struct {
	repo Repository
}

func (s *Service) GetStats(ctx context.Context, principal kernel.Principal) (*Stats, error) {
	userID, err := identity.IntID(principal)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.GetStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard stats: %w", err)
	}

	return stats, nil
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}
