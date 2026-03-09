package memory

import (
	"context"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type UserRepository struct {
	mu      sync.RWMutex
	nextID  int64
	byID    map[int64]entities.User
	byEmail map[string]int64
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		nextID:  1,
		byID:    make(map[int64]entities.User),
		byEmail: make(map[string]int64),
	}
}

func (r *UserRepository) Create(_ context.Context, user entities.User) (entities.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byEmail[user.Email]; exists {
		return entities.User{}, domainerrors.ErrAlreadyExists
	}

	now := time.Now().UTC()
	user.ID = r.nextID
	r.nextID++
	user.CreatedAt = now
	user.UpdatedAt = now

	r.byID[user.ID] = user
	r.byEmail[user.Email] = user.ID

	return user, nil
}

func (r *UserRepository) GetByEmail(_ context.Context, email string) (entities.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.byEmail[email]
	if !exists {
		return entities.User{}, domainerrors.ErrNotFound
	}

	return r.byID[id], nil
}

func (r *UserRepository) GetByID(_ context.Context, id int64) (entities.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.byID[id]
	if !exists {
		return entities.User{}, domainerrors.ErrNotFound
	}

	return user, nil
}
