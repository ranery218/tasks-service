GOOSE ?= goose
MIGRATIONS_DIR ?= ./migrations
MYSQL_DSN ?= root:root@tcp(localhost:3306)/tasks_service?parseTime=true&multiStatements=true

.PHONY: run test test-usecase test-business-cover test-integration build up down logs migrate-up migrate-down migrate-status

run:
	go run ./cmd/tasks-service

test:
	GOCACHE="$(PWD)/.gocache" go test ./...

test-usecase:
	GOCACHE="$(PWD)/.gocache" go test ./internal/usecase/...

test-business-cover:
	GOCACHE="$(PWD)/.gocache" go test ./internal/usecase/... -coverprofile=coverage_usecase.out
	GOCACHE="$(PWD)/.gocache" go tool cover -func=coverage_usecase.out

test-integration:
	MYSQL_DSN_TEST="$(MYSQL_DSN)" GOCACHE="$(PWD)/.gocache" go test -tags=integration ./internal/integration/...

build:
	go build -o bin/tasks-service ./cmd/tasks-service

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f app

migrate-up:
	$(GOOSE) -dir $(MIGRATIONS_DIR) mysql "$(MYSQL_DSN)" up

migrate-down:
	$(GOOSE) -dir $(MIGRATIONS_DIR) mysql "$(MYSQL_DSN)" down

migrate-status:
	$(GOOSE) -dir $(MIGRATIONS_DIR) mysql "$(MYSQL_DSN)" status
