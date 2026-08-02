package mysql

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

// GetStats retrieves aggregated statistics for the dashboard. userID <= 0
// means "all users" (admin scope) and yields unfiltered content/media counts.
func (r *DashboardRepository) GetStats(ctx context.Context, userID int) (*dashboard.Stats, error) {
	var publishedPosts, draftPosts, registeredUsers, pendingRegistrations, mediaItems int

	// Get content counts
	contentCountQuery := `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END), 0) as published_posts,
			COALESCE(SUM(CASE WHEN status = 'draft' THEN 1 ELSE 0 END), 0) as draft_posts
		FROM content_items`
	contentCountArgs := []any{}
	if userID > 0 {
		contentCountQuery += ` WHERE user_id = ?`
		contentCountArgs = append(contentCountArgs, userID)
	}
	err := r.db.QueryRowContext(ctx, contentCountQuery, contentCountArgs...).Scan(&publishedPosts, &draftPosts)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get content counts: %w", err)
	}

	// Get content counts by post type
	ptQuery := `
		SELECT post_type, COUNT(*)
		FROM content_items`
	ptArgs := []any{}
	if userID > 0 {
		ptQuery += ` WHERE user_id = ?`
		ptArgs = append(ptArgs, userID)
	}
	ptQuery += `
		GROUP BY post_type
		ORDER BY post_type ASC`
	ptRows, err := r.db.QueryContext(ctx, ptQuery, ptArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get content counts by post type: %w", err)
	}
	defer func() { _ = ptRows.Close() }()

	contentByType := []*dashboard.PostTypeCount{}
	totalContent := 0
	for ptRows.Next() {
		var ptc dashboard.PostTypeCount
		if err := ptRows.Scan(&ptc.PostType, &ptc.Count); err != nil {
			return nil, fmt.Errorf("failed to scan post type count: %w", err)
		}
		totalContent += ptc.Count
		contentByType = append(contentByType, &ptc)
	}
	if err := ptRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate post type count rows: %w", err)
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
	mediaQuery := `
		SELECT COUNT(*)
		FROM media_files`
	mediaArgs := []any{}
	if userID > 0 {
		mediaQuery += ` WHERE user_id = ?`
		mediaArgs = append(mediaArgs, userID)
	}
	err = r.db.QueryRowContext(ctx, mediaQuery, mediaArgs...).Scan(&mediaItems)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get media count: %w", err)
	}

	// Get recent content (last 5 items)
	recentQuery := `
		SELECT id, title, slug, status, created_at
		FROM content_items`
	recentArgs := []any{}
	if userID > 0 {
		recentQuery += ` WHERE user_id = ?`
		recentArgs = append(recentArgs, userID)
	}
	recentQuery += `
		ORDER BY created_at DESC
		LIMIT 5`
	rows, err := r.db.QueryContext(ctx, recentQuery, recentArgs...)
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

		item.CreatedAt = createdAt
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
		TotalContent:         totalContent,
		ContentByType:        contentByType,
		RecentContent:        recentContent,
	}, nil
}

// NewDashboardRepository creates a new dashboard repository
func NewDashboardRepository(db *sql.DB) *DashboardRepository {
	return &DashboardRepository{
		db: db,
	}
}
