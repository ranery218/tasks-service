package tasks

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"tasks-service/internal/domain/entities"
	domainerrors "tasks-service/internal/domain/errors"
	"tasks-service/internal/domain/repository"
	"tasks-service/internal/infrastructure/repository/memory"
)

type fixtures struct {
	users    *memory.UserRepository
	teams    *memory.TeamRepository
	members  *memory.TeamMemberRepository
	tasks    *memory.TaskRepository
	comments *memory.TaskCommentRepository
	history  *memory.TaskHistoryRepository
	tasksSvc *Service
}

func newFixtures() fixtures {
	users := memory.NewUserRepository()
	teams := memory.NewTeamRepository()
	members := memory.NewTeamMemberRepository()
	tasksRepo := memory.NewTaskRepository()
	comments := memory.NewTaskCommentRepository()
	history := memory.NewTaskHistoryRepository()

	return fixtures{
		users:    users,
		teams:    teams,
		members:  members,
		tasks:    tasksRepo,
		comments: comments,
		history:  history,
		tasksSvc: NewService(tasksRepo, members, comments, history, nil, 0),
	}
}

func createUser(t *testing.T, users *memory.UserRepository, email string) entities.User {
	t.Helper()
	u, err := users.Create(context.Background(), entities.User{Email: email, Name: email, PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func createTeamAndOwnerMembership(t *testing.T, f fixtures, owner entities.User) entities.Team {
	t.Helper()
	team, err := f.teams.Create(context.Background(), entities.Team{Name: "Team", CreatedBy: owner.ID})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := f.members.Add(context.Background(), entities.TeamMember{TeamID: team.ID, UserID: owner.ID, Role: entities.TeamRoleOwner}); err != nil {
		t.Fatalf("add owner member: %v", err)
	}
	return team
}

func TestCreateTaskRequiresMembership(t *testing.T) {
	t.Parallel()
	f := newFixtures()

	owner := createUser(t, f.users, "owner@example.com")
	outsider := createUser(t, f.users, "outsider@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)

	_, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: outsider.ID}, CreateTaskInput{TeamID: team.ID, Title: "Task 1"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	task, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{TeamID: team.ID, Title: "Task 2"})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if task.Status != entities.TaskStatusTodo {
		t.Fatalf("expected default status todo, got %s", task.Status)
	}
}

func TestCreateTaskAssigneeMustBeTeamMember(t *testing.T) {
	t.Parallel()
	f := newFixtures()

	owner := createUser(t, f.users, "owner@example.com")
	nonMember := createUser(t, f.users, "nonmember@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)

	_, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
		TeamID: team.ID,
		Title:  "Task",
		AssigneeID: func() *int64 {
			v := nonMember.ID
			return &v
		}(),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestUpdateTaskWritesHistory(t *testing.T) {
	t.Parallel()
	f := newFixtures()

	owner := createUser(t, f.users, "owner@example.com")
	member := createUser(t, f.users, "member@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)
	if err := f.members.Add(context.Background(), entities.TeamMember{TeamID: team.ID, UserID: member.ID, Role: entities.TeamRoleMember}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	due := time.Now().UTC().Add(24 * time.Hour)
	task, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
		TeamID:      team.ID,
		Title:       "Task",
		Description: "Desc",
		Status:      entities.TaskStatusTodo,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	newTitle := "Task Updated"
	newStatus := entities.TaskStatusInProgress
	updated, err := f.tasksSvc.UpdateTask(context.Background(), entities.Actor{UserID: member.ID}, task.ID, UpdateTaskInput{
		Title:      &newTitle,
		Status:     &newStatus,
		DueDate:    &due,
		AssigneeID: func() *int64 { v := member.ID; return &v }(),
	})
	if err != nil {
		t.Fatalf("update task failed: %v", err)
	}
	if updated.Title != newTitle || updated.Status != newStatus {
		t.Fatalf("task not updated")
	}

	history, err := f.tasksSvc.GetHistory(context.Background(), entities.Actor{UserID: owner.ID}, task.ID)
	if err != nil {
		t.Fatalf("get history failed: %v", err)
	}
	if len(history) < 3 {
		t.Fatalf("expected at least 3 history entries, got %d", len(history))
	}
}

func TestUpdateTaskRequiresMembership(t *testing.T) {
	t.Parallel()
	f := newFixtures()

	owner := createUser(t, f.users, "owner@example.com")
	outsider := createUser(t, f.users, "outsider@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)
	task, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{TeamID: team.ID, Title: "Task"})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	newTitle := "New"
	_, err = f.tasksSvc.UpdateTask(context.Background(), entities.Actor{UserID: outsider.ID}, task.ID, UpdateTaskInput{Title: &newTitle})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestAddCommentMembershipAndHistoryReadableForAnyAuthorized(t *testing.T) {
	t.Parallel()
	f := newFixtures()

	owner := createUser(t, f.users, "owner@example.com")
	member := createUser(t, f.users, "member@example.com")
	outsider := createUser(t, f.users, "outsider@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)
	if err := f.members.Add(context.Background(), entities.TeamMember{TeamID: team.ID, UserID: member.ID, Role: entities.TeamRoleMember}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	task, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{TeamID: team.ID, Title: "Task"})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	if _, err := f.tasksSvc.AddComment(context.Background(), entities.Actor{UserID: outsider.ID}, task.ID, "Nope"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	comment, err := f.tasksSvc.AddComment(context.Background(), entities.Actor{UserID: member.ID}, task.ID, "Started")
	if err != nil {
		t.Fatalf("add comment failed: %v", err)
	}
	if comment.ID == 0 {
		t.Fatalf("expected persisted comment id")
	}

	if _, err := f.tasksSvc.GetHistory(context.Background(), entities.Actor{UserID: outsider.ID}, task.ID); err != nil {
		t.Fatalf("history should be readable by any authorized: %v", err)
	}
}

func TestListTasksWithFiltersAndPagination(t *testing.T) {
	t.Parallel()
	f := newFixtures()

	owner := createUser(t, f.users, "owner@example.com")
	member := createUser(t, f.users, "member@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)
	if err := f.members.Add(context.Background(), entities.TeamMember{TeamID: team.ID, UserID: member.ID, Role: entities.TeamRoleMember}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	for i := 0; i < 3; i++ {
		status := entities.TaskStatusTodo
		if i%2 == 0 {
			status = entities.TaskStatusDone
		}
		_, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
			TeamID:     team.ID,
			Title:      "Task",
			AssigneeID: &member.ID,
			Status:     status,
		})
		if err != nil {
			t.Fatalf("create task failed: %v", err)
		}
	}

	result, err := f.tasksSvc.ListTasks(context.Background(), entities.Actor{UserID: owner.ID}, ListInput{
		TeamID:     &team.ID,
		Status:     entities.TaskStatusDone,
		AssigneeID: &member.ID,
		Limit:      1,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("unexpected pagination/total: total=%d len=%d", result.Total, len(result.Items))
	}
}

func TestListTasksUsesCacheForTeamQueries(t *testing.T) {
	t.Parallel()

	f := newFixtures()
	cache := &testTaskListCache{store: make(map[string][]byte)}
	countingRepo := &countingTaskRepository{delegate: f.tasks}
	f.tasksSvc = NewService(countingRepo, f.members, f.comments, f.history, cache, 5*time.Minute)

	owner := createUser(t, f.users, "owner@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)

	_, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
		TeamID: team.ID,
		Title:  "Task",
		Status: entities.TaskStatusTodo,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	result1, err := f.tasksSvc.ListTasks(context.Background(), entities.Actor{UserID: owner.ID}, ListInput{
		TeamID: &team.ID, Limit: 20, Offset: 0,
	})
	if err != nil {
		t.Fatalf("first list tasks failed: %v", err)
	}
	if len(result1.Items) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result1.Items))
	}

	result2, err := f.tasksSvc.ListTasks(context.Background(), entities.Actor{UserID: owner.ID}, ListInput{
		TeamID: &team.ID, Limit: 20, Offset: 0,
	})
	if err != nil {
		t.Fatalf("second list tasks failed: %v", err)
	}
	if len(result2.Items) != 1 {
		t.Fatalf("expected cached 1 task, got %d", len(result2.Items))
	}
	if countingRepo.listCalls != 1 {
		t.Fatalf("expected repository list to be called once due to cache hit, got %d", countingRepo.listCalls)
	}
}

type testTaskListCache struct {
	store map[string][]byte
}

type countingTaskRepository struct {
	delegate  repository.TaskRepository
	listCalls int
}

func (r *countingTaskRepository) Create(ctx context.Context, task entities.Task) (entities.Task, error) {
	return r.delegate.Create(ctx, task)
}

func (r *countingTaskRepository) GetByID(ctx context.Context, id int64) (entities.Task, error) {
	return r.delegate.GetByID(ctx, id)
}

func (r *countingTaskRepository) Update(ctx context.Context, task entities.Task) (entities.Task, error) {
	return r.delegate.Update(ctx, task)
}

func (r *countingTaskRepository) List(ctx context.Context, filter entities.TaskFilter) ([]entities.Task, int, error) {
	r.listCalls++
	return r.delegate.List(ctx, filter)
}

func (c *testTaskListCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	value, ok := c.store[key]
	return value, ok, nil
}

func (c *testTaskListCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.store[key] = append([]byte(nil), value...)
	return nil
}

func (c *testTaskListCache) InvalidateTeam(_ context.Context, teamID int64) error {
	prefix := "tasks:list:team:" + strconv.FormatInt(teamID, 10) + ":"
	for key := range c.store {
		if strings.HasPrefix(key, prefix) {
			delete(c.store, key)
		}
	}
	return nil
}

func TestListTasksValidationAndDefaults(t *testing.T) {
	t.Parallel()

	f := newFixtures()
	owner := createUser(t, f.users, "owner@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)
	_, _ = f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
		TeamID: team.ID,
		Title:  "Task",
	})

	_, err := f.tasksSvc.ListTasks(context.Background(), entities.Actor{UserID: owner.ID}, ListInput{Status: "bad"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	res, err := f.tasksSvc.ListTasks(context.Background(), entities.Actor{UserID: owner.ID}, ListInput{
		TeamID: &team.ID,
		Limit:  500,
		Offset: -10,
	})
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if res.Limit != 100 {
		t.Fatalf("expected clamped limit=100, got %d", res.Limit)
	}
	if res.Offset != 0 {
		t.Fatalf("expected normalized offset=0, got %d", res.Offset)
	}

	res2, err := f.tasksSvc.ListTasks(context.Background(), entities.Actor{UserID: owner.ID}, ListInput{
		TeamID: &team.ID,
	})
	if err != nil {
		t.Fatalf("list tasks with defaults failed: %v", err)
	}
	if res2.Limit != 20 {
		t.Fatalf("expected default limit=20, got %d", res2.Limit)
	}
}

func TestCreateTaskInvalidInputAndStatus(t *testing.T) {
	t.Parallel()

	f := newFixtures()
	owner := createUser(t, f.users, "owner@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)

	_, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{}, CreateTaskInput{TeamID: team.ID, Title: "x"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for actor, got %v", err)
	}
	_, err = f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{TeamID: team.ID})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty title, got %v", err)
	}
	_, err = f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
		TeamID: team.ID, Title: "x", Status: "bad",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad status, got %v", err)
	}
}

func TestTaskNotFoundMappings(t *testing.T) {
	t.Parallel()

	svc := NewService(taskRepoStub{
		getByIDFn: func(context.Context, int64) (entities.Task, error) { return entities.Task{}, domainerrors.ErrNotFound },
	}, memberRepoStub{}, commentRepoStub{}, historyRepoStub{}, nil, 0)

	if _, err := svc.UpdateTask(context.Background(), entities.Actor{UserID: 1}, 10, UpdateTaskInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound in UpdateTask, got %v", err)
	}
	if _, err := svc.AddComment(context.Background(), entities.Actor{UserID: 1}, 10, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound in AddComment, got %v", err)
	}
	if _, err := svc.GetHistory(context.Background(), entities.Actor{UserID: 1}, 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound in GetHistory, got %v", err)
	}
}

func TestAddCommentValidation(t *testing.T) {
	t.Parallel()

	f := newFixtures()
	_, err := f.tasksSvc.AddComment(context.Background(), entities.Actor{}, 1, "x")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for actor, got %v", err)
	}
	_, err = f.tasksSvc.AddComment(context.Background(), entities.Actor{UserID: 1}, 0, "x")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for task id, got %v", err)
	}
	_, err = f.tasksSvc.AddComment(context.Background(), entities.Actor{UserID: 1}, 1, "  ")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for text, got %v", err)
	}
}

func TestUpdateTaskBranches(t *testing.T) {
	t.Parallel()

	f := newFixtures()
	owner := createUser(t, f.users, "owner@example.com")
	member := createUser(t, f.users, "member@example.com")
	outsider := createUser(t, f.users, "outsider@example.com")
	team := createTeamAndOwnerMembership(t, f, owner)
	if err := f.members.Add(context.Background(), entities.TeamMember{TeamID: team.ID, UserID: member.ID, Role: entities.TeamRoleMember}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	task, err := f.tasksSvc.CreateTask(context.Background(), entities.Actor{UserID: owner.ID}, CreateTaskInput{
		TeamID: team.ID,
		Title:  "Task",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	badStatus := "bad"
	if _, err := f.tasksSvc.UpdateTask(context.Background(), entities.Actor{UserID: owner.ID}, task.ID, UpdateTaskInput{Status: &badStatus}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad status, got %v", err)
	}

	emptyTitle := "  "
	if _, err := f.tasksSvc.UpdateTask(context.Background(), entities.Actor{UserID: owner.ID}, task.ID, UpdateTaskInput{Title: &emptyTitle}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty title, got %v", err)
	}

	if _, err := f.tasksSvc.UpdateTask(context.Background(), entities.Actor{UserID: outsider.ID}, task.ID, UpdateTaskInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for outsider, got %v", err)
	}

	if _, err := f.tasksSvc.UpdateTask(context.Background(), entities.Actor{UserID: owner.ID}, task.ID, UpdateTaskInput{
		AssigneeID: &outsider.ID,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for assignee outside team, got %v", err)
	}
}

func TestUpdateTaskPropagatesUpdateAndHistoryErrors(t *testing.T) {
	t.Parallel()

	updateErr := errors.New("update failed")
	taskRepo := taskRepoStub{
		getByIDFn: func(context.Context, int64) (entities.Task, error) {
			return entities.Task{ID: 1, TeamID: 10, Title: "A", Status: entities.TaskStatusTodo}, nil
		},
		updateFn: func(context.Context, entities.Task) (entities.Task, error) {
			return entities.Task{}, updateErr
		},
	}
	memberRepo := memberRepoStub{
		getFn: func(context.Context, int64, int64) (entities.TeamMember, error) {
			return entities.TeamMember{TeamID: 10, UserID: 1}, nil
		},
	}
	svc := NewService(taskRepo, memberRepo, commentRepoStub{}, historyRepoStub{}, nil, 0)

	title := "B"
	if _, err := svc.UpdateTask(context.Background(), entities.Actor{UserID: 1}, 1, UpdateTaskInput{Title: &title}); !errors.Is(err, updateErr) {
		t.Fatalf("expected update error, got %v", err)
	}

	historyErr := errors.New("history failed")
	taskRepo.updateFn = func(_ context.Context, task entities.Task) (entities.Task, error) { return task, nil }
	svc = NewService(taskRepo, memberRepo, commentRepoStub{}, historyRepoStub{
		createManyFn: func(context.Context, []entities.TaskHistory) error { return historyErr },
	}, nil, 0)

	if _, err := svc.UpdateTask(context.Background(), entities.Actor{UserID: 1}, 1, UpdateTaskInput{Title: &title}); !errors.Is(err, historyErr) {
		t.Fatalf("expected history error, got %v", err)
	}
}

func TestGetHistoryValidationAndErrorPropagation(t *testing.T) {
	t.Parallel()

	svc := NewService(taskRepoStub{
		getByIDFn: func(context.Context, int64) (entities.Task, error) {
			return entities.Task{ID: 1}, nil
		},
	}, memberRepoStub{}, commentRepoStub{}, historyRepoStub{
		listByTaskFn: func(context.Context, int64) ([]entities.TaskHistory, error) {
			return nil, errors.New("history list failed")
		},
	}, nil, 0)

	if _, err := svc.GetHistory(context.Background(), entities.Actor{UserID: 1}, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	if _, err := svc.GetHistory(context.Background(), entities.Actor{UserID: 1}, 1); err == nil || err.Error() != "history list failed" {
		t.Fatalf("expected propagated history list error, got %v", err)
	}
}

type taskRepoStub struct {
	createFn  func(context.Context, entities.Task) (entities.Task, error)
	getByIDFn func(context.Context, int64) (entities.Task, error)
	updateFn  func(context.Context, entities.Task) (entities.Task, error)
	listFn    func(context.Context, entities.TaskFilter) ([]entities.Task, int, error)
}

func (s taskRepoStub) Create(ctx context.Context, task entities.Task) (entities.Task, error) {
	if s.createFn == nil {
		return task, nil
	}
	return s.createFn(ctx, task)
}
func (s taskRepoStub) GetByID(ctx context.Context, id int64) (entities.Task, error) {
	if s.getByIDFn == nil {
		return entities.Task{}, domainerrors.ErrNotFound
	}
	return s.getByIDFn(ctx, id)
}
func (s taskRepoStub) Update(ctx context.Context, task entities.Task) (entities.Task, error) {
	if s.updateFn == nil {
		return task, nil
	}
	return s.updateFn(ctx, task)
}
func (s taskRepoStub) List(ctx context.Context, filter entities.TaskFilter) ([]entities.Task, int, error) {
	if s.listFn == nil {
		return nil, 0, nil
	}
	return s.listFn(ctx, filter)
}

type memberRepoStub struct {
	getFn func(context.Context, int64, int64) (entities.TeamMember, error)
}

func (s memberRepoStub) Add(context.Context, entities.TeamMember) error { return nil }
func (s memberRepoStub) Get(ctx context.Context, teamID, userID int64) (entities.TeamMember, error) {
	if s.getFn == nil {
		return entities.TeamMember{}, nil
	}
	return s.getFn(ctx, teamID, userID)
}
func (s memberRepoStub) Remove(context.Context, int64, int64) error               { return nil }
func (s memberRepoStub) ListByUserID(context.Context, int64) ([]entities.TeamMember, error) {
	return nil, nil
}
func (s memberRepoStub) RemoveByTeamID(context.Context, int64) error { return nil }

type commentRepoStub struct {
	createFn func(context.Context, entities.TaskComment) (entities.TaskComment, error)
}

func (s commentRepoStub) Create(ctx context.Context, c entities.TaskComment) (entities.TaskComment, error) {
	if s.createFn == nil {
		c.ID = 1
		return c, nil
	}
	return s.createFn(ctx, c)
}

type historyRepoStub struct {
	createManyFn func(context.Context, []entities.TaskHistory) error
	listByTaskFn func(context.Context, int64) ([]entities.TaskHistory, error)
}

func (s historyRepoStub) CreateMany(ctx context.Context, entries []entities.TaskHistory) error {
	if s.createManyFn == nil {
		return nil
	}
	return s.createManyFn(ctx, entries)
}
func (s historyRepoStub) ListByTaskID(ctx context.Context, taskID int64) ([]entities.TaskHistory, error) {
	if s.listByTaskFn == nil {
		return nil, nil
	}
	return s.listByTaskFn(ctx, taskID)
}

var _ repository.TaskRepository = taskRepoStub{}
