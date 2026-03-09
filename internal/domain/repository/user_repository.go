package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type UserRepository interface {
	Create(ctx context.Context, user entities.User) (entities.User, error)
	GetByEmail(ctx context.Context, email string) (entities.User, error)
	GetByID(ctx context.Context, id int64) (entities.User, error)
}
