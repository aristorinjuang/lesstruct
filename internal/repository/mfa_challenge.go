package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
)

// MFAChallengeRepository handles login MFA challenge data operations.
type MFAChallengeRepository struct {
	db *sql.DB
}

// Create stores a new login challenge.
func (r *MFAChallengeRepository) Create(ctx context.Context, challenge *authdomain.MFAChallenge) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	createdAt := challenge.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO mfa_challenges (user_id, token_hash, code_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, challenge.UserID, challenge.TokenHash, challenge.CodeHash, challenge.ExpiresAt, createdAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	challenge.ID = int(id)
	challenge.CreatedAt = createdAt
	return nil
}

// FindByTokenHash returns the challenge for a token hash, expired or not. The
// service owns expiry decisions so it can distinguish expired from unknown.
func (r *MFAChallengeRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*authdomain.MFAChallenge, error) {
	var challenge authdomain.MFAChallenge
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, code_hash, attempts, expires_at, created_at
		FROM mfa_challenges
		WHERE token_hash = ?
	`, tokenHash).Scan(
		&challenge.ID,
		&challenge.UserID,
		&challenge.TokenHash,
		&challenge.CodeHash,
		&challenge.Attempts,
		&challenge.ExpiresAt,
		&challenge.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &challenge, nil
}

// IncrementAttempts spends one attempt while the challenge is below the cap
// and reports whether the attempt was spent.
func (r *MFAChallengeRepository) IncrementAttempts(ctx context.Context, id, maxAttempts int) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE mfa_challenges SET attempts = attempts + 1 WHERE id = ? AND attempts < ?
	`, id, maxAttempts)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

// Consume deletes the challenge and reports whether this caller removed it.
func (r *MFAChallengeRepository) Consume(ctx context.Context, id int) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM mfa_challenges WHERE id = ?
	`, id)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

// Delete removes one challenge.
func (r *MFAChallengeRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mfa_challenges WHERE id = ?
	`, id)
	return err
}

// DeleteUserChallenges removes every challenge for a user.
func (r *MFAChallengeRepository) DeleteUserChallenges(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mfa_challenges WHERE user_id = ?
	`, userID)
	return err
}

// DeleteExpired removes expired challenges. The cutoff is bound as a
// time.Time so it matches the driver's stored timestamp format and zone
// (datetime('now') compares against a UTC string and misfires on local time).
func (r *MFAChallengeRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mfa_challenges WHERE expires_at <= ?
	`, time.Now())
	return err
}

// NewMFAChallengeRepository creates a new login MFA challenge repository.
func NewMFAChallengeRepository(db *sql.DB) *MFAChallengeRepository {
	return &MFAChallengeRepository{
		db: db,
	}
}
