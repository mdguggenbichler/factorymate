# Upgrading

FactoryMate publishes stable releases from the `main` branch. Stable container images and documentation deploy when a GitHub release is **published**.

## Release flow

1. `VERSION` in the repository is bumped on `main`.
2. CI creates a draft GitHub release (e.g. `v1.0.0`).
3. You review and **publish** the release.
4. GitHub Actions pushes:
   - Container image tags `v1.0.0` and `latest` to GHCR
   - Updated documentation to GitHub Pages

## Upgrade with Docker Compose

On your deploy host:

```bash
# 1. Back up data
cp -a data data.backup.$(date +%Y%m%d)

# 2. Pull the new image
docker compose pull

# 3. Restart with the new image
docker compose up -d
```

If your `docker-compose.yml` pins a specific version tag instead of `latest`, update the image line before pulling:

```yaml
image: ghcr.io/ghotso/factorymate:v1.0.0
```

## Upgrade with docker run

Stop the old container, pull the new image, and start with the same volume mount and environment:

```bash
docker pull ghcr.io/ghotso/factorymate:latest

docker stop factorymate
docker rm factorymate

docker run -d \
  --name factorymate \
  --restart unless-stopped \
  -p 3000:3000 \
  -v "$(pwd)/data:/data" \
  --env-file .env \
  ghcr.io/ghotso/factorymate:latest
```

Adjust ports and env to match your setup. See [Docker Run](installation/docker-run.md).

## What persists across upgrades

All of the following live in `./data/factorymate.db` (bind mount):

- User accounts and Discord links
- App settings (FRM host, Discord guild, role mappings)
- Notification targets, templates, and log history
- Connection details and player mappings

**Back up `./data` before every upgrade.**

## After upgrading

1. Check health: `curl -s http://localhost:3000/healthz`
2. Open the dashboard — confirm live FRM data loads.
3. Check **Settings → Discord** — bot still connected.
4. Send a test notification from **Settings → Notifications → Templates**.
5. Review release notes for breaking changes (e.g. webhook → bot migration required re-selecting Discord channels).

## Template defaults

Custom template edits in the database override shipped defaults. After an upgrade, use **Reset to default** on individual templates if you want updated default copy from the new version.

## Nightly builds

The `dev` branch publishes `ghcr.io/ghotso/factorymate:nightly` on every push. Use nightly only if you accept unstable builds:

```bash
docker pull ghcr.io/ghotso/factorymate:nightly
```

Stable homelab deployments should use `latest` or a pinned `v*` tag.

## Documentation

User documentation is published at:

**https://ghotso.github.io/factorymate/**

Docs update automatically when a release is published — same trigger as the stable container image.
