package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type TaskCommentRepository interface {
	Create(ctx context.Context, comment entities.TaskComment) (entities.TaskComment, error)
}
