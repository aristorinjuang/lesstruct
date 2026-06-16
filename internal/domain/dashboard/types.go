package dashboard

import (
	"errors"
)

var (
	// ErrUnauthorized is returned when user doesn't have permission to access dashboard
	ErrUnauthorized = errors.New("unauthorized access to dashboard")
)

// Stats represents dashboard statistics
type Stats struct {
	PublishedPosts       int           `json:"publishedPosts"`
	DraftPosts           int           `json:"draftPosts"`
	RegisteredUsers      int           `json:"registeredUsers"`
	PendingRegistrations int           `json:"pendingRegistrations"`
	MediaItems           int           `json:"mediaItems"`
	RecentContent        []*RecentItem `json:"recentContent,omitempty"`
}

// RecentItem represents a recent content item in the dashboard
type RecentItem struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}
