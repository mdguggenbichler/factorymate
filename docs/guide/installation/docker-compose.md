# Docker Compose

This is the recommended way to run FactoryMate in production or on a homelab server.

## 1. Pull the image

Stable releases:

```bash
docker pull ghcr.io/ghotso/factorymate:latest
```

For bleeding-edge builds from the `dev` branch:

```bash
docker pull ghcr.io/ghotso/factorymate:nightly
```

## 2. Get the compose file

Clone the repository or copy [`docker-compose.yml`](https://github.com/ghotso/factorymate/blob/main/docker-compose.yml) into your deploy directory.

The default compose file uses the `nightly` tag. For stable releases, edit the image line:

```yaml
image: ghcr.io/ghotso/factorymate:latest
```

## 3. Configure environment

Copy the env template:

```bash
cp .env.example .env
```

Edit `.env` — minimum settings:

```bash
SESSION_SECRET=<long random string>   # openssl rand -hex 32
FRM_HOST=satisfactory-server          # Docker service name, or host IP
FRM_PORT=8080                         # FRM port (inside game container or on host)

# Discord bot (required for notifications and slash commands)
DISCORD_BOT_TOKEN=<your bot token>
DISCORD_GUILD_ID=<your Discord server ID>

# Optional — shown in bot welcome/help messages
FACTORYMATE_PUBLIC_URL=https://factorymate.example.com
```

| Variable | Required | Description |
| --- | --- | --- |
| `SESSION_SECRET` | **Yes** | Cookie signing secret for the web dashboard |
| `FRM_HOST` | Yes | FRM hostname (seeded into settings on first boot) |
| `FRM_PORT` | Yes | FRM port (default `8080`) |
| `DISCORD_BOT_TOKEN` | For Discord | Bot token from Developer Portal |
| `DISCORD_GUILD_ID` | Recommended | Your Discord server ID (bootstrap until set in UI) |
| `FACTORYMATE_PORT` | No | Host port for the UI (default `3000`) |
| `SATISFACTORY_NETWORK` | No | External Docker network name for shared-network setup |

See [FRM connectivity](../frm-connectivity.md) if FactoryMate cannot reach FRM with the defaults.

## 4. Start the stack

```bash
docker compose up -d
```

Verify health:

```bash
curl -s http://localhost:3000/healthz
# {"status":"ok"}
```

## 5. Access the dashboard

Open `http://<your-host>:3000` (or your configured `FACTORYMATE_PORT`).

On first launch with no users, you are redirected to the **one-time admin setup** page. See [First run](../first-run.md).

## Data persistence

SQLite data is stored in `./data/factorymate.db` via a bind mount:

```yaml
volumes:
  - ./data:/data
```

Back up this directory before upgrades. Settings, users, notification history, and templates all live here.

## Health check

The container includes a health check against `/healthz`. View status:

```bash
docker compose ps
```

## Alternative: docker run

If you prefer a single command without Compose, see [Docker Run](docker-run.md).

## Next steps

1. [Connect FactoryMate to FRM](../frm-connectivity.md)
2. [Set up the Discord bot](../discord/setup.md)
3. [Complete first-run configuration](../first-run.md)
