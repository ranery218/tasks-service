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

type TeamMemberRepository struct {
	db *sql.DB
}

func NewTeamMemberRepository(db *sql.DB) *TeamMemberRepository {
	return &TeamMemberRepository{db: db}
}

func (r *TeamMemberRepository) Add(ctx context.Context, member entities.TeamMember) error {
	const query = `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES (?, ?, ?)
	`

	if _, err := r.db.ExecContext(ctx, query, member.TeamID, member.UserID, member.Role); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return domainerrors.ErrAlreadyExists
		}
		return fmt.Errorf("add team member: %w", err)
	}

	return nil
}

func (r *TeamMemberRepository) Get(ctx context.Context, teamID, userID int64) (entities.TeamMember, error) {
	const query = `
		SELECT team_id, user_id, role, created_at
		FROM team_members
		WHERE team_id = ? AND user_id = ?
		LIMIT 1
	`

	var member entities.TeamMember
	if err := r.db.QueryRowContext(ctx, query, teamID, userID).Scan(&member.TeamID, &member.UserID, &member.Role, &member.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.TeamMember{}, domainerrors.ErrNotFound
		}
		return entities.TeamMember{}, fmt.Errorf("get team member: %w", err)
	}

	return member, nil
}

func (r *TeamMemberRepository) Remove(ctx context.Context, teamID, userID int64) error {
	const query = `DELETE FROM team_members WHERE team_id = ? AND user_id = ?`

	res, err := r.db.ExecContext(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("member rows affected: %w", err)
	}
	if affected == 0 {
		return domainerrors.ErrNotFound
	}

	return nil
}

func (r *TeamMemberRepository) ListByUserID(ctx context.Context, userID int64) ([]entities.TeamMember, error) {
	const query = `
		SELECT team_id, user_id, role, created_at
		FROM team_members
		WHERE user_id = ?
		ORDER BY team_id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list members by user id: %w", err)
	}
	defer rows.Close()

	items := make([]entities.TeamMember, 0)
	for rows.Next() {
		var member entities.TeamMember
		if err := rows.Scan(&member.TeamID, &member.UserID, &member.Role, &member.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member row: %w", err)
		}
		items = append(items, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("member rows error: %w", err)
	}

	return items, nil
}

func (r *TeamMemberRepository) RemoveByTeamID(ctx context.Context, teamID int64) error {
	const query = `DELETE FROM team_members WHERE team_id = ?`
	if _, err := r.db.ExecContext(ctx, query, teamID); err != nil {
		return fmt.Errorf("remove members by team id: %w", err)
	}
	return nil
}
