# Proposal: Savegame download (Dedicated Server API)

**Status:** on-roadmap (M22)  
**Related:** spec §1.2 (v1 non-goals — read-only game API exception), spec §6 `/api/connection-details`, spec §8 `/connection`, `/mods` SMM profile export pattern, discord-bot-plan §8 (active-user gating), `docs/frm-docs/.../dedicatedserver.adoc`

FactoryMate should let **active, authenticated group members** download the current dedicated-server save as a `.sav` file — from the **web dashboard** and via a **Discord slash command** (DM delivery, mirroring `/mods action:export`).

---

## Recommendation: Dedicated Server HTTPS API (primary)

Use Coffee Stain’s **Dedicated Server HTTPS API** (`DownloadSaveGame` / `EnumerateSessions`) as the **only v1 backend**. Do **not** require a read-only volume mount for the first release.

| Criterion | HTTPS API | Read-only mount |
|-----------|-----------|-----------------|
| Works when FM and game run on different hosts | Yes | No (needs shared filesystem) |
| Works with managed hosts (no container access) | Yes (if API port + token exposed) | Unlikely |
| Lists multiple autosaves with metadata | Yes (`EnumerateSessions`) | Manual directory scan |
| Extra deployment config | API token + TLS skip-verify | Bind-mount path per image |
| Broader user base | **Best** | Niche (same Docker host only) |

**Optional later:** add a filesystem provider behind the same service interface for operators who cannot issue an API token. Not in v1 scope.

### Spec non-goal note

Spec §1.2 says *“No write-access to the game (no remote admin actions against the Dedicated Server API in v1).”*  
`DownloadSaveGame` is **read-only** (no load/save/upload/delete). This feature is an explicit, narrow exception: **download only**, no other Dedicated Server API write functions.

---

## Live verification (2026-08-22)

Tested against the group’s home server using env vars `SATISFACTORY_SERVER_HOST`, `SATISFACTORY_SERVER_PORT`, `SATISFACTORY_SERVER_TOKEN`.

| Step | Result |
|------|--------|
| `POST /api/v1` `QueryServerState` | `activeSessionName`: `Conveyor Belt Cult`, `isGameRunning`: true |
| `POST /api/v1` `EnumerateSessions` | Session with 9+ `saveHeaders`; newest autosave `Conveyor Belt Cult_autosave_3` @ `2026.08.22-15.38.00` |
| `POST /api/v1` `DownloadSaveGame` `{ SaveName: "Conveyor Belt Cult_autosave_3" }` | **200**, `application/octet-stream`, `Content-Disposition: attachment; filename="Conveyor Belt Cult_autosave_3.sav"`, **~1.4 MiB** |
| FRM `GET /getSessionInfo` | `SessionName`: `Conveyor Belt Cult` (matches `activeSessionName`; not interchangeable with `SaveName` in general) |

Conclusions:

- API token auth works with `Authorization: Bearer …`.
- Self-signed TLS: client must use **insecure skip verify** (same as game clients / community tools).
- Current save is **well under Discord’s 25 MiB bot upload limit** — file DM is viable today; still implement size guard for growth.

---

## Goals

- **Web:** active users download the latest autosave (or a chosen save) from `/connection` (or a dedicated card on that page).
- **Discord:** `/savegame` slash command sends the `.sav` file to the user’s DMs (with ephemeral confirmation in-channel), same delivery pattern as `/mods action:export`.
- **Admin settings:** configure API host, port, and token; test connectivity from Settings (like FRM test).
- **Default selection:** latest autosave for the **currently active session** (by `saveDateTime`, not filename sort).
- **Audit:** log download requests (user id, save name, bytes, channel web/discord) — no file contents in logs.

## Non-goals (v1)

- Upload / load / delete saves (`UploadSaveGame`, `LoadGame`, `DeleteSaveFile`, …).
- Exposing the API token to non-admin users (token is server-side only).
- Public unauthenticated download links.
- Automatic scheduled backups to object storage.
- Filesystem-mount provider (defer).

---

## Architecture

```
┌─────────────────────┐     HTTPS POST /api/v1      ┌──────────────────────────┐
│ FactoryMate backend │ ────────────────────────────► │ Satisfactory Dedicated   │
│  savegame.Service   │   QueryServerState            │ Server HTTPS API :7777   │
│                     │   EnumerateSessions           │ DownloadSaveGame         │
│                     │   DownloadSaveGame            └──────────────────────────┘
└─────────┬───────────┘
          │
    ┌─────┴─────┐
    ▼           ▼
 GET /api/    /savegame
 savegame      (Discord DM + file)
```

New package: `backend/internal/savegame/` (client + service + types).  
Mirror `backend/internal/frm/` for HTTP client shape; mirror `backend/internal/mods/` for download handler + Discord file attach.

---

## Configuration

### Storage (SQLite)

Add columns to `app_settings` (new migration, e.g. `013_savegame_api.sql`):

| Column | Type | Notes |
|--------|------|-------|
| `game_api_host` | TEXT | Defaults empty; seed from env |
| `game_api_port` | INTEGER | Default `7777` |
| `game_api_token` | TEXT | Admin API token from `server.GenerateAPIToken`; **never returned in GET responses** (write-only field like passwords — return `configured: true/false`) |

**Env bootstrap** (spec §9 pattern, like `FRM_HOST`):

| Env | Purpose |
|-----|---------|
| `SATISFACTORY_SERVER_HOST` | Seed `game_api_host` on first boot if unset |
| `SATISFACTORY_SERVER_PORT` | Seed `game_api_port` |
| `SATISFACTORY_SERVER_TOKEN` | Seed `game_api_token` |

Reuse **connection details** `gameHost` / `gamePort` only as a **fallback hint** when `game_api_host` is empty (same machine, port often 7777) — but prefer explicit `game_api_*` settings so FRM port `8080` is never confused with HTTPS API port `7777`.

### Admin UI

Extend **Settings → Connection** (or new **Settings → Server API** subsection):

- Host, port, token (masked input)
- **Test** button → calls `POST /api/settings/game-api/test` → returns `{ activeSessionName, reachable: true, saveCount }` without downloading a file
- Help text: how to generate token (`server.GenerateAPIToken` in dedicated server console)

---

## HTTP client (`backend/internal/savegame/client.go`)

- Base URL: `https://{host}:{port}/api/v1`
- TLS: `InsecureSkipVerify: true` (document why; no custom CA in v1)
- Auth: `Authorization: Bearer {token}`
- Request envelope: `{"function":"<Name>","data":{...}}` (JSON POST)
- Response parsing:
  - JSON functions: unwrap `data` object
  - `DownloadSaveGame`: raw body + parse `Content-Disposition` filename; reject if `Content-Type` is JSON (error response)
- Timeouts: **120s** for download (large saves); **10s** for metadata calls
- Context cancellation respected

### Types (subset)

```go
type ServerGameState struct {
    ActiveSessionName string `json:"activeSessionName"`
    IsGameRunning     bool   `json:"isGameRunning"`
    // ...
}

type SaveHeader struct {
    SaveName       string `json:"saveName"`
    SessionName    string `json:"sessionName"`
    SaveDateTime   string `json:"saveDateTime"` // "2026.08.22-15.38.00"
    BuildVersion   int    `json:"buildVersion"`
    IsModdedSave   bool   `json:"isModdedSave"`
    // ...
}

type Session struct {
    SessionName string       `json:"sessionName"`
    SaveHeaders []SaveHeader `json:"saveHeaders"`
}
```

---

## Save selection logic

**Default (v1): “latest autosave for active session”**

1. `QueryServerState` → `activeSessionName`
2. `EnumerateSessions` → find session where `sessionName == activeSessionName`
3. Filter `saveHeaders` where `saveName` contains `_autosave_` (or all headers if none match)
4. Pick max by parsed `saveDateTime` (`time.Parse("2006.01.02-15.04.05", …)`)
5. `DownloadSaveGame` with that `saveName`

**Explicit save (v1 optional query param):**

- `GET /api/savegame?saveName=Conveyor%20Belt%20Cult_autosave_2` — validate name exists in `EnumerateSessions` before download (path traversal / arbitrary file guard).

**List endpoint (v1):**

- `GET /api/savegame/list` → `{ activeSessionName, saves: [{ saveName, saveDateTime, buildVersion, isModdedSave }] }` sorted newest first — powers UI picker later; v1 can ship download-only with implicit latest.

---

## REST API (spec §6 additions)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/savegame` | session, **active** | Download latest autosave for active session. `Content-Type: application/octet-stream`, `Content-Disposition: attachment` |
| GET | `/api/savegame?saveName=` | session, active | Download named save (must appear in enumerate list) |
| GET | `/api/savegame/list` | session, active | Metadata list for active session |
| POST | `/api/settings/game-api/test` | admin | `{ gameApiHost, gameApiPort, gameApiToken? }` → probe only |
| PUT | `/api/settings` | admin | Extend body with `gameApiHost`, `gameApiPort`, `gameApiToken?`, `clearGameApiToken?` |

Errors:

| Status | When |
|--------|------|
| 503 | API not configured or unreachable |
| 404 | No saves / save name not found |
| 502 | Upstream API error |

---

## Discord bot

### Slash command

```
/savegame [save_name] [delivery:dm]
```

| Option | Required | Notes |
|--------|----------|-------|
| `save_name` | No | Default: latest autosave for active session |
| `delivery` | No | If true, DM file (default **true** for parity with `/connection get`); if false, ephemeral attach in channel (like `/mods export` without DM) |

**Gating:** `CanRunCommand` — same group as `/connection get` / `/mods` (**active** linked users, not inactive).

**Handler flow** (`backend/internal/discord/savegame_cmds.go`):

1. Resolve save via `savegame.Service`
2. If `len(data) <= 25*1024*1024` → `sendUserDMWithFile` (existing helper in `dm.go`)
3. Else → DM with link: `{FACTORYMATE_PUBLIC_URL}/connection` + instructions to use dashboard download (do not attempt attach)
4. Ephemeral ack: “Save sent to your DMs” / “Save too large — link sent”
5. `LogBotCommand` audit row

Register in `slashCommands()`; re-register on guild save (existing M17 behavior).

---

## Web UI

**Page:** `/connection` (extend `ConnectionDetailsView`)

- Card: **“Download save”**
- Shows: active session name, selected save name, save date, file size (from list metadata or `Content-Length` after HEAD-less enumerate)
- Primary button: **Download latest autosave** → `fetch(apiUrl('/savegame'), { credentials: 'include' })` → blob download (same pattern as `mods-view.tsx` SMM profile)
- Optional: `<Select>` of recent autosaves from `GET /api/savegame/list`
- Disabled state + alert when API not configured (admin: link to Settings → Connection)

**i18n:** new keys under `connection.savegame` or `savegame` namespace in `messages/en.json`.

---

## Security

| Risk | Mitigation |
|------|------------|
| Save contains full factory state | **Active users only** (same as connection details); not admin-only unless group prefers stricter — **default: active user** |
| API token leakage | Store in DB; never in GET settings JSON; never log; admin PUT only |
| Token grants admin API powers | Document: use dedicated API token; never expose other API functions from FM |
| Rate abuse | Soft limit: max **1 download / user / 5 minutes** (in-memory or SQLite `savegame_download_log`) |
| Wrong save served | Enumerate whitelist before `DownloadSaveGame` |

---

## Implementation checklist

### Backend

- [ ] Migration `013_savegame_api.sql` — `app_settings` columns
- [ ] Seed from `SATISFACTORY_SERVER_*` env in `db/seed.go` or startup
- [ ] `internal/savegame/client.go` — HTTP client + types
- [ ] `internal/savegame/service.go` — selection logic, `DownloadLatest`, `DownloadByName`, `ListSaves`, `TestConnection`
- [ ] `internal/savegame/service_test.go` — httptest mock API (JSON + octet-stream fixtures)
- [ ] `api/savegame_handlers.go` — GET list + download
- [ ] Extend `api/admin_handlers.go` — settings GET/PUT + test endpoint
- [ ] Wire routes in `api/routes.go`
- [ ] `cmd/server/main.go` — construct service

### Discord

- [ ] `discord/savegame_cmds.go` — handler + size guard
- [ ] Register `/savegame` in `commands.go`
- [ ] Tests with mock service / mock discordgo where feasible

### Frontend

- [ ] `api-types.ts` — `SavegameList`, settings fields
- [ ] `connection-details-view.tsx` — download card + button
- [ ] `connection-settings-form.tsx` or new settings section — API host/port/token + test
- [ ] `messages/en.json` — strings

### Docs / spec

- [ ] Update `docs/factorymate-spec.md` §6 endpoints, §8.1 `/connection` row, §9 env vars, §1.2 exception note
- [ ] Update `.env.example` with `SATISFACTORY_SERVER_*`
- [ ] Update `docs/guide/managing/settings.md` or new guide section
- [ ] Roadmap entry (new milestone or M14 item)

### Manual test plan

1. Settings → configure API host/port/token → Test succeeds
2. `/connection` → Download → valid `.sav` opens in SMM / Satisfactory
3. Discord `/savegame` → DM receives file
4. Discord `/savegame` with unconfigured API → clear error
5. Inactive user → 403 web + bot denial

---

## Open decisions (defaults chosen)

| Question | Default |
|----------|---------|
| Admin-only vs active-user? | **Active user** (matches `/connection get`) |
| Reuse `gameHost`/`gamePort` from connection details? | **Separate `game_api_*`** fields; optional fallback to connection host + port 7777 |
| Which save by default? | **Latest autosave** by `saveDateTime` for `activeSessionName` |
| Discord when file > 25 MiB? | **DM link to dashboard**, no attach |

---

## Estimated scope

~2–3 focused implementation sessions: backend client + API + Discord (~1), frontend + settings (~0.5), tests + docs (~0.5).
