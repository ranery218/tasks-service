package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"tasks-service/internal/domain/entities"
)

type TaskCommentRepository struct {
	db *sql.DB
}

func NewTaskCommentRepository(db *sql.DB) *TaskCommentRepository {
	return &TaskCommentRepository{db: db}
}

func (r *TaskCommentRepository) Create(ctx context.Context, comment entities.TaskComment) (entities.TaskComment, error) {
	const query = `
		INSERT INTO task_comments (task_id, user_id, text)
		VALUES (?, ?, ?)
	`

	res, err := r.db.ExecContext(ctx, query, comment.TaskID, comment.UserID, comment.Text)
	if err != nil {
		return entities.TaskComment{}, fmt.Errorf("create task comment: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return entities.TaskComment{}, fmt.Errorf("comment last insert id: %w", err)
	}

	const getQuery = `SELECT id, task_id, user_id, text, created_at FROM task_comments WHERE id = ? LIMIT 1`
	var created entities.TaskComment
	if err := r.db.QueryRowContext(ctx, getQuery, id).Scan(&created.ID, &created.TaskID, &created.UserID, &created.Text, &created.CreatedAt); err != nil {
		return entities.TaskComment{}, fmt.Errorf("get created comment: %w", err)
	}

	return created, nil
}
