package dashboard

import (
	"context"
	"fmt"
)

type Service struct {
	repo Repository
}

func (s *Service) GetStats(ctx context.Context, userID int) (*Stats, error) {
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
