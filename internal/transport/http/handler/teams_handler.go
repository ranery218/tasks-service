package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"tasks-service/internal/transport/http/middleware"
	"tasks-service/internal/usecase/teams"
)

type TeamsHandler struct {
	service *teams.Service
}

type createTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type inviteMemberRequest struct {
	UserID int64 `json:"user_id"`
}

type teamResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   int64  `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	Role        string `json:"role,omitempty"`
}

func NewTeamsHandler(service *teams.Service) *TeamsHandler {
	return &TeamsHandler{service: service}
}

func (h *TeamsHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req createTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	team, err := h.service.CreateTeam(r.Context(), actor, teams.CreateTeamInput{Name: req.Name, Description: req.Description})
	if err != nil {
		switch {
		case errors.Is(err, teams.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, teamResponse{
		ID:          team.ID,
		Name:        team.Name,
		Description: team.Description,
		CreatedBy:   team.CreatedBy,
		CreatedAt:   team.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Role:        "owner",
	})
}

func (h *TeamsHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	items, err := h.service.ListTeams(r.Context(), actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	result := make([]teamResponse, 0, len(items))
	for _, item := range items {
		result = append(result, teamResponse{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			CreatedBy:   item.CreatedBy,
			CreatedAt:   item.CreatedAt,
			Role:        item.Role,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *TeamsHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	teamID, err := parsePathID(chi.URLParam(r, "team_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}

	err = h.service.DeleteTeam(r.Context(), actor, teamID)
	if err != nil {
		switch {
		case errors.Is(err, teams.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid input")
		case errors.Is(err, teams.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		case errors.Is(err, teams.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "team not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TeamsHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	teamID, err := parsePathID(chi.URLParam(r, "team_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}

	var req inviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	err = h.service.InviteMember(r.Context(), actor, teams.InviteInput{TeamID: teamID, UserID: req.UserID})
	if err != nil {
		switch {
		case errors.Is(err, teams.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid input")
		case errors.Is(err, teams.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		case errors.Is(err, teams.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "team or user not found")
		case errors.Is(err, teams.ErrConflict):
			writeError(w, http.StatusConflict, "conflict", "user already in team")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team_id": teamID, "user_id": req.UserID, "role": "member"})
}

func (h *TeamsHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	teamID, err := parsePathID(chi.URLParam(r, "team_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}

	userID, err := parsePathID(chi.URLParam(r, "user_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_id", "user_id must be a positive integer")
		return
	}

	err = h.service.RemoveMember(r.Context(), actor, teams.RemoveMemberInput{TeamID: teamID, UserID: userID})
	if err != nil {
		switch {
		case errors.Is(err, teams.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid input")
		case errors.Is(err, teams.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		case errors.Is(err, teams.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "team or user not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePathID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
