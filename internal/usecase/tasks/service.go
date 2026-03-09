package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

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
	tasks    repository.TaskRepository
	members  repository.TeamMemberRepository
	comments repository.TaskCommentRepository
	history  repository.TaskHistoryRepository
	cache    repository.TaskListCache
	cacheTTL time.Duration
}

type CreateTaskInput struct {
	TeamID      int64
	Title       string
	Description string
	AssigneeID  *int64
	Status      string
	DueDate     *time.Time
}

type UpdateTaskInput struct {
	Title       *string
	Description *string
	AssigneeID  *int64
	Status      *string
	DueDate     *time.Time
}

type ListInput struct {
	TeamID     *int64
	Status     string
	AssigneeID *int64
	Limit      int
	Offset     int
}

type ListResult struct {
	Items  []entities.Task
	Total  int
	Limit  int
	Offset int
}

type noopTaskListCache struct{}

func (n noopTaskListCache) Get(_ context.Context, _ string) ([]byte, bool, error) {
	return nil, false, nil
}
func (n noopTaskListCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (n noopTaskListCache) InvalidateTeam(_ context.Context, _ int64) error { return nil }

func NewService(
	tasks repository.TaskRepository,
	members repository.TeamMemberRepository,
	comments repository.TaskCommentRepository,
	history repository.TaskHistoryRepository,
	cache repository.TaskListCache,
	cacheTTL time.Duration,
) *Service {
	if cache == nil {
		cache = noopTaskListCache{}
	}
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}

	return &Service{
		tasks:    tasks,
		members:  members,
		comments: comments,
		history:  history,
		cache:    cache,
		cacheTTL: cacheTTL,
	}
}

func (s *Service) CreateTask(ctx context.Context, actor entities.Actor, input CreateTaskInput) (entities.Task, error) {
	if actor.UserID <= 0 || input.TeamID <= 0 || strings.TrimSpace(input.Title) == "" {
		return entities.Task{}, ErrInvalidInput
	}

	if err := s.ensureTeamMember(ctx, input.TeamID, actor.UserID); err != nil {
		return entities.Task{}, err
	}

	status := input.Status
	if status == "" {
		status = entities.TaskStatusTodo
	}
	if !isValidStatus(status) {
		return entities.Task{}, ErrInvalidInput
	}

	if input.AssigneeID != nil {
		if err := s.ensureTeamMember(ctx, input.TeamID, *input.AssigneeID); err != nil {
			if errors.Is(err, ErrForbidden) {
				return entities.Task{}, ErrConflict
			}
			return entities.Task{}, err
		}
	}

	task := entities.Task{
		TeamID:      input.TeamID,
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		AssigneeID:  input.AssigneeID,
		CreatedBy:   actor.UserID,
		Status:      status,
		DueDate:     input.DueDate,
	}

	created, err := s.tasks.Create(ctx, task)
	if err != nil {
		return entities.Task{}, err
	}
	_ = s.cache.InvalidateTeam(ctx, created.TeamID)

	return created, nil
}

func (s *Service) ListTasks(ctx context.Context, _ entities.Actor, input ListInput) (ListResult, error) {
	if input.Limit <= 0 {
		input.Limit = 20
	}
	if input.Limit > 100 {
		input.Limit = 100
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
	if input.Status != "" && !isValidStatus(input.Status) {
		return ListResult{}, ErrInvalidInput
	}

	if input.TeamID != nil {
		cacheKey := buildTaskListCacheKey(*input.TeamID, input.Status, input.AssigneeID, input.Limit, input.Offset)
		if payload, hit, err := s.cache.Get(ctx, cacheKey); err == nil && hit {
			var cached ListResult
			if unmarshalErr := json.Unmarshal(payload, &cached); unmarshalErr == nil {
				return cached, nil
			}
		}

		items, total, err := s.tasks.List(ctx, entities.TaskFilter{
			TeamID:     input.TeamID,
			Status:     input.Status,
			AssigneeID: input.AssigneeID,
			Limit:      input.Limit,
			Offset:     input.Offset,
		})
		if err != nil {
			return ListResult{}, err
		}

		result := ListResult{Items: items, Total: total, Limit: input.Limit, Offset: input.Offset}
		if payload, marshalErr := json.Marshal(result); marshalErr == nil {
			_ = s.cache.Set(ctx, cacheKey, payload, s.cacheTTL)
		}
		return result, nil
	}

	items, total, err := s.tasks.List(ctx, entities.TaskFilter{
		TeamID:     input.TeamID,
		Status:     input.Status,
		AssigneeID: input.AssigneeID,
		Limit:      input.Limit,
		Offset:     input.Offset,
	})
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items, Total: total, Limit: input.Limit, Offset: input.Offset}, nil
}

func (s *Service) UpdateTask(ctx context.Context, actor entities.Actor, taskID int64, input UpdateTaskInput) (entities.Task, error) {
	if actor.UserID <= 0 || taskID <= 0 {
		return entities.Task{}, ErrInvalidInput
	}

	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return entities.Task{}, ErrNotFound
		}
		return entities.Task{}, err
	}

	if err := s.ensureTeamMember(ctx, task.TeamID, actor.UserID); err != nil {
		return entities.Task{}, err
	}

	historyEntries := make([]entities.TaskHistory, 0)

	if input.Title != nil {
		newTitle := strings.TrimSpace(*input.Title)
		if newTitle == "" {
			return entities.Task{}, ErrInvalidInput
		}
		if newTitle != task.Title {
			historyEntries = append(historyEntries, newHistory(task.ID, "title", task.Title, newTitle, actor.UserID))
			task.Title = newTitle
		}
	}

	if input.Description != nil {
		newDescription := strings.TrimSpace(*input.Description)
		if newDescription != task.Description {
			historyEntries = append(historyEntries, newHistory(task.ID, "description", task.Description, newDescription, actor.UserID))
			task.Description = newDescription
		}
	}

	if input.Status != nil {
		if !isValidStatus(*input.Status) {
			return entities.Task{}, ErrInvalidInput
		}
		if *input.Status != task.Status {
			historyEntries = append(historyEntries, newHistory(task.ID, "status", task.Status, *input.Status, actor.UserID))
			task.Status = *input.Status
		}
	}

	if input.AssigneeID != nil {
		if err := s.ensureTeamMember(ctx, task.TeamID, *input.AssigneeID); err != nil {
			if errors.Is(err, ErrForbidden) {
				return entities.Task{}, ErrConflict
			}
			return entities.Task{}, err
		}

		if !sameAssignee(task.AssigneeID, input.AssigneeID) {
			historyEntries = append(historyEntries, newHistory(task.ID, "assignee_id", formatNullableInt(task.AssigneeID), formatNullableInt(input.AssigneeID), actor.UserID))
			task.AssigneeID = input.AssigneeID
		}
	}

	if input.DueDate != nil {
		if !sameTime(task.DueDate, input.DueDate) {
			historyEntries = append(historyEntries, newHistory(task.ID, "due_date", formatNullableTime(task.DueDate), formatNullableTime(input.DueDate), actor.UserID))
			task.DueDate = input.DueDate
		}
	}

	updated, err := s.tasks.Update(ctx, task)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return entities.Task{}, ErrNotFound
		}
		return entities.Task{}, err
	}

	if len(historyEntries) > 0 {
		if err := s.history.CreateMany(ctx, historyEntries); err != nil {
			return entities.Task{}, err
		}
	}
	_ = s.cache.InvalidateTeam(ctx, updated.TeamID)

	return updated, nil
}

func (s *Service) AddComment(ctx context.Context, actor entities.Actor, taskID int64, text string) (entities.TaskComment, error) {
	if actor.UserID <= 0 || taskID <= 0 || strings.TrimSpace(text) == "" {
		return entities.TaskComment{}, ErrInvalidInput
	}

	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return entities.TaskComment{}, ErrNotFound
		}
		return entities.TaskComment{}, err
	}

	if err := s.ensureTeamMember(ctx, task.TeamID, actor.UserID); err != nil {
		return entities.TaskComment{}, err
	}

	return s.comments.Create(ctx, entities.TaskComment{TaskID: taskID, UserID: actor.UserID, Text: strings.TrimSpace(text)})
}

func (s *Service) GetHistory(ctx context.Context, _ entities.Actor, taskID int64) ([]entities.TaskHistory, error) {
	if taskID <= 0 {
		return nil, ErrInvalidInput
	}

	if _, err := s.tasks.GetByID(ctx, taskID); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return s.history.ListByTaskID(ctx, taskID)
}

func (s *Service) ensureTeamMember(ctx context.Context, teamID, userID int64) error {
	_, err := s.members.Get(ctx, teamID, userID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	return nil
}

func isValidStatus(status string) bool {
	switch status {
	case entities.TaskStatusTodo, entities.TaskStatusInProgress, entities.TaskStatusDone:
		return true
	default:
		return false
	}
}

func newHistory(taskID int64, field, oldVal, newVal string, changedBy int64) entities.TaskHistory {
	return entities.TaskHistory{TaskID: taskID, Field: field, OldValue: oldVal, NewValue: newVal, ChangedBy: changedBy}
}

func sameAssignee(oldValue *int64, newValue *int64) bool {
	if oldValue == nil && newValue == nil {
		return true
	}
	if oldValue == nil || newValue == nil {
		return false
	}
	return *oldValue == *newValue
}

func sameTime(oldValue *time.Time, newValue *time.Time) bool {
	if oldValue == nil && newValue == nil {
		return true
	}
	if oldValue == nil || newValue == nil {
		return false
	}
	return oldValue.UTC().Equal(newValue.UTC())
}

func formatNullableInt(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func formatNullableTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

func buildTaskListCacheKey(teamID int64, status string, assigneeID *int64, limit, offset int) string {
	assignee := "none"
	if assigneeID != nil {
		assignee = strconv.FormatInt(*assigneeID, 10)
	}

	return "tasks:list:team:" + strconv.FormatInt(teamID, 10) +
		":status:" + status +
		":assignee:" + assignee +
		":limit:" + strconv.Itoa(limit) +
		":offset:" + strconv.Itoa(offset)
}
