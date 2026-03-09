package handler

import (
	"errors"
	"net/http"

	"tasks-service/internal/transport/http/middleware"
	"tasks-service/internal/usecase/reports"
)

type ReportsHandler struct {
	service *reports.Service
}

func NewReportsHandler(service *reports.Service) *ReportsHandler {
	return &ReportsHandler{service: service}
}

func (h *ReportsHandler) TeamStats(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	rows, err := h.service.TeamStats(r.Context(), actor)
	if err != nil {
		handleReportsError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"team_id":                row.TeamID,
			"team_name":              row.TeamName,
			"members_count":          row.MembersCount,
			"done_tasks_last_7_days": row.DoneTasksLast7Days,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ReportsHandler) TopCreatorsByTeam(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	rows, err := h.service.TopCreatorsByTeam(r.Context(), actor)
	if err != nil {
		handleReportsError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"team_id":       row.TeamID,
			"user_id":       row.UserID,
			"tasks_created": row.TasksCreated,
			"rank":          row.Rank,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ReportsHandler) InvalidAssignees(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	rows, err := h.service.InvalidAssignees(r.Context(), actor)
	if err != nil {
		handleReportsError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"task_id":     row.TaskID,
			"team_id":     row.TeamID,
			"assignee_id": row.AssigneeID,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleReportsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, reports.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid input")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
