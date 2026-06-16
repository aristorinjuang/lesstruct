package dashboard_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	"github.com/stretchr/testify/assert"
)

func TestStats(t *testing.T) {
	tests := []struct {
		name  string
		stats *dashboard.Stats
	}{
		{
			name: "valid stats with no content",
			stats: &dashboard.Stats{
				PublishedPosts:       0,
				DraftPosts:           0,
				RegisteredUsers:      1,
				PendingRegistrations: 0,
				MediaItems:           0,
				RecentContent:        []*dashboard.RecentItem{},
			},
		},
		{
			name: "valid stats with content",
			stats: &dashboard.Stats{
				PublishedPosts:       10,
				DraftPosts:           5,
				RegisteredUsers:      3,
				PendingRegistrations: 2,
				MediaItems:           25,
				RecentContent: []*dashboard.RecentItem{
					{
						ID:        15,
						Title:     "Latest Post",
						Slug:      "latest-post",
						Status:    "published",
						CreatedAt: "2026-04-10T10:30:00Z",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Stats is a simple struct, just verify it can be created
			assert.NotNil(t, tt.stats)
		})
	}
}

func TestRecentItem(t *testing.T) {
	item := &dashboard.RecentItem{
		ID:        1,
		Title:     "Test Post",
		Slug:      "test-post",
		Status:    "published",
		CreatedAt: "2026-04-10T10:30:00Z",
	}

	assert.Equal(t, 1, item.ID)
	assert.Equal(t, "Test Post", item.Title)
	assert.Equal(t, "test-post", item.Slug)
	assert.Equal(t, "published", item.Status)
	assert.Equal(t, "2026-04-10T10:30:00Z", item.CreatedAt)
}

func TestErrUnauthorized(t *testing.T) {
	err := dashboard.ErrUnauthorized
	assert.Error(t, err)
	assert.Equal(t, "unauthorized access to dashboard", err.Error())
}
