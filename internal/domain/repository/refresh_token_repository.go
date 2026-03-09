package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, token entities.RefreshToken) error
	GetActiveByHash(ctx context.Context, tokenHash string) (entities.RefreshToken, error)
	RevokeByHash(ctx context.Context, tokenHash string) error
}
