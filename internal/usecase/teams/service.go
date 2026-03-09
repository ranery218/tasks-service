package teams

import (
	"context"
	"errors"
	"strings"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
	"tasks-service/internal/domain/repository"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

type Service struct {
	teams   repository.TeamRepository
	members repository.TeamMemberRepository
	users   repository.UserRepository
	cache   repository.TaskListCache
}

type CreateTeamInput struct {
	Name        string
	Description string
}

type TeamItem struct {
	ID          int64
	Name        string
	Description string
	CreatedBy   int64
	CreatedAt   string
	Role        string
}

type InviteInput struct {
	TeamID int64
	UserID int64
}

type RemoveMemberInput struct {
	TeamID int64
	UserID int64
}

func NewService(
	teams repository.TeamRepository,
	members repository.TeamMemberRepository,
	users repository.UserRepository,
	cache repository.TaskListCache,
) *Service {
	return &Service{teams: teams, members: members, users: users, cache: cache}
}

func (s *Service) CreateTeam(ctx context.Context, actor entities.Actor, input CreateTeamInput) (entities.Team, error) {
	name := strings.TrimSpace(input.Name)
	if actor.UserID <= 0 || name == "" {
		return entities.Team{}, ErrInvalidInput
	}

	team, err := s.teams.Create(ctx, entities.Team{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		CreatedBy:   actor.UserID,
	})
	if err != nil {
		return entities.Team{}, err
	}

	if err := s.members.Add(ctx, entities.TeamMember{TeamID: team.ID, UserID: actor.UserID, Role: entities.TeamRoleOwner}); err != nil {
		return entities.Team{}, err
	}

	return team, nil
}

func (s *Service) ListTeams(ctx context.Context, actor entities.Actor) ([]TeamItem, error) {
	if actor.UserID <= 0 {
		return nil, ErrInvalidInput
	}

	memberships, err := s.members.ListByUserID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}

	items := make([]TeamItem, 0, len(memberships))
	for _, member := range memberships {
		team, err := s.teams.GetByID(ctx, member.TeamID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrNotFound) {
				continue
			}
			return nil, err
		}

		items = append(items, TeamItem{
			ID:          team.ID,
			Name:        team.Name,
			Description: team.Description,
			CreatedBy:   team.CreatedBy,
			CreatedAt:   team.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Role:        member.Role,
		})
	}

	return items, nil
}

func (s *Service) DeleteTeam(ctx context.Context, actor entities.Actor, teamID int64) error {
	if actor.UserID <= 0 || teamID <= 0 {
		return ErrInvalidInput
	}

	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if !canDeleteTeam(actor, team) {
		return ErrForbidden
	}

	if err := s.members.RemoveByTeamID(ctx, teamID); err != nil {
		return err
	}

	if err := s.teams.DeleteByID(ctx, teamID); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if s.cache != nil {
		_ = s.cache.InvalidateTeam(ctx, teamID)
	}

	return nil
}

func (s *Service) InviteMember(ctx context.Context, actor entities.Actor, input InviteInput) error {
	if actor.UserID <= 0 || input.TeamID <= 0 || input.UserID <= 0 {
		return ErrInvalidInput
	}

	team, err := s.teams.GetByID(ctx, input.TeamID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if !canInviteMember(actor, team) {
		return ErrForbidden
	}

	if _, err := s.users.GetByID(ctx, input.UserID); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if err := s.members.Add(ctx, entities.TeamMember{TeamID: input.TeamID, UserID: input.UserID, Role: entities.TeamRoleMember}); err != nil {
		if errors.Is(err, domainerrors.ErrAlreadyExists) {
			return ErrConflict
		}
		return err
	}
	if s.cache != nil {
		_ = s.cache.InvalidateTeam(ctx, input.TeamID)
	}

	return nil
}

func (s *Service) RemoveMember(ctx context.Context, actor entities.Actor, input RemoveMemberInput) error {
	if actor.UserID <= 0 || input.TeamID <= 0 || input.UserID <= 0 {
		return ErrInvalidInput
	}

	team, err := s.teams.GetByID(ctx, input.TeamID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if !canRemoveMember(actor, team) {
		return ErrForbidden
	}

	member, err := s.members.Get(ctx, input.TeamID, input.UserID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if member.Role == entities.TeamRoleOwner {
		return ErrForbidden
	}

	if err := s.members.Remove(ctx, input.TeamID, input.UserID); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if s.cache != nil {
		_ = s.cache.InvalidateTeam(ctx, input.TeamID)
	}

	return nil
}
