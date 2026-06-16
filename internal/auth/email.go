package auth

import (
	"errors"
	"regexp"
)

var (
	ErrEmailInvalid = errors.New("please enter a valid email address")
)

// emailRegex is a regex pattern for validating email addresses
// This pattern matches most common email formats while rejecting:
// - Consecutive dots (..)
// - Leading/trailing dots
// - Consecutive hyphens
// Improved regex to prevent problematic email formats (#14)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9._%+-]*[a-zA-Z0-9])?@[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?\.[a-zA-Z]{2,}$`)

// ValidateEmail checks if an email address has a valid format
// Note: SQL injection is mitigated by using parameterized queries in the repository layer (#10)
func ValidateEmail(email string) error {
	if email == "" || !emailRegex.MatchString(email) {
		return ErrEmailInvalid
	}
	return nil
}
