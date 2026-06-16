package auth_test

import (
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestValidateEmail_Valid(t *testing.T) {
	validEmails := []string{
		"admin@example.com",
		"user.name@example.com",
		"user+tag@example.com",
		"test@sub.example.com",
		"user123@test-domain.co.uk",
		"first.last@domain.info",
	}

	for _, email := range validEmails {
		t.Run(email, func(t *testing.T) {
			err := appauth.ValidateEmail(email)
			assert.NoError(t, err, "ValidateEmail(%q) unexpected error", email)
		})
	}
}

func TestValidateEmail_InvalidFormat(t *testing.T) {
	invalidEmails := []string{
		"noat.com",
		"missing@domain",
		"@nodomain.com",
		"spaces in@email.com",
		"double@@at.com",
		"nodomain@",
		"",
	}

	for _, email := range invalidEmails {
		t.Run(email, func(t *testing.T) {
			err := appauth.ValidateEmail(email)
			assert.Error(t, err, "ValidateEmail(%q) expected error", email)
			assert.EqualError(t, err, "please enter a valid email address", "ValidateEmail(%q) unexpected error message", email)
		})
	}
}

func TestValidateEmail_Empty(t *testing.T) {
	err := appauth.ValidateEmail("")
	assert.Error(t, err, "ValidateEmail() expected error for empty email")
	assert.EqualError(t, err, "please enter a valid email address", "ValidateEmail() unexpected error message")
}

func TestValidateEmail_Subdomains(t *testing.T) {
	subdomainEmails := []string{
		"user@sub.example.com",
		"admin@mail.company.co.uk",
		"test@deep.nested.sub.domain.com",
	}

	for _, email := range subdomainEmails {
		t.Run(email, func(t *testing.T) {
			err := appauth.ValidateEmail(email)
			assert.NoError(t, err, "ValidateEmail(%q) unexpected error", email)
		})
	}
}

func TestValidateEmail_SpecialChars(t *testing.T) {
	specialCharEmails := []string{
		"user+tag@example.com",
		"user.name@example.com",
		"user_name@example.com",
		"user-name@example.com",
	}

	for _, email := range specialCharEmails {
		t.Run(email, func(t *testing.T) {
			err := appauth.ValidateEmail(email)
			assert.NoError(t, err, "ValidateEmail(%q) unexpected error", email)
		})
	}
}
