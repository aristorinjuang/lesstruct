package middleware

import (
	"net/http"
)

// CommentatorMiddleware handles authorization for Commentator role
type CommentatorMiddleware struct {
	authMiddleware *AuthMiddleware
}

// NewCommentatorMiddleware creates a new commentator authorization middleware
func NewCommentatorMiddleware(authMiddleware *AuthMiddleware) *CommentatorMiddleware {
	return &CommentatorMiddleware{
		authMiddleware: authMiddleware,
	}
}

// CommentatorOnly validates JWT token and checks if user has Commentator (or higher) role
// Commentators can view published content and submit comments, but cannot access admin features
func (m *CommentatorMiddleware) CommentatorOnly(next http.Handler) http.Handler {
	return m.authMiddleware.RequireRole(
		RoleCommentator,
		RoleContributor,
		RoleAdmin,
	)(next)
}
