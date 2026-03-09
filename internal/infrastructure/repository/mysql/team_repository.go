package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
)

type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(ctx context.Context, team entities.Team) (entities.Team, error) {
	const query = `
		INSERT INTO teams (name, description, created_by)
		VALUES (?, ?, ?)
	`

	res, err := r.db.ExecContext(ctx, query, team.Name, team.Description, team.CreatedBy)
	if err != nil {
		return entities.Team{}, fmt.Errorf("create team: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return entities.Team{}, fmt.Errorf("team last insert id: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *TeamRepository) GetByID(ctx context.Context, id int64) (entities.Team, error) {
	const query = `
		SELECT id, name, description, created_by, created_at, updated_at
		FROM teams
		WHERE id = ?
		LIMIT 1
	`

	var team entities.Team
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.Description,
		&team.CreatedBy,
		&team.CreatedAt,
		&team.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.Team{}, domainerrors.ErrNotFound
		}
		return entities.Team{}, fmt.Errorf("get team by id: %w", err)
	}

	return team, nil
}

func (r *TeamRepository) DeleteByID(ctx context.Context, id int64) error {
	const query = `DELETE FROM teams WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("team rows affected: %w", err)
	}
	if affected == 0 {
		return domainerrors.ErrNotFound
	}

	return nil
}

func (r *TeamRepository) ListByUserID(ctx context.Context, userID int64) ([]entities.Team, error) {
	const query = `
		SELECT t.id, t.name, t.description, t.created_by, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = ?
		ORDER BY t.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list teams by user id: %w", err)
	}
	defer rows.Close()

	items := make([]entities.Team, 0)
	for rows.Next() {
		var team entities.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.Description, &team.CreatedBy, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan team row: %w", err)
		}
		items = append(items, team)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("team rows error: %w", err)
	}

	return items, nil
}
