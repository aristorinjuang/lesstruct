package user

import (
	"time"
)

// EmailUpdateToken represents an email update token in the system
type EmailUpdateToken struct {
	ID        int
	Token     string
	UserID    int
	NewEmail  string
	ExpiresAt string
	CreatedAt time.Time
}
