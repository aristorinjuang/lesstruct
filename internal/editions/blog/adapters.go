// Adapters between the platform services and the blog edition's modules:
// comments, SEO, the SSR engine, and the WordPress importer. They live in
// the edition — not internal/modules — so the leaner editions' binaries
// never link the module types they adapt.
package blog

import (
	"context"
	"errors"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	commentapi "github.com/aristorinjuang/lesstruct/modules/comments/api"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	wordpress "github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	seoapi "github.com/aristorinjuang/lesstruct/modules/seo/api"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// projectContent maps a content entity onto the site-ssr projection: every
// field the assembler reads, nothing it does not. Identity, ownership, and
// moderation state stay behind the seam by construction.
func projectContent(item *contentdomain.Content) sitessr.ContentRef {
	return sitessr.ContentRef{
		ID:                 item.ID,
		Slug:               item.Slug,
		Title:              item.Title,
		Content:            item.Content,
		Tags:               item.Tags,
		Format:             string(item.Format),
		PostType:           item.PostType,
		Language:           item.Language,
		TranslationGroupID: item.TranslationGroupID,
		Author:             item.Author,
		Username:           item.Username,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		AllowComments:      item.AllowComments,
		CustomFields:       item.CustomFields,
		MetaDescription:    item.MetaDescription,
		OGTitle:            item.OGTitle,
		OGDescription:      item.OGDescription,
		FeaturedImage:      item.FeaturedImage,
	}
}

func projectContents(items []*contentdomain.Content) []sitessr.ContentRef {
	refs := make([]sitessr.ContentRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, projectContent(item))
	}
	return refs
}

// NewAllowCommentsChecker adapts the content service to the comments
// module's SubmitComment gate. A missing item translates to the domain's
// ErrContentNotFound so the submit path maps it exactly as before; any
// other failure propagates for the 500 path.
func NewAllowCommentsChecker(content *contentservice.Service) commentdomain.AllowCommentsChecker {
	return func(ctx context.Context, contentID int) (bool, error) {
		item, err := content.GetByID(ctx, contentID)
		if err != nil {
			if errors.Is(err, contentdomain.ErrContentNotFound) {
				return false, commentdomain.ErrContentNotFound
			}
			return false, err
		}
		return item.AllowComments, nil
	}
}

// NewBrowserContentResolver adapts the content service to the comments
// module's browser seam: slug in, comment-relevant facts out. A missing item
// translates to the seam's ErrContentNotFound; any other failure propagates
// for the handler's 500 path.
func NewBrowserContentResolver(content *contentservice.Service) commentapi.ContentResolver {
	return func(ctx context.Context, slug, language string) (commentapi.ContentRef, error) {
		item, err := content.GetPublishedBySlug(ctx, slug, language)
		if err != nil {
			if errors.Is(err, contentdomain.ErrContentNotFound) {
				return commentapi.ContentRef{}, commentapi.ErrContentNotFound
			}
			return commentapi.ContentRef{}, err
		}
		return commentapi.ContentRef{
			ID:            item.ID,
			AllowComments: item.AllowComments,
		}, nil
	}
}

// NewAgentContentLookup adapts the content service to the comments module's
// agent seam: content ID in, visibility and gate facts out. A missing item
// translates to the seam's ErrContentNotFound; any other failure propagates
// for the handler's 500 path.
func NewAgentContentLookup(content *contentservice.Service) commentagent.ContentLookup {
	return func(ctx context.Context, id int) (commentagent.ContentDetail, error) {
		item, err := content.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, contentdomain.ErrContentNotFound) {
				return commentagent.ContentDetail{}, commentagent.ErrContentNotFound
			}
			return commentagent.ContentDetail{}, err
		}
		return commentagent.ContentDetail{
			ID:            item.ID,
			UserID:        item.UserID,
			Published:     item.Status == contentdomain.StatusPublished,
			AllowComments: item.AllowComments,
		}, nil
	}
}

// NewSEOPublishedPages adapts the content service to the seo module's
// sitemap seam: published items in, sitemap-relevant projections out.
// Lookup failures propagate for the handler's 500 path.
func NewSEOPublishedPages(content *contentservice.Service) seoapi.PublishedPages {
	return func(ctx context.Context, limit, offset int) ([]seoapi.PageRef, error) {
		items, err := content.GetPublished(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		pages := make([]seoapi.PageRef, 0, len(items))
		for _, item := range items {
			pages = append(pages, seoapi.PageRef{
				ID:                 item.ID,
				Slug:               item.Slug,
				PostType:           item.PostType,
				Language:           item.Language,
				TranslationGroupID: item.TranslationGroupID,
				UpdatedAt:          item.UpdatedAt,
			})
		}
		return pages, nil
	}
}

// siteSSRContentReader adapts the content service (plus the comments module
// service) to the site-ssr ContentReader seam.
type siteSSRContentReader struct {
	content  *contentservice.Service
	comments *commentdomain.Service
}

func (r *siteSSRContentReader) GetPublished(ctx context.Context, limit, offset int) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublished(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedPages(ctx context.Context) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedPages(ctx)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {
	return r.content.GetPublishedCustomPostTypes(ctx)
}

func (r *siteSSRContentReader) GetTranslations(ctx context.Context, groupID, excludeID int) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetTranslations(ctx, groupID, excludeID)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedByID(ctx context.Context, id int) (sitessr.ContentRef, error) {
	item, err := r.content.GetPublishedByID(ctx, id)
	if err != nil {
		return sitessr.ContentRef{}, err
	}
	return projectContent(item), nil
}

func (r *siteSSRContentReader) GetPublishedByPostType(
	ctx context.Context,
	postType string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedByPostType(ctx, postType, languages, year, month, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedTags(ctx context.Context) ([]string, error) {
	return r.content.GetPublishedTags(ctx)
}

func (r *siteSSRContentReader) GetPublishedBySlugAny(ctx context.Context, slug string) (sitessr.ContentRef, error) {
	item, err := r.content.GetPublishedBySlugAny(ctx, slug)
	if err != nil {
		return sitessr.ContentRef{}, err
	}
	return projectContent(item), nil
}

func (r *siteSSRContentReader) GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	return r.comments.GetCommentsForContent(ctx, contentID)
}

func (r *siteSSRContentReader) GetRelated(ctx context.Context, id, limit int) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetRelated(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedByAuthorUsername(
	ctx context.Context,
	username string,
	languages []string,
	limit, offset int,
) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedByAuthorUsername(ctx, username, languages, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) AuthorExists(ctx context.Context, username string) (bool, error) {
	return r.content.AuthorExists(ctx, username)
}

func (r *siteSSRContentReader) GetPublishedByTag(
	ctx context.Context,
	tag string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedByTag(ctx, tag, languages, year, month, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

var _ sitessr.ContentReader = (*siteSSRContentReader)(nil)

// NewSiteSSRContentReader adapts the content and comments services to the
// site-ssr ContentReader seam.
func NewSiteSSRContentReader(
	content *contentservice.Service,
	comments *commentdomain.Service,
) sitessr.ContentReader {
	return &siteSSRContentReader{
		content:  content,
		comments: comments,
	}
}

// authorStore is the user-repository subset the WordPress importer needs.
// repository.UserRepo satisfies it; the narrow shape keeps the adapter
// decoupled from the full user-management surface.
type authorStore interface {
	GetUserByUsername(ctx context.Context, username string) (*userdomain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*userdomain.User, error)
	CreateUser(ctx context.Context, user *userdomain.User) error
}

// invalidEmailError carries a platform email-syntax rejection across the
// provisioner seam. Its message is the platform error verbatim (so import
// job output stays byte-identical to the pre-module text) while errors.Is
// still matches the module's ErrInvalidEmail sentinel for the resolver's
// legacy "invalid email" mapping.
type invalidEmailError struct {
	err error
}

func (e *invalidEmailError) Error() string {
	return e.err.Error()
}

func (e *invalidEmailError) Is(target error) bool {
	return target == wordpress.ErrInvalidEmail
}

// WordPressUserProvisioner adapts the user repository to the importers
// module's UserProvisioner seam. Account policy stays platform-side: email
// syntax is enforced with auth.ValidateEmail (mapped to the module's
// ErrInvalidEmail sentinel so job output keeps the legacy "invalid email"
// failure), passwords are generated and hashed here, and provisioned
// accounts are Contributors with verified status.
type WordPressUserProvisioner struct {
	users authorStore
}

// FindByUsername returns the author account for username, or (nil, nil) when
// no account exists.
func (p *WordPressUserProvisioner) FindByUsername(ctx context.Context, username string) (*wordpress.AuthorAccount, error) {
	user, err := p.users.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &wordpress.AuthorAccount{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

// FindByEmail returns the author account for email, or (nil, nil) when no
// account exists.
func (p *WordPressUserProvisioner) FindByEmail(ctx context.Context, email string) (*wordpress.AuthorAccount, error) {
	user, err := p.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &wordpress.AuthorAccount{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

// CreateAuthor provisions a Contributor with verified status and a
// platform-generated password, returning the new account with its ID
// populated.
func (p *WordPressUserProvisioner) CreateAuthor(ctx context.Context, username, email, name string) (*wordpress.AuthorAccount, error) {
	if err := auth.ValidateEmail(email); err != nil {
		return nil, &invalidEmailError{err: err}
	}

	plainPassword, err := auth.GeneratePassword(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	passwordHash, err := auth.HashPassword(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &userdomain.User{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        email,
		Name:         name,
		Role:         "Contributor",
		Status:       "verified",
	}
	if err := p.users.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &wordpress.AuthorAccount{
		ID:       newUser.ID,
		Username: username,
		Email:    email,
	}, nil
}

// NewWordPressUserProvisioner creates a WordPressUserProvisioner.
func NewWordPressUserProvisioner(users authorStore) *WordPressUserProvisioner {
	return &WordPressUserProvisioner{
		users: users,
	}
}

// commentRoleCapabilities adapts the config-driven role registry to the
// comments module's two seams (browser and agent comment-posting gates).
// A nil service yields nil, and the module preserves the legacy allow-all
// behavior.
type commentRoleCapabilities struct {
	service *roledomain.Service
}

func (a *commentRoleCapabilities) CanComment(name string) bool {
	return a.service.CanComment(name)
}

// NewCommentRoleCapabilities adapts the config-driven role registry to the
// comments module's browser seam (comment-posting gate).
func NewCommentRoleCapabilities(service *roledomain.Service) commentapi.RoleCapabilities {
	if service == nil {
		return nil
	}
	return &commentRoleCapabilities{
		service: service,
	}
}

// NewAgentCommentRoleCapabilities adapts the config-driven role registry to
// the comments module's agent seam (comment-posting gate).
func NewAgentCommentRoleCapabilities(service *roledomain.Service) commentagent.RoleCapabilities {
	if service == nil {
		return nil
	}
	return &commentRoleCapabilities{
		service: service,
	}
}
