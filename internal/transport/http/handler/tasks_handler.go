package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"tasks-service/internal/domain/entities"
	"tasks-service/internal/transport/http/middleware"
	"tasks-service/internal/usecase/tasks"
)

type TasksHandler struct {
	service *tasks.Service
}

type createTaskRequest struct {
	TeamID      int64   `json:"team_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	AssigneeID  *int64  `json:"assignee_id"`
	Status      string  `json:"status"`
	DueDate     *string `json:"due_date"`
}

type updateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	AssigneeID  *int64  `json:"assignee_id"`
	Status      *string `json:"status"`
	DueDate     *string `json:"due_date"`
}

type addCommentRequest struct {
	Text string `json:"text"`
}

type taskResponse struct {
	ID          int64   `json:"id"`
	TeamID      int64   `json:"team_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	AssigneeID  *int64  `json:"assignee_id,omitempty"`
	CreatedBy   int64   `json:"created_by"`
	Status      string  `json:"status"`
	DueDate     *string `json:"due_date,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func NewTasksHandler(service *tasks.Service) *TasksHandler {
	return &TasksHandler{service: service}
}

func (h *TasksHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	dueDate, err := parseOptionalTime(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "due_date must be RFC3339")
		return
	}

	task, err := h.service.CreateTask(r.Context(), actor, tasks.CreateTaskInput{
		TeamID:      req.TeamID,
		Title:       req.Title,
		Description: req.Description,
		AssigneeID:  req.AssigneeID,
		Status:      req.Status,
		DueDate:     dueDate,
	})
	if err != nil {
		handleTaskError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapTask(task))
}

func (h *TasksHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	teamID, err := parseOptionalInt64(r.URL.Query().Get("team_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "team_id must be a positive integer")
		return
	}
	assigneeID, err := parseOptionalInt64(r.URL.Query().Get("assignee_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "assignee_id must be a positive integer")
		return
	}
	limit, err := parsePositiveIntWithDefault(r.URL.Query().Get("limit"), 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
		return
	}
	offset, err := parseNonNegativeIntWithDefault(r.URL.Query().Get("offset"), 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "offset must be non-negative integer")
		return
	}

	result, err := h.service.ListTasks(r.Context(), actor, tasks.ListInput{
		TeamID:     teamID,
		Status:     r.URL.Query().Get("status"),
		AssigneeID: assigneeID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		handleTaskError(w, err)
		return
	}

	items := make([]taskResponse, 0, len(result.Items))
	for _, task := range result.Items {
		items = append(items, mapTask(task))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"meta":  map[string]any{"limit": result.Limit, "offset": result.Offset, "total": result.Total},
	})
}

func (h *TasksHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	taskID, err := parsePathID(chi.URLParam(r, "task_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "task_id must be positive integer")
		return
	}

	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	dueDate, err := parseOptionalTime(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "due_date must be RFC3339")
		return
	}

	updated, err := h.service.UpdateTask(r.Context(), actor, taskID, tasks.UpdateTaskInput{
		Title:       req.Title,
		Description: req.Description,
		AssigneeID:  req.AssigneeID,
		Status:      req.Status,
		DueDate:     dueDate,
	})
	if err != nil {
		handleTaskError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapTask(updated))
}

func (h *TasksHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	taskID, err := parsePathID(chi.URLParam(r, "task_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "task_id must be positive integer")
		return
	}

	var req addCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	comment, err := h.service.AddComment(r.Context(), actor, taskID, req.Text)
	if err != nil {
		handleTaskError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         comment.ID,
		"task_id":    comment.TaskID,
		"user_id":    comment.UserID,
		"text":       comment.Text,
		"created_at": comment.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *TasksHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	taskID, err := parsePathID(chi.URLParam(r, "task_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "task_id must be positive integer")
		return
	}

	items, err := h.service.GetHistory(r.Context(), actor, taskID)
	if err != nil {
		handleTaskError(w, err)
		return
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":         item.ID,
			"task_id":    item.TaskID,
			"field":      item.Field,
			"old_value":  item.OldValue,
			"new_value":  item.NewValue,
			"changed_by": item.ChangedBy,
			"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}

func handleTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tasks.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid input")
	case errors.Is(err, tasks.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, tasks.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "constraint violation")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func mapTask(task entities.Task) taskResponse {
	var dueDate *string
	if task.DueDate != nil {
		value := task.DueDate.UTC().Format(time.RFC3339)
		dueDate = &value
	}

	return taskResponse{
		ID:          task.ID,
		TeamID:      task.TeamID,
		Title:       task.Title,
		Description: task.Description,
		AssigneeID:  task.AssigneeID,
		CreatedBy:   task.CreatedBy,
		Status:      task.Status,
		DueDate:     dueDate,
		CreatedAt:   task.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   task.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func parseOptionalInt64(v string) (*int64, error) {
	if v == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, errors.New("invalid int")
	}
	return &parsed, nil
}

func parsePositiveIntWithDefault(v string, def int) (int, error) {
	if v == "" {
		return def, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid int")
	}
	return parsed, nil
}

func parseNonNegativeIntWithDefault(v string, def int) (int, error) {
	if v == "" {
		return def, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed < 0 {
		return 0, errors.New("invalid int")
	}
	return parsed, nil
}

func parseOptionalTime(v *string) (*time.Time, error) {
	if v == nil || *v == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}
