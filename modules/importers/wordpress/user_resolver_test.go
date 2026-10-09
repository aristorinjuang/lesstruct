package wordpress_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createdAuthor records a CreateAuthor call seen by the stub provisioner.
type createdAuthor struct {
	username string
	email    string
	name     string
	id       int
}

// stubUserProvisioner is a test double for the wordpress.UserProvisioner
// interface. Account policy (password, role, status) lives in the real
// platform adapter; the stub only records the author facts the module passes.
type stubUserProvisioner struct {
	byUsername map[string]*wordpress.AuthorAccount
	byEmail    map[string]*wordpress.AuthorAccount
	created    []createdAuthor
	createErr  error
}

func (s *stubUserProvisioner) FindByUsername(_ context.Context, username string) (*wordpress.AuthorAccount, error) {
	return s.byUsername[username], nil
}

func (s *stubUserProvisioner) FindByEmail(_ context.Context, email string) (*wordpress.AuthorAccount, error) {
	return s.byEmail[email], nil
}

func (s *stubUserProvisioner) CreateAuthor(_ context.Context, username, email, name string) (*wordpress.AuthorAccount, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	id := len(s.created) + 1
	s.created = append(s.created, createdAuthor{
		username: username,
		email:    email,
		name:     name,
		id:       id,
	})
	acct := &wordpress.AuthorAccount{ID: id, Username: username, Email: email}
	if s.byUsername == nil {
		s.byUsername = make(map[string]*wordpress.AuthorAccount)
	}
	if s.byEmail == nil {
		s.byEmail = make(map[string]*wordpress.AuthorAccount)
	}
	s.byUsername[username] = acct
	s.byEmail[email] = acct
	return acct, nil
}

func TestUserResolver_ResolveOrCreate(t *testing.T) {
	tests := []struct {
		name         string
		provisioner  *stubUserProvisioner
		login        string
		email        string
		displayName  string
		wantID       int
		wantCreated  bool
		wantErr      bool
		wantUsername string
		wantName     string
		wantEmail    string
	}{
		{
			name: "success - existing user by username is reused",
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{
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
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail: map[string]*wordpress.AuthorAccount{
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
			name: "success - new user is provisioned",
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
			},
			login:        "newauthor",
			email:        "newauthor@example.com",
			displayName:  "New Author",
			wantID:       1,
			wantCreated:  true,
			wantErr:      false,
			wantUsername: "newauthor",
			wantName:     "New Author",
			wantEmail:    "newauthor@example.com",
		},
		{
			name: "success - username with dots is sanitized",
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
			},
			login:        "arief.kapuaspost",
			email:        "arief@example.com",
			displayName:  "Muhammad Arif",
			wantID:       1,
			wantCreated:  true,
			wantErr:      false,
			wantUsername: "arief-kapuaspost",
			wantName:     "Muhammad Arif",
			wantEmail:    "arief@example.com",
		},
		{
			name: "success - username with spaces is sanitized",
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
			},
			login:        "Anggita Diah Hadi Ratnasari",
			email:        "anggita@example.com",
			displayName:  "Anggita Hadi",
			wantID:       1,
			wantCreated:  true,
			wantErr:      false,
			wantUsername: "Anggita-Diah-Hadi-Ratnasari",
			wantName:     "Anggita Hadi",
			wantEmail:    "anggita@example.com",
		},
		{
			name: "error - empty email",
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
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
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
				createErr:  wordpress.ErrInvalidEmail,
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
			provisioner: &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
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
			resolver := wordpress.NewUserResolver(tt.provisioner, nil)
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
				require.Len(t, tt.provisioner.created, 1)
				createdAuthor := tt.provisioner.created[0]
				assert.Equal(t, tt.wantUsername, createdAuthor.username)
				assert.Equal(t, tt.wantName, createdAuthor.name)
				assert.Equal(t, tt.wantEmail, createdAuthor.email)
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
			provisioner := &stubUserProvisioner{
				byUsername: map[string]*wordpress.AuthorAccount{},
				byEmail:    map[string]*wordpress.AuthorAccount{},
			}
			resolver := wordpress.NewUserResolver(provisioner, nil)
			_, _, err := resolver.ResolveOrCreate(
				context.Background(),
				tt.login,
				fmt.Sprintf("%s@example.com", tt.login),
				tt.displayName,
			)
			require.NoError(t, err)
			require.Len(t, provisioner.created, 1)
			assert.Equal(t, tt.wantName, provisioner.created[0].name)
		})
	}
}
