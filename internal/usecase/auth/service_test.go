package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
	jwtservice "tasks-service/internal/infrastructure/jwt"
	"tasks-service/internal/infrastructure/repository/memory"
)

func newTestService() *Service {
	users := memory.NewUserRepository()
	refresh := memory.NewRefreshTokenRepository()
	jwt := jwtservice.NewService("test-access-secret")

	return NewService(users, refresh, jwt, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)
}

func TestRegister(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	user, err := svc.Register(context.Background(), RegisterInput{
		Email:    "  USER@Example.com ",
		Password: "strong-pass",
		Name:     " Ivan ",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	if user.ID == 0 {
		t.Fatalf("expected user id to be set")
	}
	if user.Email != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}
	if user.Name != "Ivan" {
		t.Fatalf("expected trimmed name, got %q", user.Name)
	}
	if user.PasswordHash == "strong-pass" {
		t.Fatalf("password should be hashed")
	}

	_, err = svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "another-pass",
		Name:     "Ivan2",
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestRegisterInvalidInput(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	_, err := svc.Register(context.Background(), RegisterInput{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "pass-123",
		Name:     "User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	pair, err := svc.Login(context.Background(), LoginInput{Email: "user@example.com", Password: "pass-123"})
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens")
	}
	if pair.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", pair.TokenType)
	}
	if pair.ExpiresInSec != int((15 * time.Minute).Seconds()) {
		t.Fatalf("unexpected expires in: %d", pair.ExpiresInSec)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "pass-123",
		Name:     "User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err = svc.Login(context.Background(), LoginInput{Email: "user@example.com", Password: "bad-pass"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for bad password, got %v", err)
	}

	_, err = svc.Login(context.Background(), LoginInput{Email: "missing@example.com", Password: "pass-123"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
}

func TestLoginInvalidInput(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	_, err := svc.Login(context.Background(), LoginInput{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "pass-123",
		Name:     "User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginPair, err := svc.Login(context.Background(), LoginInput{Email: "user@example.com", Password: "pass-123"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	refreshPair, err := svc.Refresh(context.Background(), loginPair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshPair.RefreshToken == "" || refreshPair.RefreshToken == loginPair.RefreshToken {
		t.Fatalf("expected rotated refresh token")
	}

	_, err = svc.Refresh(context.Background(), loginPair.RefreshToken)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected old token to be revoked, got %v", err)
	}
}

func TestRefreshInvalidInputAndUnauthorized(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	_, err := svc.Refresh(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	_, err = svc.Refresh(context.Background(), "not-found-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRegisterInvalidBcryptCostReturnsError(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	refresh := memory.NewRefreshTokenRepository()
	jwt := jwtservice.NewService("test-access-secret")
	svc := NewService(users, refresh, jwt, 15*time.Minute, 24*time.Hour, 100)

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "pass-123",
		Name:     "User",
	})
	if err == nil {
		t.Fatalf("expected register error with invalid bcrypt cost")
	}
}

func TestLogout(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "pass-123",
		Name:     "User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	pair, err := svc.Login(context.Background(), LoginInput{Email: "user@example.com", Password: "pass-123"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected revoked token to be unauthorized, got %v", err)
	}
}

func TestLogoutInvalidCases(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	err := svc.Logout(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	err = svc.Logout(context.Background(), "unknown-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRefreshReturnsInternalErrorOnRevokeFailure(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	user, err := users.Create(context.Background(), entities.User{
		Email:        "user@example.com",
		PasswordHash: "hash",
		Name:         "User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	refreshRepo := &revokeErrorRefreshRepo{
		token: entities.RefreshToken{UserID: user.ID},
		err:   fmt.Errorf("db down"),
	}
	jwt := jwtservice.NewService("test-access-secret")
	svc := NewService(users, refreshRepo, jwt, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)

	_, err = svc.Refresh(context.Background(), "any-refresh")
	if err == nil || err.Error() != "db down" {
		t.Fatalf("expected internal revoke error, got %v", err)
	}
}

func TestRefreshUnauthorizedWhenUserMissing(t *testing.T) {
	t.Parallel()

	refreshRepo := &revokeErrorRefreshRepo{
		token: entities.RefreshToken{UserID: 999},
	}
	jwt := jwtservice.NewService("test-access-secret")
	svc := NewService(memory.NewUserRepository(), refreshRepo, jwt, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)

	_, err := svc.Refresh(context.Background(), "any-refresh")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized when user missing, got %v", err)
	}
}

func TestLoginReturnsCreateRefreshError(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	jwt := jwtservice.NewService("test-access-secret")
	svc := NewService(users, &createErrorRefreshRepo{err: fmt.Errorf("insert failed")}, jwt, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "pass-123",
		Name:     "User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err = svc.Login(context.Background(), LoginInput{Email: "user@example.com", Password: "pass-123"})
	if err == nil || err.Error() != "insert failed" {
		t.Fatalf("expected create refresh error, got %v", err)
	}
}

func TestLogoutReturnsInternalError(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	jwt := jwtservice.NewService("test-access-secret")
	svc := NewService(users, &createErrorRefreshRepo{err: fmt.Errorf("storage fail")}, jwt, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)

	err := svc.Logout(context.Background(), "refresh")
	if err == nil || err.Error() != "storage fail" {
		t.Fatalf("expected storage fail error, got %v", err)
	}
}

type failingRefreshRepo struct{}

func (f failingRefreshRepo) Create(context.Context, entities.RefreshToken) error { return nil }
func (f failingRefreshRepo) GetActiveByHash(context.Context, string) (entities.RefreshToken, error) {
	return entities.RefreshToken{}, nil
}
func (f failingRefreshRepo) RevokeByHash(context.Context, string) error {
	return domainerrors.ErrNotFound
}

func TestRefreshMapsNotFoundOnRevokeToUnauthorized(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	jwt := jwtservice.NewService("test-access-secret")
	svc := NewService(users, failingRefreshRepo{}, jwt, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)

	_, err := svc.Refresh(context.Background(), "some-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

type createErrorRefreshRepo struct {
	err error
}

func (r *createErrorRefreshRepo) Create(context.Context, entities.RefreshToken) error { return r.err }
func (r *createErrorRefreshRepo) GetActiveByHash(context.Context, string) (entities.RefreshToken, error) {
	return entities.RefreshToken{}, domainerrors.ErrNotFound
}
func (r *createErrorRefreshRepo) RevokeByHash(context.Context, string) error { return r.err }

type revokeErrorRefreshRepo struct {
	token entities.RefreshToken
	err   error
}

func (r *revokeErrorRefreshRepo) Create(context.Context, entities.RefreshToken) error { return nil }
func (r *revokeErrorRefreshRepo) GetActiveByHash(context.Context, string) (entities.RefreshToken, error) {
	return r.token, nil
}
func (r *revokeErrorRefreshRepo) RevokeByHash(context.Context, string) error { return r.err }
