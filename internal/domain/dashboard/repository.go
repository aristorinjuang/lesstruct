package dashboard

import (
	"context"
)

// Repository defines the interface for dashboard repository operations
type Repository interface {
	// GetStats retrieves aggregated statistics. userID <= 0 means "all users"
	// (admin scope) and yields unfiltered counts; userID > 0 scopes content,
	// media, and recent-content counts to that user. User counts are always
	// global.
	GetStats(ctx context.Context, userID int) (*Stats, error)
}
