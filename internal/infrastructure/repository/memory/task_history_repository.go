package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
)

type TaskHistoryRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64]entities.TaskHistory
}

func NewTaskHistoryRepository() *TaskHistoryRepository {
	return &TaskHistoryRepository{nextID: 1, items: make(map[int64]entities.TaskHistory)}
}

func (r *TaskHistoryRepository) CreateMany(_ context.Context, entries []entities.TaskHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for i := range entries {
		entries[i].ID = r.nextID
		r.nextID++
		entries[i].CreatedAt = now
		r.items[entries[i].ID] = entries[i]
	}

	return nil
}

func (r *TaskHistoryRepository) ListByTaskID(_ context.Context, taskID int64) ([]entities.TaskHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]entities.TaskHistory, 0)
	for _, item := range r.items {
		if item.TaskID == taskID {
			result = append(result, item)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}
