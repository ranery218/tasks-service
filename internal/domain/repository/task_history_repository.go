package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type TaskHistoryRepository interface {
	CreateMany(ctx context.Context, entries []entities.TaskHistory) error
	ListByTaskID(ctx context.Context, taskID int64) ([]entities.TaskHistory, error)
}
