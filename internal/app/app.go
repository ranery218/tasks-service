package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"tasks-service/internal/config"
	"tasks-service/internal/domain/repository"
	cacheinfra "tasks-service/internal/infrastructure/cache"
	jwtservice "tasks-service/internal/infrastructure/jwt"
	metricsinfra "tasks-service/internal/infrastructure/metrics"
	"tasks-service/internal/infrastructure/ratelimit"
	memoryrepo "tasks-service/internal/infrastructure/repository/memory"
	mysqlrepo "tasks-service/internal/infrastructure/repository/mysql"
	"tasks-service/internal/infrastructure/repository/readiness"
	httptransport "tasks-service/internal/transport/http"
	"tasks-service/internal/transport/http/handler"
	"tasks-service/internal/transport/http/middleware"
	"tasks-service/internal/usecase/auth"
	"tasks-service/internal/usecase/health"
	"tasks-service/internal/usecase/reports"
	"tasks-service/internal/usecase/tasks"
	"tasks-service/internal/usecase/teams"
)

type App struct {
	cfg    config.Config
	server *http.Server
	db     *sql.DB
	redis  *redis.Client
}

func New(cfg config.Config) (*App, error) {
	jwt := jwtservice.NewService(cfg.JWTAccessSecret)
	var db *sql.DB
	var redisClient *redis.Client

	var (
		userRepo        repository.UserRepository
		refreshRepo     repository.RefreshTokenRepository
		teamRepo        repository.TeamRepository
		teamMemberRepo  repository.TeamMemberRepository
		taskRepo        repository.TaskRepository
		taskCommentRepo repository.TaskCommentRepository
		taskHistoryRepo repository.TaskHistoryRepository
		reportRepo      repository.ReportRepository
		taskListCache   repository.TaskListCache
		healthPingers   []health.DependencyPinger
	)

	if cfg.MySQLDSN != "" {
		if !hasSQLDriver("mysql") {
			return nil, fmt.Errorf("mysql driver is not registered; add a mysql driver import before using MYSQL_DSN")
		}

		var err error
		db, err = sql.Open("mysql", cfg.MySQLDSN)
		if err != nil {
			return nil, fmt.Errorf("open mysql: %w", err)
		}

		db.SetMaxOpenConns(30)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
		db.SetConnMaxIdleTime(5 * time.Minute)

		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping mysql (ensure mysql driver is linked): %w", err)
		}

		userRepo = mysqlrepo.NewUserRepository(db)
		refreshRepo = mysqlrepo.NewRefreshTokenRepository(db)
		teamRepo = mysqlrepo.NewTeamRepository(db)
		teamMemberRepo = mysqlrepo.NewTeamMemberRepository(db)
		taskRepo = mysqlrepo.NewTaskRepository(db)
		taskCommentRepo = mysqlrepo.NewTaskCommentRepository(db)
		taskHistoryRepo = mysqlrepo.NewTaskHistoryRepository(db)
		reportRepo = mysqlrepo.NewReportRepository(db)
		healthPingers = []health.DependencyPinger{readiness.NewMySQLPinger(db)}
	} else {
		memUserRepo := memoryrepo.NewUserRepository()
		memRefreshRepo := memoryrepo.NewRefreshTokenRepository()
		memTeamRepo := memoryrepo.NewTeamRepository()
		memTeamMemberRepo := memoryrepo.NewTeamMemberRepository()
		memTaskRepo := memoryrepo.NewTaskRepository()
		memTaskCommentRepo := memoryrepo.NewTaskCommentRepository()
		memTaskHistoryRepo := memoryrepo.NewTaskHistoryRepository()
		memReportRepo := memoryrepo.NewReportRepository(memTeamRepo, memTeamMemberRepo, memTaskRepo)

		userRepo = memUserRepo
		refreshRepo = memRefreshRepo
		teamRepo = memTeamRepo
		teamMemberRepo = memTeamMemberRepo
		taskRepo = memTaskRepo
		taskCommentRepo = memTaskCommentRepo
		taskHistoryRepo = memTaskHistoryRepo
		reportRepo = memReportRepo
		healthPingers = []health.DependencyPinger{readiness.NewNoopPinger("http-server")}
	}

	taskListCache = cacheinfra.NewNoopTaskListCache()
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})

		redisPingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := redisClient.Ping(redisPingCtx).Err(); err != nil {
			if db != nil {
				_ = db.Close()
			}
			_ = redisClient.Close()
			return nil, fmt.Errorf("ping redis: %w", err)
		}

		taskListCache = cacheinfra.NewRedisTaskListCache(redisClient)
		healthPingers = append(healthPingers, readiness.NewRedisPinger(redisClient))
	}

	authService := auth.NewService(userRepo, refreshRepo, jwt, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, cfg.BcryptCost)
	teamsService := teams.NewService(teamRepo, teamMemberRepo, userRepo, taskListCache)
	tasksService := tasks.NewService(taskRepo, teamMemberRepo, taskCommentRepo, taskHistoryRepo, taskListCache, 5*time.Minute)
	reportsService := reports.NewService(reportRepo)
	healthService := health.NewService(healthPingers)

	healthHandler := handler.NewHealthHandler(healthService)
	authHandler := handler.NewAuthHandler(authService)
	teamsHandler := handler.NewTeamsHandler(teamsService)
	tasksHandler := handler.NewTasksHandler(tasksService)
	reportsHandler := handler.NewReportsHandler(reportsService)
	authMiddleware := middleware.NewAuth(jwt)
	var rlStore ratelimit.Store = ratelimit.NewInMemoryStore()
	if redisClient != nil {
		rlStore = ratelimit.NewRedisStore(redisClient)
	}
	rateLimiter := middleware.NewRateLimit(rlStore, 100, time.Minute)
	httpMetrics := metricsinfra.NewHTTPMetrics(prometheus.DefaultRegisterer)
	router := httptransport.NewRouter(
		healthHandler,
		authHandler,
		teamsHandler,
		tasksHandler,
		reportsHandler,
		httpMetrics.Middleware,
		rateLimiter,
		authMiddleware,
	)

	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &App{
		cfg:    cfg,
		server: server,
		db:     db,
		redis:  redisClient,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("http server started on %s", a.server.Addr)
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		if a.db != nil {
			if err := a.db.Close(); err != nil {
				return fmt.Errorf("close db: %w", err)
			}
		}
		if a.redis != nil {
			if err := a.redis.Close(); err != nil {
				return fmt.Errorf("close redis: %w", err)
			}
		}

		return nil
	case err := <-errCh:
		return err
	}
}

func hasSQLDriver(name string) bool {
	for _, driver := range sql.Drivers() {
		if driver == name {
			return true
		}
	}

	return false
}
