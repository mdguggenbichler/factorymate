# FactoryMate — Specification

**Project:** FactoryMate — Satisfactory Server Monitoring & Notification Sidecar for CBC | Conveyor Belt Cult
**Status:** Draft v1
**Author context:** Solo AI-assisted build (Cursor), for a private ~5–6 player Satisfactory dedicated server group

---

## 1. Overview

FactoryMate is a self-hosted sidecar service that polls a Satisfactory Dedicated Server's [FicsIt Remote Monitoring (FRM)](https://docs.ficsit.app/ficsitremotemonitoring/latest/index.html) API, detects meaningful state changes, sends formatted notifications to configurable destinations (Discord first, extensible later), and exposes a small authenticated web dashboard showing live and historical factory statistics (most notably: produced vs. consumed per item).

It replaces an earlier n8n-based polling workflow with a single self-contained Go service plus a Next.js dashboard, deployed as one or two containers alongside the existing Satisfactory server on the group's home server.

### 1.1 Goals

- Reliable, self-hosted polling of FRM with correct diff/edge-triggered event detection (no duplicate or missed notifications across restarts).
- Rich, well-formatted Discord notifications (embeds with color, fields, footer, and native timestamp — §5.4) — not limited to plain text.
- A central place to configure **where** notifications go (Notification Targets) and **which** message types go **where** (assignment), decoupled from message formatting.
- A templating system so operators get sensible default messages out of the box, but can edit the wording/format per message type without touching code.
- A small dashboard, protected by login, that can be shared with all group members — not just the admin — covering player status, power status, production stats (produced vs. consumed per item, over time, plus per-machine detail), the AWESOME Sink, drones, Lizard Doggos, milestones, M.A.M. research, vehicles, and Space Elevator progress.
- Clean extensibility: adding a new notification provider (e.g. ntfy, Telegram) or a new trackable event type should not require architectural changes.

### 1.2 Non-Goals (v1)

- No support for multiple concurrent Satisfactory servers (single FRM endpoint per deployment).
- No mobile app / push notifications outside of chat-style notification providers.
- No write-access to the game (no remote admin actions against the Dedicated Server API in v1).
- No fine-grained per-user permission system — all logged-in users see the same dashboard; only distinction is "admin" (can edit settings/templates/targets) vs. "viewer" (read-only dashboard).
- No multi-tenancy — this is a single-group, single-save deployment.
- No in-app UI language switcher in v1 — i18n infrastructure ships with English only (§8.2); additional locale files are a post-MVP addition (§10).

---

## 2. Architecture

```
                    ┌─────────────────────────────┐
                    │   Satisfactory Dedicated      │
                    │   Server (wolveix image)      │
                    │   + SML + FicsIt Remote        │
                    │     Monitoring (FRM)           │
                    │   HTTP API :8080 (mapped :8889)│
                    └───────────────┬────────────────┘
                                    │ polled every N seconds
                                    ▼
                    ┌─────────────────────────────┐
                    │   FactoryMate — Backend (Go)  │
                    │                                │
                    │  ┌──────────────────────────┐  │
                    │  │ Poller / Diff Engine       │  │
                    │  └──────────────┬───────────┘  │
                    │                 │ state change   │
                    │                 ▼                │
                    │  ┌──────────────────────────┐  │
                    │  │ Template Renderer          │  │
                    │  └──────────────┬───────────┘  │
                    │                 │ rendered msg   │
                    │                 ▼                │
                    │  ┌──────────────────────────┐  │
                    │  │ Notification Dispatcher    │  │
                    │  │  (Provider interface)      │  │
                    │  └──────────────┬───────────┘  │
                    │                 │                │
                    │  ┌──────────────────────────┐  │
                    │  │ REST API (dashboard data,  │  │
                    │  │ settings, templates, auth) │  │
                    │  └──────────────────────────┘  │
                    │                                │
                    │  SQLite (state, history,       │
                    │  config, templates, users)      │
                    └───────────────┬────────────────┘
                                    │ REST/JSON
                                    ▼
                    ┌─────────────────────────────┐
                    │  FactoryMate — Frontend        │
                    │  Next.js + shadcn/ui + Tailwind│
                    │  + Recharts                    │
                    └─────────────────────────────┘
                                    │
                                    ▼
                         ┌───────────────────┐
                         │  Discord (v1)       │
                         │  (Provider iface,   │
                         │  more providers      │
                         │  later: ntfy, etc.)  │
                         └───────────────────┘
```

### 2.1 Tech Stack

| Layer | Choice | Notes |
|---|---|---|
| Backend language | Go | Single static binary, easy Docker image, good HTTP + concurrency primitives for the poller loop |
| Backend framework | net/http + **chi** router | Lightweight REST API — chi is the chosen router for v1 |
| Database | SQLite (`modernc.org/sqlite`, pure Go, no CGO) | Sufficient for single-server scale; simplifies deployment (no separate DB container) |
| Notification delivery | **Custom `Provider` interface**, no third-party notification library | Full control over Discord embed structure (title, description, color, fields, footer, timestamp). Discord is the primary/only provider in v1; interface is designed so ntfy/Telegram/Slack providers can be added later without touching the dispatcher or templating system. |
| Templating | Custom `{VarName}` substitution (plain + structured embed model) — see §5.4 | Two render paths: plain string, or structured embed object |
| Frontend framework | Next.js (App Router) | |
| Frontend i18n | **next-intl** | All UI strings via locale files — see §8.2; English only in v1 |
| UI components | shadcn/ui + Tailwind CSS | Copy-paste component model, minimal custom CSS |
| Charts | Recharts | Pairs natively with shadcn's chart components |
| Auth | Session-cookie based; username/password for setup and break-glass invites; **Discord OAuth** (`identify` scope) for linked Discord users when `DISCORD_CLIENT_SECRET` + `FACTORYMATE_PUBLIC_URL` are set | Discord-only users have `password_hash` NULL |
| Deployment | Docker, **one container** (Go backend + Next.js frontend) | Runs alongside the existing `satisfactory-server` container on the group's host; Next.js proxies `/api` to the Go process on localhost (see §2.4) |

### 2.2 Why not shoutrrr

`shoutrrr` (containrrr/shoutrrr) was evaluated and rejected for v1. It provides a unified plain-string API across ~18 notification services, which is valuable for broad multi-provider support, but its Discord service only exposes a single message body, a title, an accent color per severity class (`Color`/`ColorInfo`/`ColorWarn`/`ColorError`/`ColorDebug`), and username/avatar override — it does **not** expose structured embed fields. Achieving the rich, per-message-type embeds this project wants (fields like "Tech Tier" / "New Recipes" / "Options") would require bypassing shoutrrr's own formatting via its `JSON=true` passthrough mode and constructing the full Discord embed payload manually anyway — at which point the dependency adds indirection without adding value. Since Discord is confirmed as the primary and, for now, only target, a small first-party `DiscordProvider` implementing a generic `Provider` interface gives full embed control today and a clean seam for additional providers later.

### 2.3 Notification Provider interface

```go
// internal/notify/types.go — conceptual shape; implement in M4.

type NotificationTarget struct {
    ID           int64
    Name         string
    ProviderType string // "discord" in v1
    ConfigJSON   string // provider-specific JSON (see §5.1)
    Enabled      bool
}

type RenderedMessage struct {
    Plain string          // populated for plain-text providers
    Embed *DiscordEmbed   // populated when provider_type == "discord"
}

type DiscordEmbed struct {
    Title       string
    Description string
    Color       string // hex, e.g. "#57F287"
    Fields      []DiscordEmbedField
    Footer      string // rendered footer text (optional)
    Timestamp   string // ISO 8601 for Discord native timestamp (optional)
}

type DiscordEmbedField struct {
    Name   string
    Value  string
    Inline bool
}

type Provider interface {
    Type() string
    Send(ctx context.Context, target NotificationTarget, msg RenderedMessage) error
}

// DirectMessageProvider extends Provider with per-user DM delivery (bot token transport).
type DirectMessageProvider interface {
    Provider
    SendDirect(ctx context.Context, platform, externalUserID string, msg RenderedMessage) error
}
```

Discord is the v1 implementation: channel posts use `Send` with `notification_targets.config_json.channel_id`; player DMs (welcome, connection details, password reset, notification prefs) use `SendDirect` with the user's `external_user_id` when `external_platform = 'discord'`.

### 2.4 Frontend / backend wiring

v1 uses **one container** in `docker-compose.yml` running two processes:

| Process | Role | Port (default) |
|---|---|---|
| Go backend | API + poller + SQLite | `8080` (localhost only inside container) |
| Next.js | App Router UI | `3000` (exposed to host) |

**Production:** Next.js listens on `:3000` and proxies `/api/*` and `/healthz` to the Go backend (`BACKEND_URL=http://127.0.0.1:8080` in `next.config` rewrites). Browser calls same-origin `/api/...`; session cookies use `SameSite=Lax` with no cross-origin setup.

**Local dev:** `frontend` on `:3000`, `backend` on `:8080`; `NEXT_PUBLIC_API_URL=http://localhost:8080` so the dev client can call the API directly (or use the same rewrite pattern).

**CORS:** Not required in production (same-origin via proxy). For local dev with direct API URL, backend enables CORS for `http://localhost:3000` only.

---

## 3. Data Model (SQLite)

```sql
-- Users with dashboard access
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    player_id TEXT REFERENCES player_state(player_id),  -- optional mapping to game player
    created_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('pending_approval', 'active')),
    external_platform TEXT
        CHECK (external_platform IS NULL OR external_platform IN ('discord', 'slack')),
    external_user_id TEXT,
    external_username TEXT,
    external_display_name TEXT,
    external_linked_at TEXT,
    pending_player_name TEXT,
    registration_source TEXT NOT NULL DEFAULT 'web_invite'
        CHECK (registration_source IN ('setup', 'web_invite', 'discord')),
    dm_player_personal BOOLEAN NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_users_external_identity
    ON users(external_platform, external_user_id)
    WHERE external_user_id IS NOT NULL;

CREATE TABLE invites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    accepted_at TEXT,
    accepted_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TEXT
);
CREATE INDEX idx_invites_token ON invites(token);
CREATE INDEX idx_invites_pending ON invites(accepted_at, revoked_at, expires_at);
CREATE INDEX idx_users_player_id ON users(player_id);

-- OAuth CSRF state (M17): SHA-256 hashed nonce, 10-minute TTL, single-use
CREATE TABLE oauth_states (
    token_hash TEXT PRIMARY KEY,
    purpose TEXT NOT NULL CHECK (purpose IN ('login', 'register', 'link', 'register_complete')),
    external_user_id TEXT,
    external_username TEXT,
    external_display_name TEXT,
    force_approve INTEGER NOT NULL DEFAULT 0,
    fm_role TEXT,
    user_id INTEGER REFERENCES users(id),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);
CREATE INDEX idx_oauth_states_expires ON oauth_states(expires_at);

-- Server-side session store (SQLite — sessions survive process restarts)
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,                -- cryptographically random session ID
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- Where notifications can be sent
CREATE TABLE notification_targets (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,                 -- display name, e.g. "Main Discord Channel"
    provider_type TEXT NOT NULL,        -- 'discord' (v1), extensible later
    config_json TEXT NOT NULL,          -- provider-specific config only (see §5.1) — NOT the outer provider_type wrapper
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL
);

-- Fixed catalog of message types (seeded, not user-created in v1).
-- Startup seeding INSERTs new keys and upserts default_template_json and
-- variables_json on every startup; `enabled` is preserved on existing rows.
CREATE TABLE message_types (
    key TEXT PRIMARY KEY,               -- e.g. 'player_joined'
    label TEXT NOT NULL,                -- human readable, e.g. "Player Joined"
    category TEXT NOT NULL,             -- 'player' | 'power' | 'progression' | 'vehicle' | 'server'
    enabled BOOLEAN NOT NULL DEFAULT 1, -- admin on/off switch, independent of target assignment (see §5.3)
    default_template_json TEXT NOT NULL,-- built-in default (see §5.4)
    variables_json TEXT NOT NULL        -- JSON array of variable name strings, e.g. ["PlayerName","OnlineCount"] — for template editor chips (§5.2)
);

-- Per-message-type template override (if absent, default_template_json from message_types is used)
CREATE TABLE message_templates (
    message_type_key TEXT PRIMARY KEY REFERENCES message_types(key),
    template_json TEXT NOT NULL,
    updated_by INTEGER REFERENCES users(id),
    updated_at TEXT NOT NULL
);

-- Which targets receive which message types (many-to-many)
CREATE TABLE message_type_targets (
    message_type_key TEXT NOT NULL REFERENCES message_types(key),
    target_id INTEGER NOT NULL REFERENCES notification_targets(id) ON DELETE CASCADE,
    PRIMARY KEY (message_type_key, target_id)
);

-- Per-user personal DM opt-in by message type (M18). Absent row = admin default
-- from app_setting_kv `notifications.dm_defaults_json` (per-type booleans).
-- Excludes `connection_details` / `connection_details_changed` (mandatory connection DMs).
CREATE TABLE user_notification_prefs (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_type_key TEXT NOT NULL REFERENCES message_types(key),
    dm_enabled BOOLEAN NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, message_type_key)
);

-- Audit log of sent notifications (dispatch outcomes only — NOT a substitute for
-- player_session_events or power_circuit_events; disabled message types produce
-- no rows here even when the underlying game event occurred)
CREATE TABLE notification_log (
    id INTEGER PRIMARY KEY,
    message_type_key TEXT NOT NULL,
    target_id INTEGER,                  -- NULL for DM rows; may reference deleted notification_targets for channel rows (no FK)
    rendered_preview TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    sent_at TEXT NOT NULL,
    delivery_mode TEXT NOT NULL DEFAULT 'channel'
        CHECK (delivery_mode IN ('channel', 'dm')),
    recipient_external_user_id TEXT    -- populated when delivery_mode = 'dm'
);

-- Discrete player join/leave events for /api/players/history (written by M3 on
-- every transition, regardless of message_types.enabled)
CREATE TABLE player_session_events (
    id INTEGER PRIMARY KEY,
    player_id TEXT NOT NULL,
    player_name TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('joined', 'left')),
    online_count INTEGER NOT NULL,
    occurred_at TEXT NOT NULL
);
CREATE INDEX idx_player_session_events_time ON player_session_events (occurred_at);

-- Discrete fuse trip/restore events for /api/power/history (written by M3 on
-- every transition, regardless of message_types.enabled). Retention: indefinite
-- (discrete events are small; prune only if needed in a future version).
CREATE TABLE power_circuit_events (
    id INTEGER PRIMARY KEY,
    circuit_id INTEGER NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('fuse_tripped', 'power_restored')),
    occurred_at TEXT NOT NULL
);
CREATE INDEX idx_power_circuit_events_time ON power_circuit_events (occurred_at);

-- Diagnostic log for Space Elevator phase-lookup misses (see §4.2).
-- Populated only when a poll's CurrentPhase ClassName set does not match
-- any row in the phase-mapping reference data; never populated on a match.
CREATE TABLE elevator_phase_unknown_log (
    id INTEGER PRIMARY KEY,
    raw_current_phase_json TEXT NOT NULL,  -- full CurrentPhase[] array as returned by FRM (Name + ClassName + amounts)
    detected_at TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT 0,   -- admin can mark as resolved once the reference table has been updated
    resolved_at TEXT
);

-- Poller state (single row, mirrors previous n8n JSON state, now normalized)
CREATE TABLE server_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    server_online BOOLEAN,
    updated_at TEXT NOT NULL
);

CREATE TABLE player_state (
    player_id TEXT PRIMARY KEY,         -- FRM Char_Player_C_... ID
    name TEXT,
    online BOOLEAN NOT NULL,
    last_seen_at TEXT                   -- set to poll timestamp on player_left (online true→false); unchanged while online; NULL until first leave
);

CREATE TABLE circuit_state (
    circuit_id INTEGER PRIMARY KEY,     -- FRM CircuitGroupID
    tripped BOOLEAN NOT NULL,
    power_capacity REAL,
    power_production REAL,
    power_consumed REAL,
    power_max_consumed REAL,
    battery_differential REAL,
    battery_percent REAL,
    battery_capacity REAL,
    battery_time_empty TEXT,
    battery_time_full TEXT,
    updated_at TEXT NOT NULL
);

CREATE TABLE schematic_state (
    schematic_id TEXT PRIMARY KEY,      -- FRM Schematic ID
    name TEXT NOT NULL,                 -- FRM Name — for /milestones display and {SchematicName}
    type TEXT NOT NULL,                 -- Milestone | Hard Drive | Alternate | M.A.M. | ...
    purchased BOOLEAN NOT NULL,
    locked BOOLEAN NOT NULL,
    tech_tier INTEGER,
    recipes_json TEXT,                  -- JSON array of FRM Recipes[] — for {RecipeNames}/{RecipeOptions} and Hard Drive UI
    purchased_at TEXT,                  -- set once when Purchased flips false→true; used for latestMilestone ordering (§7.1); NULL if never purchased
    updated_at TEXT NOT NULL            -- updated every poll upsert (do not use for milestone recency)
);

CREATE TABLE elevator_state (
    elevator_id TEXT PRIMARY KEY,
    name TEXT,                          -- FRM Name — for {ElevatorName} and /elevator display
    upgrade_ready BOOLEAN NOT NULL,
    phase_number INTEGER,               -- derived via static ClassName-set lookup, NULL if no match (see §4.2)
    current_phase_json TEXT,            -- full CurrentPhase[] from FRM — for /elevator progress bars
    updated_at TEXT NOT NULL
);

CREATE TABLE train_state (
    train_id TEXT PRIMARY KEY,          -- FRM Train ID
    name TEXT,
    derailed BOOLEAN NOT NULL,
    pending_derail BOOLEAN NOT NULL,
    status TEXT,                        -- e.g. 'Self-Driving', 'Manual'
    self_driving_error TEXT,            -- e.g. 'SDLE_NoError'
    docking_status TEXT,                -- e.g. 'TDS_Docked'
    path_status TEXT,                   -- e.g. 'PDE_NoError'
    station TEXT,                       -- current/last TrainStation
    updated_at TEXT NOT NULL
);

CREATE TABLE vehicle_state (
    vehicle_id TEXT PRIMARY KEY,        -- FRM Vehicle ID (JSON string or integer — normalize to TEXT)
    vehicle_type TEXT NOT NULL,         -- FRM VehicleType, e.g. 'Explorer'
    display_name TEXT NOT NULL,         -- same as vehicle_type for v1 — populates {VehicleName}
    status TEXT,
    driver TEXT,
    autopilot BOOLEAN,
    following_path BOOLEAN,
    forward_speed REAL,
    fuel_empty BOOLEAN NOT NULL,        -- derived: true if Fuel[] sums to 0
    low_speed_since TEXT,               -- timestamp the poller first observed near-zero speed while on autopilot/route; NULL whenever the raw condition isn't currently met — used only to compute the debounce, see §4.2
    stuck BOOLEAN NOT NULL DEFAULT 0,   -- the actual edge-triggered derived state (debounce already applied); vehicle_stuck fires on this flipping false→true, exactly like fuse_tripped — see §4.2
    updated_at TEXT NOT NULL
);

-- M.A.M. research nodes (getResearchTrees). Distinct from schematic_state's
-- Milestone/Hard Drive entries — see §4.1 for why these are polled and
-- modeled separately despite both ultimately being "schematics" internally.
CREATE TABLE research_node_state (
    node_id TEXT PRIMARY KEY,           -- FRM Research Node ID
    tree_name TEXT NOT NULL,            -- parent M.A.M. tree/category Name
    name TEXT NOT NULL,
    category TEXT,
    state TEXT NOT NULL,                -- as reported by FRM; confirmed value 'Purchased' seen, 'Hidden' documented but not observed, other intermediate values possible (see §4.2)
    tech_tier INTEGER,
    cost_json TEXT NOT NULL DEFAULT '[]', -- FRM Cost[] — for /research display
    updated_at TEXT NOT NULL
);

-- Everything below this point is populated by the SLOW poll (§4.1), not the
-- fast event-detection poll — none of it drives notifications, it exists
-- purely to back read-only dashboard pages, so it's refreshed on the same
-- cadence as production_snapshots (default 5 min), not every 20s.

-- Resource/AWESOME Sink, singleton current-value snapshot (FRM only ever reports one)
CREATE TABLE resource_sink_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    num_coupon INTEGER NOT NULL,
    percent REAL NOT NULL,              -- progress toward next coupon
    points_to_coupon INTEGER NOT NULL,
    total_points INTEGER NOT NULL,
    updated_at TEXT NOT NULL
);

-- "Overall Prod" — current per-item production/consumption snapshot (getProdStats).
-- This is the CURRENT-state table backing the dashboard's item table; historical
-- trend data for the same items lives separately in production_snapshots (below),
-- populated from the same poll so both are always in sync.
CREATE TABLE prod_stats_state (
    item_class_name TEXT PRIMARY KEY,
    item_display_name TEXT NOT NULL,
    prod_per_min_label TEXT NOT NULL,   -- FRM's own pre-formatted "P: x/min - C: y/min" string, displayed as-is
    prod_percent REAL NOT NULL,
    cons_percent REAL NOT NULL,
    current_prod REAL NOT NULL,
    max_prod REAL NOT NULL,
    current_consumed REAL NOT NULL,
    max_consumed REAL NOT NULL,
    transfer_type TEXT NOT NULL,        -- 'Belt' | 'Pipe'
    updated_at TEXT NOT NULL
);

-- "Detailed Prod" — per-machine production status (getFactory, which covers
-- every producer building type — Assembler/Foundry/Smelter/Constructor/etc. —
-- in one call; see §4.1).
CREATE TABLE factory_machine_state (
    machine_id TEXT PRIMARY KEY,        -- FRM building ID
    building_type TEXT NOT NULL,        -- e.g. "Assembler", "Foundry" — derived from ClassName
    recipe TEXT,
    manu_speed REAL NOT NULL,
    is_configured BOOLEAN NOT NULL,
    is_producing BOOLEAN NOT NULL,
    is_paused BOOLEAN NOT NULL,
    power_consumed REAL,
    max_power_consumed REAL,
    circuit_group_id INTEGER,
    ingredients_json TEXT NOT NULL DEFAULT '[]',  -- FRM ingredients[] — for /production Detailed expand
    production_json TEXT NOT NULL DEFAULT '[]',   -- FRM production[] — for /production Detailed expand
    updated_at TEXT NOT NULL
);

-- Drone overview (getDrone)
CREATE TABLE drone_state (
    drone_id TEXT PRIMARY KEY,
    home_station TEXT,
    paired_station TEXT,
    has_paired_station BOOLEAN NOT NULL,
    current_destination TEXT,
    flying_speed REAL,
    max_speed REAL,
    current_flying_mode TEXT,           -- 'Unknown' | 'Flying' | 'None' | 'Travelling'
    updated_at TEXT NOT NULL
);

-- Lizard Doggo overview (getDoggo). Pure gimmick/fun page, no notification —
-- deliberately not on the fast poll since nothing here is time-critical.
CREATE TABLE doggo_state (
    doggo_id TEXT PRIMARY KEY,          -- FRM Doggo ID
    name TEXT,                          -- player-given pet name (Name field is player-set, not a fixed species name)
    inventory_json TEXT NOT NULL,       -- Inventory[] as returned by FRM, stored as-is (each item already carries its own display Name — e.g. "SAM" — no ClassName matching needed to show what a doggo is carrying)
    updated_at TEXT NOT NULL
);

-- Time-series snapshots for the production dashboard
CREATE TABLE production_snapshots (
    id INTEGER PRIMARY KEY,
    item_class_name TEXT NOT NULL,
    item_display_name TEXT NOT NULL,
    produced_per_min REAL NOT NULL,
    consumed_per_min REAL NOT NULL,
    captured_at TEXT NOT NULL
);
CREATE INDEX idx_production_snapshots_item_time
    ON production_snapshots (item_class_name, captured_at);

-- Time-series snapshots for the /resource-sink chart. Deliberately NOT a
-- passthrough of FRM's own GraphPoints (that's a fixed, non-interval-
-- selectable rolling window — see §4.1's discussion of why this project
-- builds its own history here instead of mirroring FRM's native chart).
CREATE TABLE resource_sink_snapshots (
    id INTEGER PRIMARY KEY,
    num_coupon INTEGER NOT NULL,
    percent REAL NOT NULL,
    total_points INTEGER NOT NULL,
    captured_at TEXT NOT NULL
);
CREATE INDEX idx_resource_sink_snapshots_time ON resource_sink_snapshots (captured_at);

-- Time-series snapshots for the /power chart, one row per circuit per
-- capture. Sourced from circuit_state (already kept fresh by the fast poll,
-- §4.1) rather than a new FRM call — see §4.1 for the capture mechanics.
CREATE TABLE circuit_snapshots (
    id INTEGER PRIMARY KEY,
    circuit_id INTEGER NOT NULL,
    power_production REAL NOT NULL,
    power_consumed REAL NOT NULL,
    power_capacity REAL NOT NULL,
    battery_percent REAL,
    captured_at TEXT NOT NULL
);
CREATE INDEX idx_circuit_snapshots_circuit_time ON circuit_snapshots (circuit_id, captured_at);

-- App-level configuration (single row)
CREATE TABLE app_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    server_name TEXT NOT NULL DEFAULT 'Satisfactory Server',  -- auto-synced from FRM getSessionInfo.SessionName; cached for {ServerName} in templates
    frm_host TEXT NOT NULL DEFAULT '',
    frm_port INTEGER NOT NULL DEFAULT 8080,
    frm_auth_token TEXT,                -- optional; sent as X-FRM-Authorization when set (see §4.1)
    poll_interval_seconds INTEGER NOT NULL DEFAULT 20,
    production_snapshot_interval_seconds INTEGER NOT NULL DEFAULT 300,
    production_snapshot_retention_days INTEGER NOT NULL DEFAULT 30
);
```

---

## 4. FRM Integration

### 4.1 Endpoints Polled

Two separate cadences: the **fast poll** drives notification/event detection (§4.2); the **slow poll** only feeds read-only dashboard pages that have no diffing or notification logic behind them (§3's note above the dashboard-only tables applies here).

**Fast poll** (every `poll_interval_seconds`, default 20s):

| Endpoint | Purpose |
|---|---|
| `GET /getPlayer` | Player list + online status |
| `GET /getPower` | Power circuits — fuse status (drives `fuse_tripped`/`power_restored`) *and* the full production/consumption/battery fields now persisted in `circuit_state` for the `/power` dashboard, from the same call — no separate slow-poll call needed for power data |
| `GET /getSchematics` | Milestones, Hard Drive unlock status |
| `GET /getSpaceElevator` | Space Elevator phase progress |
| `GET /getResearchTrees` | M.A.M. research tree nodes → `research_node_state`, drives `research_unlocked` |
| `GET /getTrains` | All trains — derailment/self-driving status → `train_state`, drives `train_derailed` |
| `GET /getVehicles` | All wheeled vehicles (Explorer/Tractor/Truck/Factory Cart in one call, same aggregation pattern as `getFactory`) → `vehicle_state`, drives `vehicle_out_of_fuel`/`vehicle_stuck` |

**Server display name** (not part of fast/slow poll entity tables): when FRM is reachable and `frm_host` is set, FactoryMate calls `GET /getSessionInfo` and caches `SessionName` into `app_settings.server_name`. Also fetched on `PUT /api/settings` (when host is set) and via `POST /api/settings/frm/test` for admin preview.

**Slow poll** (every `production_snapshot_interval_seconds`, default 5 min):

| Endpoint | Purpose |
|---|---|
| `GET /getProdStats` | Produced/consumed rates per item — feeds both `prod_stats_state` (current snapshot, "Overall Prod" table) and `production_snapshots` (historical trend, appended each cycle) from the same call |
| `GET /getResourceSink` | A.W.E.S.O.M.E. Sink coupon/points status → `resource_sink_state` (current) and `resource_sink_snapshots` (history, for the `/resource-sink` chart). FRM's own response includes a rolling `GraphPoints` window, but that's a fixed, non-interval-selectable view with no real historical depth — this project builds its own proper time series here rather than mirroring that limitation, same as it already does for production data. |
| `GET /getFactory` | Every producer-type building (Assembler, Foundry, Smelter, Constructor, Manufacturer, Blender, Packager, Refinery, Converter, Encoder, Particle Accelerator) → `factory_machine_state`, "Detailed Prod" table. `getFactory` is a single aggregated endpoint covering all of these — not one call per building type. Derive `building_type` from `ClassName` using the mapping table below. |
| `GET /getDrone` | All drones → `drone_state` |
| `GET /getDoggo` | All Lizard Doggos → `doggo_state`, storing the `Inventory[]` array as-is — each item already has a display `Name` (e.g. "SAM"), so no separate "found SAM" flag or ClassName matching is needed |
| `GET /getModList` | Server mod manifest (dashboard `/mods`, SMM profile export, Discord `/mods`) |

**`getModList` notes:** Returns game build, SML version, and per-mod metadata (`Name`, `Version`, `RemoteVersionRange`, `CreatedBy`, docs URL, `RequiredOnRemote`). Fetched on demand via `frm.Client.GetModList()` and cached in memory (15-minute TTL, shared by API + Discord); admin `POST /api/mods/refresh` busts the cache. Not part of the recurring slow-poll loop.

**`getFactory` `building_type` mapping:** `factory_machine_state.building_type` is derived from each machine's `ClassName`. FRM's per-type endpoints (`getAssembler`, `getFoundry`, `getSmelter`, `getConstructor`, `getManufacturer`, `getBlender`, `getPackager`, `getRefinery`, `getConverter`, `getEncoder`, `getParticle`) have **no separate `.adoc` files** in `docs/frm-docs` — they all xref to `getFactory.adoc`, whose example response contains only a Constructor. ClassNames below are only those that appear as a `"ClassName"` field in vendored docs; Mk2/Mk3 (and any other) variants are listed only when found. Do not invent missing ClassNames.

| Building type | ClassName(s) | Source / notes |
|---|---|---|
| Constructor | `Build_ConstructorMk1_C` | `getFactory.adoc` example response. No Mk2/Mk3 ClassName in vendored docs. |
| Refinery | `Build_OilRefinery_C` | `getPowerUsage.adoc` example (`ClassName` of a factory building). Not present in `getFactory.adoc`. No other refinery variants in vendored docs. |
| Assembler | — | not found in docs, verify against a live `getFactory` response before implementing |
| Foundry | — | not found in docs, verify against a live `getFactory` response before implementing |
| Smelter | — | not found in docs, verify against a live `getFactory` response before implementing |
| Manufacturer | — | not found in docs, verify against a live `getFactory` response before implementing |
| Blender | — | not found in docs, verify against a live `getFactory` response before implementing |
| Packager | — | not found in docs, verify against a live `getFactory` response before implementing |
| Converter | — | not found in docs, verify against a live `getFactory` response before implementing |
| Encoder | — | not found in docs, verify against a live `getFactory` response before implementing |
| Particle Accelerator | — | not found in docs, verify against a live `getFactory` response before implementing |

**FRM client notes (validated against `docs/frm-docs`):**

- All endpoints: `GET http://{frm_host}:{frm_port}/{endpoint}` — no pagination; each returns a JSON array of all entities.
- **Authentication:** Read endpoints do not require a token in normal deployments. FRM supports optional `X-FRM-Authorization: <token>` when configured; FactoryMate sends `app_settings.frm_auth_token` when non-empty.
- **Game-thread endpoints** (`getPlayer`, `getSchematics`, `getResearchTrees`, `getDrone`, `getDoggo`): may add latency on dedicated servers — acceptable at the 20s fast-poll cadence.
- **`getProdStats` requires the [Production Stats mod](https://ficsit.app/mod/3tsvcG3A6gqKX1)** (Andre Aquila) on the Satisfactory server. Without it, production dashboard data will be empty.
- **`getResourceSink`:** response is a JSON **array** (typically one element); use the first element. Alias endpoint `getExplorationSink` exists but FactoryMate uses `getResourceSink`.
- **`getPower` vs `getPowerUsage`:** use `getPower` for circuit-level fuse detection and dashboard metrics; `getPowerUsage` is per-building detail and is not polled.
- **JSON quirks:** `getVehicles.ID` may be string or integer; `getVehicles.Status` is in the adoc field table but missing from the example — treat as optional string; `getVehicles.FollowingPath` is response-confirmed but missing from the adoc field table; `getDrone` speed fields may be number or string; `getResourceSink.GraphPoints` may use `Value` or `value`; `getFactory` `ingredients`/`production` `Amount` may be string or integer in JSON — flexible unmarshal in M2.

Each slow-poll cycle also writes to `circuit_snapshots` (§3) for the `/power` chart — this does **not** require an additional FRM call; `circuit_state` is already kept current by the fast poll's `getPower` call (see the Fast poll table above), so the slow-poll job simply copies its current values into a new `circuit_snapshots` row each cycle. This keeps power's historical chart resolution at the slower cadence (sensible for a trend view) without duplicating the fuse-detection polling.

**Retention:** A background job, running on the same cadence as the snapshot capture itself, deletes rows older than `production_snapshot_retention_days` (default 30) from all three history tables — `production_snapshots`, `resource_sink_snapshots`, and `circuit_snapshots` — immediately after each successful slow-poll cycle. One shared retention setting for all three, since they're captured together on the same schedule; no need for three separate config knobs. This keeps the tables bounded without a separate scheduler; if the poller is down for longer than the retention window, the next successful run prunes everything that aged out during the downtime in one pass. The remaining slow-poll tables (`resource_sink_state`, `prod_stats_state`, `factory_machine_state`, `drone_state`) are current-snapshot upserts, not history — nothing to prune there.

All requests target `http://{frm_host}:{frm_port}/{endpoint}`. A request timeout (5s) and connection failure are both treated as "unreachable" for the purposes of server-online/offline detection — this applies to the fast poll only; a slow-poll failure logs an error but does not affect `server_state`, since the fast poll already owns that determination and runs far more frequently. When `frm_auth_token` is set, include header `X-FRM-Authorization: <token>` on every request.

### 4.1.1 FRM → DB field mapping (fast + slow poll)

Poll upsert rules: `updated_at` on every successful upsert for state tables. Exception: `schematic_state.purchased_at` is set only on `Purchased` `false → true` and never cleared.

| FRM endpoint | FRM field | DB table.column |
|---|---|---|
| `getPlayer` | `ID` | `player_state.player_id` |
| `getPlayer` | `Name` | `player_state.name` |
| `getPlayer` | `Online` | `player_state.online` |
| `getPlayer` | (on leave) | `player_state.last_seen_at` ← poll timestamp when `Online` `true → false` |
| `getPower` | `CircuitGroupID` | `circuit_state.circuit_id` |
| `getPower` | `FuseTriggered` | `circuit_state.tripped` |
| `getPower` | `PowerProduction` … `BatteryTimeFull` | matching `circuit_state.*` columns (see §3) |
| `getSchematics` | `ID` | `schematic_state.schematic_id` |
| `getSchematics` | `Name` | `schematic_state.name` |
| `getSchematics` | `Type` | `schematic_state.type` |
| `getSchematics` | `Purchased` / `Locked` / `TechTier` | `schematic_state.purchased` / `locked` / `tech_tier` |
| `getSchematics` | `Recipes` | `schematic_state.recipes_json` |
| `getSchematics` | (on `Purchased` false→true) | `schematic_state.purchased_at` ← poll timestamp |
| `getSpaceElevator` | `ID` | `elevator_state.elevator_id` |
| `getSpaceElevator` | `Name` | `elevator_state.name` |
| `getSpaceElevator` | `UpgradeReady` | `elevator_state.upgrade_ready` |
| `getSpaceElevator` | `CurrentPhase` | `elevator_state.current_phase_json` |
| `getResearchTrees` | `Nodes[].ID` | `research_node_state.node_id` |
| `getResearchTrees` | tree `Name` | `research_node_state.tree_name` |
| `getResearchTrees` | node fields | `research_node_state.name`, `category`, `state`, `tech_tier` |
| `getResearchTrees` | `Nodes[].Cost` | `research_node_state.cost_json` |
| `getTrains` | `ID` | `train_state.train_id` |
| `getTrains` | `Name` | `train_state.name` |
| `getTrains` | `Derailed` | `train_state.derailed` |
| `getTrains` | `PendingDerail` | `train_state.pending_derail` |
| `getTrains` | `Status` | `train_state.status` |
| `getTrains` | `TrainStation` | `train_state.station` |
| `getTrains` | `SelfDriving` | `train_state.self_driving_error` |
| `getTrains` | `Docking` | `train_state.docking_status` |
| `getTrains` | `Path` | `train_state.path_status` |
| `getVehicles` | `ID` (string or int) | `vehicle_state.vehicle_id` (TEXT) |
| `getVehicles` | `VehicleType` | `vehicle_state.vehicle_type` and `vehicle_state.display_name` |
| `getVehicles` | `Status` | `vehicle_state.status` (optional in live JSON) |
| `getVehicles` | `Driver` | `vehicle_state.driver` |
| `getVehicles` | `AutoPilot` | `vehicle_state.autopilot` |
| `getVehicles` | `FollowingPath` | `vehicle_state.following_path` |
| `getVehicles` | `ForwardSpeed` | `vehicle_state.forward_speed` |
| `getVehicles` | `Fuel[]` (sum `Amount`) | `vehicle_state.fuel_empty` derived; fuels `vehicle_out_of_fuel` (§4.2) |
| `getProdStats` | all rate fields | `prod_stats_state.*` + append `production_snapshots` |
| `getResourceSink` | coupon/points fields | `resource_sink_state.*` + append `resource_sink_snapshots` |
| `getFactory` | machine + power fields | `factory_machine_state.*` (see §3) |
| `getFactory` | `ingredients` / `production` | `factory_machine_state.ingredients_json` / `production_json` |
| `getDrone` | all listed fields | `drone_state.*` (see §3) |
| `getDoggo` | `ID`, `Name`, `Inventory` | `doggo_state.*` |

### 4.2 Diff / Event Detection Logic

All detection is **edge-triggered** (fires only on state transitions, not on every poll where a condition remains true), mirroring the working logic from the prior n8n implementation. Detection and dispatch are decoupled: the poller always evaluates every transition below (except First Observation — no previous row to diff against, see below) and updates state regardless of a message type's `enabled` flag (see §5.3) — a disabled type simply skips the render-and-send step. This matters because several of these transitions are edge-triggered against the *previous stored value* (e.g. `player_left` only fires if `player_joined` was previously observed as `true`); if detection paused while a type was disabled, re-enabling it later could misfire or miss the next transition.

| Message Type | Trigger Condition |
|---|---|
| `server_online` | Previous poll unreachable/offline → current poll reachable |
| `server_offline` | Previous poll reachable → current poll unreachable (timeout or connection error on any polled endpoint) |
| `player_joined` | Player's `Online` flag: `false` → `true` |
| `player_left` | Player's `Online` flag: `true` → `false` |
| `fuse_tripped` | Circuit's `FuseTriggered`: `false` → `true` |
| `power_restored` | Circuit's `FuseTriggered`: `true` → `false` |
| `milestone_unlocked` | Schematic with `Type == "Milestone"`: `Purchased` `false` → `true` |
| `hub_tier_complete` | Same poll emitted at least one `milestone_unlocked` for TechTier `T`, and every schematic with `Type == "Milestone"` and `TechTier == T` in the current poll payload has `Purchased == true` |
| `hard_drive_ready` | Schematic with `Type == "Hard Drive"`: `Locked` `true` → `false` AND `Purchased == false` (i.e. newly available, awaiting recipe selection) |
| `elevator_phase_complete` | Space Elevator's `UpgradeReady`: `false` → `true` (all parts delivered; elevator ready to send) |
| `elevator_phase_done` | Space Elevator's `UpgradeReady`: `true` → `false` (elevator sent; next phase requirements now active) |
| `research_unlocked` | Research Node's `State`: transitions **to** `"Purchased"` from any other value (deliberately not keyed off a specific "from" state — see note below on why) |
| `train_derailed` | Train's `Derailed`: `false` → `true` |
| `vehicle_out_of_fuel` | Vehicle's total `Fuel[]` amount: `> 0` → `0` |
| `vehicle_stuck` | `vehicle_state.stuck`: `false` → `true` (itself a debounced, heuristic-derived value — see note below, not a raw API field) |

**First Observation:** When the poller has no previous state row for an entity (player, circuit, schematic, research node, train, vehicle, elevator), it inserts the baseline row with the entity's current values but does **not** evaluate or emit any of the transitions in the trigger table for that entity on that poll. There is no real previous value to diff against, so nothing fires. A missing row must not be treated as an implicit `false`/`off`/zero. This applies uniformly to every message type in the table — the intended first deploy is an already-progressed save, and treating "unknown" as `false` would fire a burst of `milestone_unlocked`, `research_unlocked`, `fuse_tripped`, and similar events for work that completed long before FactoryMate was installed.

The same rule applies to `server_online`: `server_state.server_online` starts `NULL` (unset) at first boot; the first successful (reachable) poll sets it to `true` silently without firing `server_online`.

`vehicle_stuck` needs no extra special-case under this rule: that trigger requires the derived `stuck` flag to hold across 3 consecutive polls before flipping `false → true`, so it cannot fire on a first observation regardless.

**`research_unlocked` state uncertainty:** `getResearchTrees` documents `State` as "Purchase/Hidden of the Research Node," and the confirmed example only shows `"Purchased"`. Given the response also includes `UnhiddenBy` (nodes that reveal this one), there may be an intermediate "visible but not yet purchased" state not named in the documentation. The trigger is therefore written to fire on *any* transition into `"Purchased"` rather than specifically `"Hidden" → "Purchased"`, so it stays correct regardless of how many intermediate states actually exist — this should be spot-checked against a live response the same way `getPower`'s fields were, before shipping.

**`vehicle_stuck` heuristic:** No FRM field directly reports "stuck" the way `Derailed` does for trains. This is approximated in two layers — a raw candidate condition and a debounced, edge-triggered derived state — so the notification fires exactly once per stuck episode, the same way `fuse_tripped` fires once per trip rather than on every poll while still tripped:

1. **Raw candidate**, evaluated every poll: (`AutoPilot == true` OR `FollowingPath == true`) AND `ForwardSpeed < 0.5`.
2. If the raw candidate is true and `vehicle_state.low_speed_since` is currently `NULL`, set it to this poll's timestamp (don't overwrite it on later polls — that would keep resetting the debounce window). If the raw candidate is false, immediately reset `low_speed_since` to `NULL` and `stuck` to `false`.
3. Once `low_speed_since` has been continuously set for 3 consecutive fast-poll cycles (~1 minute at the default 20s interval) — i.e. the debounce has elapsed — set `vehicle_state.stuck = true`. `vehicle_stuck` fires on **this** flipping `false → true`, exactly like `fuse_tripped`'s edge-trigger, so it fires once per episode and stays silent on every subsequent poll until the vehicle starts moving again (which resets `stuck` back to `false` via step 2, allowing a future episode to fire again).

This debounce avoids false positives from a vehicle merely stopped at a station or waiting at a junction. Because this is a heuristic rather than a confirmed game signal, false positives (e.g. a vehicle briefly paused for a legitimate reason lasting over a minute) are still possible and the threshold/debounce values may need tuning after real-world observation.

**Phase number derivation:** `getSpaceElevator` does not return an explicit phase index, but the *set of part types* required for a given phase is fixed game content data — it does not change between playthroughs or save files, and (absent gameplay-altering mods, which this deployment avoids per the SML/client-compatibility findings above) stays stable across sessions. Required *quantities* can differ (e.g. via the Space Elevator deliverable cost multiplier set at world creation), but the *types* of parts per phase do not, and each phase's set of part types is unique — no two phases share the same combination — so matching on type alone (ignoring `Amount`/`RemainingCost`/`TotalCost`) is sufficient to identify the phase.

**Source-level confirmation (via direct inspection of the FRM repository, `porisius/FicsitRemoteMonitoring`):** `getSpaceElevator`'s `CurrentPhase` field is a direct pass-through of the base game's own `AFGGamePhaseManager::GetRemainingPhaseCosts()` — FRM defines no phase-to-part mapping itself, and no phase index is serialized anywhere in its source or documentation. Each item's `Name` is populated from `UFGItemDescriptor::GetItemName()` (the item's *current, localized display name* — confirmed subject to change, e.g. the historical Automated Wiring/"High-speed Wiring" rename), while `ClassName` is populated from `GetClassDisplayName()` (the stable Unreal class identifier). This is direct source-level confirmation that `ClassName`, not `Name`, is the correct field to key the lookup table on.

**Live verification, Phase 2 (confirmed 2026-08-16):** A real `getSpaceElevator` call against the group's own server, taken while on Phase 2, returned exactly 3 items in `CurrentPhase` — `Desc_SpaceElevatorPart_1_C` (Smart Plating), `Desc_SpaceElevatorPart_2_C` (Versatile Framework), `Desc_SpaceElevatorPart_3_C` (Automated Wiring) — matching Phase 2's expected part count and ClassNames exactly. This confirms both open questions for this phase: **the "current phase only" scoping assumption holds** (no cumulative aggregation across phases), and **`_1_C`/`_2_C`/`_3_C` are correct against live data**, not just the wiki.

**Live verification, Phase 3 (confirmed 2026-08-19):** While on Phase 3, live `getSpaceElevator` returned `Desc_SpaceElevatorPart_2_C` (Versatile Framework), `Desc_SpaceElevatorPart_4_C` (Modular Engine), and `Desc_SpaceElevatorPart_5_C` (Adaptive Control Unit) — matching Phase 3's expected ClassName set exactly. Phases 4–5 remain wiki-sourced only, to be confirmed the same way as the group naturally reaches them (see self-correcting diagnostic logging below).

Also observed in the live response but not previously modeled: each `CurrentPhase[]` item includes `Amount` (current stock/delivered amount at the elevator) and `MaxAmount` (per-delivery cap, e.g. 50) in addition to `RemainingCost`/`TotalCost`. These aren't needed for phase-number derivation but are available if a future dashboard view wants to show delivered-vs-needed progress per item within the current phase.

Reference data (source: [official Satisfactory Wiki, Space Elevator](https://satisfactory.wiki.gg/wiki/Space_Elevator) and per-item wiki pages, default 1× cost multiplier). Every ClassName below is directly confirmed from the official Satisfactory Wiki (each Project Part's own page states its ClassName) — none are guessed from a naming pattern. Note that the numbering does **not** follow phase-appearance order (e.g. Magnetic Field Generator is `_6_C` and Assembly Director System is `_7_C`, even though Assembly Director System is listed first within Phase 4) — it reflects internal implementation order, so this table should not be extrapolated from pattern alone if the game adds more parts in the future:

| Phase | Name | Required Part Types | ClassName | Verified |
|---|---|---|---|---|
| 1 | Distribution Platform | Smart Plating | `Desc_SpaceElevatorPart_1_C` | ✅ Live (2026-08-16, via Phase 2 response) |
| 2 | Construction Dock | Smart Plating, Versatile Framework, Automated Wiring | `Desc_SpaceElevatorPart_1_C`, `Desc_SpaceElevatorPart_2_C`, `Desc_SpaceElevatorPart_3_C` | ✅ Live (2026-08-16) |
| 3 | Main Body | Versatile Framework, Modular Engine, Adaptive Control Unit | `Desc_SpaceElevatorPart_2_C`, `Desc_SpaceElevatorPart_4_C`, `Desc_SpaceElevatorPart_5_C` | ✅ Live (2026-08-19) |
| 4 | Propulsion | Assembly Director System, Magnetic Field Generator, Thermal Propulsion Rocket, Nuclear Pasta | `Desc_SpaceElevatorPart_7_C`, `Desc_SpaceElevatorPart_6_C`, `Desc_SpaceElevatorPart_8_C`, `Desc_SpaceElevatorPart_9_C` | Wiki only |
| 5 | Assembly | Nuclear Pasta, Biochemical Sculptor, AI Expansion Server, Ballistic Warp Drive | `Desc_SpaceElevatorPart_9_C`, `Desc_SpaceElevatorPart_10_C`, `Desc_SpaceElevatorPart_12_C`, `Desc_SpaceElevatorPart_11_C` | Wiki only |

All twelve ClassNames (`_1_C` through `_12_C`) are individually confirmed from official wiki pages; Phases 1–3 are additionally confirmed against the group's own live server (see above). Phases 4–5 remain wiki-sourced only.

FactoryMate ships this table as a maintained data file (not hardcoded inline), matches the live, sorted `CurrentPhase[].ClassName` set against it on every poll, and sets `phase_number` accordingly. **If no match is found** (e.g. a future Satisfactory content update reshuffles phase part lists, the deliverable cost multiplier was set to something other than 1× at world creation — which changes quantities but not types, so this alone should not break matching — or a modded part is present), the backend does not guess: `phase_number` is left `null`/unknown, and any template referencing `{PhaseNumber}` falls back to omitting it (e.g. rendering "the next phase" instead of "Phase 4") rather than showing a wrong number.

**Self-correcting verification, not exhaustive upfront verification:** Fully confirming this table against live API data for every phase would require playing through the entire game (Phase 5 alone requires tens of thousands of several parts), which is impractical purely for spec verification. Instead, whenever a poll's `CurrentPhase` set doesn't match any table entry, the raw unmatched set (item names + ClassNames as returned by FRM) is written to `elevator_phase_unknown_log` (see §3) — a dedicated diagnostic table, not repurposed notification-send history — rather than being silently discarded. **Dedup rule:** insert a new row only when no unresolved row (`resolved = 0`) already exists with the same sorted ClassName set (compare JSON arrays of ClassNames); do not spam one row per failed poll. Entries are surfaced as an admin-visible alert on the `/elevator` page (see §8) and can be marked resolved once the reference table has been corrected.

On unreachable state, only the `server_offline` transition is evaluated; player/power/schematic/elevator/research/train/vehicle state (i.e. every fast-poll table) is left untouched (not reset), so that when the server comes back, transitions are computed against the last known-good state rather than an empty one.

**`server_state` updates:** On every fast poll, upsert `server_state` row `id=1`. Emit `server_online` when previous poll was unreachable and current is reachable; emit `server_offline` when previous was reachable and current is unreachable. First successful poll when `server_online` IS NULL follows the First Observation rule above (set `true`, do not emit `server_online`).

### 4.2.1 Event variable population

When M3 emits `(message_type_key, variables map[string]string)`, populate variables as follows. Missing optional values become empty strings; the renderer treats empty optional variables per §5.4.

| Message type | Variable | Source |
|---|---|---|
| **all types** | `{Timestamp}` | Dispatch time (UTC), formatted e.g. `Aug 17, 2026 · 14:37 UTC` — injected at send, not from FRM |
| **all types** | `{ServerName}` | `app_settings.server_name` (also injected at dispatch when absent from event vars) |
| `server_online`, `server_offline` | `{InGameTime}` | FRM `getSessionInfo`: `Day {PassedDays}, {Hours}:{Minutes}` (zero-padded hours/minutes) |
| `player_joined`, `player_left` | `{PlayerName}` | FRM `getPlayer.Name` for the transitioning player |
| `player_joined`, `player_left` | `{OnlineCount}` | Count of players with `Online == true` **after** applying this poll's player upserts (integer string; templates add "players online") |
| `fuse_tripped`, `power_restored` | `{CircuitID}` | FRM `getPower.CircuitGroupID` as string |
| `fuse_tripped`, `power_restored` | `{PowerProduction}`, `{PowerConsumed}`, `{PowerCapacity}` | FRM circuit at event time, formatted MW |
| `fuse_tripped`, `power_restored` | `{BatteryPercent}`, `{BatteryTimeEmpty}` | FRM circuit when `BatteryCapacity > 0`; omitted (empty) when no batteries |
| `milestone_unlocked` | `{SchematicName}` | FRM `getSchematics.Name` |
| `milestone_unlocked` | `{TechTier}` | FRM `getSchematics.TechTier` as string |
| `milestone_unlocked` | `{RecipeNames}` | Comma-joined `Recipes[].Name` |
| `hub_tier_complete` | `{TechTier}` | Tech tier as string |
| `hub_tier_complete` | `{MilestoneNames}` | Newline-joined names of all `Type == "Milestone"` schematics with that `TechTier` in the current poll payload |
| `hub_tier_complete` | `{MilestoneCount}` | Count of milestones in that tier (integer string) |
| `hard_drive_ready` | `{SchematicName}` | FRM `getSchematics.Name` |
| `hard_drive_ready` | `{RecipeOptions}` | Newline-joined `Recipes[].Name` |
| `elevator_phase_complete` | `{ElevatorName}` | FRM `getSpaceElevator.Name` |
| `elevator_phase_complete` | `{PhaseNumber}` | `elevator_state.phase_number` as string; empty if NULL |
| `elevator_phase_complete` | `{PhaseRequirements}` | Newline-joined `CurrentPhase[]` at ready time (`Name: delivered/TotalCost` where `delivered = TotalCost - RemainingCost`) |
| `elevator_phase_done` | `{ElevatorName}` | FRM `getSpaceElevator.Name` |
| `elevator_phase_done` | `{PhaseNumber}` | **Previous** `elevator_state.phase_number` (the phase just sent); empty if NULL |
| `elevator_phase_done` | `{PhaseRequirements}` | Newline-joined items from **previous** `elevator_state.current_phase_json` (`Name: delivered/TotalCost`) |
| `research_unlocked` | `{NodeName}` | FRM node `Name` |
| `research_unlocked` | `{TreeName}` | Parent tree `Name` |
| `research_unlocked` | `{TechTier}` | Node `TechTier` as string |
| `research_unlocked` | `{ResearchCost}` | Newline-joined node `Cost[]` (`Name × Amount`) |
| `train_derailed` | `{TrainName}` | FRM `getTrains.Name` |
| `train_derailed` | `{StationName}` | FRM `getTrains.TrainStation` (empty string if absent) |
| `train_derailed` | `{TrainStatus}` | FRM `getTrains.Status` |
| `train_derailed` | `{SelfDriving}` | Humanized FRM `getTrains.SelfDriving` code |
| `vehicle_out_of_fuel`, `vehicle_stuck` | `{VehicleType}` | FRM `getVehicles.VehicleType` |
| `vehicle_out_of_fuel`, `vehicle_stuck` | `{VehicleName}` | FRM `Name` when set, else `VehicleType` |
| `vehicle_out_of_fuel`, `vehicle_stuck` | `{Driver}` | FRM `Driver`, or `—` if empty |
| `vehicle_out_of_fuel`, `vehicle_stuck` | `{ForwardSpeed}` | FRM `ForwardSpeed` as km/h, one decimal |

**Event history persistence (M3):** On `player_joined` / `player_left`, INSERT into `player_session_events`. On `fuse_tripped` / `power_restored`, INSERT into `power_circuit_events`. These run regardless of `message_types.enabled`. Backed by `/api/players/history` and `/api/power/history` — not `notification_log`.

### 4.3 Known FRM Limitations (carried over from investigation)

- `getSpaceElevator` provides `UpgradeReady` (boolean) but no explicit phase index/number in its own response. FactoryMate derives it separately via a static ClassName-set lookup table (see §4.2) — Phases 1–3 confirmed against the group's own live server, Phases 4–5 wiki-sourced pending live confirmation. If the current phase's part set doesn't match any table entry, `phase_number` is left unknown rather than guessed, and elevator notifications' `{PhaseNumber}` variable is simply omitted for that occurrence. **`elevator_phase_done`** uses the stored phase number from *before* the send transition, because `CurrentPhase` already reflects the next phase once `UpgradeReady` drops back to `false`.
- FRM's own built-in webhook notification system (JSON template files, configured via the in-game Server Manager UI) was evaluated and found unreliable in testing (a HUB milestone unlock incorrectly fired the Hard Drive notification template). FactoryMate's own polling-and-diffing approach is used instead, giving full control over trigger logic.
- FRM's web server does not autostart by default; `Web_Autostart` must be enabled via the in-game **Server Manager → Server Settings** UI (this is a client-in-game setting, not a config file, as of the `FGUserSettings`-based config system introduced with Satisfactory 1.2 — legacy `.cfg` files under `FactoryGame/Configs/FicsitRemoteMonitoring/` are no longer read).
- `getProdStats` requires the [Production Stats mod](https://ficsit.app/mod/3tsvcG3A6gqKX1) on the Satisfactory server (§4.1).
- Any Satisfactory mod installed server-side (including FRM, which requires SML) causes SML's client/server mod-list compatibility check to reject vanilla (unmodded) clients. At least one additional mod that clients are expected to install anyway (e.g. a QoL mod) resolves this, since it requires clients to have SML installed regardless of FRM.

---

## 5. Notification System

### 5.1 Notification Targets

A **Notification Target** is a named destination with a provider type and provider-specific configuration, configured once centrally and reused across message types.

**v1 provider: Discord**

`notification_targets.provider_type` = `"discord"`. `config_json` stores **only** the inner object below (not `provider_type`):

```json
{
  "channel_id": "123456789012345678",
  "thread_id": "optional"
}
```

Legacy webhook targets (`webhook_url`, `username_override`, `avatar_url_override`) are deprecated as of M15. After upgrade, admins re-select a Discord channel for each target in the UI.

API request/response bodies for targets use `{ "name", "providerType", "config", "enabled" }` where `config` matches the shape above (§7.2).

Targets can be created, edited, disabled (without deleting, to preserve message-type assignments), deleted, and **test-sent** (sends a sample notification using that target's config and a placeholder message, without needing a real trigger to fire). Deleting a target cascades to remove its `message_type_targets` assignments (`ON DELETE CASCADE`, see §3) — message types themselves and their templates are unaffected, they simply lose that one destination. The UI surfaces a confirmation showing how many message types are currently assigned to a target before deletion.

### 5.2 Message Type Catalog

Fixed, seeded set of message types (not user-creatable in v1 — new event types require a code change, but their templates and target assignments are fully user-configurable).

| Key | Label | Category | Available Template Variables |
|---|---|---|---|
| `server_online` | Server Online | server | `{Timestamp}`, `{ServerName}`, `{InGameTime}` |
| `server_offline` | Server Offline | server | `{Timestamp}`, `{ServerName}`, `{InGameTime}` |
| `player_joined` | Player Joined | player | `{Timestamp}`, `{ServerName}`, `{PlayerName}`, `{OnlineCount}` |
| `player_left` | Player Disconnected | player | `{Timestamp}`, `{ServerName}`, `{PlayerName}`, `{OnlineCount}` |
| `fuse_tripped` | Fuse Tripped | power | `{Timestamp}`, `{ServerName}`, `{CircuitID}`, `{PowerProduction}`, `{PowerConsumed}`, `{PowerCapacity}`, `{BatteryPercent}`, `{BatteryTimeEmpty}` |
| `power_restored` | Power Restored | power | `{Timestamp}`, `{ServerName}`, `{CircuitID}`, `{PowerProduction}`, `{PowerConsumed}`, `{PowerCapacity}`, `{BatteryPercent}`, `{BatteryTimeEmpty}` |
| `milestone_unlocked` | Milestone Unlocked | progression | `{Timestamp}`, `{ServerName}`, `{SchematicName}`, `{TechTier}`, `{RecipeNames}` (comma-joined) |
| `hub_tier_complete` | HUB Tier Complete | progression | `{Timestamp}`, `{ServerName}`, `{TechTier}`, `{MilestoneNames}`, `{MilestoneCount}` |
| `hard_drive_ready` | Hard Drive Ready | progression | `{Timestamp}`, `{ServerName}`, `{SchematicName}`, `{RecipeOptions}` (newline-joined list) |
| `elevator_phase_complete` | Elevator Phase Ready | progression | `{Timestamp}`, `{ServerName}`, `{ElevatorName}`, `{PhaseNumber}` (omitted if unknown), `{PhaseRequirements}` |
| `elevator_phase_done` | Elevator Phase Complete | progression | `{Timestamp}`, `{ServerName}`, `{ElevatorName}`, `{PhaseNumber}` (omitted if unknown), `{PhaseRequirements}` |
| `research_unlocked` | Research Unlocked | progression | `{Timestamp}`, `{ServerName}`, `{NodeName}`, `{TreeName}`, `{TechTier}`, `{ResearchCost}` |
| `train_derailed` | Train Derailed | vehicle | `{Timestamp}`, `{ServerName}`, `{TrainName}`, `{StationName}`, `{TrainStatus}`, `{SelfDriving}` |
| `vehicle_out_of_fuel` | Vehicle Out of Fuel | vehicle | `{Timestamp}`, `{ServerName}`, `{VehicleType}`, `{VehicleName}`, `{Driver}`, `{ForwardSpeed}` |
| `vehicle_stuck` | Vehicle Stuck | vehicle | `{Timestamp}`, `{ServerName}`, `{VehicleType}`, `{VehicleName}`, `{Driver}`, `{ForwardSpeed}` |

Each row's `variables_json` in the DB is a **JSON array of strings** matching the variable names in the table above, e.g. `["PlayerName","OnlineCount"]` for `player_joined`. Seeded from §5.2 at M1.

### 5.3 Enabling & Assignment: Message Type → Targets

Each message type has its own `enabled` flag (default: on), independent of which targets it's assigned to. This is the mechanism for "only notify on join/leave": leave `player_joined`/`player_left` enabled with the Discord target assigned, and either disable the other message types outright or simply leave them unassigned — disabling is the clearer choice when the intent is "we don't want this at all right now, but keep the target assignment for later," whereas leaving a type enabled with zero targets just means "nothing to send it to yet."

Target assignment itself is many-to-many: an enabled message type can be sent to zero, one, or multiple targets (e.g. everything to the main status channel, but `fuse_tripped` additionally to a personal ntfy target once that provider exists). A disabled message type is never dispatched regardless of its target assignments — the poller still detects the underlying state transition (so player online/offline state stays correct either way), it simply skips rendering and sending for that type. `message_types.enabled` is also the **global kill switch for personal DMs**: when a type is off, FactoryMate sends neither channel posts nor DMs. Channel delivery still requires at least one assigned enabled target; personal DMs still require the user's per-type opt-in (or the personal-player toggle for matching join/leave).

**Two independent layers.** Guild/channel routing (this page: enabled + target assignment) is separate from personal DMs (`/account/notifications`, coarse Discord `/notifications`). A type can be channel-only, DM-only, both, or neither (enabled but no targets and no DM prefs). Overlap is informational — users may enable a DM for a type that already posts to a channel.

Both the enable toggle and target checkboxes live on the **Notification Templates** dashboard page (`/settings/notifications/templates`, see §8) alongside the template editor for that type, so operators see "is it on," "what it looks like," and "where it goes" together.

**First boot:** Do not seed `notification_targets` or `message_type_targets`. All message types default to `enabled = 1` but zero assignments — notifications are inert until an admin configures targets via the UI (M12).

### 5.4 Templating System

Variable substitution uses a **custom `{VarName}` syntax** (not Go `text/template`). The renderer replaces `{VariableName}` with the corresponding string from the event variables map. Unknown variables in a template cause validation failure at save time.

**Category color palette** (preset hex values for the template editor UI):

| Category | Hex | Use |
|---|---|---|
| server | `#5865F2` | blue |
| player | `#57F287` | green |
| power (alert) | `#ED4245` | red |
| power (restored) / progression | `#FEE75C` | gold/yellow |
| vehicle | `#9B59B6` | purple |

**Emoji lexicon** (default templates — anchor emoji in embed title; state conveyed by wording + color, not by swapping the anchor):

| Category | Anchor | Message types | Title pattern |
|---|---|---|---|
| server | 🌐 | `server_online`, `server_offline` | `🌐 Server is back online` / `🌐 Server went offline` |
| player | 👤 | `player_joined`, `player_left` | `👤 A player joined the server` / `👤 A player disconnected` |
| power | ⚡ | `fuse_tripped`, `power_restored` | Always ⚡ — tripped vs restored differentiated by title wording and color |
| progression | 🏆 / 💾 / 🚀 / 🔬 | milestone, hard drive, elevator, research | Per-type anchor (milestone 🏆, hard drive 💾, elevator 🚀, research 🔬) |
| vehicle | 🚂 / ⛽ / 🛑 | train, fuel, stuck | Train 🚂, fuel ⛽, stuck 🛑 |

Field labels use supporting emojis (e.g. power: 🔌 Circuit, 📊 Demand, ⚡ Production, 🔋 Batteries).

Two render paths exist, selected automatically by the target's `provider_type` at send time:

**A. Plain-text render** (used by any future provider that only accepts a message string, e.g. ntfy, Telegram)
```
👤 **{PlayerName}** joined the server. ({OnlineCount} players online)
```

**B. Structured embed render** (used when `provider_type == "discord"`)
```json
{
  "title": "👤 A player joined the server",
  "description": "",
  "color": "#57F287",
  "fields": [
    { "name": "👤 Player", "value": "{PlayerName}", "inline": true },
    { "name": "🏭 Factory", "value": "{ServerName}", "inline": true },
    { "name": "👥 Online", "value": "{OnlineCount} players online", "inline": true }
  ],
  "footer": "🏭 {ServerName} · {Timestamp}",
  "show_timestamp": true
}
```
`title`, `description`, each field's `name`/`value`, `color`, `footer`, and `show_timestamp` are independently editable. `footer` supports `{VarName}` interpolation. When `show_timestamp` is true, Discord renders a native ISO timestamp on the right side of the footer row. `color` accepts a hex string; the preset palette above is offered in the UI alongside a free-form color picker.

**Defaults:** Every message type ships with built-in defaults in `backend/data/message_defaults.json` (canonical source) and is copied into `message_types.default_template_json` at seed time. On each startup, seed upserts `default_template_json` and `variables_json` while preserving `enabled`. Shape: `{ "plain": "...", "embed": { "title": ..., "description": ..., "color": ..., "fields": [...], "footer": "...", "show_timestamp": true|false } }`.

**Overrides:** `message_templates.template_json` contains **only overridden keys** — e.g. `{"embed": {...}}` when only the embed variant is customized. An absent key falls back to `default_template_json` for that variant. `PUT /api/message-types/:key/template` (§7) accepts a partial body of the same shape (`{ "plain": "..." }`, `{ "embed": {...} }`, or both) and merges it into the existing override row (creating one if absent), so an admin can customize just the `embed` variant while leaving `plain` unset — matching the independent-override design here. `POST .../template/reset?variant=embed` removes only the `embed` key; deletes the entire row if both keys would be absent.

**Optional variables:** When a variable value is empty (e.g. `{PhaseNumber}` when unknown), the renderer substitutes an empty string. Embed fields whose rendered `value` is empty are omitted from the Discord payload. Plain templates should phrase around optional variables (e.g. "Phase {PhaseNumber} complete" → "Phase  complete" if empty — prefer templates that work without optional vars, or omit the field in embed defaults).

**Validation:** On save, render against §5.4.1 sample values and reject if unknown variables, invalid JSON shape, or Discord limits exceeded (title ≤256 chars, description ≤4096 chars, footer ≤2048 chars, ≤25 fields, field name ≤256 / value ≤1024 chars).

**Live preview:** The template editor renders a live preview using sample data as the operator types.

### 5.4.1 Sample data (validation + preview)

| Message type | Sample variables |
|---|---|
| **all types** | `Timestamp` = `Aug 17, 2026 · 14:37 UTC`, `ServerName` = `CBC | Conveyor Belt Cult` |
| `server_online` / `server_offline` | `InGameTime` = `Day 42, 14:37` |
| `player_joined` / `player_left` | `PlayerName` = `Michael`, `OnlineCount` = `4` |
| `fuse_tripped` / `power_restored` | `CircuitID` = `1`, `PowerProduction` = `120`, `PowerConsumed` = `95`, `PowerCapacity` = `100`, `BatteryPercent` = `68`, `BatteryTimeEmpty` = `2h 15m` |
| `milestone_unlocked` | `SchematicName` = `Oil Processing`, `TechTier` = `5`, `RecipeNames` = `Plastic, Rubber` |
| `hub_tier_complete` | `TechTier` = `6`, `MilestoneNames` = `Industrial Manufacturing\nMonorail Train Technology\n…`, `MilestoneCount` = `5` |
| `hard_drive_ready` | `SchematicName` = `Hard Drive (MAM)`, `RecipeOptions` = `Steel Screw\nCopper Sheet` |
| `elevator_phase_complete` | `ElevatorName` = `Space Elevator`, `PhaseNumber` = `2`, `PhaseRequirements` = `Smart Plating: 1000/1000\nVersatile Framework: 500/500\nAutomated Wiring: 100/100` |
| `elevator_phase_done` | `ElevatorName` = `Space Elevator`, `PhaseNumber` = `2`, `PhaseRequirements` = `Smart Plating: 1000/1000\nVersatile Framework: 500/500\nAutomated Wiring: 100/100` |
| `research_unlocked` | `NodeName` = `Oil Processing`, `TreeName` = `MAM`, `TechTier` = `5`, `ResearchCost` = `Copper Sheet × 10\nCable × 15` |
| `train_derailed` | `TrainName` = `Train 1`, `StationName` = `Main Station`, `TrainStatus` = `Self-Driving`, `SelfDriving` = `No error` |
| `vehicle_out_of_fuel` / `vehicle_stuck` | `VehicleType` = `Explorer`, `VehicleName` = `Tractor`, `Driver` = `Michael`, `ForwardSpeed` = `0.0 km/h` |

### 5.5 Default template catalog

All 17 message types (15 game events plus optional `connection_details_changed` and `connection_details` for template editing): see `backend/data/message_defaults.json`. M1 seed reads this file verbatim into `message_types.default_template_json` per key. The `connection_details_changed` type is editable in the template UI but delivery remains via `ConnectionDetailsService` (mandatory DM broadcast), not the game-event dispatcher.

---

## 6. Authentication & Authorization

- Username + password login for **setup** and **break-glass web invites**; session cookie (HTTP-only, secure, SameSite=Lax), backed by the **`sessions` table in SQLite** (§3) — sessions survive process restarts; expired rows cleaned up periodically.
- **Discord OAuth** (M17): when `DISCORD_CLIENT_SECRET` and `FACTORYMATE_PUBLIC_URL` are configured, linked Discord users sign in via **Continue with Discord** on `/login`. OAuth uses the same Discord application as the bot; scopes: `identify` only. OAuth state: random nonce, **SHA-256 stored** in `oauth_states`, 10-minute TTL, single-use. **`GET /api/auth/discord`** (login) provisions **no** accounts — unknown Discord IDs receive an error directing them to `/register` in Discord. **`/register`** slash DMs an OAuth URL (`purpose=register`); callback redirects to `/register/complete` (username + in-game name, no password). **Account → Link Discord** (`purpose=link`) attaches Discord to an existing password session. Discord-origin users have **`password_hash` NULL**; password login rejects NULL hash.
- New passwords (setup, invite accept, admin reset, self-service change) require at least **8 characters** (Discord-only users may omit password until an admin sets one via Settings → Users).
- **Session cookie:** name `factorymate_session`; value = opaque session ID (matches `sessions.id`). **Max age:** 30 days (`expires_at = now + 30d` on login). **Flags:** `HttpOnly`, `Secure` when request is HTTPS (omit in local HTTP dev), `SameSite=Lax`, `Path=/`. **Cleanup:** background job every hour deletes rows where `expires_at < now()` and invalidates matching cookies on next request.
- Passwords hashed with bcrypt.
- Two roles:
  - **admin** — full access: settings, notification targets, message templates, target assignment, user management.
  - **viewer** — read-only access to all dashboard pages (status, players, production, power, resource sink, drones, doggos, milestones, research, vehicles, elevator); no access to settings/templates/targets/users pages (these routes 403 for viewers, and are hidden from navigation).
- First-run: if the `users` table is empty, the app serves a one-time setup page to create the first admin account instead of the login page.
- Additional accounts are created primarily via **Discord `/register`** (M15, M17 OAuth completion on web). Admins generate break-glass **web invite links** only when Discord registration is unavailable: single-use links (preset role, 7-day TTL); invitees set their own username and password at `/invite/:token`. No admin-set passwords.

---

## 7. REST API Reference

All endpoints under `/api`, JSON in/out, session-cookie authenticated unless noted.

**Pagination** (`/api/players/history`, `/api/power/history`, `/api/notification-log`): `?limit=` (default 50, max 200) and `?offset=` (default 0). Response envelope: `{ "items": [...], "total": N }`. Sort: `occurred_at DESC` or `sent_at DESC`.

**Date ranges** (`from`, `to` on metrics/history endpoints): ISO 8601 UTC strings, e.g. `2026-08-17T00:00:00Z`. Inclusive start, exclusive end. Omit `from`/`to` to return all retained data (bounded by retention settings).

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/healthz` | none | Liveness probe — `200 {"status":"ok"}`; no DB or FRM check |
| POST | `/api/auth/setup` | none (only when no users exist) | Create first admin account |
| POST | `/api/auth/login` | none | Login, sets session cookie (rejects users with NULL `password_hash`) |
| GET | `/api/auth/config` | none | `{ "discordOAuthEnabled": bool }` — true when OAuth env configured |
| GET | `/api/auth/discord` | none | Start Discord OAuth (default `purpose=login`; optional `state` from prior server-side create) |
| GET | `/api/auth/discord/callback` | none | OAuth callback — sets session (login/link) or redirects to `/register/complete` |
| POST | `/api/auth/register/complete` | none | `{ token, username, pendingPlayerName }` — finish Discord registration after OAuth |
| GET | `/api/account/discord/link` | session, active | Start logged-in OAuth link flow |
| GET | `/api/invites/:token` | none | Validate invite; return `{ role, expiresAt, status }` if pending |
| POST | `/api/invites/:token/accept` | none | `{ username, password }` — create account, mark invite accepted, set session cookie |
| POST | `/api/auth/logout` | session | Clear session |
| GET | `/api/auth/me` | session | Current user + role |
| PUT | `/api/account/password` | session | Change the current user's own password (any role — this is distinct from `/api/users/:id`, which is admin-only and manages *other* users) |
| GET | `/api/account/notifications` | session, active | Per-user DM preferences by message type, personal player-event toggle, and a viewer-safe catalog (see §7.1) |
| PUT | `/api/account/notifications` | session, active | Partial update of the current user's DM preferences (`types` map and/or `dmPlayerPersonal`) |
| GET | `/api/status` | session | Server online state, online player count, active fuse trips, latest milestone summary, elevator phase summary (see §7.1) |
| GET | `/api/players` | session | Current player list with online status |
| GET | `/api/players/history` | session | Join/leave events from `player_session_events`, paginated |
| GET | `/api/power` | session | Current circuit states, including production/consumption/battery detail |
| GET | `/api/power/history` | session | Fuse trip/restore events from `power_circuit_events`, paginated |
| GET | `/api/power/metrics?circuit=&from=&to=` | session | Historical power production/consumption/battery series for chart rendering (`circuit_snapshots`); `circuit` optional filter |
| GET | `/api/production?item=&from=&to=` | session | Historical production snapshot series for chart rendering (`production_snapshots`); `item` optional filter |
| GET | `/api/production/items` | session | Distinct list of tracked item class names (for filter dropdown) |
| GET | `/api/production/current` | session | Current per-item snapshot ("Overall Prod" table, `prod_stats_state`) |
| GET | `/api/production/machines` | session | Current per-machine production status ("Detailed Prod" table, `factory_machine_state`) |
| GET | `/api/resource-sink` | session | Current A.W.E.S.O.M.E. Sink coupon/points status |
| GET | `/api/resource-sink/history?from=&to=` | session | Historical coupon/points series for chart rendering (`resource_sink_snapshots`) |
| GET | `/api/drones` | session | Current drone list and status |
| GET | `/api/doggos` | session | Current Lizard Doggo list, with each doggo's carried inventory items (raw item names, as returned by FRM) |
| GET | `/api/milestones` | session | Current schematic state, grouped by type (Milestone / Hard Drive / Alternate / ...) |
| GET | `/api/research` | session | Current M.A.M. research tree state, grouped by tree/category |
| GET | `/api/vehicles` | session | Current trains and wheeled vehicles, with derailed/fuel/stuck status |
| GET | `/api/elevator` | session | Current elevator phase state |
| GET | `/api/mods` | session, active | Cached server mod list + `gameBuild`, `smlVersion`, `cachedAt`, `frmReachable` |
| GET | `/api/mods/smmprofile` | session, active | Downloadable `.smmprofile` file |
| GET | `/api/connection-details` | session, active | Game join details (`gameHost`, `gamePort`, `gamePassword`, `notes`, `smmProfileName`) |
| GET | `/api/elevator/unknown-log` | admin | Unresolved entries plus resolved entries from the last 30 days (`resolved_at` within window), max 50 rows total |
| POST | `/api/elevator/unknown-log/:id/resolve` | admin | Mark a diagnostic entry as resolved (after correcting the reference table) |
| GET | `/api/notification-targets` | admin | List targets |
| POST | `/api/notification-targets` | admin | Create target |
| PUT | `/api/notification-targets/:id` | admin | Update target |
| DELETE | `/api/notification-targets/:id` | admin | Delete target |
| POST | `/api/notification-targets/:id/test` | admin | Send a sample notification through this target |
| GET | `/api/message-types` | admin | List message types with `enabled` state, current template (override or default), and assigned target IDs |
| PUT | `/api/message-types/:key/enabled` | admin | Toggle a message type on/off |
| PUT | `/api/message-types/:key/template` | admin | Save template override. Request body only needs the variant(s) being updated — a partial `{ "plain": "..." }`, `{ "embed": {...} }`, or both. The backend merges provided keys into the existing `message_templates` override row (creating one if absent) rather than requiring a full replace of both variants; omitted variants are left unchanged. Matches independent plain/embed overrides in §5.4. |
| POST | `/api/message-types/:key/template/reset?variant=plain\|embed\|all` | admin | Delete override for the given variant (or both), revert to default |
| POST | `/api/message-types/:key/template/preview` | admin | Render given (unsaved) template against sample data, return rendered result for live preview |
| PUT | `/api/message-types/:key/targets` | admin | Replace target assignment set for this message type |
| GET | `/api/notification-log?type=&target=&delivery=&limit=&offset=` | admin | Recent sent-notification audit log (`notification_log`), paginated. Response items include `deliveryMode` (`channel` \| `dm`), nullable `targetId`, `targetName`, and `recipientExternalUserId` (DM rows). Query with a LEFT JOIN on `notification_targets` (not INNER JOIN) so rows whose `target_id` no longer exists are still returned; `target_id` has no FK (§3). |
| GET | `/api/settings` | admin | App settings (FRM host/port, poll intervals, retention, cached serverName) |
| PUT | `/api/settings` | admin | Update app settings; when `frmHost` is set, probes FRM `getSessionInfo` and updates `serverName` |
| GET | `/api/settings/notification-defaults` | admin | Admin per-type DM defaults for new users plus catalog (see §7.1) |
| PUT | `/api/settings/notification-defaults` | admin | Partial update of admin DM defaults for new users |
| POST | `/api/settings/frm/test` | admin | `{ frmHost, frmPort, frmAuthToken? }` → `{ sessionName, reachable: true }` (preview only) |
| GET | `/api/users` | admin | List users (includes `status`, optional `playerId`/`playerName`) |
| PUT | `/api/users/:id` | admin | Update user (`role?`, `password?`, `playerId?` — `null` clears mapping) |
| DELETE | `/api/users/:id` | admin | Delete user (cannot delete last admin) |
| POST | `/api/invites` | admin | `{ role }` → invite with `invitePath`, `token`, `expiresAt` |
| GET | `/api/invites` | admin | List invites with derived status |
| DELETE | `/api/invites/:id` | admin | Revoke pending invite |
| POST | `/api/mods/refresh` | admin | Re-fetch mod list from FRM and bust SMM cache |
| PUT | `/api/connection-details` | admin | Update join details; triggers mandatory DM broadcast to linked users |
| GET | `/api/discord/settings` | admin | Bot status, guild ID, role mappings, `autoApprove` |
| PUT | `/api/discord/settings` | admin | Update guild ID, role mappings, `autoApprove` |
| GET | `/api/discord/channels` | admin | Guild text channels for notification target picker |
| GET | `/api/discord/invite-url` | admin | OAuth URL to add the bot to the guild |
| GET | `/api/registrations/pending` | admin | Pending registration approval queue |
| POST | `/api/registrations/:id/approve` | admin | Approve pending registration |
| POST | `/api/registrations/:id/reject` | admin | `{ comment? }` — reject and notify registrant |
| GET | `/api/players/unmapped` | admin | Server players with no linked FactoryMate user |
| PUT | `/api/users/:id/external` | admin | Override or unlink external identity |

### 7.1 Response schemas

JSON field names use **camelCase** in API responses; DB columns remain snake_case. M8 tests assert against the shapes below. Endpoints not listed here return objects that mirror §3 table columns (same fields, camelCase) — e.g. `GET /api/power` → `{ "circuits": [ { "circuitId", "tripped", "powerProduction", ... } ] }`.

**`GET /healthz`**
```json
{ "status": "ok" }
```

**`GET /api/status`**
```json
{
  "serverOnline": true,
  "serverName": "GuggiRaid Factory",
  "onlinePlayerCount": 3,
  "trippedCircuits": [1],
  "latestMilestone": { "name": "Oil Processing", "techTier": 5, "unlockedAt": "2026-08-16T14:30:00Z" },
  "elevator": { "name": "Space Elevator", "phaseNumber": 2, "upgradeReady": false, "percentComplete": 45.2 }
}
```
- `latestMilestone`: row in `schematic_state` with `type = "Milestone"` and `purchased = true`, highest `purchased_at` (§3); `null` if none. `unlockedAt` = `purchased_at` ISO string.
- `elevator.percentComplete`: `100 * (1 - sum(RemainingCost) / sum(TotalCost))` over items in `current_phase_json`; `null` if empty or sums are zero.

**`GET /api/players`**
```json
{ "players": [{ "id": "...", "name": "Guggi", "online": true, "lastSeenAt": "2026-08-17T10:00:00Z" }] }
```

**`GET /api/players/history`**
```json
{ "items": [{ "id": 1, "playerId": "...", "playerName": "Guggi", "eventType": "joined", "onlineCount": 3, "occurredAt": "..." }], "total": 42 }
```

**`GET /api/power/history`**
```json
{ "items": [{ "id": 1, "circuitId": 1, "eventType": "fuse_tripped", "occurredAt": "..." }], "total": 10 }
```

**`GET /api/elevator`**
```json
{
  "elevatorId": "...",
  "name": "Space Elevator",
  "upgradeReady": false,
  "phaseNumber": 2,
  "currentPhase": [{ "name": "Smart Plating", "className": "...", "amount": 10, "remainingCost": 100, "totalCost": 500 }]
}
```

**`GET /api/milestones`**
```json
{ "groups": [{ "type": "Milestone", "techTier": 5, "schematics": [{ "id": "...", "name": "...", "purchased": true, "locked": false, "recipes": [{ "name": "Plastic", "className": "..." }] }] }] }
```

**`GET /api/research`**
```json
{ "trees": [{ "name": "MAM", "nodes": [{ "id": "...", "name": "...", "state": "Purchased", "techTier": 5, "cost": [{ "name": "...", "amount": 100 }] }] }] }
```

**`GET /api/production/machines`**
```json
{ "machines": [{ "machineId": "...", "buildingType": "Assembler", "recipe": "...", "ingredients": [...], "production": [...], "isProducing": true }] }
```

**`GET /api/settings`**
```json
{
  "serverName": "...",
  "frmHost": "192.168.178.42",
  "frmPort": 8889,
  "frmAuthToken": "",
  "pollIntervalSeconds": 20,
  "productionSnapshotIntervalSeconds": 300,
  "productionSnapshotRetentionDays": 30
}
```

**`GET /api/account/notifications`** and **`PUT /api/account/notifications`** response:
```json
{
  "types": { "fuse_tripped": false, "power_restored": true },
  "dmPlayerPersonal": false,
  "catalog": [
    {
      "key": "fuse_tripped",
      "label": "Fuse tripped",
      "category": "power",
      "globallyEnabled": true,
      "channelTargets": [{ "id": 1, "name": "Factory alerts" }]
    }
  ]
}
```

`catalog` is viewer-safe (no admin `GET /api/message-types` required). It omits `connection_details` and `connection_details_changed`. `channelTargets` lists enabled assigned guild targets for overlap hints. Missing per-user rows inherit admin defaults on read.

**`GET /api/settings/notification-defaults`** and **`PUT /api/settings/notification-defaults`** response:
```json
{
  "types": { "fuse_tripped": false, "power_restored": false },
  "dmPlayerPersonalDefault": false,
  "catalog": []
}
```

`types` keys are `message_types.key` values (not categories). `notifications.dm_defaults_json` stores the same per-type booleans.

**Errors:** `400` validation, `401` unauthenticated, `403` forbidden, `404` not found, `500` internal. Error body: `{ "error": "message" }`.

### 7.2 Request bodies (mutating endpoints)

| Endpoint | Body |
|---|---|
| `POST /api/auth/setup` | `{ "username", "password" }` |
| `POST /api/auth/login` | `{ "username", "password" }` |
| `PUT /api/account/password` | `{ "password" }` (new password) |
| `PUT /api/account/notifications` | `{ "types"?: { "<message_type_key>": bool }, "dmPlayerPersonal"?: bool }` — omitted keys are left unchanged; unknown/`connection_*` keys ignored |
| `POST /api/notification-targets` | `{ "name", "providerType": "discord", "config": { "channel_id", "thread_id?" }, "enabled": true }` |
| `PUT /api/notification-targets/:id` | same fields, partial update allowed |
| `PUT /api/connection-details` | `{ "gameHost", "gamePort", "gamePassword?", "notes?", "clearPassword?", "smmProfileName?" }` |
| `PUT /api/discord/settings` | `{ "guildId?", "autoApprove?", "roleMappings": { ... } }` per discord-bot-plan §10.2 |
| `PUT /api/message-types/:key/enabled` | `{ "enabled": boolean }` |
| `PUT /api/message-types/:key/template` | Partial `{ "plain"?: "...", "embed"?: { title, description, color, fields } }` — merge into the existing override; omitted variants are left unchanged (not a full replace; see §5.4, §7) |
| `POST /api/message-types/:key/template/preview` | `{ "variant", "template" }` |
| `PUT /api/message-types/:key/targets` | `{ "targetIds": [1, 2, ...] }` |
| `PUT /api/settings` | subset of settings fields from `GET /api/settings` (excluding `serverName` — read-only, auto-synced) |
| `PUT /api/settings/notification-defaults` | `{ "types"?: { "<message_type_key>": bool }, "dmPlayerPersonalDefault"?: bool }` — partial; omitted keys unchanged |
| `POST /api/settings/frm/test` | `{ "frmHost", "frmPort", "frmAuthToken"? }` |
| `POST /api/invites` | `{ "role": "admin" \| "viewer" }` |
| `POST /api/auth/register/complete` | `{ "token", "username", "pendingPlayerName" }` — Discord OAuth registration after `/register` |
| `POST /api/invites/:token/accept` | `{ "username", "password" }` |
| `PUT /api/users/:id` | `{ "role"?, "password"?, "playerId"? }` (`playerId: null` clears mapping) |

---

## 8. Page Inventory (Frontend)

| Route | Access | Purpose | Key Components |
|---|---|---|---|
| `/setup` | public, only if no users exist | First-run: create initial admin account | Form (username, password, confirm) |
| `/login` | public | Login | Form (username/password); **Continue with Discord** when OAuth configured |
| `/register/complete` | public | Finish Discord OAuth registration (username + in-game name) | Form |
| `/invite/:token` | public | Accept invite — set username/password, preset role from invite | Form (username, password, confirm) |
| `/` (Dashboard Overview) | viewer, admin | At-a-glance status from `GET /api/status` (server badge, players, fuse warnings, latest milestone, elevator progress) | Status cards, badge, progress bar |
| `/players` | viewer, admin | Full player roster (online/offline), last-seen timestamps, join/leave history timeline | Table, timeline list |
| `/production` | viewer, admin | Two views, mirroring FRM's own web UI which this was modeled after: **Overall** — a table of every tracked item (name, `ProdPerMin` label, prod/cons %, current/max produced, current/max consumed, from `prod_stats_state`); clicking a row expands a historical trend chart for that item below the table, backed by `production_snapshots`. **Detailed** — a table of every producer machine (building type, recipe, manufacturing speed %, producing/paused status, from `factory_machine_state`); clicking a row expands that machine's full ingredient/output breakdown. | Recharts line chart (on row expand), item combobox, date-range picker, `Table` for both views, `Tabs` to switch between them |
| `/power` | viewer, admin | Per-circuit table: fuse status, power capacity/production/consumption/max-consumption, battery differential/percent/capacity/time-empty/time-full — full detail from `circuit_state`, not just the trip/restore boolean; a discrete fuse trip/restore event history; and a proper interval-selectable historical chart (production/consumption/battery over time, per circuit), backed by `circuit_snapshots` — not FRM's own fixed-window graphing | Table, history list, Recharts chart with date-range control |
| `/resource-sink` | viewer, admin | Current A.W.E.S.O.M.E. Sink status (coupon count, progress-to-next-coupon, total points) as cards, plus a proper interval-selectable historical chart of points/coupons over time, backed by `resource_sink_snapshots` — deliberately not a passthrough of FRM's own fixed rolling graph (see §4.1) | Cards, Recharts chart with date-range control |
| `/drones` | viewer, admin | Drone list: home/paired station, current destination, flying mode, current/max speed | Table |
| `/doggos` | viewer, admin | Lizard Doggo list with their player-given names and whatever they're currently carrying (raw item names, no filtering — SAM just shows up in the list like anything else) — pure fun/gimmick page | Table, Badge |
| `/milestones` | viewer, admin | Grouped schematic view: Milestones (unlocked/locked, by tech tier), Hard Drives awaiting selection (with recipe options shown), Alternate recipes unlocked | Tabs or grouped accordion, badges |
| `/research` | viewer, admin | M.A.M. research trees grouped by tree name, each node showing purchase state, tech tier, and cost — flat grouped view for v1, not the visual node-graph the underlying coordinate/parent data would technically support (possible future enhancement, not built now) | Accordion (grouped by tree), Table, Badge |
| `/vehicles` | viewer, admin | Trains (derailed/pending-derail flags, self-driving status, current station) and wheeled vehicles (type, driver, fuel status, autopilot/route status) as two grouped tables | Tabs (Trains / Wheeled Vehicles), Table, Badge |
| `/elevator` | viewer, admin | Current phase required-items progress bars, upgrade-ready indicator; admins additionally see an alert banner if `elevator_phase_unknown_log` has unresolved entries, with the raw mismatched data and a "mark resolved" action | Progress bars, card, admin-only alert |
| `/mods` | viewer, admin (active users) | Full server mod list from FRM `getModList`; game build + SML header; disclaimer; download SMM profile; admin refresh | Table, cards, alert |
| `/settings/notifications/targets` | admin only | CRUD for Notification Targets (Discord channel picker), per-target "Send test" button; legacy webhook deprecation banner | Data table, dialog forms |
| `/settings/notifications/defaults` | admin only | Admin per-type DM defaults for newly registered users (`dmPlayerPersonalDefault` included); two-layer copy + overlap hints | Form, switches |
| `/settings/notifications/templates` | admin only | List of message types (17 keys including optional `connection_details_changed` and `connection_details`); selecting one opens the template editor (plain-text + embed fields, variable picker, live preview, target assignment checkboxes for that type, reset-to-default). Short callout that this page is the channel layer vs personal DMs. | Data table + detail panel, live preview card |
| `/settings/notifications/log` | admin only | Recent sent notifications with success/failure status | Data table |
| `/settings/general` | admin only | FRM host/port/token, poll interval, production snapshot interval/retention; server display name shown read-only (auto-fetched from FRM) | Form, test-connection button |
| `/settings/discord` | admin only | Bot status, invite URL, guild ID, role mapping editor, auto-approve toggle | Form, table, badges |
| `/settings/connection` | admin only | Game join details + SMM profile name | Form |
| `/settings/users` | admin only | User management: Discord-first onboarding copy, break-glass invites, pending approval queue, unmapped players panel, external identity columns, promote to admin, optional player mapping, edit/delete | Data table, dialog forms |
| `/account` | viewer, admin | Change own password (when set); **Link Discord** when OAuth configured | Form, button |
| `/account/notifications` | viewer, admin (active users) | Per-type DM checkboxes grouped by category, two-layer intro, overlap hints, personal player-event toggle | Form, switches |

Navigation: a persistent sidebar (shadcn `Sidebar` pattern) with the dashboard pages always visible to both roles, and a "Settings" section only rendered/routable for admins.

### 8.1 Component Mapping (shadcn/ui)

Concrete shadcn/ui components — and, where one exists, a full shadcn **block** (a pre-assembled, literally copy-paste page/section) — per route. `shadcn add <name>` installs each into the project as owned source, not a package dependency.

| Route | shadcn Block | shadcn Components | Custom composition needed |
|---|---|---|---|
| `/setup`, `/login` | `login-01` (centered card login form) as starting point | `Card`, `Form`, `Input`, `Label`, `Button` | None — block covers this directly |
| App shell (all pages) | `sidebar-07` (collapsible sidebar with grouped nav + user menu footer) | `Sidebar`, `Breadcrumb`, `Avatar`, `DropdownMenu`, `Separator` | Nav items/groups (every viewer-accessible route from §8's table, plus a Settings group admin-only) are your own data, block only provides the shell |
| `/` (Overview) | `dashboard-01` as loose layout reference (card grid) | `Card`, `Badge`, `Avatar` (online players), `Progress` (elevator bar from `/api/status`) | Status cards from `GET /api/status` — no elevator unknown-log alert here (admin alert is on `/elevator` only, §8) |
| `/players` | — | `Table` (+ `@tanstack/react-table` for sort/filter — shadcn's "Data Table" pattern), `Avatar`, `Badge` (online/offline) | **Timeline** for join/leave history has no stock shadcn component — compose from `Card` + `Separator` + a simple vertical list |
| `/production` | — | `Tabs` (Overall / Detailed), `Table` (Data Table pattern, both views), `Chart` (shadcn's Recharts wrapper: `ChartContainer`/`ChartTooltip`/`ChartLegend`, rendered inline below an expanded row), `Calendar`+`Popover`/`Select` (date-range for the expanded chart), `Badge` (producing/paused status) | The "click a row to expand a chart/detail panel beneath it" interaction (mirroring FRM's own web UI, per the page's purpose in §8) isn't a single stock component — compose from `Table`'s row click handler + a conditionally rendered `Chart`/`Card` block |
| `/power` | — | `Table` (Data Table pattern, full battery/power column set), `Progress` (capacity/consumption bars), `Badge`/`Alert` (tripped state), `Chart` (Recharts wrapper, per-circuit historical trend), `Combobox` (circuit picker) + `Calendar`+`Popover`/`Select` (date-range, same pattern as `/production`) | None |
| `/resource-sink` | — | `Card` (current coupon count, progress, totals), `Chart` (Recharts wrapper, historical trend), `Calendar`+`Popover`/`Select` (date-range, same pattern as `/production`) | None |
| `/drones` | — | `Table` (Data Table pattern), `Badge` (flying mode) | None |
| `/doggos` | — | `Table` (Data Table pattern, inventory items shown as a badge list or comma-joined cell per row) | None |
| `/milestones` | — | `Tabs` (Milestone / Hard Drive / Alternate), `Accordion` (grouped by tech tier), `Badge` (locked/unlocked/tier), `Card` (hard-drive recipe options) | None |
| `/research` | — | `Accordion` (grouped by tree name), `Table` or `Card` per node, `Badge` (Purchased/Hidden state) | None — same composition pattern as `/milestones` |
| `/vehicles` | — | `Tabs` (Trains / Wheeled Vehicles), `Table` (both groups), `Badge` (derailed/fuel-empty/stuck states) | None |
| `/elevator` | — | `Card`, `Progress` (per required item), `Alert` (destructive variant, admin-only unresolved diagnostics) | None |
| `/settings/notifications/targets` | — | `Table`, `Dialog` (create/edit form), `AlertDialog` (delete confirmation — important given cascade-delete warning in §5.1), `Form`, `Input`, `Select` (provider type), `Switch` (enabled) | None |
| `/settings/notifications/defaults` | — | `Card`, `Form`, `Alert` (two-layer copy), `Switch` (per type + personal default) | Informational overlap hints (not `AlertDialog`) |
| `/settings/notifications/templates` | — | `Table`/list (left pane, with `Switch` per row for `enabled`), `Alert` (two-layer callout), `Tabs` (Plain / Embed sub-editor), `Textarea` (plain template, embed description), `Input` (embed title, footer), `Switch` (show timestamp), `Command`+`Popover` (variable-insert picker), `Checkbox`/`Toggle Group` (target assignment), `Card`+`Separator` (embed live-preview with footer row) | **Three real gaps here, no stock component:** (1) a repeatable "Fields" array editor for embed fields (build with `react-hook-form`'s `useFieldArray` + `Input` pairs + an "Add field" `Button`); (2) a hex color picker (shadcn has none — pair a `Popover` with a small custom swatch grid, or a plain `Input type="color"`); (3) the Discord-embed-style preview card itself (colored left border, title/description/fields/footer layout) is a custom composition of `Card`+`Separator`, not a stock look |
| `/settings/notifications/log` | — | `Table`, `Badge` (success/fail) | Channel rows: when `target_id` no longer resolves to a `notification_targets` row (target was deleted; `target_id` has no FK, §3), render "Deleted target" (or similar) instead of crashing or showing a blank. DM rows: show delivery mode and `recipientExternalUserId`. `GET /api/notification-log` LEFT JOINs so channel rows are present (§7). |
| `/settings/general` | — | `Form`, `Input`, `Label` | None |
| `/settings/users` | — | `Table`, `Dialog`, `AlertDialog`, `Select` (role) | None |
| `/account` | — | `Card`, `Form`, `Input`, `Button` | None |
| `/account/notifications` | — | `Card`, `Form`, `Alert` (two-layer copy), `Switch` (per type + personal player events) | Informational overlap hints (not `AlertDialog`); disabled row when `globallyEnabled` is false |
| Global | — | `Sonner` (toast: save confirmations, test-send results, errors) | None |

**Net effect:** most of the app — auth, shell/navigation, tables, dialogs, forms, charts, badges — is genuinely copy-paste from shadcn's registry with no custom styling work. The one page that needs real hand-built UI is the notification template editor (`/settings/notifications/templates`), specifically the repeatable embed-fields array, the color picker, and the Discord-style preview card — all straightforward compositions of existing primitives, just not single-command installs.

### 8.2 Internationalization (i18n)

The frontend uses **proper i18n from the first commit** — no user-facing hardcoded strings in components, pages, or layouts.

| Decision | v1 choice |
|---|---|
| Library | **next-intl** (App Router) |
| MVP locale | **English (`en`) only** — ship one locale file; no language switcher in UI |
| Locale files | `frontend/messages/en.json` (nested keys by area) |
| Access pattern | `useTranslations('<namespace>')` in client components; `getTranslations` in server components / RSC |

**Rules (mandatory for all frontend work):**

1. **No hardcoded UI strings** in JSX/TSX — labels, buttons, headings, nav items, empty states, validation messages shown in the UI, toast text, dialog titles, table column headers, badge labels for *UI state* (e.g. "Online", "Tripped"), and `aria-label` / `title` attributes must come from locale files.
2. **Namespace layout** in `en.json` — group by feature, e.g. `nav`, `auth`, `players`, `power`, `settings`, `common` (shared: Save, Cancel, Loading, errors).
3. **Interpolation** for dynamic UI text: `t('players.onlineCount', { count: n })` — not string concatenation in components.
4. **Pluralization** where needed: use next-intl ICU message syntax in JSON (e.g. `{count, plural, one {# player} other {# players}}`).

**What is NOT translated via i18n (display as-is from API/game):**

- Player names, schematic names, item names, recipe names, train names, doggo inventory item names — FRM/game data
- Discord notification templates (§5 — operator-editable, separate system)
- Raw enum/status strings from FRM when shown as data (e.g. train `SelfDriving` code) — may add display mapping later; v1 can show raw value or a simple i18n map keyed by known enums if a human label is needed

**Future languages:** Adding `de.json` (or others) is a file + config change only — no component rewrites if v1 follows this rule. Locale switcher UI is **out of scope for v1** (see §10).

**M0 sets up** next-intl wiring; **every page milestone (M10–M12)** adds keys to `en.json` alongside the UI — never defer i18n to a later cleanup pass.

### 8.3 Page acceptance criteria (M11 DoD)

Verifier checks for M11 edge cases — concrete acceptance beyond "renders real data":

| Page | Acceptance |
|---|---|
| `/elevator` | When `elevator.phaseNumber` is `2`, progress bars reflect Phase 2 required items from `currentPhase`; `percentComplete` matches §7.1 formula |
| `/elevator` | Admin sees destructive `Alert` when `elevator_phase_unknown_log` has unresolved rows; viewer does not |
| `/power` | Tripped circuit shows `Badge`/`Alert` tripped state; fuse trip/restore events appear in history list; expanded chart shows per-circuit metrics from `circuit_snapshots` |
| `/production` | Row expand reveals chart (Overall) or ingredient/production breakdown (Detailed) — not a navigation away |
| `/players` | Join/leave timeline composed from history endpoint (custom composition per §8.1) |
| `/settings/notifications/log` | Rows with deleted `target_id` show "Deleted target" (or i18n equivalent), not blank or crash |

---

## 9. Configuration / Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Backend HTTP port |
| `DATABASE_PATH` | `/data/factorymate.db` | SQLite file path (mounted volume) |
| `SESSION_SECRET` | — (**required**) | Cookie signing secret — process refuses to start if unset |
| `FRM_HOST` | `""` | Initial FRM host; seeded into `app_settings.frm_host` on first boot (editable via UI) |
| `FRM_PORT` | `8080` | Initial FRM port; seeded into `app_settings.frm_port` |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Frontend dev: direct backend URL. Production: omit (same-origin `/api` proxy) |
| `DISCORD_BOT_TOKEN` | `""` | Discord bot gateway token (slash commands, channel posts, DMs) |
| `DISCORD_CLIENT_SECRET` | `""` | Discord OAuth client secret (same application as bot). Required with `FACTORYMATE_PUBLIC_URL` for SSO |
| `FACTORYMATE_PUBLIC_URL` | `""` | Public dashboard base URL (OAuth redirect `{url}/api/auth/discord/callback`, bot copy). Required for SSO |
| `DISCORD_GUILD_ID` | `""` | Optional env fallback for guild ID (prefer Settings → Discord) |

**Startup behavior:** If `SESSION_SECRET` is unset, the backend exits with a clear error. If `FRM_HOST` is unset, seed `app_settings` with empty `frm_host` — poller logs errors until configured via `/settings/general`.

Notification target credentials (Discord **channel IDs** for channel posts) live in the `notification_targets` table, configured via the UI — not environment variables. The Discord bot token (`DISCORD_BOT_TOKEN`) enables channel posts via the bot API and direct messages; webhook URLs are **not** used in v1.

---

## 10. Deferred Decisions & Defaults

Everything below was genuinely open earlier in this project's design discussion and has since been resolved with a deliberate default, not left dangling — each can be revisited if the group's actual usage later calls for it, but v1 ships without them.

- **`hard_drive_ready` follow-up notification on recipe selection** (`Purchased` transitioning to `true` after the fact): **decided against for v1.** The moment worth notifying on is "a choice is now available" — the actual selection is a single-person, low-stakes action with little group-relevant signal. `schematic_state` already captures `purchased`/`locked` either way, so adding this later is a small, additive change to §4.2's diff table, not a schema migration.
- **Additional notification providers** (ntfy, Telegram, generic webhook): **decided against for v1.** Discord is the group's actual, confirmed destination; building providers with no real target to point them at is speculative work. The `Provider` interface (§2.3) exists specifically so this is a contained addition later — a new struct implementing `Send`/`Type`, no changes to the poller, templating, or dispatch layers.
- **Per-message-type polling cadence** (e.g. faster polling for player join/leave than for schematics): **decided against for v1.** A single shared `poll_interval_seconds` (default 20s, §4.1) is well under any latency the group would notice for a 5–6 player casual server, and per-type scheduling would meaningfully complicate M3's poll loop for no observed benefit.
- **Confirming Phases 4–5's Space Elevator ClassName mapping against live data**: not a decision to make, just not yet possible — the group hasn't reached those phases. The self-correcting `elevator_phase_unknown_log` mechanism (§4.2) closes this automatically as the save progresses; no action needed until an entry actually appears there. Phase 3 was confirmed live on 2026-08-19.
- **Additional UI languages** (German, etc.): **deferred for v1.** i18n infrastructure ships with English only (§8.2); adding locales is additive (`messages/de.json` + switcher) once the group wants it.
- **Discord embed footer/timestamp:** implemented in v1 — embed model includes `footer` (interpolated text) and `show_timestamp` (Discord native ISO timestamp). Footer icon URL is not supported in v1.
