package dashboard

import (
	"errors"
	"time"
)

var (
	// ErrUnauthorized is returned when user doesn't have permission to access dashboard
	ErrUnauthorized = errors.New("unauthorized access to dashboard")
)

// Stats represents dashboard statistics
type Stats struct {
	PublishedPosts       int             `json:"publishedPosts"`
	DraftPosts           int             `json:"draftPosts"`
	RegisteredUsers      int             `json:"registeredUsers"`
	PendingRegistrations int             `json:"pendingRegistrations"`
	MediaItems           int             `json:"mediaItems"`
	TotalContent         int             `json:"totalContent"`
	ContentByType        []*PostTypeCount `json:"contentByType,omitempty"`
	RecentContent        []*RecentItem   `json:"recentContent,omitempty"`
}

// PostTypeCount represents the number of content items of a given post type
type PostTypeCount struct {
	PostType string `json:"postType"`
	Count    int    `json:"count"`
}

// RecentItem represents a recent content item in the dashboard
type RecentItem struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
