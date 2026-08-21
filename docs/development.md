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
| `BACKEND_URL` | Next.js rewrite target (`http://localhost:8080` local; `http://127.0.0.1:8080` inside the Docker container) |
| `SATISFACTORY_NETWORK` | External Docker network shared with `satisfactory-server` (default `satisfactory-server_default`) |
| `FACTORYMATE_PORT` | Host port mapping for the single compose service (default `3000`) |
| `DISCORD_BOT_TOKEN` | Discord bot token (soft dependency — bot features disabled when unset) |
| `DISCORD_CLIENT_SECRET` | Discord OAuth client secret (same application as the bot). With `FACTORYMATE_PUBLIC_URL`, enables **Continue with Discord** and `/register` OAuth completion |
| `FACTORYMATE_PUBLIC_URL` | Public dashboard base URL — OAuth redirect `{url}/api/auth/discord/callback` and bot copy |
| `DISCORD_GUILD_ID` | Bootstrap guild ID until set in Settings → Discord |
| `DISCORD_ADMIN_ROLE_IDS` | Optional comma-separated admin role IDs before UI role mapping is configured |
| `FACTORYMATE_PUBLIC_URL` | Public dashboard URL used in bot welcome/help copy |

| `PLANNER_DOCS_PATH` | UTF-16 LE BOM `FactoryGame-Docs.json` path for catalog parse fallback (Docker: `/app/planner-data/FactoryGame-Docs.json`) |
| `PLANNER_CATALOG_PATH` | Slim UTF-8 catalog JSON (preferred at runtime; Docker: `/app/data/factory_catalog.json`) |
| `PLANNER_ICONS_DIR` / `PLANNER_ICONS_JSON` | Satisfactory icon PNG dir + `Build_*`→`Desc_*` map (Docker: `/app/planner-data/icons`, `/app/planner-data/icons.json`) |

## Discord bot setup (M15)

1. Create a Discord application in the [Developer Portal](https://discord.com/developers/applications).
2. **Bot** tab → create bot → copy token → set `DISCORD_BOT_TOKEN` in `.env`.
3. **OAuth2** → copy **Client Secret** → set `DISCORD_CLIENT_SECRET` in `.env`. Add redirect URI: `{FACTORYMATE_PUBLIC_URL}/api/auth/discord/callback` (e.g. `https://factorymate.example.com/api/auth/discord/callback`).
4. **OAuth2 → URL Generator** → scopes: `bot`, `applications.commands`; permissions: View Channels, Send Messages, Embed Links, Use Slash Commands, Send Messages in Threads, Create Private Channels (for DMs).
4. Complete FactoryMate `/setup` (first admin) before or after adding the bot token.
5. Open **Settings → Discord** in the dashboard → load invite URL → add bot to your guild.
6. Set guild ID and role mappings; toggle auto-approve if manual registration approval is desired.
7. **Settings → Notifications → Targets** → pick a channel (replaces legacy webhook URLs).
8. Ask players to run `/register` in Discord (primary onboarding). Web invites under Settings → Users → break-glass section are for recovery only.

Slash commands register per guild when the bot starts and when an admin saves a guild ID in **Settings → Discord**. Set `FACTORYMATE_PUBLIC_URL` before testing OAuth flows locally (e.g. `http://localhost:3000` with the dev proxy).

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

Build the single image:

```bash
docker compose build
```

Run the stack (requires `SESSION_SECRET`; external game network is optional):

```bash
export SESSION_SECRET="$(openssl rand -hex 32)"
docker compose up -d
```

- **Single container** — Go API + poller on `localhost:8080` (internal), Next.js on `:3000` (exposed). Next.js proxies `/api/*` and `/healthz` to the backend via `BACKEND_URL`.
- **SQLite** — persisted in `./data` via bind mount.
- **FRM reachability** — default compose uses an isolated `factorymate` network. Use host IP + mapped FRM port in `.env`, or add a `docker-compose.override.yml` to join the game stack network (see README §FRM connectivity).

### Shared Docker network (optional)

When FactoryMate and the Satisfactory/FRM container run on the **same Docker host**, you can attach to the game stack’s external network so `FRM_HOST=satisfactory-server` resolves without publishing FRM on the host:

```yaml
# docker-compose.override.yml
services:
  factorymate:
    networks:
      - factorymate
      - satisfactory-server

networks:
  factorymate:
  satisfactory-server:
    external: true
    name: ${SATISFACTORY_NETWORK:-satisfactory-server_default}
```

Discover the network: `docker network ls | grep satisfactory`. Set `SATISFACTORY_NETWORK` in `.env` if the name differs.

## CI/CD and container images

GitHub Actions uses branch/PR entry workflows with reusable `_*.yml` workflows — see [`.github/workflows/README.md`](../.github/workflows/README.md).

| Branch / event | What runs |
| --- | --- |
| Pull request | CI only (`_ci.yml`) — backend, frontend, Docker smoke build |
| `push` → `dev` | CI + push `ghcr.io/ghotso/factorymate:nightly` and `:{sha7}` |
| `push` → `main` | CI + draft release when root `VERSION` semver-increases and `v{VERSION}` tag is missing |
| Release published (`v*`) | Push `ghcr.io/ghotso/factorymate:{version}` and `:latest`; deploy user docs to GitHub Pages |

**Release flow:** bump [`VERSION`](../VERSION) on `main` (e.g. `0.1.0`) → merge → workflow creates draft release `v0.1.0` → review and publish → stable images land on GHCR.

**Pull nightly image:**

```bash
docker pull ghcr.io/ghotso/factorymate:nightly
```

Ensure **Settings → Actions → General → Workflow permissions** allows read/write for releases and packages.

## GuggiRaid production deploy

Manual smoke test per project DoD — not run in CI.

1. **On the host** (alongside the existing `satisfactory-server` container):
   - **Recommended:** add `docker-compose.override.yml` to join the game stack network (see Shared Docker network above).
   - Confirm the network exists: `docker network ls | grep satisfactory-server`.
   - If the game stack uses a different network name, set `SATISFACTORY_NETWORK` in `.env`.

2. **Configure `.env`** at the repo root on GuggiRaid:
   ```bash
   SESSION_SECRET=<long random string>
   FRM_HOST=satisfactory-server
   FRM_PORT=8080
   # Optional: FACTORYMATE_PORT if 3000 is already in use
   ```

3. **Deploy:**
   ```bash
   docker compose pull   # when using a registry; otherwise build on host
   docker compose build
   docker compose up -d
   ```

4. **First-run setup:** open `http://<host>:3000`, complete admin setup, configure FRM host/port in `/settings/general` if needed, configure the Discord bot under `/settings/discord`, add a notification target channel, and assign it to a message type (e.g. `player_joined`). New players register via Discord `/register`.

5. **Smoke checks:**
   - `curl -s http://localhost:3000/healthz` → `{"status":"ok"}`
   - Dashboard loads with live FRM data
   - `docker compose restart` — SQLite volume persists settings and history
   - Stop FRM / game server briefly → `server_offline` Discord notification; bring FRM back → `server_online` (M3 auto-recover)

## Project conventions

- **Spec wins** on product conflicts; roadmap sequences work only
- **Migrations:** numbered `.sql` files only — no hand-written DDL outside `internal/db/migrations/`
- **Frontend i18n:** all user-facing strings in `frontend/messages/en.json` (spec §8.2)
- **shadcn:** install via MCP/CLI; edit components in-place under `frontend/components/ui/`

## User documentation (MkDocs)

Published at [https://ghotso.github.io/factorymate/](https://ghotso.github.io/factorymate/) on release publish. Local preview:

```bash
pip install -r docs/requirements-docs.txt
mkdocs serve
```

Source: `docs/guide/` + root `mkdocs.yml`.

## Orchestrator development

See [`AGENTS.md`](../AGENTS.md) and [`.agents/project/orchestrator/milestone-scopes.md`](../.agents/project/orchestrator/milestone-scopes.md).

## Item icons

Satisfactory item icons under `assets/icons/` are extracted from the game via FRM for dashboard display. They are synced into `frontend/public/icons/` at dev/build time (`scripts/sync-item-icons.mjs`). Game assets remain property of Coffee Stain Studios.
