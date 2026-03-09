package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"tasks-service/internal/domain/entities"
	"tasks-service/internal/infrastructure/repository/memory"
)

func TestReportsService(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	teams := memory.NewTeamRepository()
	members := memory.NewTeamMemberRepository()
	tasks := memory.NewTaskRepository()
	reportRepo := memory.NewReportRepository(teams, members, tasks)
	svc := NewService(reportRepo)
	fixedNow := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	svc.nowFn = func() time.Time { return fixedNow }

	ownerA, _ := users.Create(context.Background(), entities.User{Email: "a@example.com", Name: "A", PasswordHash: "h"})
	ownerB, _ := users.Create(context.Background(), entities.User{Email: "b@example.com", Name: "B", PasswordHash: "h"})
	memberA, _ := users.Create(context.Background(), entities.User{Email: "m@example.com", Name: "M", PasswordHash: "h"})
	nonMember, _ := users.Create(context.Background(), entities.User{Email: "x@example.com", Name: "X", PasswordHash: "h"})

	teamA, _ := teams.Create(context.Background(), entities.Team{Name: "Team A", CreatedBy: ownerA.ID})
	teamB, _ := teams.Create(context.Background(), entities.Team{Name: "Team B", CreatedBy: ownerB.ID})

	_ = members.Add(context.Background(), entities.TeamMember{TeamID: teamA.ID, UserID: ownerA.ID, Role: entities.TeamRoleOwner})
	_ = members.Add(context.Background(), entities.TeamMember{TeamID: teamA.ID, UserID: memberA.ID, Role: entities.TeamRoleMember})
	_ = members.Add(context.Background(), entities.TeamMember{TeamID: teamB.ID, UserID: ownerB.ID, Role: entities.TeamRoleOwner})

	t1, _ := tasks.Create(context.Background(), entities.Task{TeamID: teamA.ID, Title: "t1", CreatedBy: ownerA.ID, Status: entities.TaskStatusDone})
	t2, _ := tasks.Create(context.Background(), entities.Task{TeamID: teamA.ID, Title: "t2", CreatedBy: ownerA.ID, Status: entities.TaskStatusDone})
	t3, _ := tasks.Create(context.Background(), entities.Task{TeamID: teamA.ID, Title: "t3", CreatedBy: memberA.ID, Status: entities.TaskStatusInProgress})
	t4, _ := tasks.Create(context.Background(), entities.Task{TeamID: teamB.ID, Title: "t4", CreatedBy: ownerB.ID, Status: entities.TaskStatusDone})

	t1.UpdatedAt = fixedNow.Add(-2 * 24 * time.Hour)
	t2.UpdatedAt = fixedNow.Add(-10 * 24 * time.Hour)
	t3.UpdatedAt = fixedNow.Add(-1 * 24 * time.Hour)
	t4.UpdatedAt = fixedNow.Add(-3 * 24 * time.Hour)
	t1.CreatedAt = fixedNow.Add(-5 * 24 * time.Hour)
	t2.CreatedAt = fixedNow.Add(-15 * 24 * time.Hour)
	t3.CreatedAt = fixedNow.Add(-3 * 24 * time.Hour)
	t4.CreatedAt = fixedNow.Add(-2 * 24 * time.Hour)
	_, _ = tasks.Update(context.Background(), t1)
	_, _ = tasks.Update(context.Background(), t2)
	_, _ = tasks.Update(context.Background(), t3)
	_, _ = tasks.Update(context.Background(), t4)

	invalidAssignee := nonMember.ID
	badTask, _ := tasks.Create(context.Background(), entities.Task{TeamID: teamA.ID, Title: "bad", CreatedBy: ownerA.ID, Status: entities.TaskStatusTodo, AssigneeID: &invalidAssignee})
	_ = badTask

	actor := entities.Actor{UserID: ownerA.ID}

	stats, err := svc.TeamStats(context.Background(), actor)
	if err != nil {
		t.Fatalf("team stats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 team stats rows, got %d", len(stats))
	}
	for _, row := range stats {
		if row.TeamID == teamA.ID && row.DoneTasksLast7Days != 2 {
			t.Fatalf("expected teamA done last 7 days = 2, got %d", row.DoneTasksLast7Days)
		}
	}

	top, err := svc.TopCreatorsByTeam(context.Background(), actor)
	if err != nil {
		t.Fatalf("top creators: %v", err)
	}
	if len(top) == 0 {
		t.Fatalf("expected top creators rows")
	}

	invalid, err := svc.InvalidAssignees(context.Background(), actor)
	if err != nil {
		t.Fatalf("invalid assignees: %v", err)
	}
	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid assignee row, got %d", len(invalid))
	}
}

type reportRepoStub struct {
	teamStatsFn       func(ctx context.Context, doneSince time.Time) ([]entities.TeamStatsRow, error)
	topCreatorsFn     func(ctx context.Context, createdSince time.Time, limitPerTeam int) ([]entities.TopCreatorRow, error)
	invalidAssigneeFn func(ctx context.Context) ([]entities.InvalidAssigneeRow, error)
}

func (s reportRepoStub) TeamStats(ctx context.Context, doneSince time.Time) ([]entities.TeamStatsRow, error) {
	return s.teamStatsFn(ctx, doneSince)
}

func (s reportRepoStub) TopCreatorsByTeam(ctx context.Context, createdSince time.Time, limitPerTeam int) ([]entities.TopCreatorRow, error) {
	return s.topCreatorsFn(ctx, createdSince, limitPerTeam)
}

func (s reportRepoStub) InvalidAssignees(ctx context.Context) ([]entities.InvalidAssigneeRow, error) {
	return s.invalidAssigneeFn(ctx)
}

func TestReportsInvalidActor(t *testing.T) {
	t.Parallel()

	repo := reportRepoStub{
		teamStatsFn:       func(context.Context, time.Time) ([]entities.TeamStatsRow, error) { return nil, nil },
		topCreatorsFn:     func(context.Context, time.Time, int) ([]entities.TopCreatorRow, error) { return nil, nil },
		invalidAssigneeFn: func(context.Context) ([]entities.InvalidAssigneeRow, error) { return nil, nil },
	}
	svc := NewService(repo)

	actor := entities.Actor{}
	if _, err := svc.TeamStats(context.Background(), actor); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for TeamStats, got %v", err)
	}
	if _, err := svc.TopCreatorsByTeam(context.Background(), actor); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for TopCreatorsByTeam, got %v", err)
	}
	if _, err := svc.InvalidAssignees(context.Background(), actor); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for InvalidAssignees, got %v", err)
	}
}

func TestReportsPropagatesRepoErrorsAndArguments(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	expectedStatsSince := fixedNow.Add(-7 * 24 * time.Hour)
	expectedCreatorsSince := fixedNow.Add(-30 * 24 * time.Hour)
	repoErr := errors.New("repo failed")

	calledStats := false
	calledTop := false
	calledInvalid := false

	repo := reportRepoStub{
		teamStatsFn: func(_ context.Context, doneSince time.Time) ([]entities.TeamStatsRow, error) {
			calledStats = true
			if !doneSince.Equal(expectedStatsSince) {
				t.Fatalf("unexpected doneSince: %v", doneSince)
			}
			return nil, repoErr
		},
		topCreatorsFn: func(_ context.Context, createdSince time.Time, limitPerTeam int) ([]entities.TopCreatorRow, error) {
			calledTop = true
			if !createdSince.Equal(expectedCreatorsSince) {
				t.Fatalf("unexpected createdSince: %v", createdSince)
			}
			if limitPerTeam != 3 {
				t.Fatalf("unexpected limitPerTeam: %d", limitPerTeam)
			}
			return nil, repoErr
		},
		invalidAssigneeFn: func(context.Context) ([]entities.InvalidAssigneeRow, error) {
			calledInvalid = true
			return nil, repoErr
		},
	}
	svc := NewService(repo)
	svc.nowFn = func() time.Time { return fixedNow }
	actor := entities.Actor{UserID: 1}

	if _, err := svc.TeamStats(context.Background(), actor); !errors.Is(err, repoErr) {
		t.Fatalf("expected repo error in TeamStats, got %v", err)
	}
	if _, err := svc.TopCreatorsByTeam(context.Background(), actor); !errors.Is(err, repoErr) {
		t.Fatalf("expected repo error in TopCreatorsByTeam, got %v", err)
	}
	if _, err := svc.InvalidAssignees(context.Background(), actor); !errors.Is(err, repoErr) {
		t.Fatalf("expected repo error in InvalidAssignees, got %v", err)
	}
	if !calledStats || !calledTop || !calledInvalid {
		t.Fatalf("expected all repo methods to be called")
	}
}
