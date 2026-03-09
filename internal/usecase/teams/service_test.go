package teams

import (
	"context"
	"errors"
	"testing"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
	"tasks-service/internal/domain/repository"
	"tasks-service/internal/infrastructure/repository/memory"
)

func createUser(t *testing.T, users *memory.UserRepository, email string, isAdmin bool) entities.User {
	t.Helper()

	user, err := users.Create(context.Background(), entities.User{
		Email:        email,
		Name:         email,
		PasswordHash: "hash",
		IsAdmin:      isAdmin,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}

func TestCreateTeamAssignsOwnerMembership(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	members := memory.NewTeamMemberRepository()
	teamsRepo := memory.NewTeamRepository()
	svc := NewService(teamsRepo, members, users, nil)

	user := createUser(t, users, "owner@example.com", false)

	team, err := svc.CreateTeam(context.Background(), entities.Actor{UserID: user.ID}, CreateTeamInput{Name: "Backend"})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	member, err := members.Get(context.Background(), team.ID, user.ID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if member.Role != entities.TeamRoleOwner {
		t.Fatalf("expected owner role, got %s", member.Role)
	}
}

func TestInviteMemberAuthzOwnerAndAdmin(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	members := memory.NewTeamMemberRepository()
	teamsRepo := memory.NewTeamRepository()
	svc := NewService(teamsRepo, members, users, nil)

	owner := createUser(t, users, "owner@example.com", false)
	admin := createUser(t, users, "admin@example.com", true)
	outsider := createUser(t, users, "outsider@example.com", false)
	target := createUser(t, users, "target@example.com", false)

	team, err := svc.CreateTeam(context.Background(), entities.Actor{UserID: owner.ID}, CreateTeamInput{Name: "Team1"})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	if err := svc.InviteMember(context.Background(), entities.Actor{UserID: outsider.ID}, InviteInput{TeamID: team.ID, UserID: target.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for outsider, got %v", err)
	}

	if err := svc.InviteMember(context.Background(), entities.Actor{UserID: admin.ID, IsAdmin: true}, InviteInput{TeamID: team.ID, UserID: target.ID}); err != nil {
		t.Fatalf("admin invite failed: %v", err)
	}

	if err := svc.InviteMember(context.Background(), entities.Actor{UserID: owner.ID}, InviteInput{TeamID: team.ID, UserID: target.ID}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate invite, got %v", err)
	}
}

func TestDeleteTeamAuthzOwnerAndAdmin(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	members := memory.NewTeamMemberRepository()
	teamsRepo := memory.NewTeamRepository()
	svc := NewService(teamsRepo, members, users, nil)

	owner := createUser(t, users, "owner@example.com", false)
	admin := createUser(t, users, "admin@example.com", true)
	member := createUser(t, users, "member@example.com", false)

	team, err := svc.CreateTeam(context.Background(), entities.Actor{UserID: owner.ID}, CreateTeamInput{Name: "Team1"})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	if err := svc.InviteMember(context.Background(), entities.Actor{UserID: owner.ID}, InviteInput{TeamID: team.ID, UserID: member.ID}); err != nil {
		t.Fatalf("invite member: %v", err)
	}

	if err := svc.DeleteTeam(context.Background(), entities.Actor{UserID: member.ID}, team.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for member, got %v", err)
	}

	if err := svc.DeleteTeam(context.Background(), entities.Actor{UserID: admin.ID, IsAdmin: true}, team.ID); err != nil {
		t.Fatalf("admin delete team failed: %v", err)
	}
}

func TestRemoveMemberPermissionsAndOwnerProtection(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	members := memory.NewTeamMemberRepository()
	teamsRepo := memory.NewTeamRepository()
	svc := NewService(teamsRepo, members, users, nil)

	owner := createUser(t, users, "owner@example.com", false)
	admin := createUser(t, users, "admin@example.com", true)
	member := createUser(t, users, "member@example.com", false)

	team, err := svc.CreateTeam(context.Background(), entities.Actor{UserID: owner.ID}, CreateTeamInput{Name: "Team1"})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := svc.InviteMember(context.Background(), entities.Actor{UserID: owner.ID}, InviteInput{TeamID: team.ID, UserID: member.ID}); err != nil {
		t.Fatalf("invite member: %v", err)
	}

	if err := svc.RemoveMember(context.Background(), entities.Actor{UserID: member.ID}, RemoveMemberInput{TeamID: team.ID, UserID: member.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for member self-remove, got %v", err)
	}

	if err := svc.RemoveMember(context.Background(), entities.Actor{UserID: admin.ID, IsAdmin: true}, RemoveMemberInput{TeamID: team.ID, UserID: owner.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden removing owner membership, got %v", err)
	}

	if err := svc.RemoveMember(context.Background(), entities.Actor{UserID: owner.ID}, RemoveMemberInput{TeamID: team.ID, UserID: member.ID}); err != nil {
		t.Fatalf("owner remove member failed: %v", err)
	}
}

func TestListTeamsReturnsUserMemberships(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	members := memory.NewTeamMemberRepository()
	teamsRepo := memory.NewTeamRepository()
	svc := NewService(teamsRepo, members, users, nil)

	user := createUser(t, users, "user@example.com", false)
	team, err := teamsRepo.Create(context.Background(), entities.Team{
		Name:      "Team A",
		CreatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := members.Add(context.Background(), entities.TeamMember{TeamID: team.ID, UserID: user.ID, Role: entities.TeamRoleOwner}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	items, err := svc.ListTeams(context.Background(), entities.Actor{UserID: user.ID})
	if err != nil {
		t.Fatalf("list teams: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Role != entities.TeamRoleOwner {
		t.Fatalf("expected owner role, got %s", items[0].Role)
	}
	if _, err := time.Parse("2006-01-02T15:04:05Z", items[0].CreatedAt); err != nil {
		t.Fatalf("unexpected created_at format: %s", items[0].CreatedAt)
	}
}

func TestListTeamsSkipsMissingTeamAndValidatesActor(t *testing.T) {
	t.Parallel()

	users := memory.NewUserRepository()
	members := memory.NewTeamMemberRepository()
	svc := NewService(memory.NewTeamRepository(), members, users, nil)

	user := createUser(t, users, "user@example.com", false)
	if err := members.Add(context.Background(), entities.TeamMember{
		TeamID: 999, UserID: user.ID, Role: entities.TeamRoleMember,
	}); err != nil {
		t.Fatalf("add dangling membership: %v", err)
	}

	items, err := svc.ListTeams(context.Background(), entities.Actor{UserID: user.ID})
	if err != nil {
		t.Fatalf("list teams with dangling membership: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items for dangling membership, got %d", len(items))
	}

	if _, err := svc.ListTeams(context.Background(), entities.Actor{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTeamsInputValidation(t *testing.T) {
	t.Parallel()

	svc := NewService(memory.NewTeamRepository(), memory.NewTeamMemberRepository(), memory.NewUserRepository(), nil)
	actor := entities.Actor{UserID: 1}

	if _, err := svc.CreateTeam(context.Background(), entities.Actor{}, CreateTeamInput{Name: "x"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for CreateTeam actor, got %v", err)
	}
	if _, err := svc.CreateTeam(context.Background(), actor, CreateTeamInput{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}
	if err := svc.DeleteTeam(context.Background(), entities.Actor{}, 1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for DeleteTeam actor, got %v", err)
	}
	if err := svc.InviteMember(context.Background(), actor, InviteInput{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for InviteMember, got %v", err)
	}
	if err := svc.RemoveMember(context.Background(), actor, RemoveMemberInput{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for RemoveMember, got %v", err)
	}
}

type teamRepoStub struct {
	createFn func(context.Context, entities.Team) (entities.Team, error)
	getByID  func(context.Context, int64) (entities.Team, error)
	deleteFn func(context.Context, int64) error
}

func (s teamRepoStub) Create(ctx context.Context, team entities.Team) (entities.Team, error) {
	return s.createFn(ctx, team)
}
func (s teamRepoStub) GetByID(ctx context.Context, id int64) (entities.Team, error) {
	return s.getByID(ctx, id)
}
func (s teamRepoStub) DeleteByID(ctx context.Context, id int64) error { return s.deleteFn(ctx, id) }
func (s teamRepoStub) ListByUserID(context.Context, int64) ([]entities.Team, error) {
	return nil, nil
}

type teamMembersStub struct {
	addFn          func(context.Context, entities.TeamMember) error
	getFn          func(context.Context, int64, int64) (entities.TeamMember, error)
	removeFn       func(context.Context, int64, int64) error
	listByUserFn   func(context.Context, int64) ([]entities.TeamMember, error)
	removeByTeamFn func(context.Context, int64) error
}

func (s teamMembersStub) Add(ctx context.Context, member entities.TeamMember) error {
	return s.addFn(ctx, member)
}
func (s teamMembersStub) Get(ctx context.Context, teamID, userID int64) (entities.TeamMember, error) {
	return s.getFn(ctx, teamID, userID)
}
func (s teamMembersStub) Remove(ctx context.Context, teamID, userID int64) error {
	return s.removeFn(ctx, teamID, userID)
}
func (s teamMembersStub) ListByUserID(ctx context.Context, userID int64) ([]entities.TeamMember, error) {
	return s.listByUserFn(ctx, userID)
}
func (s teamMembersStub) RemoveByTeamID(ctx context.Context, teamID int64) error {
	return s.removeByTeamFn(ctx, teamID)
}

type usersStub struct {
	getByIDFn func(context.Context, int64) (entities.User, error)
}

func (s usersStub) Create(context.Context, entities.User) (entities.User, error) { return entities.User{}, nil }
func (s usersStub) GetByEmail(context.Context, string) (entities.User, error) { return entities.User{}, domainerrors.ErrNotFound }
func (s usersStub) GetByID(ctx context.Context, id int64) (entities.User, error) {
	return s.getByIDFn(ctx, id)
}

type cacheStub struct {
	invalidateFn func(context.Context, int64) error
}

func (s cacheStub) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (s cacheStub) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (s cacheStub) InvalidateTeam(ctx context.Context, teamID int64) error {
	return s.invalidateFn(ctx, teamID)
}

func TestTeamsPropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("repo error")
	team := entities.Team{ID: 10, CreatedBy: 1}

	teamRepo := teamRepoStub{
		createFn: func(context.Context, entities.Team) (entities.Team, error) { return entities.Team{}, repoErr },
		getByID:  func(context.Context, int64) (entities.Team, error) { return team, nil },
		deleteFn: func(context.Context, int64) error { return repoErr },
	}
	members := teamMembersStub{
		addFn:          func(context.Context, entities.TeamMember) error { return repoErr },
		getFn:          func(context.Context, int64, int64) (entities.TeamMember, error) { return entities.TeamMember{Role: entities.TeamRoleMember}, nil },
		removeFn:       func(context.Context, int64, int64) error { return repoErr },
		listByUserFn:   func(context.Context, int64) ([]entities.TeamMember, error) { return nil, repoErr },
		removeByTeamFn: func(context.Context, int64) error { return repoErr },
	}
	users := usersStub{
		getByIDFn: func(context.Context, int64) (entities.User, error) { return entities.User{}, repoErr },
	}
	calledInvalidate := false
	cache := cacheStub{
		invalidateFn: func(context.Context, int64) error {
			calledInvalidate = true
			return errors.New("cache down")
		},
	}

	svc := NewService(teamRepo, members, users, cache)

	if _, err := svc.CreateTeam(context.Background(), entities.Actor{UserID: 1}, CreateTeamInput{Name: "A"}); !errors.Is(err, repoErr) {
		t.Fatalf("expected create repo error, got %v", err)
	}

	if _, err := svc.ListTeams(context.Background(), entities.Actor{UserID: 1}); !errors.Is(err, repoErr) {
		t.Fatalf("expected list repo error, got %v", err)
	}

	if err := svc.DeleteTeam(context.Background(), entities.Actor{UserID: 1, IsAdmin: true}, 10); !errors.Is(err, repoErr) {
		t.Fatalf("expected delete repo error, got %v", err)
	}

	if err := svc.InviteMember(context.Background(), entities.Actor{UserID: 1}, InviteInput{TeamID: 10, UserID: 2}); !errors.Is(err, repoErr) {
		t.Fatalf("expected invite user lookup error, got %v", err)
	}

	if err := svc.RemoveMember(context.Background(), entities.Actor{UserID: 1}, RemoveMemberInput{TeamID: 10, UserID: 2}); !errors.Is(err, repoErr) {
		t.Fatalf("expected remove repo error, got %v", err)
	}

	// Ensure cache invalidation failure is ignored on success path.
	successSvc := NewService(teamRepoStub{
		createFn: func(context.Context, entities.Team) (entities.Team, error) { return entities.Team{}, nil },
		getByID:  func(context.Context, int64) (entities.Team, error) { return team, nil },
		deleteFn: func(context.Context, int64) error { return nil },
	}, teamMembersStub{
		addFn:        func(context.Context, entities.TeamMember) error { return nil },
		getFn:        func(context.Context, int64, int64) (entities.TeamMember, error) { return entities.TeamMember{}, nil },
		removeFn:     func(context.Context, int64, int64) error { return nil },
		listByUserFn: func(context.Context, int64) ([]entities.TeamMember, error) { return nil, nil },
		removeByTeamFn: func(context.Context, int64) error {
			return nil
		},
	}, users, cache)

	if err := successSvc.DeleteTeam(context.Background(), entities.Actor{UserID: 1, IsAdmin: true}, 10); err != nil {
		t.Fatalf("unexpected error on successful delete: %v", err)
	}
	if !calledInvalidate {
		t.Fatalf("expected cache invalidation to be attempted")
	}
}

func TestTeamsNotFoundMappings(t *testing.T) {
	t.Parallel()

	teamRepo := teamRepoStub{
		createFn: func(context.Context, entities.Team) (entities.Team, error) { return entities.Team{}, nil },
		getByID:  func(context.Context, int64) (entities.Team, error) { return entities.Team{}, domainerrors.ErrNotFound },
		deleteFn: func(context.Context, int64) error { return domainerrors.ErrNotFound },
	}
	members := teamMembersStub{
		addFn:          func(context.Context, entities.TeamMember) error { return nil },
		getFn:          func(context.Context, int64, int64) (entities.TeamMember, error) { return entities.TeamMember{}, domainerrors.ErrNotFound },
		removeFn:       func(context.Context, int64, int64) error { return domainerrors.ErrNotFound },
		listByUserFn:   func(context.Context, int64) ([]entities.TeamMember, error) { return nil, nil },
		removeByTeamFn: func(context.Context, int64) error { return nil },
	}
	users := usersStub{
		getByIDFn: func(context.Context, int64) (entities.User, error) { return entities.User{}, domainerrors.ErrNotFound },
	}

	svc := NewService(teamRepo, members, users, nil)
	admin := entities.Actor{UserID: 1, IsAdmin: true}

	if err := svc.DeleteTeam(context.Background(), admin, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for DeleteTeam, got %v", err)
	}
	if err := svc.InviteMember(context.Background(), admin, InviteInput{TeamID: 1, UserID: 2}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for InviteMember, got %v", err)
	}
	if err := svc.RemoveMember(context.Background(), admin, RemoveMemberInput{TeamID: 1, UserID: 2}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for RemoveMember, got %v", err)
	}
}

var _ repository.TeamRepository = teamRepoStub{}
var _ repository.TeamMemberRepository = teamMembersStub{}
var _ repository.UserRepository = usersStub{}
