package postgresql

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

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO mfa_challenges (user_id, token_hash, code_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, challenge.UserID, challenge.TokenHash, challenge.CodeHash, challenge.ExpiresAt, createdAt).Scan(&challenge.ID)
	if err != nil {
		return err
	}

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
		WHERE token_hash = $1
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
		UPDATE mfa_challenges SET attempts = attempts + 1 WHERE id = $1 AND attempts < $2
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
		DELETE FROM mfa_challenges WHERE id = $1
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
		DELETE FROM mfa_challenges WHERE id = $1
	`, id)
	return err
}

// DeleteUserChallenges removes every challenge for a user.
func (r *MFAChallengeRepository) DeleteUserChallenges(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mfa_challenges WHERE user_id = $1
	`, userID)
	return err
}

// DeleteExpired removes expired challenges. The cutoff is bound as a
// time.Time so it matches the driver's stored timestamp zone.
func (r *MFAChallengeRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mfa_challenges WHERE expires_at <= $1
	`, time.Now())
	return err
}

// NewMFAChallengeRepository creates a new login MFA challenge repository.
func NewMFAChallengeRepository(db *sql.DB) *MFAChallengeRepository {
	return &MFAChallengeRepository{
		db: db,
	}
}
