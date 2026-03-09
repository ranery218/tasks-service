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

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user entities.User) (entities.User, error) {
	const query = `
		INSERT INTO users (email, password_hash, name, is_admin)
		VALUES (?, ?, ?, ?)
	`

	res, err := r.db.ExecContext(ctx, query, user.Email, user.PasswordHash, user.Name, user.IsAdmin)
	if err != nil {
		if isDuplicateEntry(err) {
			return entities.User{}, domainerrors.ErrAlreadyExists
		}
		return entities.User{}, fmt.Errorf("create user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return entities.User{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (entities.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, created_at, updated_at
		FROM users
		WHERE email = ?
		LIMIT 1
	`

	var user entities.User
	if err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.User{}, domainerrors.ErrNotFound
		}
		return entities.User{}, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (entities.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, created_at, updated_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`

	var user entities.User
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.User{}, domainerrors.ErrNotFound
		}
		return entities.User{}, fmt.Errorf("get user by id: %w", err)
	}

	return user, nil
}

func isDuplicateEntry(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
