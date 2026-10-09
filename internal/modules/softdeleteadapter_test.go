package modules_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSoftDeleteRepo implements repository.SoftDeleteRepo with canned records.
type stubSoftDeleteRepo struct {
	byUser  map[int][]*repository.SoftDeletedContent
	byID    map[int]*repository.SoftDeletedContent
	byIDErr error
}

func (s *stubSoftDeleteRepo) SoftDeleteContent(_ context.Context, _ string, _ int, _ int, _ int, _ string) error {
	return nil
}

func (s *stubSoftDeleteRepo) GetSoftDeletedContentByUser(_ context.Context, userID int) ([]*repository.SoftDeletedContent, error) {
	return s.byUser[userID], nil
}

func (s *stubSoftDeleteRepo) GetSoftDeletedContentByID(_ context.Context, id int) (*repository.SoftDeletedContent, error) {
	if s.byIDErr != nil {
		return nil, s.byIDErr
	}
	return s.byID[id], nil
}

func (s *stubSoftDeleteRepo) RestoreContent(_ context.Context, _ string, _ int) error {
	return nil
}

func (s *stubSoftDeleteRepo) PermanentDeleteContent(_ context.Context, _ string, _ int) error {
	return nil
}

func TestSoftDeleteStore(t *testing.T) {
	tests := []struct {
		name    string
		repo    *stubSoftDeleteRepo
		run     func(t *testing.T, store *modules.SoftDeleteStore)
		wantErr bool
	}{
		{
			name: "success - reason present and absent",
			repo: &stubSoftDeleteRepo{
				byUser: map[int][]*repository.SoftDeletedContent{
					7: {
						{
							ID:          1,
							ContentType: "post",
							ContentID:   42,
							DeletedAt:   "2026-01-01",
							DeletedBy:   2,
							Reason:      sql.NullString{String: "spam", Valid: true},
						},
						{
							ID:          2,
							ContentType: "page",
							ContentID:   43,
							DeletedAt:   "2026-01-02",
							DeletedBy:   2,
						},
					},
				},
			},
			run: func(t *testing.T, store *modules.SoftDeleteStore) {
				items, err := store.GetSoftDeletedContentByUser(context.Background(), 7)
				require.NoError(t, err)
				require.Len(t, items, 2)
				assert.Equal(t, "spam", items[0].Reason)
				assert.True(t, items[0].HasReason)
				assert.Empty(t, items[1].Reason)
				assert.False(t, items[1].HasReason)
				assert.Equal(t, "post", items[0].ContentType)
				assert.Equal(t, 42, items[0].ContentID)
			},
			wantErr: false,
		},
		{
			name: "success - restore delegates",
			repo: &stubSoftDeleteRepo{},
			run: func(t *testing.T, store *modules.SoftDeleteStore) {
				require.NoError(t, store.RestoreContent(context.Background(), "post", 42))
			},
			wantErr: false,
		},
		{
			name: "error - by id propagates",
			repo: &stubSoftDeleteRepo{byIDErr: assert.AnError},
			run: func(t *testing.T, store *modules.SoftDeleteStore) {
				_, err := store.GetSoftDeletedContentByID(context.Background(), 1)
				require.Error(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := modules.NewSoftDeleteStore(tt.repo)
			tt.run(t, store)
		})
	}
}
