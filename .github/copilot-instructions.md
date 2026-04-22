# CommitLens Core Engine — Copilot Instructions

## Project Overview

Go backend for CommitLens — a GitHub integration platform that fetches commits, generates embeddings via Azure OpenAI, and enables AI-powered workspace queries.

**Stack:** Go 1.25 · Echo v3 · GORM (PostgreSQL + pgvector) · Redis (Asynq) · Azure OpenAI · Firebase

**Module name:** `core` (all internal imports use `core/...`)

---

## Architecture

Strict 4-layer dependency flow — never skip layers:

```
domain/   → raw DB queries (GORM) — no business logic
service/  → business logic, orchestrates domain calls
handler/  → HTTP bind/validate/respond only, calls one service
route/    → dependency injection + route registration
```

- Dependency injection happens in [route/app.go](../route/app.go); all structs are wired there
- Routes registered in [route/v1.go](../route/v1.go) under `/api/v1/`
- Add a new feature: create files in all 4 layers, wire in `App()`, register in `v1Routes()`

## Code Conventions

**Responses** always use `models.BasicResp{Message, Data}` or `models.BasicRespWithMeta` for paginated results:

```go
return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success, Data: data})
return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
```

**Validation** lives in `handler/validation/` — call it before invoking the service:

```go
if err = validation.RegisterUser(param); err != nil {
    return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
}
```

**JWT middleware** applied per-group: `middleware.JWTVerify()` — see [route/v1.go](../route/v1.go) for which groups are protected.

**GORM:** `SingularTable: true` — table names are singular (e.g. `user`, not `users`). Add new models to `db.AutoMigrate(...)` in [config/database.go](../config/database.go).

**Queue (Asynq/Redis):** Three priority queues — `webhooks` (6), `default` (3), `embeddings` (1). Task types defined in [queue/tasks.go](../queue/tasks.go), handlers registered in [route/task_handlers.go](../route/task_handlers.go).

**Config:** All env vars loaded via `config.GetConfig()` from [config/config.go](../config/config.go). Never read `os.Getenv` directly.

## Build and Run

```bash
# Install dependencies
go mod tidy

# Run server (port 8000)
go run main.go

# Build binary
go build -o server main.go
```

## Integration Points

| Component             | Details                                                                               |
| --------------------- | ------------------------------------------------------------------------------------- |
| PostgreSQL + pgvector | Commit file embeddings stored as vectors; used for semantic search                    |
| Azure OpenAI          | Embeddings + chat completions via `domain/ai.go`                                      |
| GitHub App            | OAuth, webhooks, installation tokens — `connect_org` and `github_repository` handlers |
| Asynq / Redis         | Async processing: webhook events, repo fetching, embedding backfill                   |
| Firebase              | Auth token validation (alongside JWT)                                                 |
| Email (gomail)        | Templates in `template/` — register, reset_password, invite                           |

## Security Notes

- JWT secret and GitHub App private key are env vars — never hardcode
- GitHub webhook signature verified in `config/webhook.go`
- OAuth state tokens generated with `crypto/rand` — see [handler/auth.go](../handler/auth.go)
