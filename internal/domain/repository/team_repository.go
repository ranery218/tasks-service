package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type TeamRepository interface {
	Create(ctx context.Context, team entities.Team) (entities.Team, error)
	GetByID(ctx context.Context, id int64) (entities.Team, error)
	DeleteByID(ctx context.Context, id int64) error
	ListByUserID(ctx context.Context, userID int64) ([]entities.Team, error)
}
