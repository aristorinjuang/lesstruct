package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/alias"
)

type AliasRepository struct {
	db *sql.DB
}

func (r *AliasRepository) Create(ctx context.Context, a *alias.Alias) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO content_aliases (content_id, alias)
		VALUES ($1, $2)
		RETURNING id
	`, a.ContentID, a.Alias).Scan(&a.ID)
	if err != nil {
		if isUniqueConstraintError(err) {
			return alias.ErrAliasAlreadyExists
		}
		return err
	}
	return nil
}

func (r *AliasRepository) DeleteByContentID(ctx context.Context, contentID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `DELETE FROM content_aliases WHERE content_id = $1`, contentID)
	return err
}

func (r *AliasRepository) FindByAlias(ctx context.Context, aliasStr string) (*alias.Alias, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, content_id, alias FROM content_aliases WHERE alias = $1`, aliasStr)
	var a alias.Alias
	if err := row.Scan(&a.ID, &a.ContentID, &a.Alias); err != nil {
		if err == sql.ErrNoRows {
			return nil, alias.ErrAliasNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *AliasRepository) FindByContentID(ctx context.Context, contentID int) ([]*alias.Alias, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, content_id, alias FROM content_aliases WHERE content_id = $1`, contentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var aliases []*alias.Alias
	for rows.Next() {
		var a alias.Alias
		if err := rows.Scan(&a.ID, &a.ContentID, &a.Alias); err != nil {
			return nil, err
		}
		aliases = append(aliases, &a)
	}
	return aliases, rows.Err()
}

func (r *AliasRepository) FindAll(ctx context.Context) ([]*alias.Alias, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, content_id, alias FROM content_aliases`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var aliases []*alias.Alias
	for rows.Next() {
		var a alias.Alias
		if err := rows.Scan(&a.ID, &a.ContentID, &a.Alias); err != nil {
			return nil, err
		}
		aliases = append(aliases, &a)
	}
	return aliases, rows.Err()
}

func NewAliasRepository(db *sql.DB) *AliasRepository {
	return &AliasRepository{db: db}
}
