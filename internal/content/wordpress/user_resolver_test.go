package wordpress_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUserResolverRepo is a test double for the userResolverRepo interface.
type fakeUserResolverRepo struct {
	byUsername map[string]*repository.User
	byEmail    map[string]*repository.User
	created    []*repository.User
	createErr  error
}

func (f *fakeUserResolverRepo) GetUserByUsername(_ context.Context, username string) (*repository.User, error) {
	return f.byUsername[username], nil
}

func (f *fakeUserResolverRepo) GetUserByEmail(_ context.Context, email string) (*repository.User, error) {
	return f.byEmail[email], nil
}

func (f *fakeUserResolverRepo) CreateUser(_ context.Context, user *repository.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	user.ID = len(f.created) + 1
	f.created = append(f.created, user)
	if f.byUsername == nil {
		f.byUsername = make(map[string]*repository.User)
	}
	if f.byEmail == nil {
		f.byEmail = make(map[string]*repository.User)
	}
	f.byUsername[user.Username] = user
	f.byEmail[user.Email] = user
	return nil
}

func TestUserResolver_ResolveOrCreate(t *testing.T) {
	tests := []struct {
		name          string
		repo          *fakeUserResolverRepo
		login         string
		email         string
		displayName   string
		wantID        int
		wantCreated   bool
		wantErr       bool
		wantUsername  string
		wantName      string
		wantRole      string
		wantStatus    string
	}{
		{
			name: "success - existing user by username is reused",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{
					"alice": {ID: 42, Username: "alice"},
				},
			},
			login:       "alice",
			email:       "alice@example.com",
			displayName: "Alice",
			wantID:      42,
			wantCreated: false,
			wantErr:     false,
		},
		{
			name: "success - existing user by email is reused",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail: map[string]*repository.User{
					"alice@example.com": {ID: 42, Username: "alice2"},
				},
			},
			login:       "alice",
			email:       "alice@example.com",
			displayName: "Alice",
			wantID:      42,
			wantCreated: false,
			wantErr:     false,
		},
		{
			name: "success - new user created as Contributor with verified status",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
			},
			login:       "newauthor",
			email:       "newauthor@example.com",
			displayName: "New Author",
			wantID:      1,
			wantCreated: true,
			wantErr:     false,
			wantUsername: "newauthor",
			wantName:     "New Author",
			wantRole:     "Contributor",
			wantStatus:   "verified",
		},
		{
			name: "success - username with dots is sanitized",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
			},
			login:       "arief.kapuaspost",
			email:       "arief@example.com",
			displayName: "Muhammad Arif",
			wantID:      1,
			wantCreated: true,
			wantErr:     false,
			wantUsername: "arief-kapuaspost",
			wantName:     "Muhammad Arif",
			wantRole:     "Contributor",
			wantStatus:   "verified",
		},
		{
			name: "success - username with spaces is sanitized",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
			},
			login:       "Anggita Diah Hadi Ratnasari",
			email:       "anggita@example.com",
			displayName: "Anggita Hadi",
			wantID:      1,
			wantCreated: true,
			wantErr:     false,
			wantUsername: "Anggita-Diah-Hadi-Ratnasari",
			wantName:     "Anggita Hadi",
			wantRole:     "Contributor",
			wantStatus:   "verified",
		},
		{
			name: "error - empty email",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
			},
			login:       "ghostwriter",
			email:       "",
			displayName: "Ghost Writer",
			wantID:      0,
			wantCreated: false,
			wantErr:     true,
		},
		{
			name: "error - invalid email",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
			},
			login:       "bademail",
			email:       "not-an-email",
			displayName: "Bad Email",
			wantID:      0,
			wantCreated: false,
			wantErr:     true,
		},
		{
			name: "error - create fails",
			repo: &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
				createErr:  errors.New("db connection lost"),
			},
			login:       "newauthor",
			email:       "newauthor@example.com",
			displayName: "New Author",
			wantID:      0,
			wantCreated: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := wordpress.NewUserResolver(tt.repo, nil)
			id, created, err := resolver.ResolveOrCreate(
				context.Background(),
				tt.login,
				tt.email,
				tt.displayName,
			)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.wantID, id)
				assert.False(t, created)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantCreated, created)

			if tt.wantCreated {
				require.Len(t, tt.repo.created, 1)
				createdUser := tt.repo.created[0]
				assert.Equal(t, tt.wantUsername, createdUser.Username)
				assert.Equal(t, tt.wantName, createdUser.Name)
				assert.Equal(t, tt.wantRole, createdUser.Role)
				assert.Equal(t, tt.wantStatus, createdUser.Status)
				assert.NotEmpty(t, createdUser.PasswordHash)
			}
		})
	}
}

func TestUserResolver_ResolveOrCreate_DisplayNameFallback(t *testing.T) {
	tests := []struct {
		name        string
		login       string
		displayName string
		wantName    string
	}{
		{
			name:        "success - empty displayName falls back to login",
			login:       "noname",
			displayName: "",
			wantName:    "noname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserResolverRepo{
				byUsername: map[string]*repository.User{},
				byEmail:    map[string]*repository.User{},
			}
			resolver := wordpress.NewUserResolver(repo, nil)
			_, _, err := resolver.ResolveOrCreate(
				context.Background(),
				tt.login,
				fmt.Sprintf("%s@example.com", tt.login),
				tt.displayName,
			)
			require.NoError(t, err)
			require.Len(t, repo.created, 1)
			assert.Equal(t, tt.wantName, repo.created[0].Name)
		})
	}
}
