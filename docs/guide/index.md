# FactoryMate

Self-hosted monitoring and Discord notifications for **Satisfactory** dedicated servers using [FicsIt Remote Monitoring (FRM)](https://docs.ficsit.app/ficsitremotemonitoring/latest/index.html).

FactoryMate polls your FRM HTTP API, detects edge-triggered game events (players joining, fuse trips, milestones, trains, and more), sends rich Discord embeds to channels you choose, and provides a small authenticated web dashboard for live factory stats and history.

Built for small private groups, deployed as a **single Docker container** alongside your game server.

## What you get

- **Event detection** — Fast-poll diff engine for players, power, schematics, research, trains, vehicles, and server reachability (no duplicate spam; survives restarts).
- **Discord bot** — One bot handles slash commands, channel notifications, and direct messages. No webhook URLs to manage.
- **Dashboard** — Players, power, production, Resource Sink, drones, doggos, milestones, M.A.M. research, vehicles, Space Elevator, and mod list.
- **Discord registration** — Players run `/register` in your Discord server to create accounts and link their in-game identity.
- **Read-only FRM** — FactoryMate never writes to the game; it only polls FRM Read endpoints.

## Architecture

```
Satisfactory + FRM (:8080)
        │  HTTP GET (polled)
        ▼
FactoryMate container
  ├─ Go backend  — poller, diff engine, REST API, SQLite, Discord bot
  └─ Next.js UI  — :3000, proxies /api → backend
        │
        ▼
Discord API (channel posts + DMs + slash commands)
```

## Quick setup checklist

1. [Requirements](requirements.md) — Satisfactory server with SML + FRM, Docker installed.
2. [Install with Docker Compose](installation/docker-compose.md) — pull image, configure `.env`, start container.
3. [Connect to FRM](frm-connectivity.md) — LAN IP or shared Docker network.
4. [Discord Developer Portal](discord/setup.md) — create application, copy bot token.
5. [First run](first-run.md) — admin account, invite bot, pick notification channels.
6. Ask players to run `/register` in Discord.

## Container image

Pre-built images are published to GitHub Container Registry:

| Tag | When to use |
| --- | --- |
| `ghcr.io/ghotso/factorymate:latest` | Stable releases |
| `ghcr.io/ghotso/factorymate:nightly` | Latest from `dev` branch |

See [Upgrading](upgrading.md) for release updates.
