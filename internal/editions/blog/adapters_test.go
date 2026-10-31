package blog

import (
	"context"
	"errors"
	"testing"
	"time"

	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/config"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/role"
	commentapi "github.com/aristorinjuang/lesstruct/modules/comments/api"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	commentmocks "github.com/aristorinjuang/lesstruct/modules/comments/domain/mocks"
	wordpress "github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	seoapi "github.com/aristorinjuang/lesstruct/modules/seo/api"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewBrowserContentResolver(t *testing.T) {
	tests := []struct {
		name         string
		slug         string
		setupRepo    func(*contentmocks.MockRepository)
		wantRef      commentapi.ContentRef
		wantErr      error
		wantContains string
	}{
		{
			name: "success - projects id and gate flag",
			slug: "hello",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "hello", "en").Return(&contentdomain.Content{
					ID:            5,
					AllowComments: true,
				}, nil)
			},
			wantRef: commentapi.ContentRef{ID: 5, AllowComments: true},
			wantErr: nil,
		},
		{
			name: "error - missing content translates to the seam sentinel",
			slug: "gone",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "gone", "en").Return(nil, contentdomain.ErrContentNotFound)
			},
			wantErr: commentapi.ErrContentNotFound,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			slug: "boom",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublishedBySlug", mock.Anything, "boom", "en").Return(nil, errors.New("database error"))
			},
			wantContains: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			ref, err := NewBrowserContentResolver(content)(context.Background(), tt.slug, "en")

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRef, ref)
		})
	}
}

func TestNewAgentContentLookup(t *testing.T) {
	tests := []struct {
		name         string
		id           int
		setupRepo    func(*contentmocks.MockRepository)
		want         commentagent.ContentDetail
		wantErr      error
		wantContains string
	}{
		{
			name: "success - projects visibility and gate facts",
			id:   5,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 5).Return(&contentdomain.Content{
					ID:            5,
					UserID:        7,
					Status:        contentdomain.StatusPublished,
					AllowComments: true,
				}, nil)
			},
			want: commentagent.ContentDetail{ID: 5, UserID: 7, Published: true, AllowComments: true},
		},
		{
			name: "success - draft projects unpublished",
			id:   6,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 6).Return(&contentdomain.Content{
					ID:            6,
					UserID:        7,
					Status:        contentdomain.StatusDraft,
					AllowComments: true,
				}, nil)
			},
			want: commentagent.ContentDetail{ID: 6, UserID: 7, Published: false, AllowComments: true},
		},
		{
			name: "error - missing content translates to the seam sentinel",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantErr: commentagent.ErrContentNotFound,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, errors.New("database error"))
			},
			wantContains: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			detail, err := NewAgentContentLookup(content)(context.Background(), tt.id)

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.ID, detail.ID)
			assert.Equal(t, tt.want.UserID, detail.UserID)
			assert.Equal(t, tt.want.Published, detail.Published)
			assert.Equal(t, tt.want.AllowComments, detail.AllowComments)
		})
	}
}

func TestNewAllowCommentsChecker(t *testing.T) {
	tests := []struct {
		name         string
		id           int
		setupRepo    func(*contentmocks.MockRepository)
		wantAllow    bool
		wantErr      error
		wantContains string
	}{
		{
			name: "success - allowed content passes the gate",
			id:   5,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 5).Return(&contentdomain.Content{
					ID:            5,
					AllowComments: true,
				}, nil)
			},
			wantAllow: true,
			wantErr:   nil,
		},
		{
			name: "success - disallowed content fails the gate",
			id:   5,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 5).Return(&contentdomain.Content{
					ID:            5,
					AllowComments: false,
				}, nil)
			},
			wantAllow: false,
			wantErr:   nil,
		},
		{
			name: "error - missing content translates to the domain sentinel",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantAllow: false,
			wantErr:   commentdomain.ErrContentNotFound,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			id:   9,
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetByID", mock.Anything, 9).Return(nil, errors.New("database error"))
			},
			wantAllow:    false,
			wantContains: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			allow, err := NewAllowCommentsChecker(content)(context.Background(), tt.id)

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAllow, allow)
		})
	}
}

func TestNewSiteSSRContentReader(t *testing.T) {
	groupID := 3
	updatedAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		setupRepos  func(*contentmocks.MockRepository, *commentmocks.MockCommentRepository)
		call        func(sitessr.ContentReader) (any, error)
		want        any
		wantErr     bool
		wantErrText string
	}{
		{
			name: "success - projects pages with translation groups",
			setupRepos: func(repo *contentmocks.MockRepository, comments *commentmocks.MockCommentRepository) {
				repo.On("GetPublishedPages", mock.Anything).Return([]*contentdomain.Content{
					{
						ID:                 1,
						Slug:               "hello",
						Title:              "Hello",
						PostType:           "page",
						Language:           "en",
						TranslationGroupID: &groupID,
						Author:             "Author",
						CreatedAt:          updatedAt,
						AllowComments:      true,
						CustomFields:       map[string]any{"k": "v"},
					},
				}, nil)
			},
			call: func(reader sitessr.ContentReader) (any, error) {
				return reader.GetPublishedPages(context.Background())
			},
			want: []sitessr.ContentRef{
				{
					ID:                 1,
					Slug:               "hello",
					Title:              "Hello",
					PostType:           "page",
					Language:           "en",
					TranslationGroupID: &groupID,
					Author:             "Author",
					CreatedAt:          updatedAt,
					AllowComments:      true,
					CustomFields:       map[string]any{"k": "v"},
				},
			},
			wantErr: false,
		},
		{
			name: "success - delegates comment reads to the comments service",
			setupRepos: func(repo *contentmocks.MockRepository, comments *commentmocks.MockCommentRepository) {
				comments.On("GetByContentID", mock.Anything, 1).Return([]*commentdomain.Comment{
					{ID: 7, ContentID: 1, Comment: "Nice"},
				}, nil)
			},
			call: func(reader sitessr.ContentReader) (any, error) {
				return reader.GetCommentsForContent(context.Background(), 1)
			},
			want: []*commentdomain.Comment{
				{ID: 7, ContentID: 1, Comment: "Nice"},
			},
			wantErr: false,
		},
		{
			name: "error - content failure propagates",
			setupRepos: func(repo *contentmocks.MockRepository, comments *commentmocks.MockCommentRepository) {
				repo.On("GetPublishedPages", mock.Anything).Return(nil, errors.New("database error"))
			},
			call: func(reader sitessr.ContentReader) (any, error) {
				return reader.GetPublishedPages(context.Background())
			},
			wantErr:     true,
			wantErrText: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			commentRepo := commentmocks.NewMockCommentRepository(t)
			tt.setupRepos(repo, commentRepo)

			content := contentservice.NewService(repo, nil, nil)
			commentsService := commentdomain.NewService(commentRepo)
			reader := NewSiteSSRContentReader(content, commentsService)

			got, err := tt.call(reader)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
			provisioner := NewWordPressUserProvisioner(tt.store)

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
			provisioner := NewWordPressUserProvisioner(tt.store)

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

func TestNewCommentRoleCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		service *role.Service
		wantNil bool
	}{
		{
			name:    "success - wraps registry",
			service: role.NewService(),
			wantNil: false,
		},
		{
			name:    "success - nil service yields nil",
			service: nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewCommentRoleCapabilities(tt.service)
			if tt.wantNil {
				assert.Nil(t, adapter)
				return
			}
			require.NotNil(t, adapter)
			assert.True(t, adapter.CanComment("Commentator"))
		})
	}
}

func TestNewAgentCommentRoleCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		service *role.Service
		wantNil bool
	}{
		{
			name:    "success - wraps registry",
			service: role.NewService(),
			wantNil: false,
		},
		{
			name:    "success - nil service yields nil",
			service: nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAgentCommentRoleCapabilities(tt.service)
			if tt.wantNil {
				assert.Nil(t, adapter)
				return
			}
			require.NotNil(t, adapter)
			assert.True(t, adapter.CanComment("Commentator"))
		})
	}
}

func TestNewSEOPublishedPages(t *testing.T) {
	groupID := 3
	tests := []struct {
		name      string
		setupRepo func(*contentmocks.MockRepository)
		wantPages []seoapi.PageRef
		wantErr   bool
	}{
		{
			name: "success - projects sitemap facts",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublished", mock.Anything, 1000, 0).Return([]*contentdomain.Content{
					{
						ID:                 1,
						Slug:               "hello",
						PostType:           "post",
						Language:           "en",
						TranslationGroupID: &groupID,
						UpdatedAt:          time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
					},
				}, nil)
			},
			wantPages: []seoapi.PageRef{
				{
					ID:                 1,
					Slug:               "hello",
					PostType:           "post",
					Language:           "en",
					TranslationGroupID: &groupID,
					UpdatedAt:          time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
				},
			},
			wantErr: false,
		},
		{
			name: "error - lookup failure propagates for the 500 path",
			setupRepo: func(m *contentmocks.MockRepository) {
				m.On("GetPublished", mock.Anything, 1000, 0).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentmocks.NewMockRepository(t)
			tt.setupRepo(repo)
			content := contentservice.NewService(repo, nil, nil)

			pages, err := NewSEOPublishedPages(content)(context.Background(), 1000, 0)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, "database error")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPages, pages)
		})
	}
}

func TestSiteSSRConfig(t *testing.T) {
	tests := []struct {
		name   string
		config config.SiteConfig
		want   sitessr.SiteConfig
	}{
		{
			name:   "success - maps identity fields and ratio",
			config: config.SiteConfig{Name: "Merch", Logo: "/uploads/logo.png", Favicon: "/uploads/icon.png", ProductImageRatio: "1 / 1"},
			want:   sitessr.SiteConfig{Name: "Merch", Logo: "/uploads/logo.png", Favicon: "/uploads/icon.png", ProductImageRatio: "1 / 1"},
		},
		{
			name:   "success - zero value maps empty",
			config: config.SiteConfig{},
			want:   sitessr.SiteConfig{},
		},
		{
			name:   "success - hero fields map through",
			config: config.SiteConfig{HeroImage: "/uploads/hero.webp", HeroTitle: "Custom title", HeroSubtitle: "Custom subtitle"},
			want:   sitessr.SiteConfig{HeroImage: "/uploads/hero.webp", HeroTitle: "Custom title", HeroSubtitle: "Custom subtitle"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, siteSSRConfig(tt.config))
		})
	}
}

func TestLoadUICatalog(t *testing.T) {
	catalog := loadUICatalog([]string{"en", "id"})
	require.NotNil(t, catalog)
	assert.Equal(t, "Browse %s.", catalog.T("en", "ui.browse_posts"))
	assert.Equal(t, "Jelajahi %s.", catalog.T("id", "ui.browse_posts"))
	assert.Equal(t, "Postingan oleh %s.", catalog.T("id", "ui.posts_by"))
	assert.Equal(t, "Terima", catalog.T("id", "ui.cookie_accept"))
}
