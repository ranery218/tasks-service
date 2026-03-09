package memory

import (
	"context"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type TeamRepository struct {
	mu     sync.RWMutex
	nextID int64
	byID   map[int64]entities.Team
}

func NewTeamRepository() *TeamRepository {
	return &TeamRepository{
		nextID: 1,
		byID:   make(map[int64]entities.Team),
	}
}

func (r *TeamRepository) Create(_ context.Context, team entities.Team) (entities.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	team.ID = r.nextID
	r.nextID++
	team.CreatedAt = now
	team.UpdatedAt = now

	r.byID[team.ID] = team
	return team, nil
}

func (r *TeamRepository) GetByID(_ context.Context, id int64) (entities.Team, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	team, exists := r.byID[id]
	if !exists {
		return entities.Team{}, domainerrors.ErrNotFound
	}

	return team, nil
}

func (r *TeamRepository) DeleteByID(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[id]; !exists {
		return domainerrors.ErrNotFound
	}

	delete(r.byID, id)
	return nil
}

func (r *TeamRepository) ListByUserID(_ context.Context, userID int64) ([]entities.Team, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]entities.Team, 0)
	for _, team := range r.byID {
		if team.CreatedBy == userID {
			items = append(items, team)
		}
	}

	return items, nil
}
