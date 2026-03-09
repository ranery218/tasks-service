package reports

import (
	"context"
	"errors"
	"time"

	"tasks-service/internal/domain/entities"
	"tasks-service/internal/domain/repository"
)

var ErrInvalidInput = errors.New("invalid input")

type Service struct {
	reports repository.ReportRepository
	nowFn   func() time.Time
}

func NewService(reports repository.ReportRepository) *Service {
	return &Service{reports: reports, nowFn: time.Now().UTC}
}

func (s *Service) TeamStats(ctx context.Context, actor entities.Actor) ([]entities.TeamStatsRow, error) {
	if actor.UserID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.reports.TeamStats(ctx, s.nowFn().Add(-7*24*time.Hour))
}

func (s *Service) TopCreatorsByTeam(ctx context.Context, actor entities.Actor) ([]entities.TopCreatorRow, error) {
	if actor.UserID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.reports.TopCreatorsByTeam(ctx, s.nowFn().Add(-30*24*time.Hour), 3)
}

func (s *Service) InvalidAssignees(ctx context.Context, actor entities.Actor) ([]entities.InvalidAssigneeRow, error) {
	if actor.UserID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.reports.InvalidAssignees(ctx)
}
