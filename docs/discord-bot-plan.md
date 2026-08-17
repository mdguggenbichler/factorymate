# FactoryMate Discord Bot — Planning Document

**Status:** Implemented v13 (M15 + M16 verified 2026-08-17)  
**Last reviewed:** 2026-08-17 — SQLite migration pattern + secrets-in-logs policy  
**Date:** 2026-08-17  
**Scope:** Unified Discord bot for onboarding, game-event notifications, connection details, and slash commands — single-container deployment, **no webhooks**.

---

## 1. Executive Summary

FactoryMate today uses **admin-created web invite links** for account creation and **outbound Discord webhooks** for game-event notifications. There is no external chat identity, no game join credentials storage, and no interactive Discord surface.

This plan introduces a **single Discord bot** (same Go process, same container) that becomes the **only** Discord integration:

1. Users self-register via a Discord slash command, automatically capturing an **external identity ↔ FactoryMate** mapping.
2. During registration, users provide their **in-game player name** (always saved; auto-linked when they appear on the server).
3. Optional **registration approval** (`auto_approve` on by default) before dashboard access and connection credentials.
4. Admins and authorized users can **set and retrieve game connection details** (host, port, optional client password) via Discord commands and the web UI.
5. When connection details change, all **active** linked players receive an automatic DM broadcast.
6. **Game-event notifications** are delivered by the **same bot** to configured channels (replacing webhooks), with optional per-user DM opt-in by category (M16).
7. **Server mod list** (`getModList`) on web `/mods` and Discord `/mods` — full list with game build + SML version.
8. **SMM profile export** — one-click `.smmprofile` download for Satisfactory Mod Manager import.

**Self-host setup becomes:** one Discord application → one bot token → one invite link → pick channels in the UI. No webhook URLs.

**Abstraction strategy:** extend the existing `Provider` interface for all **outbound messaging** (channel posts + DMs). Keep **interactive bot logic** Discord-specific (`internal/discord/`). Use **generic identity columns** in the DB so a future Slack target is possible without redesigning users — but do **not** build a shared command framework until a second platform is actually needed.

---

## 2. Goals

| # | Goal |
|---|------|
| G1 | Replace web-only invite creation with a Discord command that gates registration and captures external user ID at signup time |
| G2 | Prompt new users for in-game username during onboarding; always save `pending_player_name`; auto-link when player appears on server |
| G3 | Allow admins to view and override all identity mappings (external ID, FM username, player mapping) in the web UI |
| G4 | Store and manage **game join connection details** separately from FRM monitoring settings |
| G5 | Expose get/set connection details via web UI and Discord commands |
| G6 | DM all registered players when connection details change, regardless of change source |
| G7 | Run the bot inside the existing single-container stack (Go backend goroutine) |
| G8 | Define a clear, maintainable permission model for bot commands |
| G9 | **Single Discord setup for self-hosters** — one application, one bot token, one invite; no separate webhook URLs |
| G10 | **Refactor notification transport** — bot channel posts replace webhooks; keep template engine + dispatcher |
| G11 | **Pending player mapping** — save in-game name at registration even if not on server; auto-link when player appears |
| G12 | **Hybrid notification routing** — admin channel config + per-user DM opt-in by category |
| G13 | **Optional registration approval** — `auto_approve` default on; when off, admin approve/reject via Discord DM buttons |
| G14 | **Server mod list** — web page + `/mods` Discord command from FRM `getModList` (full list + game build + SML) |
| G15 | **SMM profile export** — downloadable `.smmprofile` generated from live mod list + ficsit.app lockfile resolution |

## 3. Non-Goals (this initiative)

- Discord OAuth login for the web dashboard (session cookies remain; Discord is identity for bot flows, not web SSO)
- Replacing FRM polling or game-event detection
- Multi-guild bot deployments
- Write access to the Satisfactory server via FRM
- Fine-grained per-dashboard-page permissions (admin vs viewer stays as today)
- Separate bot container or sidecar process
- **Discord incoming webhooks** — removed; bot posts to channels instead
- **Generic interactive bot framework** — no `CommandProvider` interface until a second platform (e.g. Slack) is actually built
- **TeamSpeak interactive bot** — out of scope; TS would be notification-only if ever added (see §5.4 abstraction table)

---

## 4. Current State

### 4.1 What exists today

| Area | Implementation | Key paths |
|------|----------------|-----------|
| Invites | Admin creates 7-day single-use token; invitee visits `/invite/:token`, sets username + password | `backend/internal/auth/invite.go`, `frontend/components/invite-accept-form.tsx` |
| Users | `admin` / `viewer` roles; optional `player_id` → `player_state` (admin-assigned) | `users` table, `users-view.tsx` |
| Discord | Outbound webhooks — rich embeds for 13 game event types | `backend/internal/notify/discord.go`, Settings → Notifications |
| FRM connection | `frm_host`, `frm_port`, `frm_auth_token` in `app_settings` | `/settings/general` |
| Notification abstraction | `Provider` interface + dispatcher + templates | `backend/internal/notify/types.go`, `dispatch.go` |
| Container | Go `:8080` + Next.js `:3000` in one image | `Dockerfile`, `scripts/docker-entrypoint.sh` |

### 4.2 Gaps this plan addresses

| Gap | Plan |
|-----|------|
| No external chat identity on users | Generic `external_platform` + `external_user_id` columns |
| Double Discord setup (webhook + bot) | Unified bot for channel posts and DMs |
| Manual webhook URL paste | Channel picker populated via bot API |
| Manual invite URL copy/paste | Discord `/register` command |
| No game join credentials | New `connection.details_json` settings |
| No server mod list for players | FRM `getModList` → `/mods` page + `/mods` command |
| No registration approval gate | Optional `registration.auto_approve` (default on) |
| No DM capability | `Provider.SendDirect()` via bot REST API |
| No Discord-side permission gating | Discord role → command permission mapping in settings |

---

## 5. Architecture

### 5.1 Single-container layout

```
┌─────────────────────────────────────────────────────────────┐
│  factorymate container (:3000)                              │
│                                                             │
│  ┌──────────────┐   ┌─────────────────────────────────────┐ │
│  │ Next.js      │   │ Go backend                          │ │
│  │ :3000        │──▶│ :8080 REST API                      │ │
│  │ /api proxy   │   │ Poller                              │ │
│  └──────────────┘   │ Template renderer                   │ │
│                     │ Dispatcher ──▶ Provider interface   │ │
│                     │ SQLite                              │ │
│                     │ ┌─────────────────────────────────┐ │ │
│                     │ │ internal/discord/               │ │ │
│                     │ │ - discordgo gateway             │ │ │
│                     │ │ - Slash commands (interactive)  │ │ │
│                     │ │ - DiscordProvider (Send/SendDM) │ │ │
│                     │ └─────────────────────────────────┘ │ │
│                     └─────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
         │ HTTP GET (FRM)              │ HTTPS
         ▼                               ▼
   Game server FRM API            Discord API (Gateway + REST)
```

- **Library:** [`discordgo`](https://github.com/bwmarrin/discordgo).
- **Process model:** Bot starts in `main()` when `DISCORD_BOT_TOKEN` is set; graceful shutdown on SIGTERM alongside poller.
- **Shared services:** Registration, connection details, and role checks live in `auth` / `settings` packages — called by both REST handlers and `internal/discord/` command handlers.
- **Config:** Bot token via env; guild ID + role mappings via `app_settings` + admin UI.

### 5.2 Unified bot — why retire webhooks

| Concern | Webhooks (today) | Bot (target) |
|---------|------------------|--------------|
| Admin setup | Create webhook per channel in Discord UI + paste URL in FM | Bot token + invite once; pick channel in FM UI |
| Game event alerts | HTTP POST with embed JSON | `ChannelMessageSendEmbed` — same embed payload |
| User registration | ❌ | ✅ Slash commands + modals |
| DMs (connection details, broadcast) | ❌ | ✅ `UserChannelCreate` + send |
| Ephemeral replies | ❌ | ✅ Interaction response |
| Per-target display name/avatar | ✅ `username_override` / `avatar_url_override` | ❌ Single bot identity (set in Developer Portal) |
| Requires gateway connection | ❌ | ✅ (already needed for commands anyway) |

**Conclusion:** For a self-hosted private group, **one bot beats webhook + bot**. The only meaningful loss is per-target username/avatar overrides — acceptable; configure the bot's display name and avatar once in the Discord Developer Portal.

**What stays unchanged:** Poller, diff engine, message type catalog, template editor, enable toggles, target assignment, `notification_log`. Only the last mile (`DiscordProvider.Send`) changes transport.

### 5.3 Notification transport refactor

**Before** (`discord.go` today):

```json
{ "webhook_url": "https://discord.com/api/webhooks/...", "username_override": "...", "avatar_url_override": "..." }
```

**After:**

```json
{ "channel_id": "123456789012345678", "thread_id": "optional" }
```

`DiscordProvider` holds a reference to the live `discordgo.Session` and implements:

```go
// Existing — channel posts (game events, test-send)
Send(ctx, target NotificationTarget, msg RenderedMessage) error

// New — direct messages (connection broadcast, /connection reply)
SendDirect(ctx, platform, externalUserID string, msg RenderedMessage) error
```

Embed building (`buildDiscordPayload`, `toDiscordAPIEmbed`, limit truncation) is **reused** — only the HTTP POST to a webhook URL is replaced.

**UI change:** Settings → Notifications → Targets shows a **channel dropdown** (fetched via `GET /api/discord/channels`) instead of a webhook URL field. Remove `username_override` and `avatar_url_override` fields.

### 5.4 Abstraction boundaries — what to generalize, what not to

**Principle:** *YAGNI for commands, DRY for messages.*

```
┌─────────────────────────────────────────────────────────┐
│  Poller + Templates + Dispatcher     (already abstract) │
└────────────────────────┬────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
  DiscordProvider   SlackProvider    TeamSpeakProvider?
  Send / SendDirect   (future)         (future, Send only)
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  internal/discord/          NOT behind generic interface │
│  - Gateway + slash commands                              │
│  - Registration / link / connection flows                │
│  - Role mapping (Discord-specific config)                │
└─────────────────────────────────────────────────────────┘
```

| Layer | Abstract? | Notes |
|-------|-----------|-------|
| **Outbound messaging** (channel + DM) | **Yes** — extend existing `Provider` | Slack can implement `Send()` with blocks; same dispatcher |
| **Template engine + message types** | **Keep as-is** | Already provider-agnostic |
| **Interactive commands** (`/register`, modals, role gating) | **No** — `internal/discord/` only | Slack would get its own `internal/slack/` when needed |
| **Identity in DB** | **Yes** — generic columns | `external_platform` + `external_user_id` from day one |
| **Platform-specific settings** | **No** — namespaced keys | `discord.guild_id`, `discord.role_mappings_json`; not a shared schema |
| **TeamSpeak** | **Notification-only if ever** | No slash commands, no embeds; unlikely to share bot framework |

**Do not** introduce a `ChatBotProvider` or `CommandProvider` interface with `RegisterUser()`, `HandleCommand()`, `MapRoles()` — each platform's interaction model is too different and the abstraction would leak immediately.

**When Slack arrives (hypothetical):** Add `SlackProvider` for notifications + `internal/slack/` for slash commands. Share only domain services (`auth`, connection details, template renderer) — not a unified bot framework.

### 5.5 Message categories — templated vs fixed

The webhook → bot refactor changes **transport only** for game-event notifications. Do **not** replace the template system with fixed messages.

| Category | Examples | Approach | Rationale |
|----------|----------|----------|-----------|
| **Templated** (keep full system) | All 13 poller message types (`player_joined`, `fuse_tripped`, …) | Existing dispatcher → `template.Render()` → `DiscordProvider.Send` / `SendDirect` (M16 DM fan-out) | Admins already customize embeds; test-send, `notification_log`, and future providers depend on this |
| **Fixed** (bot handlers) | `/register` modals, `/help`, approval buttons, `/mods list` embed, ephemeral errors | Hardcoded strings in `internal/discord/` (or i18n keys) | Predictable UX; admins must not be able to break slash-command flows by editing templates |
| **Hybrid** | Connection-detail DM broadcast | M15: fixed sensible default via `ConnectionDetailsService`; M16 optional `connection_details_changed` message type in template editor | Operational reliability first; customization when requested |

**What stays 1:1:** embed shape (title, description, color, fields, footer, timestamp), template editor UI, enable toggles, target assignment, validation limits, sample preview.

**What changes:** target config (`channel_id` instead of `webhook_url`); per-target username/avatar overrides removed (single bot identity).

### 5.6 Self-host admin setup (target UX)

```
1. Discord Developer Portal → create application → copy bot token
2. .env: DISCORD_BOT_TOKEN=...  DISCORD_GUILD_ID=...
3. Invite bot to server (OAuth link shown in Settings → Discord)
4. FactoryMate /setup or /login → first admin account
5. Settings → Discord → verify "Connected" + configure role mappings
6. Settings → Notifications → Targets → pick #factory-alerts channel
7. Settings → Notifications → Templates → assign message types to target
8. Done — no webhook URLs anywhere
```

---

## 6. Identity & Onboarding

### 6.1 Registration flow (replaces admin-created web invites for normal users)

```
User runs /register in Discord
        │
        ▼
Bot checks: guild member? allowed role? not already registered?
        │
        ▼
Bot opens Modal (single interaction — Discord allows one modal submit per flow):
  - "In-game username" (required)
  - "Dashboard password" (required, ≥8 chars)
        │
        ▼
Backend creates users row:
  - username derived from Discord display name (deduplicated)
  - external_platform = 'discord', external_user_id, external_username
  - pending_player_name = in-game name from modal (always saved)
  - player_id set only if player_state match found; otherwise NULL (§7.2)
  - role from Discord role mapping (default: viewer)
  - status = active OR pending_approval (§6.2)
        │
        ├─ auto_approve = true (default) ──▶ §6.1a
        └─ auto_approve = false ──────────▶ §6.2 (approval flow)
```

#### 6.1a Flow when approved immediately (`auto_approve = true`)

```
Bot replies ephemeral: registration summary (player linked or pending)
Bot DMs user: dashboard URL + "/connection" + "/mods" hints
User can log in and use /connection, /mods immediately
```

**Username derivation:** Prefer Discord `global_name` / `username`, sanitized (`[a-z0-9_-]`, max 32 chars) and deduplicated (`michael`, `michael-2`, `michael-3`, …). On collision: **auto-suffix only** — no extra modal field in v1 (§19 O5). Ephemeral confirmation shows the assigned FM username. Admin username rename is out of scope v1; suffix is permanent unless we add rename later.

**In-game name:** Always persisted as `pending_player_name`. Registration never fails because the player has not joined the server yet.

**Deprecate, don't delete immediately:** Web invite endpoints and `/invite/:token` remain for break-glass admin recovery. Admin web UI "Create invite" moves behind "Advanced".

### 6.2 Registration approval (optional gate)

Admins can require manual approval before a new registrant gets dashboard access or connection credentials. Controlled by a single setting:

| Setting | Key | Default | Meaning |
|---------|-----|---------|---------|
| **Auto-approve registrations** | `registration.auto_approve` | **`true`** | New `/register` users are active immediately |
| | `false` | Admin must approve before access is granted |

**Recommendation: default `auto_approve = true`.** For a private ~5–6 player group, Discord **role gating** (`allow_self_register` + mapped roles) is the primary control — adding mandatory approval on top creates friction for every new friend. Flip the toggle off when the guild is semi-public, the game password is sensitive, or admins want an explicit vetting step.

Configure via **web UI** (Settings → Discord → Registration) or **Discord admin command** `/registration auto-approve on|off` (same setting, either surface).

#### Flow when `auto_approve = true` (default)

See §6.1a — account `status = active` immediately.

#### Flow when `auto_approve = false`

```
User runs /register → modals (in-game name + password)
        │
        ▼
Backend creates users row with status = pending_approval
  (password hash stored; login blocked; /connection blocked)
        │
        ▼
Bot replies ephemeral: "Registration submitted — waiting for admin approval"
        │
        ▼
FOR EACH FM admin with linked `external_user_id`:
  DM with embed: registrant summary + [Approve] [Reject] buttons
        │
        ├─ Approve (button interaction)
        │     SET status = active
        │     DM registrant: welcome + dashboard URL + connection details + /mods hint
        │     Ephemeral confirm to approving admin
        │
        └─ Reject (button → optional reason modal)
              INSERT `registration_audit_log` (action = rejected)
              DELETE user row
              DM registrant: "Registration declined" + optional admin comment
              Ephemeral confirm to rejecting admin
```

#### Bypass rules (always auto-approve regardless of setting)

| Path | Why |
|------|-----|
| `/setup` (first admin) | Bootstrap — no approver exists |
| `/register @user` (admin-initiated) | Admin is explicitly vouching for them |
| Web break-glass invite accept | Existing invite flow; admin created the invite |
| Approve action in web UI | Same as Discord button |

#### Admin surfaces for pending registrations

| Surface | Purpose |
|---------|---------|
| Discord DM buttons | Primary — fast approve/reject on mobile |
| Web UI queue | **Required fallback** — Settings → Users → "Pending approvals" (admins without linked Discord, expired buttons, batch review) |
| `/registrations list` (admin, M16) | Discord command listing pending count + links |

#### Interaction details

- Approve/Reject use Discord **message components** (buttons) on the admin DM; `custom_id` encodes `registration_request_id` or `user_id`.
- Reject: second step opens a **modal** with optional "Reason" field (max 500 chars); empty reason = generic decline message.
- Buttons expire after 7 days; expired pending registrations remain in web UI queue.
- Only one admin needs to act; first approve/reject wins (optimistic lock on `users.status`).

### 6.3 Admin-initiated registration (M15)

`/register @user` — **admin / server-admin only** (Discord `admin` command group or FM admin with linked Discord).

```
Admin runs /register user:@michael
        │
        ▼
Bot validates: target is guild member, not already registered/linked
        │
        ▼
Bot DMs @michael: "An admin invited you — tap Complete Registration"
  (button opens the same modals as self-/register: in-game name + password)
        │
        ▼
Same backend path as §6.1 → always status = active (§6.2 bypass)
        │
        ▼
Admin gets ephemeral confirm; target gets welcome DM on completion
```

**Why include in M15:** Registration flow is already being built; this is one slash command + a DM-with-button pattern. Admins vouch for the user by initiating — we do not collect passwords on the admin's behalf.

**Permission:** Discord role in `admin` command group **or** FM `admin` role (if linked). Not available to viewers.

### 6.4 Linking existing web-only accounts

`/link` command + modal (dashboard username + password) to attach external identity to an existing **active** account. One external account ↔ one FactoryMate user (enforced unique on `external_platform` + `external_user_id`). Does not apply to `pending_approval` rows — those must be approved/rejected first.

---

## 7. User Mappings

### 7.1 Three-layer identity model

```
External Chat User     FactoryMate User         In-game Player
──────────────────     ────────────────         ──────────────
external_user_id  ──▶  users.id            ──▶  player_state.player_id
external_username      users.username           player_state.name
(external_platform)
```

| Mapping | Set by | Overridable by admin (web UI) |
|---------|--------|-------------------------------|
| External ↔ FM user | `/register` or `/link` | ✅ Unlink / re-link / edit external fields |
| FM user ↔ player | Registration modal (name match) or `/set-player` | ✅ Existing player dropdown in Users settings |
| FM role (admin/viewer) | Discord role mapping at registration; admin promote | ✅ Existing role controls |

### 7.2 Player mapping lifecycle

Player mapping has three states. The in-game name from registration is **always saved** — missing server data must not block signup.

| State | `pending_player_name` | `player_id` | UI badge |
|-------|----------------------|-------------|----------|
| **Linked** | set (or cleared after link) | set | `Michael` |
| **Pending** | set | NULL | `Michael (pending)` |
| **Unlinked** | NULL | NULL | `—` |

#### Resolution paths (in priority order)

1. **Immediate match** (registration or `/set-player`): case-insensitive exact match on `player_state.name` → set `player_id`, keep `pending_player_name` as display fallback.
2. **Auto-link on poller ingest** (M15 — not deferred): whenever `player_state` is upserted or a `player_joined` event fires, run:
   ```
   FOR EACH user WHERE pending_player_name IS NOT NULL AND player_id IS NULL:
     IF player_state.name equals pending_player_name (case-insensitive):
       SET player_id = player_state.player_id
   ```
   Optionally DM the user on auto-link (M16 nice-to-have).
3. **Admin manual link** (web UI): assign `player_id` from dropdown, or pick an unmapped server player (§7.4).
4. **User correction** (`/set-player`): updates `pending_player_name` and re-runs immediate match.

#### Edge cases

| Case | Behaviour |
|------|-----------|
| Name not on server yet at registration | Pending state; auto-link when they first join |
| User typo in name | Stays pending until `/set-player` or admin fix |
| Two users claim same `pending_player_name` | Allowed while pending; first auto-link wins `player_id`. Second stays pending → admin resolves |
| Admin assigns player already linked to another user | Reject with clear error (one `player_id` ↔ one user) |
| Player renamed in-game (rare) | Mapping breaks; admin remaps or user `/set-player` |

#### Poller hook location

Add `auth.TryResolvePendingPlayers(ctx, db, playerID, playerName)` call in `poller/store.go` after `player_state` upsert — same transaction boundary as player state write, no separate background job required.

### 7.3 Web UI changes (Users settings)

Extend `/settings/users` table:

| Column | Notes |
|--------|-------|
| Platform / user | Discord avatar + `@username` (rendered per `external_platform`) |
| In-game player | Existing dropdown + pending state badge |
| Registration source | `discord` / `web_invite` / `setup` |
| Linked at | `external_linked_at` timestamp |
| Player mapping | Linked name, or `pending` badge with claimed in-game name |

### 7.4 Unmapped server players (admin)

New section on Users settings (or `/players` admin panel) listing `player_state` rows with **no** linked `users.player_id`:

| Server player | Last seen | Online | Action |
|---------------|-----------|--------|--------|
| `Guggi` | 2h ago | yes | Link to user… (dropdown of pending/unlinked FM users) |

This covers the reverse direction: a player appears on the server before anyone registers, or admin wants to attach an existing session to a FM account. Complements pending auto-link (user → server name) with admin-driven server player → user assignment.

---

## 8. Connection Details

### 8.1 Distinction from FRM settings

| Setting group | Purpose | Who needs it |
|---------------|---------|--------------|
| **FRM** (`frm_host`, `frm_port`, `frm_auth_token`) | FactoryMate polling the monitoring API | Admins only (General settings) |
| **Connection details** (new) | How players join the Satisfactory game | Active registered players (`status = active`) |

### 8.2 Fields

| Field | Required | Visible to players | Notes |
|-------|----------|-------------------|-------|
| `game_host` | yes | yes | Public hostname or IP |
| `game_port` | yes | yes | Default Satisfactory port (often `7777`) |
| `game_password` | no | yes (when set) | Client join password; admin-only to set |
| `notes` | no | yes | Free text, e.g. "Use Epic, not Steam" |
| `updated_at` | — | — | Audit |
| `updated_by_user_id` | — | admin | Audit |

Store as `connection.details_json` in `app_settings` — single JSON blob for atomic read/write.

### 8.3 Retrieval surfaces

| Surface | Auth | Delivery |
|---------|------|----------|
| `GET /api/connection-details` | FM session, `status = active` | JSON |
| Web UI | FM session, `status = active` | Admin: edit at `/settings/connection`; viewers: read-only same page or dashboard "How to join" card |
| `/connection` Discord command | Registered external user, `status = active` | **DM** (default) or **ephemeral** with `public` flag |
| `/connection set` | Discord admin permission | Updates + triggers DM broadcast |

**Blocked for `pending_approval`:** login, `/connection`, `/mods`, and all dashboard routes except a static "awaiting approval" page.

**Security:** Client passwords never posted to public channels. Default: DM. See §8.6 — **`game_password` must never appear in audit or debug logs** (only in authorized user-facing surfaces).

### 8.4 Change broadcast

When connection details change (REST `PUT /api/connection-details` or bot `/connection set`):

1. Persist in a single transaction.
2. For each user with `external_user_id` set, `status = active`, and FRM account not deleted:
   - `DiscordProvider.SendDirect("discord", external_user_id, msg)` with diff summary.
3. **Always sent** — connection-detail DMs are mandatory for all active linked users; not subject to `user_notification_prefs` opt-out in v1.
4. Rate-limit DMs (~5/sec); retry with backoff.
5. Log DM deliveries — extend `notification_log` (§12.5). `rendered_preview` for connection DMs must be **redacted** (no `game_password` value — §8.6).

Reuse template renderer for message body. Optional `connection_details_changed` message type in M16 for template editing in admin UI — delivery path stays the same.

### 8.6 Secrets in audit and debug logs

**Rule:** Never log the `game_password` field value in plaintext — not in `notification_log`, `bot_command_log`, `registration_audit_log`, HTTP access logs, structured debug output, or test assertion dumps.

| Surface | Allowed | Forbidden |
|---------|---------|-----------|
| User-facing DM (`/connection`, welcome on approval) | Full password when set (spoilers/`||…||` in Discord) | — |
| Web UI / API response (`GET /api/connection-details`) | Full password for `status = active` users | Logging the API response body |
| `notification_log.rendered_preview` | `"Connection details updated (password changed)"` or omit password line | `Password: hunter2` |
| `bot_command_log.detail` for `/connection set` | `"updated: game_host, game_port, game_password"` | Password value or JSON body with `game_password` |
| `PUT /api/connection-details` handler | Audit that password field was touched | Request body in logs |
| Unit/integration tests | Assert redaction helper; use fake passwords only in DM content fixtures | Asserting log rows contain password strings |

Implement a shared redaction helper (e.g. `connection.RedactForLog(details)`) used by `ConnectionDetailsService`, `DiscordProvider` audit hooks, and bot command logging. Connection-detail **DM message bodies** sent to users are exempt — only **server-side logs** are redacted.

**Dashboard passwords** (`users.password_hash`): never log registration/login request bodies; existing auth discipline unchanged.

### 8.5 Server mod list (FRM `getModList`)

FRM exposes **`GET /getModList`** — a read-only endpoint returning all mods installed on the dedicated server. Verified on the group's live server; no auth required on current deployment.

#### FRM response shape (per mod)

| Field | Example | UI use |
|-------|---------|--------|
| `Name` | `Depot Sorting` | Display name |
| `SMRName` | `FactoryGame`, `SML`, `DepotSorter` | Row key; detect game/SML entries |
| `Version` | `1.0.1` / `502094.0.0` | **Match this exactly on your client** |
| `RemoteVersionRange` | `>=1.0.1` | Secondary column (semver hint from mod author) |
| `RequiredOnRemote` | `true` / `false` | **Informational only** — see reliability note below |
| `Description` | … | Expand row / tooltip |
| `DocsURL` / `SupportURL` | … | Link out |
| `CreatedBy` | `SirDigby` | Secondary column |

Live example (Aug 2026): AutoSort 3.2.2, Depot Sorting 1.0.1, FRM 1.5.3, **SML 3.12.0**, **FactoryGame 502094.0.0** — five entries total.

#### Game version + SML (prominent header)

Extract from `getModList` and show **above** the table on web UI and at the top of Discord `/mods`:

| Derived field | Source row | Example |
|---------------|------------|---------|
| **Game build** | `SMRName == "FactoryGame"` → `Version` | `502094.0.0` |
| **SML version** | `SMRName == "SML"` → `Version` | `3.12.0` |

API envelope:

```json
{
  "gameBuild": "502094.0.0",
  "smlVersion": "3.12.0",
  "mods": [ /* all rows, unfiltered */ ],
  "cachedAt": "2026-08-17T14:30:00Z",
  "frmReachable": true
}
```

Map `gameBuild` to a human label in UI if/when we maintain a lookup (optional M16); **v1 shows raw build number only** — this matches the in-game title screen (e.g. `502094`). Build `502094` = patch **v1.2.4.0** per [Satisfactory Wiki](https://satisfactory.wiki.gg/wiki/Patch_1.2.4.0); no reliable live API for build→label mapping, so we do not fetch wiki data at runtime.

#### `RequiredOnRemote` — not reliable for join planning

Research summary (SML docs, FRM docs, hosting guides):

| Source | Finding |
|--------|---------|
| [SML ReleaseMod docs](https://docs.ficsit.app/satisfactory-modding/latest/Development/BeginnersGuide/ReleaseMod.html) | `RequiredOnRemote` is a **mod-author uplugin flag** (defaults `true`). Authors explicitly set `false` to opt out of bidirectional checks. |
| [SML 3.9+](https://docs.ficsit.app/satisfactory-modding/latest/Development/UpdatingFromSml38.html) | Join blocking uses `RequiredOnRemote` + `RemoteVersionRange` — but **only as declared by each mod author**. |
| [FRM docs](https://docs.ficsit.app/ficsitremotemonitoring/latest/index.html) | FRM lists `RequiredOnRemote: false`, yet states: *"If you're using this mod on a dedicated server, each client must have **SML** installed."* FRM is not required on the client — but the server is modded. |
| Community / hosting guides | Practical rule: **client and server must have the same full mod list and versions** (SMM profile export/import). Do not trust a filtered subset. |

**Why your FRM-less player couldn't join:** Any server-side mod can register content (pak assets). Join failures ("content block" / header mismatches) often mean the **client mod list doesn't mirror the server**, even when `RequiredOnRemote` is `false` on individual mods (e.g. FRM). SML 3.9+ also blocks joins when the client is missing mods the server has (per author flags) — but content sync issues can bite beyond that flag.

**Product decision:** **List every mod. No hiding. No "install only these" filter as default guidance.**

| Do | Don't |
|----|-------|
| Show full table: game build, SML, FRM, all content mods | Hide SML / FactoryGame by default |
| Show `RequiredOnRemote` as a passive column with tooltip disclaimer | Badge/filter implying "you only need these" |
| Banner: *"Install **all** mods below at matching versions (use SMM profile export from server)."* | Treat `RequiredOnRemote: false` as "optional on client" |

#### Polling strategy

Mods change rarely (only on server update). **Do not** add to fast/slow poll loops.

| Approach | Detail |
|----------|--------|
| Fetch | On-demand via `frm.Client.GetModList()` |
| Cache | In-memory TTL **15 minutes** (configurable); shared by API + Discord command |
| Refresh | `POST /api/mods/refresh` (admin) busts cache after server mod update |
| Offline | Return empty `mods`, preserve last `gameBuild`/`smlVersion` if cached; `frmReachable: false` |

#### Web UI — `/mods` (viewer + admin)

| Element | Detail |
|---------|--------|
| **Header cards** | Game build + SML version (large), last fetched timestamp |
| **Disclaimer banner** | Install all mods at listed versions; `RequiredOnRemote` is author-reported and may be wrong |
| **Table** | All rows — Name, Version, `RemoteVersionRange`, `RequiredOnRemote` (yes/no), Author, Docs |
| **Sort** | Name (default), version |
| **Optional filter** | Text search by name only — no "required only" quick filter |
| **Admin** | "Refresh now" button |
| **Download** | **"Download SMM profile"** button → `GET /api/mods/smmprofile` |

#### SMM profile export (M15)

Players need a one-click way to mirror the server mod list in Satisfactory Mod Manager. FactoryMate generates a `.smmprofile` file matching the format SMM imports (reference fixture: `docs/examples/smm_profiles/Default-2026-08-17-13-07-01.smmprofile`).

**Verified against live server (Aug 2026):** FRM `getModList` mods align with the example profile — AutoSort 3.2.2, DepotSorter 1.0.1, FRM 1.5.3, SML 3.12.0; `metadata.gameVersion: 502094` matches `FactoryGame` version `502094.0.0`.

| Input | Source |
|-------|--------|
| Mod names + pinned versions | FRM `getModList` — see profile vs lockfile rules below |
| `metadata.gameVersion` | Integer parsed from `FactoryGame.Version` (`502094.0.0` → `502094`) |
| Lockfile hashes + download links | [ficsit.app GraphQL API](https://api.ficsit.app/v2/query) — batch `resolveModVersions` per `SMRName` + `Version` (see §8.6) |

**Profile vs lockfile rules** (match `docs/examples/smm_profiles/Default-2026-08-17-13-07-01.smmprofile`):

| Section | Include | Exclude |
|---------|---------|---------|
| `profile.mods` | All server mods except `FactoryGame` and **`SML`** | `FactoryGame`, `SML` |
| `lockfile.mods` | All server mods except `FactoryGame` (includes **SML** + content mods) | `FactoryGame` only |

SML belongs in the lockfile (download pins) but not in `profile.mods` — same as a manual SMM export from this server.

**Generated shape** (matches example):

```json
{
  "profile": {
    "name": "FactoryMate Server",
    "required_targets": ["Windows", "LinuxServer"],
    "mods": {
      "AutoSort": { "version": ">=0.0.0", "enabled": true },
      "DepotSorter": { "version": ">=0.0.0", "enabled": true },
      "FicsitRemoteMonitoring": { "version": ">=0.0.0", "enabled": true }
    }
  },
  "lockfile": {
    "version": 1,
    "mods": {
      "AutoSort": { "version": "3.2.2", "targets": { "…": "…" } },
      "SML": { "version": "3.12.0", "targets": { "…": "…" } }
    }
  },
  "metadata": { "gameVersion": 502094 }
}
```

**Implementation notes:**

- `SMMProfileService`: FRM `getModList` → single GraphQL `resolveModVersions` batch → assemble JSON.
- **Do not** call `getModByReference` per mod (returns all versions; wasteful). Use `resolveModVersions` with exact version pins from FRM.
- GraphQL returns relative download paths (`/v1/version/…/Windows/download`); prefix with `https://api.ficsit.app` in the lockfile (matches SMM export format).
- Include all three lockfile targets per mod: `Windows`, `LinuxServer`, `WindowsServer`.
- Cache generated profile alongside mod list cache (invalidate together on refresh).
- If ficsit.app lookup fails for a mod: export fails with clear error listing which mod could not be resolved (do not emit a broken profile).
- Golden test: generated output for live fixture mods must match `docs/examples/smm_profiles/Default-2026-08-17-13-07-01.smmprofile` structure; lockfile hashes may be compared against fixture when using same server state.
- Profile name default `"FactoryMate Server"`; admin-configurable in Settings → Connection (optional string).

**Verified GraphQL query** (Aug 2026 — hashes match fixture):

```graphql
query ResolveServerMods($filter: [ModVersionConstraint!]!) {
  resolveModVersions(filter: $filter) {
    mod_reference
    versions {
      id
      version
      targets { targetName hash link }
    }
  }
}
```

Variables (built from FRM `getModList`, excluding `FactoryGame`):

```json
{
  "filter": [
    { "modIdOrReference": "AutoSort", "version": "3.2.2" },
    { "modIdOrReference": "DepotSorter", "version": "1.0.1" },
    { "modIdOrReference": "FicsitRemoteMonitoring", "version": "1.5.3" },
    { "modIdOrReference": "SML", "version": "3.12.0" }
  ]
}
```

- Endpoint: `POST https://api.ficsit.app/v2/query` (`Content-Type: application/json`).
- No auth required for read queries.
- API docs: [ficsit.app/api-docs](https://ficsit.app/api-docs); playground at [api.ficsit.app/v2](https://api.ficsit.app/v2).

**Surfaces:**

| Surface | Delivery |
|---------|----------|
| Web `/mods` | "Download SMM profile" button |
| `GET /api/mods/smmprofile` | `Content-Disposition: attachment; filename="factorymate-server.smmprofile"` |
| Discord `/mods export` | Bot calls `SMMProfileService` internally; attach `.smmprofile` to ephemeral reply or DM (see §22.2 — no session cookie in Discord) |

#### Discord — `/mods` command

| Aspect | Choice |
|--------|--------|
| Permission | Active registered user |
| Subcommands | `list` (default) — embed with all mods; `export` — SMM profile download link |
| Delivery | Ephemeral (default) or DM via `delivery:dm` option |
| Format | Embed: `Game build: …` / `SML: …` in description; one line per mod (`Name — Version`); link to web `/mods` for full detail |
| Long lists | Paginate or link to dashboard — still list everything, no subset |
| Welcome DM | *"Use `/mods` for the mod list; `/mods export` for an SMM profile to import."* |

#### API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/mods` | session, `status = active` | Full cached list + `gameBuild`, `smlVersion`, `cachedAt`, `frmReachable` |
| GET | `/api/mods/smmprofile` | session, `status = active` | Downloadable `.smmprofile` (generated per §8.5) |
| POST | `/api/mods/refresh` | admin | Force re-fetch from FRM + bust SMM cache |

#### Backend types

```go
type Mod struct {
    Name               string `json:"name"`
    SMRName            string `json:"smrName"`
    Version            string `json:"version"`
    Description        string `json:"description,omitempty"`
    DocsURL            string `json:"docsUrl,omitempty"`
    SupportURL         string `json:"supportUrl,omitempty"`
    CreatedBy          string `json:"createdBy,omitempty"`
    RemoteVersionRange string `json:"remoteVersionRange,omitempty"`
    RequiredOnRemote   bool   `json:"requiredOnRemote"` // informational; do not use for filtering
}

type ModListResponse struct {
    GameBuild     string `json:"gameBuild"`
    SMLVersion    string `json:"smlVersion"`
    Mods          []Mod  `json:"mods"`
    CachedAt      string `json:"cachedAt"`
    FRMReachable  bool   `json:"frmReachable"`
}
```

No SQLite persistence in v1 — mod list + SMM profile cache is ephemeral (in-memory, shared TTL).

#### Integration points

- **Welcome / approval DM:** `/mods` + `/mods export` + "import profile in SMM before joining."
- **Registration flow:** informational only — not a compatibility gate.
- **Test fixture:** `docs/examples/smm_profiles/Default-2026-08-17-13-07-01.smmprofile`
- **Spec / roadmap:** add `getModList` to `factorymate-spec.md` §4.1 + `/mods` route in §8.1 when implementing.

#### ficsit.app API compliance (§8.6)

FactoryMate uses the public SMR GraphQL API **only** to resolve mod version metadata (version IDs, per-target hashes, download links) when generating an SMM profile. It does **not** bulk-download mod binaries.

**Relevant terms** ([ficsit.app/tos](https://ficsit.app/tos), last updated 2020-06-25):

| Rule | FactoryMate posture |
|------|---------------------|
| API may not be used in **closed-source** applications | **Compliant** — FactoryMate is MIT-licensed open source (`LICENSE`) |
| Use services **reasonably and as intended** | **Compliant** — same metadata resolution SMM performs for profile import; helps players install mods from SMR |
| **Fair use:** no automated bulk download/recording that drastically exceeds expected average | **Compliant** — one batched `resolveModVersions` call per export/refresh; cache result; no mod file mirroring |
| Do not redistribute mod binaries without permission | **Compliant** — profile contains official SMR download URLs; SMM downloads on the player's machine |

**Operational guardrails (implementation):**

1. Batch all mods in a single `resolveModVersions` request (typically 3–5 mods).
2. Cache profile + mod list together; only re-query on admin refresh or TTL expiry.
3. Do not scrape all mods/versions, prefetch unrelated metadata, or proxy mod downloads through FactoryMate.
4. Fail clearly if a mod is not on SMR (private/git-only mods cannot be exported).
5. Keep FactoryMate source open if distributing binaries/images that use this integration.

**Risk assessment:** Low for a self-hosted private server. Volume is comparable to one SMM user exporting a profile. If FactoryMate is ever used at large scale, consider a courtesy heads-up to ficsit.app staff via [their Discord](https://discord.gg/xkVJ73E) — not required for homelab use.

**Verdict:** Use case is **allowed** under current ToS, provided FactoryMate remains open source and API usage stays low-volume with caching.

---

## 9. Game Event Notification Routing

### 9.1 Can messages go to specific users only?

**Yes.** With a linked bot, the dispatcher has two delivery modes:

| Mode | API | Who sees it | Requires |
|------|-----|-------------|----------|
| **Channel** | `Send(channel_id, msg)` | Everyone in that channel | Admin assigns message type → channel target |
| **DM** | `SendDirect(external_user_id, msg)` | Only that Discord user | User has linked external identity + opted in |
| **@mention in channel** | `Send` with `<@id>` in content | Everyone in channel (mention highlights one person) | Poor fit for alerts — avoid for game events |

The poller fires asynchronously, so per-user routing is a **dispatcher concern**: after rendering a message, fan out to channel targets and/or filtered DM recipients.

### 9.2 Recommendation: hybrid — admin channels + optional per-user DMs

For a ~5–6 player private group, **do not** make every message type fully per-user configurable in v1 — too much UI for little gain. Use two layers:

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 1 — Admin (global)          Layer 2 — User (opt-in)  │
│  Which types → which channels        Which types → my DMs     │
│  Settings → Notifications            Account → Notifications  │
│  (existing model, channel picker)    (new, category toggles)  │
└─────────────────────────────────────────────────────────────┘
```

| Layer | Who configures | What it controls | Default |
|-------|----------------|------------------|---------|
| **Channel routing** | Admin | Message type → `#factory-alerts` (etc.) | Unchanged — global stream everyone in channel sees |
| **DM preferences** | Each user | Whether *they* also get DMs for selected categories | Conservative defaults (see below) |

**Why both?** Channel is the shared "factory ticker" — one place admins tune for the group. DMs are for people who want alerts on their phone without watching the channel, or for inherently personal messages.

### 9.3 Message categories and default DM prefs

Group message types into **categories** (not 13 individual toggles per user):

| Category | Message types | Channel (admin) | DM default (user) |
|----------|---------------|-----------------|-------------------|
| `server` | `server_online`, `server_offline` | ✅ typical | off |
| `player` | `player_joined`, `player_left` | ✅ typical | off |
| `power` | `fuse_tripped`, `power_restored` | ✅ typical | off |
| `progression` | milestone, hard drive, elevator, research | ✅ typical | off |
| `vehicle` | train, fuel, stuck | optional | off |
| `account` | `connection_details_changed` (M16 template only, if added) | n/a (no channel) | n/a — connection DMs are mandatory via `ConnectionDetailsService` (§8.4), not this pref |

New users inherit admin-configured **DM defaults** (stored in `app_settings`). Users override via web UI (`/account/notifications`) or `/notifications` Discord command (M16).

### 9.4 Personal routing (M16) — "only notify me about MY player"

Once `player_id` is linked, the dispatcher can optionally send a **personal DM** for player events:

| Event | Channel post | Personal DM |
|-------|--------------|-------------|
| `player_joined` — Michael | "Michael joined (4 online)" to `#alerts` | DM to user linked as Michael: "Your character joined" |
| `player_joined` — someone else | same channel post | no DM to Michael |
| `fuse_tripped` | channel only | only if user enabled `power` DM category |

Implementation: on dispatch, if message type is `player_joined` / `player_left` and event `{PlayerName}` matches a user's linked `player_state.name` (or resolved `pending_player_name`), and user has `dm_player_personal` enabled → `SendDirect` to that user.

**Match uses linked `player_id` first**, then `pending_player_name` as fallback for personal routing before auto-link completes.

### 9.5 Dispatcher flow (target)

**M15:** channel `Send` only for game events + dedicated `ConnectionDetailsService` for connection DMs.  
**M16:** full DM fan-out below for game events per user prefs.

```
Poller event → render template
        │
        ├─▶ Channel targets (admin config, message_type_targets)     [M15]
        │       └─▶ DiscordProvider.Send(channel_id, msg)
        │
        └─▶ DM recipients (user prefs + personal routing rules)      [M16]
                └─▶ FOR EACH eligible user: SendDirect(platform, external_user_id, msg)
```

Channel and DM are **independent** — a message type can go to channels only, DMs only, or both. Audit logging records channel sends via `notification_log` (existing) and DM sends via extended schema (§12.5).

### 9.6 What we are not doing (v1)

- Per-user control over which **channels** receive messages (admin-only)
- Per-user template customization
- 13 individual per-type DM toggles (use categories instead)

---

## 10. RBAC — Permission Model

### 10.1 Layered permissions

| Layer | Governs | Mechanism |
|-------|---------|-----------|
| **FactoryMate role** (`admin` / `viewer`) | Web dashboard, REST mutating endpoints | Existing — unchanged |
| **Discord command permissions** | Who can run slash commands | Discord role ID mapping in Settings → Discord |

### 10.2 Discord role mapping

```json
{
  "guild_id": "123456789",
  "role_mappings": [
    { "discord_role_id": "111", "fm_role": "admin", "bot_commands": ["admin", "register", "player", "connection", "mods"] },
    { "discord_role_id": "222", "fm_role": "viewer", "bot_commands": ["register", "player", "connection", "mods"] }
  ],
  "default_fm_role": "viewer",
  "default_bot_commands": [],
  "allow_self_register": true,
  "admin_discord_role_ids": ["111"]
}
```

`registration.auto_approve` is a **separate** `app_settings` key (§12.4), not inside `role_mappings_json`.

**Command access rules:**

| User state | `/register` | `/register user` | `/connection`, `/mods` | Admin commands |
|------------|-------------|------------------|------------------------|----------------|
| Unregistered member | If role allows | Admin only | ❌ | ❌ |
| `pending_approval` | ❌ | ❌ | ❌ | ❌ |
| `active`, linked | ❌ | Admin only | ✅ | If admin role |
| `active`, not linked | ❌ | Admin only | ❌ | ❌; use `/link` |

**Fallback:** `DISCORD_ADMIN_ROLE_IDS` env var until UI config exists.

### 10.3 Command registration

Register slash commands **per guild** (instant updates during development). `guild_id` in settings.

---

## 11. Proposed Discord Commands

### 11.1 Core commands (M15)

| Command | Permission | Description |
|---------|------------|-------------|
| `/register` | Member with `register` permission; not already linked | Self-serve account; modals for in-game name + password |
| `/register user` | Admin (`admin` command group) | DM target user to complete registration; always auto-approved (§6.3) |
| `/link` | Discord user not yet linked; has existing active FM account | Attach Discord to existing account |
| `/set-player <name>` | Active registered user | Update in-game player mapping |
| `/connection` | Active registered user | DM current join details |
| `/connection set` | Admin | Set join details; triggers DM broadcast |
| `/mods` | Active registered user | `list` (default): full mod table; `export`: SMM profile download |
| `/whoami` | Anyone | Link status: FM username, role, mapped player, approval state |
| `/help` | Anyone | Command list + dashboard URL + onboarding steps (see Appendix G) |
| `/registration auto-approve` | Admin | Toggle `registration.auto_approve` (`on` / `off`) |

### 11.2 Admin / optional (M16)

| Command | Permission | Description |
|---------|------------|-------------|
| `/registrations list` | Admin | Pending approval queue summary |
| `/registrations approve\|reject` | Admin | CLI fallback if DM buttons expired |
| `/status` | Active registered user | Server online, player count (from FRM cache) |
| `/players` | Active registered user | Who is online |
| `/unlink @user` | Admin | Remove external link (keeps FM account) |
| `/broadcast <message>` | Admin | Admin DM all registered players |
| `/sync-roles` | Admin | Re-apply Discord → FM role mapping |
| `/password-reset @user` | Admin | Trigger password reset flow |
| `/notifications` | Active registered user | View/toggle DM category preferences |

### 11.3 Out of scope (v1 bot)

- In-game actions via FRM write endpoints
- Per-message-type DM toggles (use categories instead — §9.3)
- Generic command framework for other platforms

---

## 12. Data Model Changes

### 12.0 SQLite migration notes

FactoryMate uses SQLite. Migrations may use **`ALTER TABLE … ADD COLUMN`** (supported). SQLite does **not** support `ALTER COLUMN … DROP NOT NULL`, `MODIFY`, or dropping columns in place.

When a migration needs to change nullability, constraints, or column types, use the **table-rebuild pattern**:

1. `CREATE TABLE …_new` with the target schema  
2. `INSERT INTO …_new SELECT … FROM …` (map defaults for new columns)  
3. `DROP TABLE …`  
4. `ALTER TABLE …_new RENAME TO …`  
5. Recreate indexes if any

Apply this in §12.5 (`notification_log.target_id` nullable). Do not assume portable PostgreSQL-style `ALTER COLUMN` syntax anywhere in migrations.

### 12.1 `users` table additions

```sql
-- Generic external identity (portable to Slack etc.)
ALTER TABLE users ADD COLUMN external_platform TEXT
    CHECK (external_platform IS NULL OR external_platform IN ('discord', 'slack'));
ALTER TABLE users ADD COLUMN external_user_id TEXT;
ALTER TABLE users ADD COLUMN external_username TEXT;
ALTER TABLE users ADD COLUMN external_display_name TEXT;
ALTER TABLE users ADD COLUMN external_linked_at TEXT;
CREATE UNIQUE INDEX idx_users_external_identity
    ON users(external_platform, external_user_id)
    WHERE external_user_id IS NOT NULL;

ALTER TABLE users ADD COLUMN pending_player_name TEXT;
ALTER TABLE users ADD COLUMN registration_source TEXT NOT NULL DEFAULT 'web_invite'
    CHECK (registration_source IN ('setup', 'web_invite', 'discord'));
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('pending_approval', 'active'));
ALTER TABLE users ADD COLUMN dm_player_personal BOOLEAN NOT NULL DEFAULT 0;  -- M16 personal player DMs (§19 O6)
```

`status = pending_approval` blocks login (`RequireSession` + `RequireActiveUser` middleware) and all player commands. Rejected registrations: write `registration_audit_log`, then **DELETE** the user row (§19 O12) — no `rejected` status persisted.

```sql
CREATE TABLE registration_audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,                    -- nullable if row deleted on reject
    external_user_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('submitted', 'approved', 'rejected')),
    acted_by_user_id INTEGER REFERENCES users(id),
    comment TEXT,
    created_at TEXT NOT NULL
);
```

### 12.2 `user_notification_prefs` table (new, M16)

Per-user DM opt-in by category (§9.3). Absent row = use admin default from `app_settings`.

```sql
CREATE TABLE user_notification_prefs (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('server', 'player', 'power', 'progression', 'vehicle')),
    dm_enabled BOOLEAN NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, category)
);

-- Personal "your player joined/left" DM — on users table (§19 O6)
-- (declared above with users ALTER)
```

Admin defaults in `app_settings`: `notifications.dm_defaults_json` e.g. `{ "player": false, "power": false, ... }` plus `notifications.dm_player_personal_default` (bool, default `false`). Used starting M16. **Does not affect** mandatory connection-detail DMs (§8.4). Connection-detail broadcasts stay **outside** `message_types` / dispatcher in M15; optional `connection_details_changed` template type is M16-only if we add it (§19 O11).

### 12.3 `notification_targets` config migration

**Migration script** for existing deployments:

```sql
-- config_json shape changes from webhook_url to channel_id
-- Manual step: admin re-selects channel in UI if auto-migration not possible
```

If `webhook_url` present and bot connected: **do not auto-migrate** — show banner on Settings → Notifications → Targets: *"Webhook targets are deprecated. Re-select a channel for each target."* Safe path; no guessing channel from webhook metadata (§19 O8).

### 12.4 `app_settings` additions

| Key | Type | Notes |
|-----|------|-------|
| `discord.bot_enabled` | bool | Kill switch |
| `discord.guild_id` | string | Target guild; **wins over** `DISCORD_GUILD_ID` env when non-empty (§19 O13) |
| `discord.role_mappings_json` | JSON | See §10.2 |
| `registration.auto_approve` | bool | Default `true` — see §6.2 |
| `connection.details_json` | JSON | Host, port, password, notes, updated_at, updated_by — stored in DB for authorized retrieval; **never log password field** (§8.6) |
| `mods.smm_profile_name` | string | SMM export profile name (default `"FactoryMate Server"`) |
| `notifications.dm_defaults_json` | JSON | Default DM category toggles for new users (M16) |
| `notifications.dm_player_personal_default` | bool | Default for `users.dm_player_personal` on new registrations (M16, default `false`) |

Bot token: **env var only** (`DISCORD_BOT_TOKEN`) — never in SQLite.

### 12.5 Notification audit logging (schema extension)

Current `notification_log` requires `target_id` (channel targets only). DM deliveries need nullable `target_id` plus new columns — **requires table rebuild** (§12.0):

```sql
-- Migration: extend notification_log for DM deliveries (SQLite table rebuild)

CREATE TABLE notification_log_new (
    id INTEGER PRIMARY KEY,
    message_type_key TEXT NOT NULL,
    target_id INTEGER,                    -- nullable for DM rows (was NOT NULL)
    rendered_preview TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    sent_at TEXT NOT NULL,
    delivery_mode TEXT NOT NULL DEFAULT 'channel'
        CHECK (delivery_mode IN ('channel', 'dm')),
    recipient_external_user_id TEXT       -- set when delivery_mode = 'dm'
);

INSERT INTO notification_log_new (
    id, message_type_key, target_id, rendered_preview, success, error, sent_at,
    delivery_mode, recipient_external_user_id
)
SELECT
    id, message_type_key, target_id, rendered_preview, success, error, sent_at,
    'channel', NULL
FROM notification_log;

DROP TABLE notification_log;
ALTER TABLE notification_log_new RENAME TO notification_log;
```

DM rows use `delivery_mode = 'dm'`, `recipient_external_user_id` set, `target_id` NULL. Channel rows unchanged. **`rendered_preview` must not contain `game_password` values** (§8.6).

**`bot_command_log`** (§12.6) is for slash-command auditing, not notification delivery.

### 12.6 Bot command audit

```sql
CREATE TABLE bot_command_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_platform TEXT NOT NULL DEFAULT 'discord',
    external_user_id TEXT NOT NULL,
    command_name TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    detail TEXT,                          -- field names / outcomes only; never game_password value (§8.6)
    created_at TEXT NOT NULL
);
```

---

## 13. API Changes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/mods` | session, active | Cached mod list + `gameBuild`, `smlVersion`, `cachedAt`, `frmReachable` |
| GET | `/api/mods/smmprofile` | session, active | Downloadable `.smmprofile` file |
| POST | `/api/mods/refresh` | admin | Re-fetch FRM + bust mod/SMM cache |
| GET | `/api/connection-details` | session, active | Join details |
| PUT | `/api/connection-details` | admin | Update; triggers DM broadcast |
| GET | `/api/discord/settings` | admin | Guild ID, role mappings, bot status |
| PUT | `/api/discord/settings` | admin | Update mappings + `registration.auto_approve` |
| GET | `/api/registrations/pending` | admin | Pending approval queue |
| POST | `/api/registrations/:id/approve` | admin | Approve; triggers welcome + connection DM |
| POST | `/api/registrations/:id/reject` | admin | `{ comment? }` — DM registrant, remove/block user |
| GET | `/api/discord/channels` | admin | List guild text channels (for target picker) |
| GET | `/api/discord/invite-url` | admin | OAuth URL to add bot to guild |
| PUT | `/api/users/:id/external` | admin | Override/unlink external identity |
| GET | `/api/account/notifications` | session (M16) | Current user's DM category prefs |
| PUT | `/api/account/notifications` | session (M16) | Update DM category prefs |
| GET | `/api/settings/notification-defaults` | admin (M16) | DM defaults for new users |
| PUT | `/api/settings/notification-defaults` | admin (M16) | Update DM defaults |
| POST | `/api/notification-targets` | admin | `{ channel_id }` instead of `{ webhook_url }` |

**Internal:** `ConnectionDetailsService`, `RegistrationService`, `SMMProfileService`, and `DispatchService` shared by REST and `internal/discord/`.

**M15 vs M16 dispatch scope:**

| Delivery | M15 | M16 |
|----------|-----|-----|
| Channel posts (game events) | ✅ Replaces webhooks | unchanged |
| Connection-detail DMs | ✅ Mandatory broadcast | unchanged |
| Game-event DMs (user prefs) | ❌ | ✅ Per-category opt-in |
| Personal player-event DMs | ❌ | ✅ |

---

## 14. Web UI Changes

| Page | Change |
|------|--------|
| **Settings → Discord** (new) | Bot status, invite link, guild ID, role mapping editor, **auto-approve toggle** |
| **Settings → Connection** (new) | Game join details + SMM profile name (§19 O9) |
| **`/mods`** (new top-level page, all active users) | Full mod table; game build + SML header; disclaimer; **Download SMM profile** |
| **Settings → Notifications → Targets** | Channel picker replaces webhook URL + override fields |
| **Settings → Notifications → Defaults** (new, M16) | Admin DM category defaults for new users |
| **Settings → Users** | External identity, pending player badge, unmapped players panel, **pending approvals queue** |
| **Account → Notifications** (new, M16) | Per-user DM category toggles (all roles) |
| **Dashboard** (optional) | "How to join" card for viewers |

All strings in `messages/en.json`.

---

## 15. Configuration & Deployment

### 15.1 Environment variables

| Variable | Required | Notes |
|----------|----------|-------|
| `DISCORD_BOT_TOKEN` | no (soft) | If unset: REST/UI/poller run; bot commands, channel posts, and DMs disabled; admin warning in Settings → Discord (§22.3 resolves O1) |
| `DISCORD_GUILD_ID` | recommended | Bootstrap guild ID until set in Settings → Discord; **env used only when** `app_settings.discord.guild_id` is empty (§19 O13) |
| `DISCORD_ADMIN_ROLE_IDS` | optional | Fallback before UI config |

### 15.2 Discord Developer Portal setup

1. Create application → Bot → copy token.
2. Set bot display name + avatar (replaces per-target webhook overrides).
3. OAuth2 → invite URL with scopes: `bot`, `applications.commands`.
4. Bot permissions: `View Channels`, `Send Messages`, `Embed Links`, `Use Slash Commands`, `Send Messages in Threads`, `Create Private Channels` (for DMs).
5. **Message Content Intent:** not required for slash commands.

### 15.3 Single-container impact

- No new ports.
- Outbound HTTPS to `discord.com`.
- Gateway adds ~10–20 MB RAM.
- Entrypoint unchanged.

---

## 16. Migration & Rollout

| Phase | Action |
|-------|--------|
| **1. Schema** | External identity, `users.status`, connection details, bot config, registration audit |
| **2. Bot + Provider refactor** | discordgo gateway; `DiscordProvider` `Send`/`SendDirect`; remove webhook HTTP |
| **3. Notification targets UI** | Channel picker; remove webhook fields |
| **4. Registration** | `/register`, `/link`, approval flow, `/registration auto-approve` |
| **5. Player mapping** | Pending state, poller auto-link, unmapped players admin UI |
| **6. Connection + mods** | Connection API/UI/commands; mod list API/UI/commands |
| **7. Polish (M15)** | Users UI, Settings → Discord/Connection, tests, docs, spec update |
| **8. M16** | `/status`, `/players`, DM prefs, personal player DMs, game-event DM fan-out |

**Existing users:** `/link` to attach Discord identity.

**Existing webhook targets:** Admin re-selects channels in UI after upgrade. Document breaking change in release notes. Webhook URLs are not forward-compatible.

---

## 17. Testing Strategy

| Area | Approach |
|------|----------|
| `DiscordProvider.Send` / `SendDirect` | Mock `discordgo.Session`; assert embed payload + channel/user IDs |
| Bot command handlers | Mock interaction payloads (JSON fixtures) |
| Registration / approval | Extend `auth` tests; `pending_approval` blocks login and commands |
| Pending player auto-link | Poller test: upsert `player_state` → assert `player_id` set on matching user |
| Connection broadcast | Assert `SendDirect` called for all active linked users, not `pending_approval` |
| Connection log redaction | Assert `notification_log.rendered_preview` and `bot_command_log.detail` never contain `game_password` value after `/connection set` (§8.6) |
| DM fan-out / prefs | M16 — user with `power` DM on receives fuse event; user with off does not |
| Dispatcher regression | Existing `dispatch_test.go` — swap httptest webhook for mock session |
| Integration | Optional test guild + bot token in CI secrets |

Add bot testing section to `docs/testing.md`.

---

## 18. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Bot offline → no notifications or registration | Health indicator in admin UI; break-glass web invite |
| Discord API rate limits on broadcast | Batch DMs; backoff; log failures |
| Webhook → bot migration breaks existing targets | Release notes + channel re-pick wizard in UI |
| Loss of per-target display name | Document: set bot name/avatar in Developer Portal |
| Player name mismatch | `pending_player_name` + admin override |
| Duplicate external accounts | Unique index on `(external_platform, external_user_id)` |
| Token leak | Env-only bot token; rotate in Portal |
| Over-abstraction | Discord commands stay in `internal/discord/` — no generic interface |
| ficsit.app unavailable during SMM export | Fail with clear error; web UI shows retry; mod list view still works |
| `RequiredOnRemote` misleading players | Full mod list + disclaimer; SMM export is the real sync path |
| No admin has Discord linked | Web UI pending-approvals queue is required path (§6.2) |
| `game_password` in audit logs | Shared redaction helper; tests assert no password in log tables (§8.6) |

---

## 19. Open Questions

### Resolved

| # | Question | Decision |
|---|----------|----------|
| Q1 | Dashboard password at registration | User-chosen in modal |
| Q2 | `/status` / `/players` gating | Active registered users only (M16) |
| Q3 | Web invite UI | Keep hidden break-glass |
| Q4 | Auto-sync Discord roles | Manual `/sync-roles` + on-registration (M16) |
| Q5 | Connection details page | `/settings/connection` |
| Q6 | Viewers see game password | Yes |
| Q7 | Per-user DM prefs timing | M16 |
| Q8 | Personal player-event DMs | M16 |
| Q9 | Default auto-approve | On (`true`) |
| Q10 | Filter mods by `RequiredOnRemote` | No — full list |
| Q11 | SMM profile export timing | **M15** — with mod list |
| Q12 | Game build display | **Raw build number** (matches title screen) |
| Q13 | `/register user` timing | **M15** — admin-only, DM target to complete |
| Q14 | ficsit.app API allowed for SMM export? | **Yes** — open-source + low-volume metadata lookup; see §8.6 |
| Q15 | Keep template builder for game events? | **Yes** — transport-only refactor; fixed strings for bot UX only (§5.5) |
| Q16 | User onboarding / `/help`? | **Yes — M15** — `/help` + welcome DM + slash command descriptions (Appendix G) |

All planning questions resolved — safe defaults below.

| # | Question | Decision |
|---|----------|----------|
| O1 | Bot required at startup? | **Soft dependency** — REST/UI/poller run without token; bot features disabled + admin warning (§15.1) |
| O2 | Setup order: web `/setup` before bot? | **Yes** — `/setup` first admin unchanged; bot token anytime; Settings → Discord after |
| O3 | SMM lockfile targets | **All three** — `Windows`, `LinuxServer`, `WindowsServer` (§8.5) |
| O4 | Discord `/mods export` delivery | **Bot attaches file** — internal `SMMProfileService`; no session URL (§8.5) |
| O5 | Username collision at registration | **Auto-suffix only** (`michael-2`, …); show assigned name in ephemeral; no override field in modal v1 |
| O6 | `dm_player_personal` storage (M16) | **`users.dm_player_personal` boolean** + admin default `notifications.dm_player_personal_default`; not a category row |
| O7 | DM notification audit logging | **Extend `notification_log`** via SQLite table rebuild — nullable `target_id`, `delivery_mode`, `recipient_external_user_id` (§12.0, §12.5) |
| O8 | Webhook → channel migration UX | **Manual re-pick + UI banner** — no auto-migration from webhook metadata (§12.3) |
| O9 | `mods.smm_profile_name` setting location | **Settings → Connection** for v1 (same page as join details) |
| O10 | Rate limit on `/register` | **None in v1** — Discord role gating + `allow_self_register` sufficient for private group |
| O11 | Connection-detail message type | **Outside dispatcher in M15** — fixed `ConnectionDetailsService` copy; optional `connection_details_changed` template in M16 only |
| O12 | Reject registration persistence | **Audit log row, then DELETE user** — no `rejected` status row kept (§6.2) |
| O13 | Guild ID precedence | **`app_settings.discord.guild_id` wins** when set; else `DISCORD_GUILD_ID` env |
| O14 | Viewer access to `/mods` + connection info | **All `status = active` users** — top-level `/mods` nav; not under Settings (§22 G4) |
| O15 | `GET /api/auth/me` extensions | **Return `status`, `pendingPlayerName`, external identity fields** — powers awaiting-approval UI (M15) |
| O16 | Spec + roadmap updates | **Part of M15 DoD** — update `factorymate-spec.md` + add M15/M16 to roadmap when plan approved |
| O17 | `game_password` in logs | **Never log value** — redact in all audit/debug logs; full value only in authorized user-facing DM/API (§8.6) |

---

## 20. Suggested Roadmap Placement

### M15 — Discord Bot, Provider Refactor & Player Onboarding

- [x] Schema: external identity, `users.status`, connection details, registration audit, bot config
- [x] `internal/discord/` — gateway, slash command router
- [x] Refactor `DiscordProvider`: `Send` (channel) + `SendDirect` (DM); remove webhook HTTP
- [x] Notification targets UI: channel picker; drop webhook URL fields
- [x] `/register`, `/register user`, `/link`, `/set-player`, `/whoami`, `/help` (Appendix G copy + slash descriptions)
- [x] Registration approval: `auto_approve`, pending status, admin DM buttons, web queue
- [x] `/registration auto-approve` admin command
- [x] Pending player mapping + poller auto-link (`TryResolvePendingPlayers`)
- [x] Unmapped server players admin panel
- [x] Connection details API + `/connection`, `/connection set` + mandatory DM broadcast
- [x] FRM `getModList` + SMM profile export (`SMMProfileService` + ficsit.app) + `/mods` page/command
- [x] Settings → Discord + Connection; Users UI extensions
- [x] Deprecate primary web invite flow (keep break-glass)
- [x] Tests + `docs/development.md` + spec §5 update + log-redaction tests (§8.6)

### M16 — Notifications polish & admin commands

- [x] `/status`, `/players`, `/broadcast`, `/sync-roles`, `/notifications`
- [x] `user_notification_prefs` + Account → Notifications UI + admin DM defaults
- [x] Game-event DM fan-out (per-category opt-in)
- [x] Personal player event DMs ("your character joined")
- [x] Auto-link DM notification; guild member role change listener
- [x] `connection_details_changed` message type in template editor (optional)

---

## 21. Summary Decision Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Replace webhooks with bot? | **Yes — unified bot** | Single admin setup; bot already required for commands/DMs |
| Abstract interactive commands? | **No** — `internal/discord/` only | Platforms too different; YAGNI until Slack is real |
| Abstract outbound messaging? | **Yes** — extend `Provider` | Same dispatcher/templates; swap transport |
| Generic identity columns? | **Yes** — `external_platform` + `external_user_id` | Cheap portability without over-engineering |
| TeamSpeak support? | **Notification-only if ever** | No shared bot framework with Discord |
| Single container? | **Yes** | Bot in Go process |
| Invite system? | **Discord-first** `/register`; web invite break-glass only |
| RBAC for bot? | **Discord role mapping in web UI** + FM admin/viewer for dashboard |
| Connection vs FRM settings? | **Separate** |
| Broadcast trigger? | **Any source** — shared `ConnectionDetailsService` |
| Connection-detail DMs? | **Mandatory** for all active linked users | Critical info; not opt-outable in v1 |
| Game event routing? | **Hybrid** — admin channels (M15) + per-user DM opt-in (M16) |
| Per-user DM control? | **Yes** — category toggles; not per-message-type in v1 |
| Player mapping when not on server? | **Always save `pending_player_name`**; auto-link on poller ingest |
| Registration approval default? | **`auto_approve = true`** | Role gating suffices for private group; approval is opt-in |
| Approval UX? | **Discord DM buttons** + web queue fallback | Mobile-friendly; optional reject reason modal |
| Server mod list? | **Full list, unfiltered** | `RequiredOnRemote` unreliable; show game build + SML prominently |
| SMM profile export? | **M15** — full lockfile via ficsit.app API | One-click client sync; fixture in `docs/examples/smm_profiles/` |
| `/register user`? | **M15, admin-only** | Small add-on to registration; admin DMs target to complete |
| Game build label? | **Raw build number in v1** | Matches title screen; wiki mapping is manual/optional M16 |
| Game-event message format? | **Keep template builder 1:1** | Bot changes transport only; fixed strings for slash-command UX (§5.5) |
| ficsit.app API for SMM export? | **Allowed** — MIT + cached `resolveModVersions` | Same flow as SMM; no bulk download (§8.6) |
| User onboarding? | **`/help` + welcome DM** (M15) | Discovery via slash hints + admin word-of-mouth (Appendix G) |
| Username collision? | **Auto-suffix only** (O5) | Simple modal; no rename API in v1 |
| Open questions? | **All resolved** (§19 O1–O17) | Safe defaults for private-group homelab |
| `game_password` in logs? | **Never** — redact in audit/debug (§8.6) | User-facing DM/API only |

---

## 22. Review — gaps, logic issues, spec drift

*Review round: 2026-08-17. Items marked **Fixed in v11** were corrected in this document; others are implementation notes.*

### 22.1 Logic issues (fixed or flagged)

| # | Issue | Severity | Resolution |
|---|-------|----------|------------|
| L1 | **Two sequential modals** on `/register` — Discord allows only one modal per interaction | **High** | **Fixed:** single modal with both fields (§6.1) |
| L2 | **SMM `profile.mods` included SML** — contradicts fixture (`SML` lockfile-only) | **High** | **Fixed:** profile excludes `FactoryGame` + `SML`; lockfile excludes `FactoryGame` only (§8.5) |
| L3 | **`/mods export` via authenticated API URL** — Discord has no FM session cookie | **High** | **Fixed:** bot calls `SMMProfileService` and attaches file (§8.5, O4) |
| L4 | **`notification_log.delivery_mode`** referenced but column does not exist; DMs have no `target_id` | **Medium** | **Fixed:** §12.5 schema extension documented |
| L5 | **`DISCORD_BOT_TOKEN` required** (§15.1) vs soft dependency (O1) | **Low** | **Fixed:** §15.1 aligned with soft dependency |
| L6 | **Approval DMs to "FM admin"** — must be admins with linked Discord only | **Medium** | **Fixed:** §6.2 wording |
| L7 | **`dm_player_personal`** pref in §9.4 not in schema | **Medium** | **Resolved (O6):** `users.dm_player_personal` column |
| L8 | **Button interactions** (Approve/Reject) require `MESSAGE_COMPONENT` handler, not slash-only router | **Medium** | Implement in `internal/discord/interactions.go`; test with fixtures |
| L9 | **Connection broadcast** skips web-only users (no `external_user_id`) | **Low** | Expected — document in release notes; they see changes on web `/settings/connection` or dashboard card |
| L10 | **`/register user` button flow** — target must complete via button → modal; same single-modal rule as §6.1 | **Medium** | Reuse registration modal component; button `custom_id` starts flow |

### 22.2 Gaps — implementation checklist (decisions in §19)

| # | Gap | Decision (§19) |
|---|-----|----------------|
| G1 | **`factorymate-spec.md` not updated** | O16 — M15 DoD |
| G2 | **`factorymate-roadmap.md` has no M15/M16** | O16 — add when plan approved |
| G3 | **Nav / i18n routes** | Implement per §14; top-level `/mods` for all active users (O14) |
| G4 | **Viewer vs admin on `/mods`** | O14 — all active users; top-level nav |
| G5 | **`GET /api/auth/me` extensions** | O15 |
| G6 | **Middleware `RequireActiveUser`** | M15 — required; blocks `pending_approval` on API + frontend |
| G7 | **Discord interaction auth** | M15 — bot calls same `auth` service as REST |
| G8 | **Webhook → channel migration UX** | O8 — banner + manual re-pick |
| G9 | **`mods.smm_profile_name` placement** | O9 — Settings → Connection |
| G10 | **Rate limit on `/register`** | O10 — none v1 |
| G11 | **Bot DM permissions** | §15.2 — include `Create Private Channels` |

### 22.3 Spec drift (plan vs `factorymate-spec.md` / codebase)

| Area | Spec / code today | Plan target | Action |
|------|-------------------|-------------|--------|
| Discord transport | `webhook_url` in `notification_targets.config_json` (spec §5.1, `discord.go`) | `channel_id` + bot `Send` | Breaking change; migration §12.3 |
| Onboarding | Invite links primary (spec §7) | Discord `/register` primary; invite break-glass | Spec §7 rewrite |
| `users` schema | `id, username, password_hash, role, player_id` — no `status`, no external identity | +`status`, +`external_*`, +`pending_player_name` | Migration §12.1 |
| `notification_log` | `target_id` required, no `delivery_mode` | DM audit rows need extension | Table-rebuild migration §12.0, §12.5; redact password in previews §8.6 |
| `Provider` interface | `Send` only (`types.go`) | +`SendDirect` via `DirectMessageProvider` | Appendix F |
| API routes | No `/api/mods`, `/api/connection-details`, `/api/discord/*` | §13 | New handlers |
| UI routes | No `/mods`, `/settings/discord`, `/settings/connection` | §14 | New pages |
| Password rules | ≥8 chars (spec §7) | Same in registration modal | Aligned ✓ |
| Template system | Full embed editor (spec §5.4) | Unchanged for game events (§5.5) | Aligned ✓ |
| FRM `getModList` | Not in spec §4.1 endpoint table | New read endpoint | Add to spec |
| Seed upsert behavior | spec says seed never overwrites `enabled` | Unchanged | Aligned ✓ |
| `message_types.category` | DB has `server \| player \| power \| progression \| vehicle` | Connection DMs outside dispatcher (O11); no `account` category unless optional M16 template added |

### 22.4 Inconsistencies resolved in v11–v12

- Non-goals TeamSpeak cross-ref pointed at wrong section (§5.4 → abstraction table).
- SMM export example JSON corrected to match fixture.
- Open questions O1–O16 resolved with safe defaults (§19).
- Appendix G added for `/help` copy.
- Username collision, DM audit schema, guild ID precedence, migration UX, and reject flow clarified (v12).
- SQLite table-rebuild migration for `notification_log`; `game_password` redaction policy (§8.6, v13).

---

## Appendix A — Example `/connection` DM

```
🏭 CBC | Conveyor Belt Cult — Server Connection

Host:     play.example.com
Port:     7777
Password: ||hunter2||

Notes:    Epic Games only. Mod pack link: ...

Updated:  Aug 17, 2026 · 14:00 UTC
Dashboard: https://factorymate.example.com
```

## Appendix B — Example registration ephemeral reply (awaiting approval)

```
⏳ Registration submitted

Your request is waiting for admin approval.
You'll receive a DM when approved with dashboard access and connection details.

In-game name claimed: Michael
```

## Appendix C — Example registration ephemeral reply (auto-approved, pending player)

```
✅ You're registered!

Dashboard: https://factorymate.example.com
Username:  michael
Role:      viewer
Player:    Michael (pending — not seen on server yet)

We'll link your player automatically when you join.
Use /set-player to correct your in-game name.
Use /connection for join details. Use /mods for the full mod list.
```

## Appendix D — Example admin approval DM

```
📋 New registration pending approval

Discord:   @michael (Michael)
In-game:   Michael
FM user:   michael
Submitted: Aug 17, 2026 · 14:12 UTC

[ Approve ]  [ Reject ]
```

## Appendix E — Example `/mods` ephemeral reply

```
📦 Server mods (Aug 17, 2026 · 14:30 UTC)

Game build: 502094.0.0
SML: 3.12.0

Install ALL mods below at matching versions — or use /mods export for an SMM profile.

AutoSort — 3.2.2
Depot Sorting — 1.0.1
Ficsit Remote Monitoring — 1.5.3
Satisfactory Mod Loader — 3.12.0
Satisfactory — 502094.0.0

Full list: https://factorymate.example.com/mods
```

## Appendix F — Provider interface (target shape)

```go
// internal/notify/types.go — conceptual; extend in M15

type Provider interface {
    Type() string
    // Channel/group post (game events, test-send)
    Send(ctx context.Context, target NotificationTarget, msg RenderedMessage) error
}

// DirectMessageProvider — optional second interface; only providers that support DMs
type DirectMessageProvider interface {
    Provider
    SendDirect(ctx context.Context, platform, externalUserID string, msg RenderedMessage) error
}
```

Discord implements both. A future Slack provider likely would too. TeamSpeak might implement only `Send`.

## Appendix G — Example `/help` reply

Ephemeral reply; also use as slash-command description seed in discordgo registration.

```
🏭 FactoryMate — quick start

**New here?**
1. /register — create your dashboard account
2. /mods export — download SMM profile → import in Satisfactory Mod Manager
3. /connection — get server host, port, and password (sent to your DMs)
4. Log in: https://factorymate.example.com

**Already registered?**
/connection — join details (DM)
/mods — full mod list
/set-player — fix your in-game name mapping
/whoami — check your link status

**Have a web account but new to Discord?**
/link — attach Discord to your existing login

**Admins**
/connection set — update join details (broadcasts to all players)
/register user — invite someone to complete registration
/registration auto-approve — toggle approval gate

Dashboard: https://factorymate.example.com
```
