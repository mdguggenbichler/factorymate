# Development guide

## Prerequisites

- Go 1.22+ (toolchain may pin newer via `go.mod` once M0 lands)
- Node.js 22+
- npm

## Environment

Copy `.env.example` to `.env` at the repo root (or export vars in your shell).

| Variable | Purpose |
| --- | --- |
| `SESSION_SECRET` | **Required** for backend — cookie signing |
| `DATABASE_PATH` | SQLite file path (default `./data/factorymate.db` for local dev) |
| `FRM_HOST` / `FRM_PORT` | Initial FRM endpoint; seeded into `app_settings` on first boot. Group live server: `192.168.178.42:8889` |
| `FRM_TEST_HOST` / `FRM_TEST_PORT` | Live FRM for integration tests and fixture capture (read-only) — defaults in `.env.example` |
| `NEXT_PUBLIC_API_URL` | Frontend dev: direct backend URL (`http://localhost:8080`) |
| `BACKEND_URL` | Next.js rewrite target (`http://localhost:8080` local, `http://backend:8080` in compose) |

## Running locally

### Backend (after M0)

```bash
cd backend
go run ./cmd/server
# listens on :8080 — GET /healthz → {"status":"ok"}
```

### Frontend

```bash
cd frontend
npm install
npm run dev
# http://localhost:3000
```

Production-style API proxy: omit `NEXT_PUBLIC_API_URL` and rely on Next.js rewrites (`/api/*` → backend). Local dev typically uses `NEXT_PUBLIC_API_URL` for direct calls.

### Docker Compose (skeleton at M0; finalized M13)

```bash
docker compose up --build
```

## Project conventions

- **Spec wins** on product conflicts; roadmap sequences work only
- **Migrations:** numbered `.sql` files only — no hand-written DDL outside `internal/db/migrations/`
- **Frontend i18n:** all user-facing strings in `frontend/messages/en.json` (spec §8.2)
- **shadcn:** install via MCP/CLI; edit components in-place under `frontend/components/ui/`

## Orchestrator development

See [`AGENTS.md`](../AGENTS.md) and [`.agents/project/orchestrator/milestone-scopes.md`](../.agents/project/orchestrator/milestone-scopes.md).
