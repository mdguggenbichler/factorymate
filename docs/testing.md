# Testing strategy

FactoryMate milestones should be verifiable **without** a real Discord channel or production deploy. Live FRM is available on the home network for optional integration tests and fixture capture (see below).

## Live FRM server (read-only)

A **live FRM instance** on the group's home network is available for agents and developers. Use it to capture real response shapes, validate structs, and refresh fixtures.

| Field | Value |
| --- | --- |
| Base URL | `http://192.168.178.42:8889` |
| Auth | None on this deployment |
| Cursor rule | `.cursor/rules/04-frm-live.mdc` |

**READ ONLY:** `GET` on the 12 polled Read endpoints (spec §4.1) only. Never call FRM Write endpoints or mutate game state.

```bash
# Quick smoke test (all 12 endpoints should return HTTP 200)
for ep in getPlayer getPower getSchematics getSpaceElevator getResearchTrees getTrains getVehicles \
          getProdStats getResourceSink getFactory getDrone getDoggo; do
  curl -sS -o /dev/null -w "$ep %{http_code}\n" "http://192.168.178.42:8889/$ep"
done

# Refresh a committed fixture
curl -sS "http://192.168.178.42:8889/getPower" -o backend/testdata/frm/getPower.json
```

Committed fixtures live in `backend/testdata/frm/` (see README there). Large responses — query live rather than committing full dumps.

## Discord webhook testing (legacy — pre-M15)

> **M15+:** Game-event notifications use the Discord **bot** (`channel_id` + `SendDirect`), not incoming webhooks. Use the mock-session pattern in [Discord bot testing (M15)](#discord-bot-testing-m15) below for dispatch and provider tests. This section documents the old webhook approach for historical context only.

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

> **M15 note:** Game-event notifications now use the Discord **bot** (`channel_id` in target config), not incoming webhooks. Webhook httptest patterns remain useful for regression tests on payload shape; new provider tests should mock `discordgo.Session` channel message sends instead.

### Manual smoke test (optional, human)

Set `DISCORD_TEST_WEBHOOK_URL` in `.env` (private test channel). Not required for verifier PASS.

## Discord bot testing (M15)

The Discord bot runs inside the Go backend process. CI does **not** require a live Discord guild.

### Unit tests (recommended)

| Area | Approach |
| --- | --- |
| `DiscordProvider.Send` / `SendDirect` | Mock `discordgo.Session`; assert channel/user IDs and embed payload |
| Slash command handlers | Permission logic + service integration in `interactions_test.go`; optional JSON interaction fixtures for full handler replay |
| Registration / approval | Extend `auth` / `registration` tests; `pending_approval` blocks login |
| Pending player auto-link | Poller test: upsert `player_state` → assert `player_id` set |
| Connection broadcast | Assert `SendDirect` called for all active linked users |
| Log redaction | Assert `notification_log` and `bot_command_log` never contain `game_password` |
| Dispatcher regression | Existing `dispatch_test.go` with mock session instead of httptest webhook |

### DM fan-out and preferences (M16)

Game-event dispatch sends channel posts (admin-configured targets) **and** optional DMs per user prefs. Tests live in `backend/internal/notify/dispatch_dm_test.go`:

| Test | Asserts |
| --- | --- |
| `TestDispatcher_DMFanOutRespectsPrefs` | User with `fuse_tripped` DM enabled receives fuse-trip DM; user with that type off does not; channel send still occurs |
| `TestDispatcher_PersonalPlayerDM` | User with `dm_player_personal` and linked player name receives personal join/leave DM |

Prefs are stored in `user_notification_prefs` (per **message type key**) and `users.dm_player_personal`. New users inherit admin defaults from `app_settings` (`notifications.dm_defaults_json` per-type booleans, `notifications.dm_player_personal_default`). Frontend: `/account/notifications` (all active users) and `/settings/notifications/defaults` (admin). Discord `/notifications` is category-level only.

Connection-detail DMs bypass type prefs — see `ConnectionDetailsService` tests in `backend/internal/connection/`.

### Optional integration test guild

Configure CI secrets:

```bash
DISCORD_BOT_TOKEN=...
DISCORD_GUILD_ID=...
```

Run tagged integration tests against a private test guild when validating slash commands end-to-end. Not required for autonomous verifier PASS.

### Frontend smoke (manual)

1. Settings → Discord — verify bot status badge and invite URL load
2. Settings → Notifications → Targets — channel picker populated; legacy webhook banner if old targets exist
3. Settings → Notifications → Defaults — per-type toggles load and save
4. Settings → Notifications → Templates — `connection_details_changed` appears in message type list when seeded
5. Account → Notifications (user menu) — per-type DM toggles load and save
6. Settings → Users — pending approval queue and unmapped players panels
7. `/mods` — mod table, download SMM profile, admin refresh

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
