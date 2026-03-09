package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type RefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token entities.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`

	if _, err := r.db.ExecContext(ctx, query, token.UserID, token.TokenHash, token.ExpiresAt); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}

	return nil
}

func (r *RefreshTokenRepository) GetActiveByHash(ctx context.Context, tokenHash string) (entities.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = ?
		  AND revoked_at IS NULL
		  AND expires_at > UTC_TIMESTAMP()
		LIMIT 1
	`

	var token entities.RefreshToken
	if err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.RefreshToken{}, domainerrors.ErrNotFound
		}
		return entities.RefreshToken{}, fmt.Errorf("get active refresh token: %w", err)
	}

	return token, nil
}

func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = ?
		WHERE token_hash = ? AND revoked_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domainerrors.ErrNotFound
	}

	return nil
}
