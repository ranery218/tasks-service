//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"tasks-service/internal/domain/entities"
	mysqlrepo "tasks-service/internal/infrastructure/repository/mysql"
	jwtservice "tasks-service/internal/infrastructure/jwt"
	"tasks-service/internal/usecase/auth"
	"tasks-service/internal/usecase/reports"
	"tasks-service/internal/usecase/tasks"
	"tasks-service/internal/usecase/teams"
)

func TestMySQLFlow_EndToEndBusinessScenario(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN_TEST")
	if dsn == "" {
		dsn = "root:root@tcp(localhost:3306)/tasks_service?parseTime=true&multiStatements=true"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping mysql: %v", err)
	}

	if err := cleanupDatabase(ctx, db); err != nil {
		t.Fatalf("cleanup database: %v", err)
	}

	userRepo := mysqlrepo.NewUserRepository(db)
	refreshRepo := mysqlrepo.NewRefreshTokenRepository(db)
	teamRepo := mysqlrepo.NewTeamRepository(db)
	memberRepo := mysqlrepo.NewTeamMemberRepository(db)
	taskRepo := mysqlrepo.NewTaskRepository(db)
	commentRepo := mysqlrepo.NewTaskCommentRepository(db)
	historyRepo := mysqlrepo.NewTaskHistoryRepository(db)
	reportRepo := mysqlrepo.NewReportRepository(db)

	jwt := jwtservice.NewService("integration-secret")
	authSvc := auth.NewService(userRepo, refreshRepo, jwt, time.Hour, 30*24*time.Hour, 4)
	teamsSvc := teams.NewService(teamRepo, memberRepo, userRepo, nil)
	tasksSvc := tasks.NewService(taskRepo, memberRepo, commentRepo, historyRepo, nil, 5*time.Minute)
	reportsSvc := reports.NewService(reportRepo)

	owner, err := authSvc.Register(context.Background(), auth.RegisterInput{
		Email:    "owner@example.com",
		Password: "owner-pass",
		Name:     "Owner",
	})
	if err != nil {
		t.Fatalf("register owner: %v", err)
	}
	member, err := authSvc.Register(context.Background(), auth.RegisterInput{
		Email:    "member@example.com",
		Password: "member-pass",
		Name:     "Member",
	})
	if err != nil {
		t.Fatalf("register member: %v", err)
	}
	admin, err := authSvc.Register(context.Background(), auth.RegisterInput{
		Email:    "admin@example.com",
		Password: "admin-pass",
		Name:     "Admin",
	})
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	outsider, err := authSvc.Register(context.Background(), auth.RegisterInput{
		Email:    "outsider@example.com",
		Password: "outsider-pass",
		Name:     "Outsider",
	})
	if err != nil {
		t.Fatalf("register outsider: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), "UPDATE users SET is_admin = 1 WHERE id = ?", admin.ID); err != nil {
		t.Fatalf("set admin flag: %v", err)
	}

	ownerActor := entities.Actor{UserID: owner.ID}
	memberActor := entities.Actor{UserID: member.ID}
	adminActor := entities.Actor{UserID: admin.ID, IsAdmin: true}
	outsiderActor := entities.Actor{UserID: outsider.ID}

	team, err := teamsSvc.CreateTeam(context.Background(), ownerActor, teams.CreateTeamInput{
		Name:        "Platform",
		Description: "Backend team",
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	if err := teamsSvc.InviteMember(context.Background(), ownerActor, teams.InviteInput{TeamID: team.ID, UserID: member.ID}); err != nil {
		t.Fatalf("owner invite member: %v", err)
	}

	if err := teamsSvc.InviteMember(context.Background(), adminActor, teams.InviteInput{TeamID: team.ID, UserID: outsider.ID}); err != nil {
		t.Fatalf("admin invite outsider: %v", err)
	}

	dueDate := time.Now().UTC().Add(48 * time.Hour)
	createdTask, err := tasksSvc.CreateTask(context.Background(), memberActor, tasks.CreateTaskInput{
		TeamID:      team.ID,
		Title:       "Implement endpoint",
		Description: "Implement teams endpoint",
		Status:      entities.TaskStatusTodo,
		AssigneeID:  &member.ID,
		DueDate:     &dueDate,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	updatedStatus := entities.TaskStatusInProgress
	updatedTitle := "Implement endpoint v2"
	if _, err := tasksSvc.UpdateTask(context.Background(), memberActor, createdTask.ID, tasks.UpdateTaskInput{
		Status: &updatedStatus,
		Title:  &updatedTitle,
	}); err != nil {
		t.Fatalf("update task: %v", err)
	}

	comment, err := tasksSvc.AddComment(context.Background(), outsiderActor, createdTask.ID, "Looks good")
	if err != nil {
		t.Fatalf("add comment by team member: %v", err)
	}
	if comment.ID == 0 {
		t.Fatalf("expected persisted comment id")
	}

	list, err := tasksSvc.ListTasks(context.Background(), ownerActor, tasks.ListInput{
		TeamID: &team.ID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("unexpected list: total=%d items=%d", list.Total, len(list.Items))
	}

	history, err := tasksSvc.GetHistory(context.Background(), ownerActor, createdTask.ID)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(history) == 0 {
		t.Fatalf("expected history entries after update")
	}

	stats, err := reportsSvc.TeamStats(context.Background(), ownerActor)
	if err != nil {
		t.Fatalf("team stats: %v", err)
	}
	if len(stats) == 0 {
		t.Fatalf("expected non-empty team stats")
	}

	topCreators, err := reportsSvc.TopCreatorsByTeam(context.Background(), ownerActor)
	if err != nil {
		t.Fatalf("top creators: %v", err)
	}
	if len(topCreators) == 0 {
		t.Fatalf("expected non-empty top creators")
	}

	if err := teamsSvc.RemoveMember(context.Background(), adminActor, teams.RemoveMemberInput{
		TeamID: team.ID,
		UserID: outsider.ID,
	}); err != nil {
		t.Fatalf("admin remove member: %v", err)
	}

	if err := teamsSvc.DeleteTeam(context.Background(), ownerActor, team.ID); err != nil {
		t.Fatalf("owner delete team: %v", err)
	}

	login, err := authSvc.Login(context.Background(), auth.LoginInput{Email: owner.Email, Password: "owner-pass"})
	if err != nil {
		t.Fatalf("login owner: %v", err)
	}
	if login.AccessToken == "" || login.RefreshToken == "" {
		t.Fatalf("expected access and refresh tokens")
	}

	refreshed, err := authSvc.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh token: %v", err)
	}
	if refreshed.RefreshToken == "" || refreshed.RefreshToken == login.RefreshToken {
		t.Fatalf("expected rotated opaque refresh token")
	}

	if err := authSvc.Logout(context.Background(), refreshed.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
}

func cleanupDatabase(ctx context.Context, db *sql.DB) error {
	queries := []string{
		"SET FOREIGN_KEY_CHECKS = 0",
		"TRUNCATE TABLE task_comments",
		"TRUNCATE TABLE task_history",
		"TRUNCATE TABLE tasks",
		"TRUNCATE TABLE team_members",
		"TRUNCATE TABLE teams",
		"TRUNCATE TABLE refresh_tokens",
		"TRUNCATE TABLE users",
		"SET FOREIGN_KEY_CHECKS = 1",
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("%s: %w", query, err)
		}
	}

	return nil
}
