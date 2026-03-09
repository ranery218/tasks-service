package repository

import (
	"context"
	"time"

	"tasks-service/internal/domain/entities"
)

type ReportRepository interface {
	TeamStats(ctx context.Context, doneSince time.Time) ([]entities.TeamStatsRow, error)
	TopCreatorsByTeam(ctx context.Context, createdSince time.Time, limitPerTeam int) ([]entities.TopCreatorRow, error)
	InvalidAssignees(ctx context.Context) ([]entities.InvalidAssigneeRow, error)
}
