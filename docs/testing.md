# Testing strategy

FactoryMate milestones should be verifiable **without** a real Discord channel, production deploy, or always-on FRM server. Live tests are opt-in.

## Discord webhook testing (M4, M6)

Discord incoming webhooks are a single `POST` with a JSON body (`content` and/or `embeds`). You do **not** need Discord to verify payload shape.

### Recommended: Go `httptest` mock (CI + autonomous runs)

In `internal/notify/discord_test.go`, point `DiscordProvider` at an `httptest.NewServer` handler that:

1. Asserts `Content-Type: application/json`
2. Decodes the body and checks embed title, description, color, fields
3. Returns `204 No Content` (Discord's success response)

This satisfies M4 DoD ("sample embed reaches Discord correctly formatted") at the HTTP contract level. M6 E2E can use the same mock as the dispatch target URL stored in a test `notification_targets` row.

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    var payload map[string]any
    json.NewDecoder(r.Body).Decode(&payload)
    // assert embeds[0].fields, etc.
    w.WriteHeader(http.StatusNoContent)
}))
defer srv.Close()
// webhook URL = srv.URL
```

### Optional: fauxcord container (full Discord API mock)

For richer bot-style testing (not required for webhook-only v1), [fauxcord](https://github.com/tomacheese/fauxcord) exposes Discord API v10 including webhook execution. Add to `docker-compose.test.yml` when needed:

```yaml
services:
  fauxcord:
    image: ghcr.io/tomacheese/fauxcord:latest
    ports:
      - "3001:3000"
```

FactoryMate v1 only POSTs to webhook URLs — **httptest is sufficient** and keeps CI dependency-free.

### Manual smoke test (optional, human)

Set `DISCORD_TEST_WEBHOOK_URL` in `.env` (private test channel). Not required for verifier PASS.

## FRM client testing (M2, M3)

### Unit tests: JSON fixtures

Store captured FRM responses under `backend/testdata/frm/` (e.g. `getPlayer.json`, `getPower.json`). M2 parsing tests and M3 diff tests run against fixtures — no network.

### Integration tests: live FRM (opt-in)

Tag with `//go:build integration`. Run only when env is set:

```bash
FRM_TEST_HOST=192.168.178.42 FRM_TEST_PORT=8889 go test -tags=integration ./internal/frm/...
```

Verifier PASS for M2/M3 in autonomous runs: **fixture unit tests must pass**; live integration is `n/a` unless `FRM_TEST_HOST` is configured.

## Auth testing (M7)

Pure unit + handler tests with in-memory or temp SQLite. No external services.

## Frontend testing (M10–M12)

- `npm run lint` + `npm run build` (scoped CI gate)
- No hardcoded UI strings — grep review per `.cursor/rules/03-i18n.mdc`
- API wiring tested manually or via Playwright later (out of scope v1)

## M13 production deploy

DoD requires GuggiRaid deploy — **manual milestone**, not CI. Autonomous loop ends at M13 code complete; human verifies production smoke.

## Verifier guidance

| Milestone | Automated gate | Live/manual |
| --- | --- | --- |
| M2 | Fixture parse tests | Optional `integration` tag |
| M4 | httptest webhook assert | Optional real webhook |
| M6 | Dispatch → mock webhook + `notification_log` row | Optional in-game E2E |
| M13 | `docker compose build` | Human deploy to GuggiRaid |
