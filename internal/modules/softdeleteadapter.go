package modules

import (
	"context"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	usersapi "github.com/aristorinjuang/lesstruct/modules/users/api"
)

func convertSoftDeletedContent(item *repository.SoftDeletedContent) *usersapi.SoftDeletedContent {
	out := &usersapi.SoftDeletedContent{
		ID:          item.ID,
		ContentType: item.ContentType,
		ContentID:   item.ContentID,
		DeletedAt:   item.DeletedAt,
		DeletedBy:   item.DeletedBy,
	}
	if item.Reason.Valid {
		out.Reason = item.Reason.String
		out.HasReason = true
	}
	return out
}

// SoftDeleteStore adapts repository.SoftDeleteRepo to the users module's
// SoftDeleteStore seam. Database null types are converted at this boundary
// so the module never imports database/sql. It lives in core — the only
// layer that may import both sides.
type SoftDeleteStore struct {
	repo repository.SoftDeleteRepo
}

// GetSoftDeletedContentByUser returns the soft-deleted content of a user.
func (s *SoftDeleteStore) GetSoftDeletedContentByUser(ctx context.Context, userID int) ([]*usersapi.SoftDeletedContent, error) {
	items, err := s.repo.GetSoftDeletedContentByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*usersapi.SoftDeletedContent, len(items))
	for i, item := range items {
		out[i] = convertSoftDeletedContent(item)
	}
	return out, nil
}

// GetSoftDeletedContentByID returns one soft-deleted content record.
func (s *SoftDeleteStore) GetSoftDeletedContentByID(ctx context.Context, id int) (*usersapi.SoftDeletedContent, error) {
	item, err := s.repo.GetSoftDeletedContentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return convertSoftDeletedContent(item), nil
}

// RestoreContent restores soft-deleted content.
func (s *SoftDeleteStore) RestoreContent(ctx context.Context, contentType string, contentID int) error {
	return s.repo.RestoreContent(ctx, contentType, contentID)
}

// NewSoftDeleteStore creates a SoftDeleteStore.
func NewSoftDeleteStore(repo repository.SoftDeleteRepo) *SoftDeleteStore {
	return &SoftDeleteStore{repo: repo}
}

var _ usersapi.SoftDeleteStore = (*SoftDeleteStore)(nil)
