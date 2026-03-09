package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task entities.Task) (entities.Task, error) {
	const query = `
		INSERT INTO tasks (team_id, title, description, assignee_id, created_by, status, due_date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	res, err := r.db.ExecContext(ctx, query, task.TeamID, task.Title, task.Description, task.AssigneeID, task.CreatedBy, task.Status, task.DueDate)
	if err != nil {
		return entities.Task{}, fmt.Errorf("create task: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return entities.Task{}, fmt.Errorf("task last insert id: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *TaskRepository) GetByID(ctx context.Context, id int64) (entities.Task, error) {
	const query = `
		SELECT id, team_id, title, description, assignee_id, created_by, status, due_date, created_at, updated_at
		FROM tasks
		WHERE id = ?
		LIMIT 1
	`

	var task entities.Task
	var assignee sql.NullInt64
	var dueDate sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.TeamID,
		&task.Title,
		&task.Description,
		&assignee,
		&task.CreatedBy,
		&task.Status,
		&dueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.Task{}, domainerrors.ErrNotFound
		}
		return entities.Task{}, fmt.Errorf("get task by id: %w", err)
	}

	if assignee.Valid {
		task.AssigneeID = &assignee.Int64
	}
	if dueDate.Valid {
		t := dueDate.Time
		task.DueDate = &t
	}

	return task, nil
}

func (r *TaskRepository) Update(ctx context.Context, task entities.Task) (entities.Task, error) {
	const query = `
		UPDATE tasks
		SET title = ?, description = ?, assignee_id = ?, status = ?, due_date = ?
		WHERE id = ?
	`

	res, err := r.db.ExecContext(ctx, query, task.Title, task.Description, task.AssigneeID, task.Status, task.DueDate, task.ID)
	if err != nil {
		return entities.Task{}, fmt.Errorf("update task: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return entities.Task{}, fmt.Errorf("task rows affected: %w", err)
	}
	if affected == 0 {
		return entities.Task{}, domainerrors.ErrNotFound
	}

	return r.GetByID(ctx, task.ID)
}

func (r *TaskRepository) List(ctx context.Context, filter entities.TaskFilter) ([]entities.Task, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := make([]string, 0)
	args := make([]any, 0)
	if filter.TeamID != nil {
		where = append(where, "team_id = ?")
		args = append(args, *filter.TeamID)
	}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.AssigneeID != nil {
		where = append(where, "assignee_id = ?")
		args = append(args, *filter.AssigneeID)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM tasks" + whereSQL
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	listQuery := `
		SELECT id, team_id, title, description, assignee_id, created_by, status, due_date, created_at, updated_at
		FROM tasks` + whereSQL + `
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	listArgs := append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	items := make([]entities.Task, 0)
	for rows.Next() {
		var task entities.Task
		var assignee sql.NullInt64
		var dueDate sql.NullTime
		if err := rows.Scan(
			&task.ID,
			&task.TeamID,
			&task.Title,
			&task.Description,
			&assignee,
			&task.CreatedBy,
			&task.Status,
			&dueDate,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan task row: %w", err)
		}
		if assignee.Valid {
			task.AssigneeID = &assignee.Int64
		}
		if dueDate.Valid {
			t := dueDate.Time
			task.DueDate = &t
		}
		items = append(items, task)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("task rows error: %w", err)
	}

	return items, total, nil
}
