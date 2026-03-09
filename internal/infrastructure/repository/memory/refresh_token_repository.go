package memory

import (
	"context"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type RefreshTokenRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[string]entities.RefreshToken
}

func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{
		nextID: 1,
		items:  make(map[string]entities.RefreshToken),
	}
}

func (r *RefreshTokenRepository) Create(_ context.Context, token entities.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	token.ID = r.nextID
	r.nextID++
	token.CreatedAt = now

	r.items[token.TokenHash] = token
	return nil
}

func (r *RefreshTokenRepository) GetActiveByHash(_ context.Context, tokenHash string) (entities.RefreshToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, exists := r.items[tokenHash]
	if !exists {
		return entities.RefreshToken{}, domainerrors.ErrNotFound
	}

	if token.RevokedAt != nil || token.ExpiresAt.Before(time.Now().UTC()) {
		return entities.RefreshToken{}, domainerrors.ErrNotFound
	}

	return token, nil
}

func (r *RefreshTokenRepository) RevokeByHash(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, exists := r.items[tokenHash]
	if !exists {
		return domainerrors.ErrNotFound
	}
	if token.RevokedAt != nil {
		return domainerrors.ErrNotFound
	}

	now := time.Now().UTC()
	token.RevokedAt = &now
	r.items[tokenHash] = token
	return nil
}
