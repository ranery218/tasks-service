package health

import (
	"context"
	"time"

	domain "tasks-service/internal/domain/entities"
)

type DependencyPinger interface {
	Name() string
	Ping(ctx context.Context) error
}

type Service struct {
	pingers []DependencyPinger
	nowFn   func() time.Time
}

func NewService(pingers []DependencyPinger) *Service {
	return &Service{
		pingers: pingers,
		nowFn:   time.Now().UTC,
	}
}

func (s *Service) Health(_ context.Context) domain.HealthStatus {
	return domain.HealthStatus{
		Status:       "ok",
		CheckedAtUTC: s.nowFn(),
	}
}

func (s *Service) Ready(ctx context.Context) (domain.ReadyStatus, bool) {
	status := domain.ReadyStatus{
		Status:       "ready",
		CheckedAtUTC: s.nowFn(),
	}

	if len(s.pingers) == 0 {
		return status, true
	}

	deps := make([]domain.DependencyStatus, 0, len(s.pingers))
	allHealthy := true

	for _, pinger := range s.pingers {
		depStatus := domain.DependencyStatus{Name: pinger.Name(), Healthy: true}
		if err := pinger.Ping(ctx); err != nil {
			allHealthy = false
			depStatus.Healthy = false
			depStatus.Error = err.Error()
		}
		deps = append(deps, depStatus)
	}

	if !allHealthy {
		status.Status = "not_ready"
	}

	status.Dependencies = deps

	return status, allHealthy
}
