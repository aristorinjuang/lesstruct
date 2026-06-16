package auth_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFirstLoginService(t *testing.T) {
	service := auth.NewFirstLoginService("$2a$12$defaultHash")
	require.NotNil(t, service, "NewFirstLoginService() should not return nil")
}

func TestIsSetupComplete_NotComplete(t *testing.T) {
	service := auth.NewFirstLoginService("$2a$12$defaultHash")

	assert.False(t, service.IsSetupComplete("$2a$12$defaultHash"), "IsSetupComplete() should be false when admin hash matches default")
}

func TestIsSetupComplete_Complete(t *testing.T) {
	service := auth.NewFirstLoginService("$2a$12$defaultHash")

	assert.True(t, service.IsSetupComplete("$2a$12$differentHash"), "IsSetupComplete() should be true when admin hash differs from default")
}

func TestIsSetupComplete_EmptyHash(t *testing.T) {
	service := auth.NewFirstLoginService("$2a$12$defaultHash")

	assert.False(t, service.IsSetupComplete(""), "IsSetupComplete() should be false for empty hash")
}

func TestIsSetupComplete_SameDefaultHash(t *testing.T) {
	hash := "$2a$12$someSpecificDefault"
	service := auth.NewFirstLoginService(hash)

	assert.False(t, service.IsSetupComplete(hash), "IsSetupComplete() should be false when hashes are identical")
}
