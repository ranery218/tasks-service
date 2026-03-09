package httptransport

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tasks-service/internal/transport/http/handler"
	"tasks-service/internal/transport/http/middleware"
)

func NewRouter(
	healthHandler *handler.HealthHandler,
	authHandler *handler.AuthHandler,
	teamsHandler *handler.TeamsHandler,
	tasksHandler *handler.TasksHandler,
	reportsHandler *handler.ReportsHandler,
	metricsMiddleware func(next http.Handler) http.Handler,
	rateLimiter *middleware.RateLimit,
	authMiddleware *middleware.Auth,
) *chi.Mux {
	router := chi.NewRouter()
	router.Use(metricsMiddleware)

	router.Get("/healthz", healthHandler.Healthz)
	router.Get("/readyz", healthHandler.Readyz)
	router.Handle("/metrics", promhttp.Handler())
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(rateLimiter.Middleware)
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/token/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)

		r.Group(func(protected chi.Router) {
			protected.Use(authMiddleware.Middleware)
			protected.Post("/teams", teamsHandler.CreateTeam)
			protected.Get("/teams", teamsHandler.ListTeams)
			protected.Delete("/teams/{team_id}", teamsHandler.DeleteTeam)
			protected.Post("/teams/{team_id}/invite", teamsHandler.InviteMember)
			protected.Delete("/teams/{team_id}/members/{user_id}", teamsHandler.RemoveMember)
			protected.Post("/tasks", tasksHandler.CreateTask)
			protected.Get("/tasks", tasksHandler.ListTasks)
			protected.Put("/tasks/{task_id}", tasksHandler.UpdateTask)
			protected.Post("/tasks/{task_id}/comments", tasksHandler.AddComment)
			protected.Get("/tasks/{task_id}/history", tasksHandler.GetHistory)
			protected.Get("/reports/team-stats", reportsHandler.TeamStats)
			protected.Get("/reports/top-creators", reportsHandler.TopCreatorsByTeam)
			protected.Get("/reports/invalid-assignees", reportsHandler.InvalidAssignees)
		})
	})

	return router
}
