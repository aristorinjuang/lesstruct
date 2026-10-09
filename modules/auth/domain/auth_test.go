package auth_test

import (
	"testing"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthService(t *testing.T) {
	service := authdomain.NewAuthService("test-hash", stubPolicy{})
	require.NotNil(t, service, "NewAuthService() should not return nil")
}

func TestLogin_ValidDefaultCredentials(t *testing.T) {
	// Create a password hash for the default password
	hash, err := stubPolicy{}.HashPassword(authdomain.DefaultPassword)
	require.NoError(t, err, "Failed to hash password")

	service := authdomain.NewAuthService(hash, stubPolicy{})

	userInfo, err := service.Login(authdomain.DefaultUsername, authdomain.DefaultPassword)
	require.NoError(t, err, "Login() should succeed with valid credentials")
	require.NotNil(t, userInfo, "Login() should return userInfo")

	assert.Equal(t, authdomain.DefaultAdminID, userInfo.ID, "ID should match default admin ID")
	assert.Equal(t, authdomain.DefaultUsername, userInfo.Username, "Username should match default username")
	assert.Equal(t, authdomain.DefaultRole, userInfo.Role, "Role should match default role")
}

func TestLogin_InvalidUsername(t *testing.T) {
	hash, err := stubPolicy{}.HashPassword(authdomain.DefaultPassword)
	require.NoError(t, err, "Failed to hash password")

	service := authdomain.NewAuthService(hash, stubPolicy{})

	userInfo, err := service.Login("wronguser", authdomain.DefaultPassword)
	require.Error(t, err, "Login() should fail with invalid username")
	assert.Nil(t, userInfo, "Login() should not return userInfo for invalid username")
	assert.Contains(t, err.Error(), "invalid username or password", "Error message should indicate invalid credentials")
}

func TestLogin_InvalidPassword(t *testing.T) {
	hash, err := stubPolicy{}.HashPassword(authdomain.DefaultPassword)
	require.NoError(t, err, "Failed to hash password")

	service := authdomain.NewAuthService(hash, stubPolicy{})

	userInfo, err := service.Login(authdomain.DefaultUsername, "wrongpassword")
	require.Error(t, err, "Login() should fail with invalid password")
	assert.Nil(t, userInfo, "Login() should not return userInfo for invalid password")
	assert.Contains(t, err.Error(), "invalid username or password", "Error message should indicate invalid credentials")
}

func TestUserInfo_String(t *testing.T) {
	userInfo := &authdomain.UserInfo{
		ID:       "test-id",
		Username: "testuser",
		Role:     "User",
	}

	str := userInfo.String()
	assert.Contains(t, str, "test-id", "String() should contain ID")
	assert.Contains(t, str, "testuser", "String() should contain username")
	assert.Contains(t, str, "User", "String() should contain role")
}
