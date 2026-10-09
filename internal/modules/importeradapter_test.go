package modules_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	wordpress "github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuthorStore is a test double for the adapter's narrow user store.
type stubAuthorStore struct {
	byUsername map[string]*userdomain.User
	byEmail    map[string]*userdomain.User
	created    []*userdomain.User
	createErr  error
	lookupErr  error
}

func (s *stubAuthorStore) GetUserByUsername(_ context.Context, username string) (*userdomain.User, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return s.byUsername[username], nil
}

func (s *stubAuthorStore) GetUserByEmail(_ context.Context, email string) (*userdomain.User, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return s.byEmail[email], nil
}

func (s *stubAuthorStore) CreateUser(_ context.Context, user *userdomain.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	user.ID = len(s.created) + 1
	s.created = append(s.created, user)
	return nil
}

func TestWordPressUserProvisioner_Find(t *testing.T) {
	tests := []struct {
		name      string
		store     *stubAuthorStore
		byEmail   bool
		key       string
		wantID    int
		wantFound bool
		wantErr   bool
	}{
		{
			name: "success - username hit",
			store: &stubAuthorStore{
				byUsername: map[string]*userdomain.User{
					"alice": {ID: 7, Username: "alice", Email: "alice@example.com"},
				},
			},
			byEmail:   false,
			key:       "alice",
			wantID:    7,
			wantFound: true,
			wantErr:   false,
		},
		{
			name:      "success - username miss returns nil account",
			store:     &stubAuthorStore{},
			byEmail:   false,
			key:       "ghost",
			wantID:    0,
			wantFound: false,
			wantErr:   false,
		},
		{
			name: "success - email hit",
			store: &stubAuthorStore{
				byEmail: map[string]*userdomain.User{
					"bob@example.com": {ID: 9, Username: "bob", Email: "bob@example.com"},
				},
			},
			byEmail:   true,
			key:       "bob@example.com",
			wantID:    9,
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "error - lookup failure propagates",
			store: &stubAuthorStore{
				lookupErr: errors.New("db connection lost"),
			},
			byEmail:   false,
			key:       "alice",
			wantID:    0,
			wantFound: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioner := modules.NewWordPressUserProvisioner(tt.store)

			var (
				acct *wordpress.AuthorAccount
				err  error
			)
			if tt.byEmail {
				acct, err = provisioner.FindByEmail(context.Background(), tt.key)
			} else {
				acct, err = provisioner.FindByUsername(context.Background(), tt.key)
			}
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, acct)
				return
			}
			require.NoError(t, err)
			if !tt.wantFound {
				assert.Nil(t, acct)
				return
			}
			require.NotNil(t, acct)
			assert.Equal(t, tt.wantID, acct.ID)
		})
	}
}

func TestWordPressUserProvisioner_CreateAuthor(t *testing.T) {
	tests := []struct {
		name         string
		store        *stubAuthorStore
		username     string
		email        string
		displayName  string
		wantID       int
		wantErr      bool
		wantSentinel error
		wantMsg      string
	}{
		{
			name:         "success - contributor with verified status and hashed password",
			store:        &stubAuthorStore{},
			username:     "newauthor",
			email:        "newauthor@example.com",
			displayName:  "New Author",
			wantID:       1,
			wantErr:      false,
			wantSentinel: nil,
			wantMsg:      "",
		},
		{
			name:         "error - invalid email maps to sentinel",
			store:        &stubAuthorStore{},
			username:     "bademail",
			email:        "not-an-email",
			displayName:  "Bad Email",
			wantID:       0,
			wantErr:      true,
			wantSentinel: wordpress.ErrInvalidEmail,
			wantMsg:      "please enter a valid email address",
		},
		{
			name: "error - repository failure propagates",
			store: &stubAuthorStore{
				createErr: errors.New("db connection lost"),
			},
			username:     "newauthor",
			email:        "newauthor@example.com",
			displayName:  "New Author",
			wantID:       0,
			wantErr:      true,
			wantSentinel: nil,
			wantMsg:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioner := modules.NewWordPressUserProvisioner(tt.store)

			acct, err := provisioner.CreateAuthor(context.Background(), tt.username, tt.email, tt.displayName)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, acct)
				if tt.wantSentinel != nil {
					assert.ErrorIs(t, err, tt.wantSentinel)
				}
				if tt.wantMsg != "" {
					assert.Equal(t, tt.wantMsg, err.Error())
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, acct)
			assert.Equal(t, tt.wantID, acct.ID)
			assert.Equal(t, tt.username, acct.Username)
			assert.Equal(t, tt.email, acct.Email)

			require.Len(t, tt.store.created, 1)
			created := tt.store.created[0]
			assert.Equal(t, tt.username, created.Username)
			assert.Equal(t, tt.email, created.Email)
			assert.Equal(t, tt.displayName, created.Name)
			assert.Equal(t, "Contributor", created.Role)
			assert.Equal(t, "verified", created.Status)
			assert.NotEmpty(t, created.PasswordHash)
		})
	}
}
