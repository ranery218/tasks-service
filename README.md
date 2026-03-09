# tasks-service

Сервис управления задачами команд на Go.

Проект реализует:
- JWT access + opaque refresh с ротацией;
- команды и участников с ролями `admin / owner / member`;
- задачи, комментарии, историю изменений, пагинацию `LIMIT/OFFSET`;
- отчеты (сложные SQL-запросы, включая window function);
- health/readiness probes, metrics Prometheus, rate limiting, Redis cache.

## 1. Соответствие ТЗ

Реализовано:
- все API-методы из спецификации: см. [docs/api-spec.md](docs/api-spec.md);
- роли и права:
- `admin`: все, что owner, плюс удаление любых команд;
- `owner`: управление только своей командой, инвайт/удаление участников своей команды;
- `member`: создание задач в командах, где состоит, и создание своей команды (становится owner);
- статусы задач: `todo | in_progress | done`;
- чтение задач/истории: любой авторизованный пользователь;
- редактирование задач/комментирование: только участники команды;
- упрощенный invite: `team_id` в path, `user_id` в body;
- пагинация: `limit/offset`;
- тесты бизнес-логики (`internal/usecase`) > 85% покрытие.

Текущее покрытие бизнес-логики:
- `make test-business-cover`
- итог: `87.3%` (usecase-пакеты).

## 2. Технологии

- Go 1.24
- HTTP router: `chi`
- MySQL 8.4
- Redis 7
- goose (миграции)
- Prometheus
- Docker / Docker Compose

## 3. Архитектура

Структура слоев:
- `internal/domain/entities` — доменные сущности;
- `internal/domain/repository` — интерфейсы репозиториев;
- `internal/usecase` — бизнес-логика;
- `internal/infrastructure/repository` — реализации репозиториев (MySQL/InMemory/readiness);
- `internal/infrastructure/*` — инфраструктурные сервисы (jwt/password/cache/metrics/ratelimit);
- `internal/transport/http` — router/handlers/middleware;
- `internal/app` — composition root (wiring зависимостей).

Режимы запуска:
- без `MYSQL_DSN` -> InMemory репозитории;
- с `MYSQL_DSN` -> MySQL репозитории;
- с `REDIS_ADDR` -> Redis cache (`GET /tasks` по `team_id`) и Redis-backed rate limit store.

## 4. Быстрый старт (рекомендуется)

1. Поднять инфраструктуру и приложение:

```bash
make up
```

2. Применить миграции:

```bash
make migrate-up
```

3. Проверить готовность:

```bash
curl -sS http://localhost:8080/healthz
curl -sS http://localhost:8080/readyz
```

Сервисы:
- app: `http://localhost:8080`
- mysql: `localhost:3306`
- redis: `localhost:6379`
- prometheus: `http://localhost:9090`

## 5. Доступ к MySQL (в т.ч. для DataGrip)

Параметры из `docker-compose.yml`:
- host: `localhost`
- port: `3306`
- user: `root`
- password: `root`
- database: `tasks_service`

DSN:

```text
root:root@tcp(localhost:3306)/tasks_service?parseTime=true&multiStatements=true
```

## 6. Переменные окружения

- `HTTP_ADDR` (default `:8080`)
- `SHUTDOWN_TIMEOUT_SEC` (default `10`)
- `MYSQL_DSN` (optional)
- `REDIS_ADDR` (optional)
- `REDIS_PASSWORD` (optional)
- `REDIS_DB` (default `0`)
- `JWT_ACCESS_SECRET` (default `dev_access_secret`)
- `JWT_ACCESS_TTL_SEC` (default `3600`)
- `JWT_REFRESH_TTL_SEC` (default `2592000`)
- `BCRYPT_COST` (default `bcrypt.DefaultCost`)

## 7. Миграции

Нужен установленный `goose` CLI.

```bash
make migrate-up
make migrate-status
make migrate-down
```

Файл миграции:
- [migrations/00001_init_schema.sql](migrations/00001_init_schema.sql)

## 8. Запуск без Docker

Если нужна локальная отладка процесса приложения:

```bash
export MYSQL_DSN='root:root@tcp(localhost:3306)/tasks_service?parseTime=true&multiStatements=true'
export REDIS_ADDR='localhost:6379'
go run ./cmd/tasks-service
```

## 9. API

Полная спецификация с request/response:
- [docs/api-spec.md](docs/api-spec.md)

Базовый префикс:
- `/api/v1`

Основные группы:
- auth: register/login/refresh/logout
- teams: create/list/delete/invite/remove member
- tasks: create/list/update/comments/history
- reports: team-stats/top-creators/invalid-assignees

## 10. Наблюдаемость

Health:
- `GET /healthz` — liveness
- `GET /readyz` — readiness зависимостей (MySQL/Redis при включении)

Metrics:
- `GET /metrics`
- ключевые метрики:
- `http_requests_total`
- `http_errors_total`
- `http_request_duration_seconds`

Prometheus:
- config: [deployments/prometheus/prometheus.yml](deployments/prometheus/prometheus.yml)
- scrape target: `app:8080`

Полезные PromQL:

```promql
sum(rate(http_requests_total[5m]))
sum by (path, status) (rate(http_requests_total[5m]))
sum(rate(http_errors_total[5m]))
histogram_quantile(0.95, sum by (le, path) (rate(http_request_duration_seconds_bucket[5m])))
```

## 11. Производительность и устойчивость

Rate limiting:
- `100 req/min` на `/api/v1/*`;
- ключ: `user_id` (если авторизован), иначе IP;
- при превышении: `429 Too Many Requests`.

Кэш задач:
- кешируется `GET /api/v1/tasks` при наличии `team_id`;
- backend: Redis (если включен), иначе noop;
- TTL: `5m`;
- инвалидируется при изменениях задач/состава команды/удалении команды.

Shutdown:
- graceful shutdown по `SIGINT/SIGTERM`.

## 12. Тесты

Все тесты:

```bash
make test
```

Только бизнес-логика:

```bash
make test-usecase
make test-business-cover
```

Интеграционные тесты (реальная MySQL, build tag `integration`):

```bash
make test-integration
```

Интеграционный тест:
- [internal/integration/mysql_flow_test.go](internal/integration/mysql_flow_test.go)

Важно: перед `make test-integration` должна быть поднята MySQL и применены миграции.

## 13. Проверка вручную (минимальный smoke flow)

1. `POST /api/v1/register` (owner/member/admin/outsider)
2. `POST /api/v1/login` -> получить `access_token`
3. `POST /api/v1/teams` (owner создает команду)
4. `POST /api/v1/teams/{team_id}/invite` (добавить member)
5. `POST /api/v1/tasks` (создать задачу)
6. `PUT /api/v1/tasks/{task_id}` (сменить status/title)
7. `POST /api/v1/tasks/{task_id}/comments`
8. `GET /api/v1/tasks/{task_id}/history`
9. `GET /api/v1/reports/team-stats`
10. `POST /api/v1/token/refresh`
11. `POST /api/v1/logout`

## 14. Команды Makefile

- `make run` — запуск приложения
- `make build` — сборка бинарника
- `make up` — поднять docker-compose
- `make down` — остановить docker-compose
- `make logs` — логи app
- `make migrate-up|migrate-down|migrate-status`
- `make test|test-usecase|test-business-cover|test-integration`


