package sqlite

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

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO content_aliases (content_id, alias)
		VALUES (?, ?)
	`, a.ContentID, a.Alias)
	if err != nil {
		if isUniqueConstraintError(err) {
			return alias.ErrAliasAlreadyExists
		}
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = int(id)
	return nil
}

func (r *AliasRepository) DeleteByContentID(ctx context.Context, contentID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `DELETE FROM content_aliases WHERE content_id = ?`, contentID)
	return err
}

func (r *AliasRepository) Repoint(ctx context.Context, aliasStr string, fromContentID, toContentID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx,
		`UPDATE content_aliases SET content_id = ? WHERE alias = ? AND content_id = ?`,
		toContentID, aliasStr, fromContentID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return alias.ErrAliasNotFound
	}
	return nil
}

func (r *AliasRepository) FindByAlias(ctx context.Context, aliasStr string) (*alias.Alias, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, content_id, alias FROM content_aliases WHERE alias = ?`, aliasStr)
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

	rows, err := r.db.QueryContext(ctx, `SELECT id, content_id, alias FROM content_aliases WHERE content_id = ?`, contentID)
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
