# CLAUDE.md

Be carefull while writing code, CODEX and RABITMQ review the code
This workspace contains the Book Finder backend.

## Project Location

All source code lives in [Book-finder-backend/](Book-finder-backend/). See [Book-finder-backend/CLAUDE.md](Book-finder-backend/CLAUDE.md) for full project guidance.

## Quick Reference

**Stack:** Go 1.25, Echo, GORM, PostgreSQL (pgvector), Redis (Asynq), Azure OpenAI

**Layer structure:**
```
domain/   → database queries (GORM)
service/  → business logic
handler/  → HTTP request/response
route/    → dependency injection and route registration
```

**Run the server:**
```bash
cd Book-finder-backend
go run main.go
```
