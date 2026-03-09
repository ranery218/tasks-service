package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"tasks-service/internal/domain/entities"
	"tasks-service/internal/usecase/health"
)

type HealthHandler struct {
	service *health.Service
}

type dependencyStatusResponse struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

type readyStatusResponse struct {
	Status       string                     `json:"status"`
	CheckedAtUTC string                     `json:"checked_at_utc"`
	Dependencies []dependencyStatusResponse `json:"dependencies,omitempty"`
}

type healthStatusResponse struct {
	Status       string `json:"status"`
	CheckedAtUTC string `json:"checked_at_utc"`
}

func NewHealthHandler(service *health.Service) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	status := h.service.Health(r.Context())
	writeJSON(w, http.StatusOK, mapHealthStatus(status))
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	status, ready := h.service.Ready(r.Context())
	response := mapReadyStatus(status)
	if !ready {
		writeJSON(w, http.StatusServiceUnavailable, response)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, httpStatus int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	_ = json.NewEncoder(w).Encode(payload)
}

func mapHealthStatus(status entities.HealthStatus) healthStatusResponse {
	return healthStatusResponse{
		Status:       status.Status,
		CheckedAtUTC: status.CheckedAtUTC.Format(time.RFC3339),
	}
}

func mapReadyStatus(status entities.ReadyStatus) readyStatusResponse {
	result := readyStatusResponse{
		Status:       status.Status,
		CheckedAtUTC: status.CheckedAtUTC.Format(time.RFC3339),
	}

	if len(status.Dependencies) == 0 {
		return result
	}

	result.Dependencies = make([]dependencyStatusResponse, 0, len(status.Dependencies))
	for _, dep := range status.Dependencies {
		result.Dependencies = append(result.Dependencies, dependencyStatusResponse{
			Name:    dep.Name,
			Healthy: dep.Healthy,
			Error:   dep.Error,
		})
	}

	return result
}
