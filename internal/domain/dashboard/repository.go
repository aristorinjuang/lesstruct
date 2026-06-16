package dashboard

import (
	"context"
)

// Repository defines the interface for dashboard repository operations
type Repository interface {
	GetStats(ctx context.Context, userID int) (*Stats, error)
}
