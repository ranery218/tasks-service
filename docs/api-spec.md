# API Specification (v1)

Base path: `/api/v1`

Auth:
- Access token: `Authorization: Bearer <access_token>`
- Refresh token: `POST /token/refresh`
- Refresh token format: `opaque` random string (not JWT)

Rate limit:
- `/api/v1/*`: `100 req/min`
- key: `user_id` (если авторизован), иначе `client ip`
- exceeded response: `429 Too Many Requests`

Common error response:

```json
{
  "error": {
    "code": "forbidden",
    "message": "insufficient permissions"
  }
}
```

## 1) POST /register
Создание пользователя.

Request:
```json
{
  "email": "user@example.com",
  "password": "StrongPass123",
  "name": "Ivan"
}
```

Responses:
- `201 Created`
```json
{
  "id": 1,
  "email": "user@example.com",
  "name": "Ivan",
  "created_at": "2026-03-05T10:00:00Z"
}
```
- `400 Bad Request` (validation)
- `409 Conflict` (email exists)

## 2) POST /login
Логин и выдача access/refresh.

Request:
```json
{
  "email": "user@example.com",
  "password": "StrongPass123"
}
```

Responses:
- `200 OK`
```json
{
  "access_token": "jwt-access",
  "refresh_token": "opaque-refresh-token",
  "token_type": "Bearer",
  "expires_in": 900
}
```
- `401 Unauthorized`

## 3) POST /token/refresh
Обновление access по refresh.

Request:
```json
{
  "refresh_token": "opaque-refresh-token"
}
```

Responses:
- `200 OK` (новая пара токенов)
- `401 Unauthorized`

## 3.1) POST /logout
Выход: отзыв refresh-токена.

Request:
```json
{
  "refresh_token": "opaque-refresh-token"
}
```

Responses:
- `204 No Content`
- `401 Unauthorized`

## 4) POST /teams
Создать команду, создатель становится `owner`.

Request:
```json
{
  "name": "Platform Team",
  "description": "Core backend team"
}
```

Responses:
- `201 Created`
```json
{
  "id": 10,
  "name": "Platform Team",
  "description": "Core backend team",
  "created_by": 1,
  "created_at": "2026-03-05T10:00:00Z"
}
```
- `401 Unauthorized`

## 5) GET /teams
Список команд текущего пользователя.

Responses:
- `200 OK`
```json
{
  "items": [
    {
      "id": 10,
      "name": "Platform Team",
      "role": "owner"
    }
  ]
}
```

## 6) DELETE /teams/{id}
Удаление команды.

Rules:
- `owner`: только свою команду
- `admin`: любую команду

Responses:
- `204 No Content`
- `403 Forbidden`
- `404 Not Found`

## 7) POST /teams/{id}/invite
Упрощенный invite: добавить участника по `user_id`.

Request:
```json
{
  "user_id": 5
}
```

Rules:
- `owner`: только своей команды
- `admin`: любой команды

Responses:
- `200 OK`
```json
{
  "team_id": 10,
  "user_id": 5,
  "role": "member"
}
```
- `409 Conflict` (already member)

## 8) DELETE /teams/{id}/members/{user_id}
Удаление участника из команды.

Rules:
- `owner`: только в своей команде
- `admin`: в любой

Responses:
- `204 No Content`
- `403 Forbidden`
- `404 Not Found`

## 9) POST /tasks
Создать задачу (только участник команды).

Request:
```json
{
  "team_id": 10,
  "title": "Implement JWT refresh",
  "description": "Add refresh flow",
  "assignee_id": 5,
  "status": "todo",
  "due_date": "2026-03-20T00:00:00Z"
}
```

Responses:
- `201 Created`
```json
{
  "id": 100,
  "team_id": 10,
  "title": "Implement JWT refresh",
  "description": "Add refresh flow",
  "assignee_id": 5,
  "created_by": 1,
  "status": "todo",
  "created_at": "2026-03-05T10:00:00Z"
}
```
- `403 Forbidden` (not member)
- `409 Conflict` (assignee is not a team member)

## 10) GET /tasks
Фильтрация + пагинация (`LIMIT/OFFSET`).

Query params:
- `team_id` (optional)
- `status` (`todo|in_progress|done`)
- `assignee_id` (optional)
- `limit` (default 20, max 100)
- `offset` (default 0)

Responses:
- `200 OK`
```json
{
  "items": [
    {
      "id": 100,
      "team_id": 10,
      "title": "Implement JWT refresh",
      "status": "todo",
      "assignee_id": 5,
      "created_by": 1,
      "created_at": "2026-03-05T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 20,
    "offset": 0,
    "total": 1
  }
}
```

## 11) PUT /tasks/{id}
Обновить задачу (только участник команды).

Request:
```json
{
  "title": "Implement JWT refresh v2",
  "description": "Add rotation",
  "assignee_id": 7,
  "status": "in_progress",
  "due_date": "2026-03-25T00:00:00Z"
}
```

Responses:
- `200 OK`
- `403 Forbidden`
- `404 Not Found`

Notes:
- Каждый измененный атрибут фиксируется в `task_history`.

## 12) POST /tasks/{id}/comments
Комментарий к задаче (только участник команды).

Request:
```json
{
  "text": "Started implementation"
}
```

Responses:
- `201 Created`
```json
{
  "id": 700,
  "task_id": 100,
  "user_id": 5,
  "text": "Started implementation",
  "created_at": "2026-03-05T10:00:00Z"
}
```

## 13) GET /tasks/{id}/history
История изменений задачи (читать может любой авторизованный).

Responses:
- `200 OK`
```json
{
  "items": [
    {
      "id": 900,
      "task_id": 100,
      "field": "status",
      "old_value": "todo",
      "new_value": "in_progress",
      "changed_by": 5,
      "created_at": "2026-03-05T11:00:00Z"
    }
  ]
}
```

## 14) GET /reports/team-stats
JOIN 3+ tables + aggregation.

Goal:
- Для каждой команды: название, число участников, число задач `done` за последние 7 дней.

Response `200 OK`:
```json
{
  "items": [
    {
      "team_id": 10,
      "team_name": "Platform Team",
      "members_count": 6,
      "done_tasks_last_7_days": 14
    }
  ]
}
```

## 15) GET /reports/top-creators
Window function query.

Goal:
- Топ-3 пользователей по числу созданных задач в каждой команде за месяц.

Response `200 OK`:
```json
{
  "items": [
    {
      "team_id": 10,
      "user_id": 1,
      "tasks_created": 20,
      "rank": 1
    }
  ]
}
```

## 16) GET /reports/invalid-assignees
Связанный запрос валидации целостности.

Goal:
- Найти задачи, где `assignee_id` не состоит в команде задачи.

Response `200 OK`:
```json
{
  "items": [
    {
      "task_id": 222,
      "team_id": 10,
      "assignee_id": 99
    }
  ]
}
```

## Additional behavior
- Rate limiting: `100 requests/min/user`.
- Redis cache: list tasks by `team_id` with TTL `5m`.
- Metrics: request count, error count, latency.
- Graceful shutdown on SIGTERM/SIGINT.
