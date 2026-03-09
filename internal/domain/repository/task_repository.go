package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type TaskRepository interface {
	Create(ctx context.Context, task entities.Task) (entities.Task, error)
	GetByID(ctx context.Context, id int64) (entities.Task, error)
	Update(ctx context.Context, task entities.Task) (entities.Task, error)
	List(ctx context.Context, filter entities.TaskFilter) ([]entities.Task, int, error)
}
