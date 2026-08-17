# Docker Run

Use `docker run` when you do not want Docker Compose. The settings below match the default [`docker-compose.yml`](https://github.com/ghotso/factorymate/blob/main/docker-compose.yml) service.

## Prerequisites

- `.env` file configured (see [Docker Compose](docker-compose.md) step 3)
- `./data` directory created for SQLite persistence

```bash
mkdir -p data
```

## Stable release

```bash
docker run -d \
  --name factorymate \
  --restart unless-stopped \
  -p 3000:3000 \
  -e SESSION_SECRET="${SESSION_SECRET}" \
  -e DATABASE_PATH=/data/factorymate.db \
  -e FRM_HOST="${FRM_HOST:-satisfactory-server}" \
  -e FRM_PORT="${FRM_PORT:-8080}" \
  -e BACKEND_URL=http://127.0.0.1:8080 \
  -e DISCORD_BOT_TOKEN="${DISCORD_BOT_TOKEN}" \
  -e DISCORD_GUILD_ID="${DISCORD_GUILD_ID}" \
  -e FACTORYMATE_PUBLIC_URL="${FACTORYMATE_PUBLIC_URL}" \
  -v "$(pwd)/data:/data" \
  ghcr.io/ghotso/factorymate:latest
```

Load variables from `.env` first:

```bash
set -a && source .env && set +a
```

## Shared Docker network with game server

If FRM runs in another container on the same host, attach FactoryMate to that network:

```bash
docker run -d \
  --name factorymate \
  --restart unless-stopped \
  --network satisfactory-server_default \
  -p 3000:3000 \
  -e SESSION_SECRET="${SESSION_SECRET}" \
  -e DATABASE_PATH=/data/factorymate.db \
  -e FRM_HOST=satisfactory-server \
  -e FRM_PORT=8080 \
  -e BACKEND_URL=http://127.0.0.1:8080 \
  -e DISCORD_BOT_TOKEN="${DISCORD_BOT_TOKEN}" \
  -e DISCORD_GUILD_ID="${DISCORD_GUILD_ID}" \
  -v "$(pwd)/data:/data" \
  ghcr.io/ghotso/factorymate:latest
```

Replace `satisfactory-server_default` with your game stack's network name (`docker network ls`).

## Verify

```bash
curl -s http://localhost:3000/healthz
docker logs factorymate
```

## Stop and remove

```bash
docker stop factorymate
docker rm factorymate
```

Your data remains in `./data`.

## Next steps

Continue with [First run](../first-run.md) and [Discord setup](../discord/setup.md).
