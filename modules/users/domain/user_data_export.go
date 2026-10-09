package user

import (
	"time"
)

// UserContentItem represents a content item (post/page) created by a user
type UserContentItem struct {
	ID        int       `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UserCommentItem represents a comment posted by a user
type UserCommentItem struct {
	ID            int       `json:"id"`
	ContentItemID int       `json:"contentItemId"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// UserMediaItem represents a media file uploaded by a user
type UserMediaItem struct {
	ID               int       `json:"id"`
	Filename         string    `json:"filename"`
	OriginalFilename string    `json:"originalFilename"`
	FilePath         string    `json:"filePath"`
	FileSize         int       `json:"fileSize"`
	MimeType         string    `json:"mimeType"`
	CreatedAt        time.Time `json:"createdAt"`
}

// UserDataExport represents all user data for export
type UserDataExport struct {
	ExportDate time.Time          `json:"exportDate"`
	User       *User              `json:"user"`
	Content    []*UserContentItem `json:"content"`
	Comments   []*UserCommentItem `json:"comments"`
	Media      []*UserMediaItem   `json:"media"`
}
