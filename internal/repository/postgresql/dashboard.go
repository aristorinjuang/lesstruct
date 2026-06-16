package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
)

// DashboardRepository handles dashboard data operations
type DashboardRepository struct {
	db *sql.DB
}

// GetStats retrieves aggregated statistics for the dashboard
func (r *DashboardRepository) GetStats(ctx context.Context, userID int) (*dashboard.Stats, error) {
	var publishedPosts, draftPosts, registeredUsers, pendingRegistrations, mediaItems int

	// Get content counts
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END), 0) as published_posts,
			COALESCE(SUM(CASE WHEN status = 'draft' THEN 1 ELSE 0 END), 0) as draft_posts
		FROM content_items
		WHERE user_id = $1
	`, userID).Scan(&publishedPosts, &draftPosts)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get content counts: %w", err)
	}

	// Get users counts (global, not filtered by userID)
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'verified' THEN 1 ELSE 0 END), 0) as registered_users,
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending_registrations
		FROM users
	`).Scan(&registeredUsers, &pendingRegistrations)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get users counts: %w", err)
	}

	// Get media count
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM media_files
		WHERE user_id = $1
	`, userID).Scan(&mediaItems)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get media count: %w", err)
	}

	// Get recent content (last 5 items)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, slug, status, created_at
		FROM content_items
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	recentContent := []*dashboard.RecentItem{}
	for rows.Next() {
		var item dashboard.RecentItem
		var createdAt time.Time

		err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Slug,
			&item.Status,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recent content: %w", err)
		}

		item.CreatedAt = createdAt.Format(time.RFC3339)
		recentContent = append(recentContent, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate recent content rows: %w", err)
	}

	return &dashboard.Stats{
		PublishedPosts:       publishedPosts,
		DraftPosts:           draftPosts,
		RegisteredUsers:      registeredUsers,
		PendingRegistrations: pendingRegistrations,
		MediaItems:           mediaItems,
		RecentContent:        recentContent,
	}, nil
}

// NewDashboardRepository creates a new dashboard repository
func NewDashboardRepository(db *sql.DB) *DashboardRepository {
	return &DashboardRepository{
		db: db,
	}
}
