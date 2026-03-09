package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"tasks-service/internal/domain/entities"
)

type TaskHistoryRepository struct {
	db *sql.DB
}

func NewTaskHistoryRepository(db *sql.DB) *TaskHistoryRepository {
	return &TaskHistoryRepository{db: db}
}

func (r *TaskHistoryRepository) CreateMany(ctx context.Context, entries []entities.TaskHistory) error {
	if len(entries) == 0 {
		return nil
	}

	const query = `
		INSERT INTO task_history (task_id, field, old_value, new_value, changed_by)
		VALUES (?, ?, ?, ?, ?)
	`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare history insert: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		if _, err := stmt.ExecContext(ctx, entry.TaskID, entry.Field, entry.OldValue, entry.NewValue, entry.ChangedBy); err != nil {
			return fmt.Errorf("insert history row: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history tx: %w", err)
	}

	return nil
}

func (r *TaskHistoryRepository) ListByTaskID(ctx context.Context, taskID int64) ([]entities.TaskHistory, error) {
	const query = `
		SELECT id, task_id, field, old_value, new_value, changed_by, created_at
		FROM task_history
		WHERE task_id = ?
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task history: %w", err)
	}
	defer rows.Close()

	items := make([]entities.TaskHistory, 0)
	for rows.Next() {
		var entry entities.TaskHistory
		if err := rows.Scan(&entry.ID, &entry.TaskID, &entry.Field, &entry.OldValue, &entry.NewValue, &entry.ChangedBy, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		items = append(items, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history rows error: %w", err)
	}

	return items, nil
}
