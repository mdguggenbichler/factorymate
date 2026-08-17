# Development guide

## Prerequisites

- Go 1.22+ (toolchain may pin newer via `go.mod`)
- Node.js 22+
- npm
- Docker + Docker Compose (for M13 stack)

## Environment

Copy `.env.example` to `.env` at the repo root (or export vars in your shell).

| Variable | Purpose |
| --- | --- |
| `SESSION_SECRET` | **Required in production** for backend cookie signing — set a long random value on GuggiRaid; compose defaults to `dev-secret-change-me` for local builds |
| `DATABASE_PATH` | SQLite file path (default `./data/factorymate.db` for local dev; `/data/factorymate.db` in compose) |
| `FRM_HOST` / `FRM_PORT` | Initial FRM endpoint; seeded into `app_settings` on first boot. Group live server: `192.168.178.42:8889` (host) or `satisfactory-server:8080` (Docker network) |
| `FRM_TEST_HOST` / `FRM_TEST_PORT` | Live FRM for integration tests and fixture capture (read-only) — defaults in `.env.example` |
| `NEXT_PUBLIC_API_URL` | Frontend dev: direct backend URL (`http://localhost:8080`). **Omit in production** — browser uses same-origin `/api` proxy |
| `BACKEND_URL` | Next.js rewrite target (`http://localhost:8080` local, `http://backend:8080` in compose) |
| `SATISFACTORY_NETWORK` | External Docker network shared with `satisfactory-server` (default `satisfactory-server_default`) |
| `BACKEND_PORT` / `FRONTEND_PORT` | Host port mappings for compose (defaults `8080` / `3000`) |

See `docs/factorymate-spec.md` §9 for the full variable list.

## Running locally

### Backend

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

### Docker Compose

Build images:

```bash
docker compose build
```

Run the full stack (requires `SESSION_SECRET` and the external `satisfactory-server` network):

```bash
export SESSION_SECRET="$(openssl rand -hex 32)"
docker compose up -d
```

- **Backend** — static Go binary, SQLite on volume `factorymate-data`, polls FRM on the shared network.
- **Frontend** — Next.js standalone; proxies `/api/*` and `/healthz` to `http://backend:8080` via `BACKEND_URL`.

## GuggiRaid production deploy

Manual smoke test per project DoD — not run in CI.

1. **On the host** (alongside the existing `satisfactory-server` container):
   - Confirm the shared Docker network exists (`docker network ls | grep satisfactory-server`).
   - If the game stack uses a different network name, set `SATISFACTORY_NETWORK` in `.env`.

2. **Configure `.env`** at the repo root on GuggiRaid:
   ```bash
   SESSION_SECRET=<long random string>
   FRM_HOST=satisfactory-server
   FRM_PORT=8080
   # Optional: FRONTEND_PORT / BACKEND_PORT if defaults conflict
   ```

3. **Deploy:**
   ```bash
   docker compose pull   # when using a registry; otherwise build on host
   docker compose build
   docker compose up -d
   ```

4. **First-run setup:** open `http://<host>:3000`, complete admin setup, configure FRM host/port in `/settings/general` if needed, add a Discord notification target, and assign it to a message type (e.g. `player_joined`).

5. **Smoke checks:**
   - `curl -s http://localhost:8080/healthz` → `{"status":"ok"}`
   - Dashboard loads with live FRM data
   - `docker compose restart` — SQLite volume persists settings and history
   - Stop FRM / game server briefly → `server_offline` Discord notification; bring FRM back → `server_online` (M3 auto-recover)

## Project conventions

- **Spec wins** on product conflicts; roadmap sequences work only
- **Migrations:** numbered `.sql` files only — no hand-written DDL outside `internal/db/migrations/`
- **Frontend i18n:** all user-facing strings in `frontend/messages/en.json` (spec §8.2)
- **shadcn:** install via MCP/CLI; edit components in-place under `frontend/components/ui/`

## Orchestrator development

See [`AGENTS.md`](../AGENTS.md) and [`.agents/project/orchestrator/milestone-scopes.md`](../.agents/project/orchestrator/milestone-scopes.md).
