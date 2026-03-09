package memory

import (
	"context"
	"sort"
	"time"

	"tasks-service/internal/domain/entities"
)

type ReportRepository struct {
	teams   *TeamRepository
	members *TeamMemberRepository
	tasks   *TaskRepository
}

func NewReportRepository(teams *TeamRepository, members *TeamMemberRepository, tasks *TaskRepository) *ReportRepository {
	return &ReportRepository{teams: teams, members: members, tasks: tasks}
}

func (r *ReportRepository) TeamStats(_ context.Context, doneSince time.Time) ([]entities.TeamStatsRow, error) {
	r.teams.mu.RLock()
	r.members.mu.RLock()
	r.tasks.mu.RLock()
	defer r.tasks.mu.RUnlock()
	defer r.members.mu.RUnlock()
	defer r.teams.mu.RUnlock()

	rows := make([]entities.TeamStatsRow, 0, len(r.teams.byID))
	for _, team := range r.teams.byID {
		membersCount := 0
		for _, m := range r.members.items {
			if m.TeamID == team.ID {
				membersCount++
			}
		}

		doneCount := 0
		for _, t := range r.tasks.items {
			if t.TeamID == team.ID && t.Status == entities.TaskStatusDone && (t.UpdatedAt.After(doneSince) || t.UpdatedAt.Equal(doneSince)) {
				doneCount++
			}
		}

		rows = append(rows, entities.TeamStatsRow{
			TeamID:             team.ID,
			TeamName:           team.Name,
			MembersCount:       membersCount,
			DoneTasksLast7Days: doneCount,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].TeamID < rows[j].TeamID })
	return rows, nil
}

func (r *ReportRepository) TopCreatorsByTeam(_ context.Context, createdSince time.Time, limitPerTeam int) ([]entities.TopCreatorRow, error) {
	r.teams.mu.RLock()
	r.tasks.mu.RLock()
	defer r.tasks.mu.RUnlock()
	defer r.teams.mu.RUnlock()

	if limitPerTeam <= 0 {
		limitPerTeam = 3
	}

	rows := make([]entities.TopCreatorRow, 0)
	for _, team := range r.teams.byID {
		counts := make(map[int64]int)
		for _, t := range r.tasks.items {
			if t.TeamID == team.ID && (t.CreatedAt.After(createdSince) || t.CreatedAt.Equal(createdSince)) {
				counts[t.CreatedBy]++
			}
		}

		teamRows := make([]entities.TopCreatorRow, 0, len(counts))
		for userID, cnt := range counts {
			teamRows = append(teamRows, entities.TopCreatorRow{TeamID: team.ID, UserID: userID, TasksCreated: cnt})
		}
		sort.Slice(teamRows, func(i, j int) bool {
			if teamRows[i].TasksCreated == teamRows[j].TasksCreated {
				return teamRows[i].UserID < teamRows[j].UserID
			}
			return teamRows[i].TasksCreated > teamRows[j].TasksCreated
		})

		for i := range teamRows {
			if i >= limitPerTeam {
				break
			}
			teamRows[i].Rank = i + 1
			rows = append(rows, teamRows[i])
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TeamID == rows[j].TeamID {
			return rows[i].Rank < rows[j].Rank
		}
		return rows[i].TeamID < rows[j].TeamID
	})
	return rows, nil
}

func (r *ReportRepository) InvalidAssignees(_ context.Context) ([]entities.InvalidAssigneeRow, error) {
	r.tasks.mu.RLock()
	r.members.mu.RLock()
	defer r.members.mu.RUnlock()
	defer r.tasks.mu.RUnlock()

	rows := make([]entities.InvalidAssigneeRow, 0)
	for _, t := range r.tasks.items {
		if t.AssigneeID == nil {
			continue
		}

		key := teamMemberKey(t.TeamID, *t.AssigneeID)
		if _, exists := r.members.items[key]; !exists {
			rows = append(rows, entities.InvalidAssigneeRow{TaskID: t.ID, TeamID: t.TeamID, AssigneeID: *t.AssigneeID})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].TaskID < rows[j].TaskID })
	return rows, nil
}
