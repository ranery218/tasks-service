package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type TaskRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64]entities.Task
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{nextID: 1, items: make(map[int64]entities.Task)}
}

func (r *TaskRepository) Create(_ context.Context, task entities.Task) (entities.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	task.ID = r.nextID
	r.nextID++
	task.CreatedAt = now
	task.UpdatedAt = now
	r.items[task.ID] = task

	return task, nil
}

func (r *TaskRepository) GetByID(_ context.Context, id int64) (entities.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.items[id]
	if !ok {
		return entities.Task{}, domainerrors.ErrNotFound
	}

	return task, nil
}

func (r *TaskRepository) Update(_ context.Context, task entities.Task) (entities.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.items[task.ID]
	if !ok {
		return entities.Task{}, domainerrors.ErrNotFound
	}

	task.CreatedAt = current.CreatedAt
	task.UpdatedAt = time.Now().UTC()
	r.items[task.ID] = task
	return task, nil
}

func (r *TaskRepository) List(_ context.Context, filter entities.TaskFilter) ([]entities.Task, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := make([]entities.Task, 0)
	for _, task := range r.items {
		if filter.TeamID != nil && task.TeamID != *filter.TeamID {
			continue
		}
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}
		if filter.AssigneeID != nil {
			if task.AssigneeID == nil || *task.AssigneeID != *filter.AssigneeID {
				continue
			}
		}
		filtered = append(filtered, task)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	total := len(filtered)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []entities.Task{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total, nil
}
