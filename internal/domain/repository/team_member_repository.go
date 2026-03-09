package repository

import (
	"context"

	"tasks-service/internal/domain/entities"
)

type TeamMemberRepository interface {
	Add(ctx context.Context, member entities.TeamMember) error
	Get(ctx context.Context, teamID, userID int64) (entities.TeamMember, error)
	Remove(ctx context.Context, teamID, userID int64) error
	ListByUserID(ctx context.Context, userID int64) ([]entities.TeamMember, error)
	RemoveByTeamID(ctx context.Context, teamID int64) error
}
