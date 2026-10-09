// Package store is the single-source alias repository: one implementation
// against *sql.DB plus a kernel Dialect, replacing the three per-driver
// copies. Placeholder rendering, RETURNING support, and duplicate detection
// come from the dialect; everything else (columns, scans, messages, guards)
// ports the SQLite implementation verbatim.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/modules/seo/alias"
)

// Repository implements alias.Repository for every driver.
type Repository struct {
	db      *sql.DB
	dialect kerneldatabase.Dialect
}

// placeholders returns a positional placeholder generator: each call renders
// the next bind marker ($1, $2… on PostgreSQL; ? elsewhere), so call order
// matches argument order by construction.
func (r *Repository) placeholders() func() string {
	index := 0
	return func() string {
		marker := r.dialect.Placeholder(index)
		index++
		return marker
	}
}

func (r *Repository) Create(ctx context.Context, a *alias.Alias) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	if r.dialect.SupportsReturning() {
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO content_aliases (content_id, alias)
			VALUES (`+next()+`, `+next()+`)
			RETURNING id
		`, a.ContentID, a.Alias).Scan(&a.ID)
		if err != nil {
			if r.dialect.IsDuplicateError(err) {
				return alias.ErrAliasAlreadyExists
			}
			return err
		}
		return nil
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO content_aliases (content_id, alias)
		VALUES (`+next()+`, `+next()+`)
	`, a.ContentID, a.Alias)
	if err != nil {
		if r.dialect.IsDuplicateError(err) {
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

func (r *Repository) DeleteByContentID(ctx context.Context, contentID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	_, err := r.db.ExecContext(ctx, `DELETE FROM content_aliases WHERE content_id = `+next(), contentID)
	return err
}

func (r *Repository) Repoint(ctx context.Context, aliasStr string, fromContentID, toContentID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	result, err := r.db.ExecContext(ctx,
		`UPDATE content_aliases SET content_id = `+next()+` WHERE alias = `+next()+` AND content_id = `+next(),
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

func (r *Repository) FindByAlias(ctx context.Context, aliasStr string) (*alias.Alias, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	row := r.db.QueryRowContext(ctx, `SELECT id, content_id, alias FROM content_aliases WHERE alias = `+next(), aliasStr)
	var a alias.Alias
	if err := row.Scan(&a.ID, &a.ContentID, &a.Alias); err != nil {
		if err == sql.ErrNoRows {
			return nil, alias.ErrAliasNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *Repository) FindByContentID(ctx context.Context, contentID int) ([]*alias.Alias, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	rows, err := r.db.QueryContext(ctx, `SELECT id, content_id, alias FROM content_aliases WHERE content_id = `+next(), contentID)
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

func (r *Repository) FindAll(ctx context.Context) ([]*alias.Alias, error) {
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

// New creates a repository on db speaking dialect. Dialect must be non-nil;
// the composition root resolves it from the driver registry.
func New(db *sql.DB, dialect kerneldatabase.Dialect) *Repository {
	return &Repository{
		db:      db,
		dialect: dialect,
	}
}

var _ alias.Repository = (*Repository)(nil)
