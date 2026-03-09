package memory

import (
	"context"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
)

type TaskCommentRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64]entities.TaskComment
}

func NewTaskCommentRepository() *TaskCommentRepository {
	return &TaskCommentRepository{nextID: 1, items: make(map[int64]entities.TaskComment)}
}

func (r *TaskCommentRepository) Create(_ context.Context, comment entities.TaskComment) (entities.TaskComment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	comment.ID = r.nextID
	r.nextID++
	comment.CreatedAt = time.Now().UTC()
	r.items[comment.ID] = comment
	return comment, nil
}
