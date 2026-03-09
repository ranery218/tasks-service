package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
	"tasks-service/internal/domain/repository"
	jwtservice "tasks-service/internal/infrastructure/jwt"
	"tasks-service/internal/infrastructure/password"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
)

type Service struct {
	users      repository.UserRepository
	refresh    repository.RefreshTokenRepository
	jwt        *jwtservice.Service
	accessTTL  time.Duration
	refreshTTL time.Duration
	bcryptCost int
	nowFn      func() time.Time
}

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

type LoginInput struct {
	Email    string
	Password string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresInSec int
}

func NewService(
	users repository.UserRepository,
	refresh repository.RefreshTokenRepository,
	jwt *jwtservice.Service,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	bcryptCost int,
) *Service {
	return &Service{
		users:      users,
		refresh:    refresh,
		jwt:        jwt,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		bcryptCost: bcryptCost,
		nowFn:      time.Now().UTC,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (entities.User, error) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Name = strings.TrimSpace(input.Name)

	if input.Email == "" || input.Password == "" || input.Name == "" {
		return entities.User{}, ErrInvalidInput
	}

	hash, err := password.Hash(input.Password, s.bcryptCost)
	if err != nil {
		return entities.User{}, err
	}

	user := entities.User{
		Email:        input.Email,
		PasswordHash: hash,
		Name:         input.Name,
		IsAdmin:      false,
	}

	created, err := s.users.Create(ctx, user)
	if err != nil {
		if errors.Is(err, domainerrors.ErrAlreadyExists) {
			return entities.User{}, ErrEmailAlreadyExists
		}
		return entities.User{}, err
	}

	return created, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (TokenPair, error) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if input.Email == "" || input.Password == "" {
		return TokenPair{}, ErrInvalidInput
	}

	user, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	if err := password.Verify(user.PasswordHash, input.Password); err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return TokenPair{}, ErrInvalidInput
	}

	tokenHash := hashToken(refreshToken)
	stored, err := s.refresh.GetActiveByHash(ctx, tokenHash)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}

	if err := s.refresh.RevokeByHash(ctx, tokenHash); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return TokenPair{}, ErrUnauthorized
		}
		return TokenPair{}, err
	}

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}

	return s.issueTokens(ctx, user)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return ErrInvalidInput
	}

	tokenHash := hashToken(refreshToken)
	if err := s.refresh.RevokeByHash(ctx, tokenHash); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrUnauthorized
		}
		return err
	}

	return nil
}

func (s *Service) issueTokens(ctx context.Context, user entities.User) (TokenPair, error) {
	accessToken, err := s.jwt.GenerateAccessToken(user.ID, user.IsAdmin, s.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := generateOpaqueRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}

	expiresAt := s.nowFn().Add(s.refreshTTL)
	if err := s.refresh.Create(ctx, entities.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: expiresAt,
	}); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresInSec: int(s.accessTTL.Seconds()),
	}, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateOpaqueRefreshToken() (string, error) {
	// 32 random bytes encoded as URL-safe base64 without padding.
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
