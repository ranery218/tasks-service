package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tasks-service/internal/domain/entities"
)

type ReportRepository struct {
	db *sql.DB
}

func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) TeamStats(ctx context.Context, doneSince time.Time) ([]entities.TeamStatsRow, error) {
	const query = `
		SELECT
			t.id,
			t.name,
			COUNT(DISTINCT tm.user_id) AS members_count,
			SUM(CASE WHEN ta.status = 'done' AND ta.updated_at >= ? THEN 1 ELSE 0 END) AS done_tasks_last_7_days
		FROM teams t
		LEFT JOIN team_members tm ON tm.team_id = t.id
		LEFT JOIN tasks ta ON ta.team_id = t.id
		GROUP BY t.id, t.name
		ORDER BY t.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, doneSince)
	if err != nil {
		return nil, fmt.Errorf("team stats query: %w", err)
	}
	defer rows.Close()

	items := make([]entities.TeamStatsRow, 0)
	for rows.Next() {
		var row entities.TeamStatsRow
		if err := rows.Scan(&row.TeamID, &row.TeamName, &row.MembersCount, &row.DoneTasksLast7Days); err != nil {
			return nil, fmt.Errorf("scan team stats row: %w", err)
		}
		items = append(items, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("team stats rows error: %w", err)
	}

	return items, nil
}

func (r *ReportRepository) TopCreatorsByTeam(ctx context.Context, createdSince time.Time, limitPerTeam int) ([]entities.TopCreatorRow, error) {
	const query = `
		SELECT team_id, user_id, tasks_created, rank_num
		FROM (
			SELECT
				t.team_id,
				t.created_by AS user_id,
				COUNT(*) AS tasks_created,
				ROW_NUMBER() OVER (PARTITION BY t.team_id ORDER BY COUNT(*) DESC, t.created_by ASC) AS rank_num
			FROM tasks t
			WHERE t.created_at >= ?
			GROUP BY t.team_id, t.created_by
		) ranked
		WHERE rank_num <= ?
		ORDER BY team_id ASC, rank_num ASC
	`

	rows, err := r.db.QueryContext(ctx, query, createdSince, limitPerTeam)
	if err != nil {
		return nil, fmt.Errorf("top creators query: %w", err)
	}
	defer rows.Close()

	items := make([]entities.TopCreatorRow, 0)
	for rows.Next() {
		var row entities.TopCreatorRow
		if err := rows.Scan(&row.TeamID, &row.UserID, &row.TasksCreated, &row.Rank); err != nil {
			return nil, fmt.Errorf("scan top creators row: %w", err)
		}
		items = append(items, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("top creators rows error: %w", err)
	}

	return items, nil
}

func (r *ReportRepository) InvalidAssignees(ctx context.Context) ([]entities.InvalidAssigneeRow, error) {
	const query = `
		SELECT t.id, t.team_id, t.assignee_id
		FROM tasks t
		LEFT JOIN team_members tm
			ON tm.team_id = t.team_id AND tm.user_id = t.assignee_id
		WHERE t.assignee_id IS NOT NULL
		  AND tm.user_id IS NULL
		ORDER BY t.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("invalid assignees query: %w", err)
	}
	defer rows.Close()

	items := make([]entities.InvalidAssigneeRow, 0)
	for rows.Next() {
		var row entities.InvalidAssigneeRow
		if err := rows.Scan(&row.TaskID, &row.TeamID, &row.AssigneeID); err != nil {
			return nil, fmt.Errorf("scan invalid assignee row: %w", err)
		}
		items = append(items, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("invalid assignees rows error: %w", err)
	}

	return items, nil
}
