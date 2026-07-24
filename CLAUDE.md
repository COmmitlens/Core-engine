# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CommitLens Core Engine — a Go backend that connects to GitHub Apps/orgs, ingests commits and diffs,
generates embeddings via Azure OpenAI (through a separate internal AI backend), and powers AI-assisted
workspace queries. It also integrates Slack (bot + slash-style events) and has a built-in real-time
direct-messaging feature over WebSockets.

This repo (`Core-engine`) is one of three services in the CommitLens stack (see `docker-compose.yml`):
- `app` — this Go backend (port 8000)
- `ai` — a separate Python FastAPI service (`core-ai`, port 8001) that does the actual LLM calls;
  this repo talks to it over HTTP via `config.GetConfig().AiBackendUrl` (see `domain/ai.go`)
- `frontend` — Next.js app (`twin-ai-fe`, port 3000)

**Module name:** `core` — all internal imports use `core/...`.
**Stack:** Go 1.25 · Echo v3 · GORM (PostgreSQL + pgvector) · Redis (Asynq) · Azure OpenAI · Firebase auth (alongside JWT).

## Build, Run, Lint

```bash
# Install/sync dependencies
go mod tidy

# Run the server locally (reads .env, requires Postgres + Redis reachable)
go run main.go

# Build a binary
go build -o server main.go

# Vet / format check
go vet ./...
gofmt -l .
```

There are no automated tests in this repo (`*_test.go` files do not currently exist) and no dedicated
lint config — `go vet` and `gofmt` are the available checks. `go build ./...` is the fastest way to
confirm the whole module compiles after a change.

Local dependencies (Postgres w/ pgvector, Redis) can be brought up via `docker-compose.yml`, or pointed
at existing instances via `.env` (copy `.env.example`). The server listens on `:8000` (hardcoded in
`main.go`; `PORT` config value is not currently wired to `e.Start`).

## Architecture

Strict 4-layer dependency flow — **never skip layers, and never call a layer out of order**:

```
domain/   → raw DB queries (GORM) / outbound HTTP to the AI backend — no business logic
service/  → business logic, orchestrates one or more domain calls
handler/  → HTTP bind + validate + respond only, delegates to exactly one service
route/    → dependency injection (route/app.go) + route registration (route/v1.go)
```

- All layers are interface + `*Ctx` struct pairs, e.g. `domain.UserDomain` / `domain.UserDomainCtx`,
  wired together as plain struct fields (no DI framework). `route/app.go` is the single place every
  domain, service, and handler gets constructed and wired — read it first to see how any feature's
  dependencies fit together.
- Routes are registered in `route/v1.go` under `/v1` (not `/api/v1` — nginx in `deploy/nginx.conf`
  proxies `/` straight through with no path rewrite).
- **Adding a new feature** typically means: add/extend a domain file, add/extend a service file,
  add/extend a handler file, wire the new struct(s) into `AppModel`/`App()` in `route/app.go`, then
  register the route in `v1Routes()` in `route/v1.go`.
- `middleware.JWTVerify()` is applied per route-group (see `route/v1.go`) — not globally. Check
  neighboring routes in the same group to see whether a new endpoint should be protected.
- Two Redis-backed rate limiters are applied in `v1Routes()`: a tighter one (`authLimiter`, 50 req/60s)
  scoped to auth endpoints, and a looser one (100 req/60s) applied to the whole `/v1` group.

### Conventions

**Responses** always use `models.BasicResp{Message, Data}` (or `models.BasicRespWithMeta` when paginated):

```go
return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success, Data: data})
return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
```

**Validation** lives in `handler/validation/` and is called at the top of the handler, before invoking
the service:

```go
if err = validation.RegisterUser(param); err != nil {
    return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
}
```

**GORM:** `SingularTable: true` is set in `config/database.go` — table names are singular (`user`, not
`users`). Every new persisted model must be added to the `db.AutoMigrate(...)` list in
`config/database.go` or it will never get a table.

**Config:** all env vars are loaded through `config.GetConfig()` (`config/config.go`, backed by
`gonfig` + `godotenv`). Never read `os.Getenv` directly in domain/service/handler code — add a new
field to `Configuration` instead.

**Errors:** sentinel errors live in `utils/constant.go` (e.g. `utils.ErrEmptyEmail`,
`utils.ErrUserTokenNotExist`) and `utils.Success` is the canonical success message string — reuse
these rather than inlining new string literals for common cases.

**GitHub webhook signature verification** is in `handler/connect_org.go` (HMAC-SHA256 over
`X-Hub-Signature-256`, using `config.GithubWebhookSecret`) — not in `config/webhook.go`, which is
currently an empty stub. Slack request signing is verified similarly in `handler/slack.go`.

### Background jobs (Asynq / Redis)

Three priority queues: `webhooks` (6), `default` (3), `embeddings` (1). The asynq worker server is
started inline inside `route/app.go#App()` (not a separate binary/process).

- Task type constants + payload structs: `queue/tasks.go`
- Handler registration (`asynq.ServeMux`): `route/task_handlers.go` — one `handleXxx(svc)` closure per
  task type, unmarshals the JSON payload then calls into `service.ConnectOrgService`
- Enqueuing: look at `service/connect_org.go` / `service/github_repository.go` for `QueueClient.Enqueue(...)` call sites
- Adding a new async task: define the type constant + payload struct in `queue/tasks.go`, add a handler
  function + `mux.HandleFunc` registration in `route/task_handlers.go`, and enqueue it from whichever
  service owns the triggering logic

### Real-time (WebSockets)

`ws/hub.go` implements a single process-lifetime hub (`ws.NewHub()`, instantiated once in
`route/app.go`) that maps `conversationID → set of *Client` so direct messages are only pushed to the
two participants of a conversation. `handler/dm.go` exposes `GET /v1/dm/ws?conversation_id=1` — the JWT
is passed as `?token=` because browsers can't set `Authorization` headers on WebSocket upgrades.

### AI / embeddings

- `domain/ai.go` is the client for the separate AI backend (`AiBackendUrl`): intent classification
  (`ClassifyQueryIntent`), chat completion (`CallAzureChatCompletion`), and workspace queries — the LLM
  calls themselves happen in that other service, not in this repo.
- Commit file diffs are embedded (via the AI backend) and stored as pgvector columns
  (`models.CommitFileEmbedding`) for semantic search over commit history; see the `embeddings` queue
  and `service/github_repository.go` / `service/connect_org.go` for the embed-then-store flow.

## Repo layout notes

- `deploy/` — on-premise deployment assets: `init.sql` (pgvector extension bootstrap), `nginx.conf`
  (TLS-terminating reverse proxy in front of `app:8000`), `workflows/`
- The root also has several standalone markdown guides (`FRONTEND_AUTH_GUIDE.md`,
  `NEXTJS_AUTH_IMPLEMENTATION.md`, `ONPREMISE_DEPLOY.md`, `PRODUCTION_SECURITY_GUIDE.md`,
  `QUICK_START.md`) aimed at consumers of this API (frontend integrators / on-prem operators) rather
  than at contributors to this codebase — consult them when a task specifically concerns auth-cookie
  behavior or on-premise deployment, but they are not architecture references.
- CI (`.github/workflows/release.yml`) builds and pushes a multi-arch Docker image to
  `ghcr.io/<owner>/commitlens-core` on version tags (`v*.*.*`) and on every push to `dev` (tagged `:dev`).
