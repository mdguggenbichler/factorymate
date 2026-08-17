# FRM connectivity

FactoryMate polls FRM over HTTP. The container must be able to reach the FRM API endpoint you configure in `FRM_HOST` and `FRM_PORT`.

## Default compose networking

The default [`docker-compose.yml`](https://github.com/ghotso/factorymate/blob/main/docker-compose.yml) puts FactoryMate on an isolated `factorymate` network. FRM is **not** on that network unless you configure connectivity.

Choose one of the options below.

## Option A — LAN / host IP

Use this when FRM is published on the host (common when the game container maps port `8080` → host `8889`).

In `.env`:

```bash
FRM_HOST=192.168.1.50    # your server's LAN IP
FRM_PORT=8889            # host-mapped FRM port
```

Requirements:

- FRM HTTP port is reachable from the FactoryMate host
- Firewall allows traffic from the FactoryMate container to that IP/port

You can also override FRM host/port later in **Settings → General** without restarting (values persist in SQLite).

## Option B — shared Docker network

Use this when FactoryMate and the Satisfactory/FRM container run on the **same Docker host** and you want `FRM_HOST=satisfactory-server` without exposing FRM on the host.

### Step 1 — find the game network

```bash
docker network ls | grep satisfactory
```

Note the network name (often `satisfactory-server_default`).

### Step 2 — create docker-compose.override.yml

In your FactoryMate deploy directory:

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

### Step 3 — set .env

```bash
FRM_HOST=satisfactory-server
FRM_PORT=8080
SATISFACTORY_NETWORK=satisfactory-server_default   # if name differs
```

### Step 4 — restart

```bash
docker compose up -d
```

## Verify connectivity

1. Open **Settings → General** in the dashboard — confirm FRM host/port.
2. Check the dashboard loads live data (players, power, etc.).
3. Watch container logs for FRM connection errors:

```bash
docker compose logs -f factorymate
```

If FRM is unreachable, the poller logs errors until settings are corrected. You may receive a `server_offline` Discord notification once configured.

## FRM auth token

If your FRM instance requires authentication, set the token in **Settings → General** (`frm_auth_token`). This is stored in the database, not in `.env`.

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Dashboard empty, FRM errors in logs | Wrong host/port | Verify with `curl http://<FRM_HOST>:<FRM_PORT>/` from the host |
| Works on host, not in container | Network isolation | Use Option A (host IP) or Option B (shared network) |
| Intermittent offline alerts | Game server restarting | Expected — `server_offline` / `server_online` notifications |

See [First run](first-run.md) after FRM connectivity is confirmed.
