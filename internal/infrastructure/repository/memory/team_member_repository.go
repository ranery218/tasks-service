package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type TeamMemberRepository struct {
	mu    sync.RWMutex
	items map[string]entities.TeamMember
}

func NewTeamMemberRepository() *TeamMemberRepository {
	return &TeamMemberRepository{items: make(map[string]entities.TeamMember)}
}

func (r *TeamMemberRepository) Add(_ context.Context, member entities.TeamMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := teamMemberKey(member.TeamID, member.UserID)
	if _, exists := r.items[key]; exists {
		return domainerrors.ErrAlreadyExists
	}

	member.CreatedAt = time.Now().UTC()
	r.items[key] = member
	return nil
}

func (r *TeamMemberRepository) Get(_ context.Context, teamID, userID int64) (entities.TeamMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	member, exists := r.items[teamMemberKey(teamID, userID)]
	if !exists {
		return entities.TeamMember{}, domainerrors.ErrNotFound
	}

	return member, nil
}

func (r *TeamMemberRepository) Remove(_ context.Context, teamID, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := teamMemberKey(teamID, userID)
	if _, exists := r.items[key]; !exists {
		return domainerrors.ErrNotFound
	}

	delete(r.items, key)
	return nil
}

func (r *TeamMemberRepository) ListByUserID(_ context.Context, userID int64) ([]entities.TeamMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]entities.TeamMember, 0)
	for _, member := range r.items {
		if member.UserID == userID {
			items = append(items, member)
		}
	}
	return items, nil
}

func (r *TeamMemberRepository) RemoveByTeamID(_ context.Context, teamID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, member := range r.items {
		if member.TeamID == teamID {
			delete(r.items, key)
		}
	}
	return nil
}

func teamMemberKey(teamID, userID int64) string {
	return fmt.Sprintf("%d:%d", teamID, userID)
}
