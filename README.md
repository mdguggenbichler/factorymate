# FactoryMate

Self-hosted monitoring and Discord notifications for **Satisfactory** dedicated servers using [FicsIt Remote Monitoring (FRM)](https://docs.ficsit.app/ficsitremotemonitoring/latest/index.html).

FactoryMate polls your FRM HTTP API, detects edge-triggered game events (players joining, fuse trips, milestones, trains, and more), sends rich Discord embeds to configurable webhook targets, and provides a small authenticated web dashboard for live factory stats and history.

Built for small private groups (originally **CBC | Conveyor Belt Cult**), deployed as a single container alongside your game server.

## Features

- **Event detection** — Fast-poll diff engine for players, power, schematics, research, trains, vehicles, and server reachability (no duplicate spam; survives restarts).
- **Discord notifications** — Structured embeds with per-type title emojis, fields, footer, and native timestamps; editable per message type in the UI.
- **Notification targets** — Configure Discord webhooks once; assign which message types go to which channels.
- **Dashboard** — Players, power, production (overall + per-machine), Resource Sink, drones, doggos, milestones, M.A.M. research tree, vehicles, Space Elevator.
- **Invite-based onboarding** — Admins create single-use invite links; users set their own username and password.
- **Read-only FRM** — FactoryMate never writes to the game; it only polls FRM Read endpoints.

## Requirements

| Component | Notes |
| --- | --- |
| Satisfactory dedicated server | With **SML** and **FicsIt Remote Monitoring** installed |
| FRM HTTP API | Reachable from the FactoryMate container/host (`Web_Autostart` enabled in-game) |
| Docker (recommended) | Or Go 1.26+ and Node.js 22+ for local development |

## Getting started (Docker)

This is the recommended path for production and homelab deploys.

### 1. Pull the image

Pre-built images are published to GitHub Container Registry:

```bash
docker pull ghcr.io/mdguggenbichler/factorymate:nightly   # dev branch
# docker pull ghcr.io/mdguggenbichler/factorymate:1.0.0  # release tag
```

### 2. Configure environment

Copy the env template and set a strong session secret:

```bash
cp .env.example .env
```

Edit `.env` — minimum for Docker:

```bash
SESSION_SECRET=<long random string>   # openssl rand -hex 32
FRM_HOST=satisfactory-server          # Docker service name, or host IP if FRM is on another machine
FRM_PORT=8080                         # FRM port inside the game container (often host-mapped e.g. 8889)
# FACTORYMATE_PORT=3000             # optional host port override
```

**FRM connectivity:** FactoryMate must reach FRM over HTTP. The default compose file uses an isolated `factorymate` network — pick one of these options:

- **Option A — LAN / host IP** — Set `FRM_HOST` to the server’s IP (e.g. `192.168.1.50`) and `FRM_PORT` to the host-mapped FRM port (e.g. `8889`). Requires FRM to be published on the host.
- **Option B — shared Docker network (same host)** — Attach FactoryMate to your game stack’s Docker network so `FRM_HOST=satisfactory-server` and `FRM_PORT=8080` work **without** exposing FRM on the host. Create `docker-compose.override.yml`:

```yaml
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

Find the network name with `docker network ls | grep satisfactory`. If it differs, set `SATISFACTORY_NETWORK` in `.env`.

### 3. Start the stack

```bash
docker compose up -d
```

- **UI:** `http://localhost:3000` (or your host IP + `FACTORYMATE_PORT`)
- **Health:** `curl -s http://localhost:3000/healthz` → `{"status":"ok"}`
- **Data:** SQLite database persisted in `./data/factorymate.db` (bind mount)

### 4. First-run setup

1. Open the UI — if no users exist, you’ll see the **one-time admin setup** page.
2. Create the first **admin** account.
3. Go to **Settings → General** — confirm FRM host/port; server display name syncs from FRM automatically.
4. **Settings → Notifications → Targets** — add a Discord webhook URL.
5. **Settings → Notifications → Templates** — enable message types (e.g. `player_joined`, `player_left`), assign your Discord target, and use **Send test** to verify embeds.
6. **Settings → Users** — create **invite links** for other group members (viewer or admin role).

After that, notifications fire automatically when the poller detects state changes.

## Getting started (local development)

For contributors and debugging without Docker:

```bash
# 1. Environment
cp .env.example .env
# Set SESSION_SECRET, FRM_HOST, FRM_PORT, NEXT_PUBLIC_API_URL=http://localhost:8080

# 2. Backend (API + poller on :8080)
cd backend && go run ./cmd/server

# 3. Frontend (dashboard on :3000)
cd frontend && npm install && npm run dev
```

Full details: [`docs/development.md`](docs/development.md) — env vars, compose build, CI/CD, GuggiRaid deploy notes.

## Configuration overview

| What | Where |
| --- | --- |
| FRM host/port, poll intervals | **Settings → General** (persisted in SQLite) |
| Discord webhooks | **Settings → Notifications → Targets** |
| Message templates & assignments | **Settings → Notifications → Templates** |
| Users & invites | **Settings → Users** |
| Default embed copy (source of truth) | [`backend/data/message_defaults.json`](backend/data/message_defaults.json) |

Environment variables for Docker and first boot are documented in [`docs/factorymate-spec.md`](docs/factorymate-spec.md) §9 and [`.env.example`](.env.example).

**Template overrides:** Custom embeds saved in the UI live in the database and take precedence over shipped defaults. Use **Reset to default** on a template variant to pick up new defaults after an upgrade.

## Architecture

```
Satisfactory + FRM (:8080)
        │  HTTP GET (polled)
        ▼
FactoryMate container
  ├─ Go backend  — poller, diff engine, REST API, SQLite
  └─ Next.js UI  — :3000, proxies /api → backend
        │
        ▼
Discord webhooks (and future notification providers)
```

## Documentation

| Document | Purpose |
| --- | --- |
| [`docs/factorymate-spec.md`](docs/factorymate-spec.md) | Product contract — APIs, schemas, FRM mapping, UI routes |
| [`docs/factorymate-roadmap.md`](docs/factorymate-roadmap.md) | Milestone history (M0–M13) |
| [`docs/development.md`](docs/development.md) | Dev setup, CI/CD, production deploy |
| [`docs/testing.md`](docs/testing.md) | FRM fixtures, Discord mock testing |
| [`docs/frm-docs/`](docs/frm-docs/) | Vendored FRM API reference |
| [`AGENTS.md`](AGENTS.md) | Cursor agent / contributor conventions |

## Development & CI

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm run lint && npm run build
```

GitHub Actions runs the same gates on pull requests. Pushes to `dev` publish `ghcr.io/mdguggenbichler/factorymate:nightly`. See [`.github/workflows/README.md`](.github/workflows/README.md).

## License

FactoryMate is released under the **[MIT License](LICENSE)**.

MIT is a good fit for a self-hosted sidecar tool: permissive, widely understood, and compatible with Satisfactory’s mod ecosystem dependencies (SML/FRM remain under their own licenses). If you prefer strong copyleft for network-facing services, **AGPL-3.0** is the usual alternative; for a private fork with no redistribution, you can also keep the repo proprietary and omit a public license.

---

**Version:** see [`VERSION`](VERSION). **Container:** `ghcr.io/mdguggenbichler/factorymate`.
