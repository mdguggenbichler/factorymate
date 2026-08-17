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
- Rich, well-formatted Discord notifications (embeds with color, fields, footer) — not limited to plain text.
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
| Backend framework | net/http + chi router (or Echo/Fiber — implementer's choice) | Lightweight REST API |
| Database | SQLite (`modernc.org/sqlite`, pure Go, no CGO) | Sufficient for single-server scale; simplifies deployment (no separate DB container) |
| Notification delivery | **Custom `Provider` interface**, no third-party notification library | Full control over Discord embed structure (title, description, color, fields, footer, timestamp, username/avatar override). Discord is the primary/only provider in v1; interface is designed so ntfy/Telegram/Slack providers can be added later without touching the dispatcher or templating system. |
| Templating | Go `text/template` (plain-text targets) and a small structured embed-template model (Discord targets) — see §5.4 | Two render paths: plain string, or structured embed object |
| Frontend framework | Next.js (App Router) | |
| UI components | shadcn/ui + Tailwind CSS | Copy-paste component model, minimal custom CSS |
| Charts | Recharts | Pairs natively with shadcn's chart components |
| Auth | Session-cookie based, password login (bcrypt hashes in SQLite) | No OAuth/SSO in v1 |
| Deployment | Docker, 1–2 containers (backend + frontend, or backend serving frontend static export) | Runs alongside the existing `satisfactory-server` container on the group's host |

### 2.2 Why not shoutrrr

`shoutrrr` (containrrr/shoutrrr) was evaluated and rejected for v1. It provides a unified plain-string API across ~18 notification services, which is valuable for broad multi-provider support, but its Discord service only exposes a single message body, a title, an accent color per severity class (`Color`/`ColorInfo`/`ColorWarn`/`ColorError`/`ColorDebug`), and username/avatar override — it does **not** expose structured embed fields. Achieving the rich, per-message-type embeds this project wants (fields like "Tech Tier" / "New Recipes" / "Options") would require bypassing shoutrrr's own formatting via its `JSON=true` passthrough mode and constructing the full Discord embed payload manually anyway — at which point the dependency adds indirection without adding value. Since Discord is confirmed as the primary and, for now, only target, a small first-party `DiscordProvider` implementing a generic `Provider` interface gives full embed control today and a clean seam for additional providers later.

---

## 3. Data Model (SQLite)

```sql
-- Users with dashboard access
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    created_at TEXT NOT NULL
);

-- Where notifications can be sent
CREATE TABLE notification_targets (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,                 -- display name, e.g. "Main Discord Channel"
    provider_type TEXT NOT NULL,        -- 'discord' (v1), extensible later
    config_json TEXT NOT NULL,          -- provider-specific config (webhook URL, username/avatar override, etc.)
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL
);

-- Fixed catalog of message types (seeded, not user-created in v1).
-- Startup seeding only INSERTs keys missing from the table (new message
-- types introduced by a later app version); it never overwrites `enabled`
-- on an existing row, so an admin's on/off choice survives upgrades.
CREATE TABLE message_types (
    key TEXT PRIMARY KEY,               -- e.g. 'player_joined'
    label TEXT NOT NULL,                -- human readable, e.g. "Player Joined"
    category TEXT NOT NULL,             -- 'player' | 'power' | 'progression' | 'vehicle' | 'server'
    enabled BOOLEAN NOT NULL DEFAULT 1, -- admin on/off switch, independent of target assignment (see §5.3)
    default_template_json TEXT NOT NULL,-- built-in default (see §5.4)
    variables_json TEXT NOT NULL        -- documented available template variables for this type
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

-- Audit log of sent notifications
CREATE TABLE notification_log (
    id INTEGER PRIMARY KEY,
    message_type_key TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    rendered_preview TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    sent_at TEXT NOT NULL
);

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
    last_seen_at TEXT
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
    type TEXT NOT NULL,                 -- Milestone | Hard Drive | Alternate | M.A.M. | ...
    purchased BOOLEAN NOT NULL,
    locked BOOLEAN NOT NULL,
    tech_tier INTEGER,
    updated_at TEXT NOT NULL
);

CREATE TABLE elevator_state (
    elevator_id TEXT PRIMARY KEY,
    upgrade_ready BOOLEAN NOT NULL,
    phase_number INTEGER,                      -- derived via static ClassName-set lookup, NULL if no match (see §4.2)
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
    vehicle_id TEXT PRIMARY KEY,        -- FRM Vehicle ID
    vehicle_type TEXT NOT NULL,         -- 'Explorer' | 'Tractor' | 'Truck' | 'FactoryCart'
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
    server_name TEXT NOT NULL DEFAULT 'Satisfactory Server',  -- free-text label, used as {ServerName} in templates
    frm_host TEXT NOT NULL,
    frm_port INTEGER NOT NULL,
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

**Slow poll** (every `production_snapshot_interval_seconds`, default 5 min):

| Endpoint | Purpose |
|---|---|
| `GET /getProdStats` | Produced/consumed rates per item — feeds both `prod_stats_state` (current snapshot, "Overall Prod" table) and `production_snapshots` (historical trend, appended each cycle) from the same call |
| `GET /getResourceSink` | A.W.E.S.O.M.E. Sink coupon/points status → `resource_sink_state` (current) and `resource_sink_snapshots` (history, for the `/resource-sink` chart). FRM's own response includes a rolling `GraphPoints` window, but that's a fixed, non-interval-selectable view with no real historical depth — this project builds its own proper time series here rather than mirroring that limitation, same as it already does for production data. |
| `GET /getFactory` | Every producer-type building (Assembler, Foundry, Smelter, Constructor, Manufacturer, Blender, Packager, Refinery, Converter, Encoder, Particle Accelerator) → `factory_machine_state`, "Detailed Prod" table. `getFactory` is a single aggregated endpoint covering all of these — not one call per building type. |
| `GET /getDrone` | All drones → `drone_state` |
| `GET /getDoggo` | All Lizard Doggos → `doggo_state`, storing the `Inventory[]` array as-is — each item already has a display `Name` (e.g. "SAM"), so no separate "found SAM" flag or ClassName matching is needed |

Each slow-poll cycle also writes to `circuit_snapshots` (§3) for the `/power` chart — this does **not** require an additional FRM call; `circuit_state` is already kept current by the fast poll's `getPower` call (see the Fast poll table above), so the slow-poll job simply copies its current values into a new `circuit_snapshots` row each cycle. This keeps power's historical chart resolution at the slower cadence (sensible for a trend view) without duplicating the fuse-detection polling.

**Retention:** A background job, running on the same cadence as the snapshot capture itself, deletes rows older than `production_snapshot_retention_days` (default 30) from all three history tables — `production_snapshots`, `resource_sink_snapshots`, and `circuit_snapshots` — immediately after each successful slow-poll cycle. One shared retention setting for all three, since they're captured together on the same schedule; no need for three separate config knobs. This keeps the tables bounded without a separate scheduler; if the poller is down for longer than the retention window, the next successful run prunes everything that aged out during the downtime in one pass. The remaining slow-poll tables (`resource_sink_state`, `prod_stats_state`, `factory_machine_state`, `drone_state`) are current-snapshot upserts, not history — nothing to prune there.

All requests target `http://{frm_host}:{frm_port}/{endpoint}`. A request timeout (5s) and connection failure are both treated as "unreachable" for the purposes of server-online/offline detection — this applies to the fast poll only; a slow-poll failure logs an error but does not affect `server_state`, since the fast poll already owns that determination and runs far more frequently.

### 4.2 Diff / Event Detection Logic

All detection is **edge-triggered** (fires only on state transitions, not on every poll where a condition remains true), mirroring the working logic from the prior n8n implementation. Detection and dispatch are decoupled: the poller always evaluates every transition below and updates state regardless of a message type's `enabled` flag (see §5.3) — a disabled type simply skips the render-and-send step. This matters because several of these transitions are edge-triggered against the *previous stored value* (e.g. `player_left` only fires if `player_joined` was previously observed as `true`); if detection paused while a type was disabled, re-enabling it later could misfire or miss the next transition.

| Message Type | Trigger Condition |
|---|---|
| `server_online` | Previous poll unreachable/offline → current poll reachable |
| `server_offline` | Previous poll reachable → current poll unreachable (timeout or connection error on any polled endpoint) |
| `player_joined` | Player's `Online` flag: `false`/unknown → `true` |
| `player_left` | Player's `Online` flag: `true` → `false` |
| `fuse_tripped` | Circuit's `FuseTriggered`: `false` → `true` |
| `power_restored` | Circuit's `FuseTriggered`: `true` → `false` |
| `milestone_unlocked` | Schematic with `Type == "Milestone"`: `Purchased` `false` → `true` |
| `hard_drive_ready` | Schematic with `Type == "Hard Drive"`: `Locked` `true` → `false` AND `Purchased == false` (i.e. newly available, awaiting recipe selection) |
| `elevator_phase_complete` | Space Elevator's `UpgradeReady`: `false` → `true` |
| `research_unlocked` | Research Node's `State`: transitions **to** `"Purchased"` from any other value (deliberately not keyed off a specific "from" state — see note below on why) |
| `train_derailed` | Train's `Derailed`: `false` → `true` |
| `vehicle_out_of_fuel` | Vehicle's total `Fuel[]` amount: `> 0` → `0` |
| `vehicle_stuck` | `vehicle_state.stuck`: `false` → `true` (itself a debounced, heuristic-derived value — see note below, not a raw API field) |

**`research_unlocked` state uncertainty:** `getResearchTrees` documents `State` as "Purchase/Hidden of the Research Node," and the confirmed example only shows `"Purchased"`. Given the response also includes `UnhiddenBy` (nodes that reveal this one), there may be an intermediate "visible but not yet purchased" state not named in the documentation. The trigger is therefore written to fire on *any* transition into `"Purchased"` rather than specifically `"Hidden" → "Purchased"`, so it stays correct regardless of how many intermediate states actually exist — this should be spot-checked against a live response the same way `getPower`'s fields were, before shipping.

**`vehicle_stuck` heuristic:** No FRM field directly reports "stuck" the way `Derailed` does for trains. This is approximated in two layers — a raw candidate condition and a debounced, edge-triggered derived state — so the notification fires exactly once per stuck episode, the same way `fuse_tripped` fires once per trip rather than on every poll while still tripped:

1. **Raw candidate**, evaluated every poll: (`AutoPilot == true` OR `FollowingPath == true`) AND `ForwardSpeed < 0.5`.
2. If the raw candidate is true and `vehicle_state.low_speed_since` is currently `NULL`, set it to this poll's timestamp (don't overwrite it on later polls — that would keep resetting the debounce window). If the raw candidate is false, immediately reset `low_speed_since` to `NULL` and `stuck` to `false`.
3. Once `low_speed_since` has been continuously set for 3 consecutive fast-poll cycles (~1 minute at the default 20s interval) — i.e. the debounce has elapsed — set `vehicle_state.stuck = true`. `vehicle_stuck` fires on **this** flipping `false → true`, exactly like `fuse_tripped`'s edge-trigger, so it fires once per episode and stays silent on every subsequent poll until the vehicle starts moving again (which resets `stuck` back to `false` via step 2, allowing a future episode to fire again).

This debounce avoids false positives from a vehicle merely stopped at a station or waiting at a junction. Because this is a heuristic rather than a confirmed game signal, false positives (e.g. a vehicle briefly paused for a legitimate reason lasting over a minute) are still possible and the threshold/debounce values may need tuning after real-world observation.

**Phase number derivation:** `getSpaceElevator` does not return an explicit phase index, but the *set of part types* required for a given phase is fixed game content data — it does not change between playthroughs or save files, and (absent gameplay-altering mods, which this deployment avoids per the SML/client-compatibility findings above) stays stable across sessions. Required *quantities* can differ (e.g. via the Space Elevator deliverable cost multiplier set at world creation), but the *types* of parts per phase do not, and each phase's set of part types is unique — no two phases share the same combination — so matching on type alone (ignoring `Amount`/`RemainingCost`/`TotalCost`) is sufficient to identify the phase.

**Source-level confirmation (via direct inspection of the FRM repository, `porisius/FicsitRemoteMonitoring`):** `getSpaceElevator`'s `CurrentPhase` field is a direct pass-through of the base game's own `AFGGamePhaseManager::GetRemainingPhaseCosts()` — FRM defines no phase-to-part mapping itself, and no phase index is serialized anywhere in its source or documentation. Each item's `Name` is populated from `UFGItemDescriptor::GetItemName()` (the item's *current, localized display name* — confirmed subject to change, e.g. the historical Automated Wiring/"High-speed Wiring" rename), while `ClassName` is populated from `GetClassDisplayName()` (the stable Unreal class identifier). This is direct source-level confirmation that `ClassName`, not `Name`, is the correct field to key the lookup table on.

**Live verification, Phase 2 (confirmed 2026-08-16):** A real `getSpaceElevator` call against the group's own server, taken while on Phase 2, returned exactly 3 items in `CurrentPhase` — `Desc_SpaceElevatorPart_1_C` (Smart Plating), `Desc_SpaceElevatorPart_2_C` (Versatile Framework), `Desc_SpaceElevatorPart_3_C` (Automated Wiring) — matching Phase 2's expected part count and ClassNames exactly. This confirms both open questions for this phase: **the "current phase only" scoping assumption holds** (no cumulative aggregation across phases), and **`_1_C`/`_2_C`/`_3_C` are correct against live data**, not just the wiki. Phases 3–5 remain wiki-sourced only, to be confirmed the same way as the group naturally reaches them (see self-correcting diagnostic logging below).

Also observed in the live response but not previously modeled: each `CurrentPhase[]` item includes `Amount` (current stock/delivered amount at the elevator) and `MaxAmount` (per-delivery cap, e.g. 50) in addition to `RemainingCost`/`TotalCost`. These aren't needed for phase-number derivation but are available if a future dashboard view wants to show delivered-vs-needed progress per item within the current phase.

Reference data (source: [official Satisfactory Wiki, Space Elevator](https://satisfactory.wiki.gg/wiki/Space_Elevator) and per-item wiki pages, default 1× cost multiplier). Every ClassName below is directly confirmed from the official Satisfactory Wiki (each Project Part's own page states its ClassName) — none are guessed from a naming pattern. Note that the numbering does **not** follow phase-appearance order (e.g. Magnetic Field Generator is `_6_C` and Assembly Director System is `_7_C`, even though Assembly Director System is listed first within Phase 4) — it reflects internal implementation order, so this table should not be extrapolated from pattern alone if the game adds more parts in the future:

| Phase | Name | Required Part Types | ClassName | Verified |
|---|---|---|---|---|
| 1 | Distribution Platform | Smart Plating | `Desc_SpaceElevatorPart_1_C` | ✅ Live (2026-08-16, via Phase 2 response) |
| 2 | Construction Dock | Smart Plating, Versatile Framework, Automated Wiring | `Desc_SpaceElevatorPart_1_C`, `Desc_SpaceElevatorPart_2_C`, `Desc_SpaceElevatorPart_3_C` | ✅ Live (2026-08-16) |
| 3 | Main Body | Versatile Framework, Modular Engine, Adaptive Control Unit | `Desc_SpaceElevatorPart_2_C`, `Desc_SpaceElevatorPart_4_C`, `Desc_SpaceElevatorPart_5_C` | Wiki only |
| 4 | Propulsion | Assembly Director System, Magnetic Field Generator, Thermal Propulsion Rocket, Nuclear Pasta | `Desc_SpaceElevatorPart_7_C`, `Desc_SpaceElevatorPart_6_C`, `Desc_SpaceElevatorPart_8_C`, `Desc_SpaceElevatorPart_9_C` | Wiki only |
| 5 | Assembly | Nuclear Pasta, Biochemical Sculptor, AI Expansion Server, Ballistic Warp Drive | `Desc_SpaceElevatorPart_9_C`, `Desc_SpaceElevatorPart_10_C`, `Desc_SpaceElevatorPart_12_C`, `Desc_SpaceElevatorPart_11_C` | Wiki only |

All twelve ClassNames (`_1_C` through `_12_C`) are individually confirmed from official wiki pages; Phases 1–2 are additionally confirmed against the group's own live server (see above). Phases 3–5 remain wiki-sourced only.

FactoryMate ships this table as a maintained data file (not hardcoded inline), matches the live, sorted `CurrentPhase[].ClassName` set against it on every poll, and sets `phase_number` accordingly. **If no match is found** (e.g. a future Satisfactory content update reshuffles phase part lists, the deliverable cost multiplier was set to something other than 1× at world creation — which changes quantities but not types, so this alone should not break matching — or a modded part is present), the backend does not guess: `phase_number` is left `null`/unknown, and any template referencing `{PhaseNumber}` falls back to omitting it (e.g. rendering "the next phase" instead of "Phase 4") rather than showing a wrong number.

**Self-correcting verification, not exhaustive upfront verification:** Fully confirming this table against live API data for every phase would require playing through the entire game (Phase 5 alone requires tens of thousands of several parts), which is impractical purely for spec verification. Instead, whenever a poll's `CurrentPhase` set doesn't match any table entry, the raw unmatched set (item names + ClassNames as returned by FRM) is written to `elevator_phase_unknown_log` (see §3) — a dedicated diagnostic table, not repurposed notification-send history — rather than being silently discarded. Entries are surfaced as an admin-visible alert on the `/elevator` page (see §8) and can be marked resolved once the reference table has been corrected. This means that the first time the group naturally reaches a phase whose entry turns out to be wrong or was never live-verified, the discrepancy is surfaced immediately with the real data needed to correct the table — verification happens incidentally during normal play rather than requiring dedicated upfront effort.

On unreachable state, only the `server_offline` transition is evaluated; player/power/schematic/elevator/research/train/vehicle state (i.e. every fast-poll table) is left untouched (not reset), so that when the server comes back, transitions are computed against the last known-good state rather than an empty one.

### 4.3 Known FRM Limitations (carried over from investigation)

- `getSpaceElevator` provides `UpgradeReady` (boolean) but no explicit phase index/number in its own response. FactoryMate derives it separately via a static ClassName-set lookup table (see §4.2) — Phases 1–2 confirmed against the group's own live server, Phases 3–5 wiki-sourced pending live confirmation. If the current phase's part set doesn't match any table entry, `phase_number` is left unknown rather than guessed, and the `elevator_phase_complete` notification's `{PhaseNumber}` variable is simply omitted for that occurrence.
- FRM's own built-in webhook notification system (JSON template files, configured via the in-game Server Manager UI) was evaluated and found unreliable in testing (a HUB milestone unlock incorrectly fired the Hard Drive notification template). FactoryMate's own polling-and-diffing approach is used instead, giving full control over trigger logic.
- FRM's web server does not autostart by default; `Web_Autostart` must be enabled via the in-game **Server Manager → Server Settings** UI (this is a client-in-game setting, not a config file, as of the `FGUserSettings`-based config system introduced with Satisfactory 1.2 — legacy `.cfg` files under `FactoryGame/Configs/FicsitRemoteMonitoring/` are no longer read).
- Any Satisfactory mod installed server-side (including FRM, which requires SML) causes SML's client/server mod-list compatibility check to reject vanilla (unmodded) clients. At least one additional mod that clients are expected to install anyway (e.g. a QoL mod) resolves this, since it requires clients to have SML installed regardless of FRM.

---

## 5. Notification System

### 5.1 Notification Targets

A **Notification Target** is a named destination with a provider type and provider-specific configuration, configured once centrally and reused across message types.

**v1 provider: Discord**

```json
{
  "provider_type": "discord",
  "config": {
    "webhook_url": "https://discord.com/api/webhooks/{id}/{token}",
    "username_override": "F.I.C.S.I.T. Oracle",
    "avatar_url_override": "https://.../avatar.png"
  }
}
```

Targets can be created, edited, disabled (without deleting, to preserve message-type assignments), deleted, and **test-sent** (sends a sample notification using that target's config and a placeholder message, without needing a real trigger to fire). Deleting a target cascades to remove its `message_type_targets` assignments (`ON DELETE CASCADE`, see §3) — message types themselves and their templates are unaffected, they simply lose that one destination. The UI surfaces a confirmation showing how many message types are currently assigned to a target before deletion.

### 5.2 Message Type Catalog

Fixed, seeded set of message types (not user-creatable in v1 — new event types require a code change, but their templates and target assignments are fully user-configurable).

| Key | Label | Category | Available Template Variables |
|---|---|---|---|
| `server_online` | Server Online | server | `{ServerName}` |
| `server_offline` | Server Offline | server | `{ServerName}` |
| `player_joined` | Player Joined | player | `{PlayerName}`, `{OnlineCount}` |
| `player_left` | Player Left | player | `{PlayerName}`, `{OnlineCount}` |
| `fuse_tripped` | Fuse Tripped | power | `{CircuitID}` |
| `power_restored` | Power Restored | power | `{CircuitID}` |
| `milestone_unlocked` | Milestone Unlocked | progression | `{SchematicName}`, `{TechTier}`, `{RecipeNames}` (comma-joined) |
| `hard_drive_ready` | Hard Drive Ready | progression | `{SchematicName}`, `{RecipeOptions}` (newline-joined list) |
| `elevator_phase_complete` | Elevator Phase Complete | progression | `{ElevatorName}`, `{PhaseNumber}` (omitted from rendering if unknown — see §4.2) |
| `research_unlocked` | Research Unlocked | progression | `{NodeName}`, `{TreeName}`, `{TechTier}` |
| `train_derailed` | Train Derailed | vehicle | `{TrainName}`, `{StationName}` |
| `vehicle_out_of_fuel` | Vehicle Out of Fuel | vehicle | `{VehicleType}`, `{VehicleName}` |
| `vehicle_stuck` | Vehicle Stuck | vehicle | `{VehicleType}`, `{VehicleName}` |

Each row's `variables_json` in the DB schema is the machine-readable source of truth the dashboard's template editor uses to show "insert variable" affordances (e.g. autocomplete chips) — this table is the human-readable equivalent.

### 5.3 Enabling & Assignment: Message Type → Targets

Each message type has its own `enabled` flag (default: on), independent of which targets it's assigned to. This is the mechanism for "only notify on join/leave": leave `player_joined`/`player_left` enabled with the Discord target assigned, and either disable the other message types outright or simply leave them unassigned — disabling is the clearer choice when the intent is "we don't want this at all right now, but keep the target assignment for later," whereas leaving a type enabled with zero targets just means "nothing to send it to yet."

Target assignment itself is many-to-many: an enabled message type can be sent to zero, one, or multiple targets (e.g. everything to the main status channel, but `fuse_tripped` additionally to a personal ntfy target once that provider exists). A disabled message type is never dispatched regardless of its target assignments — the poller still detects the underlying state transition (so player online/offline state stays correct either way), it simply skips rendering and sending for that type.

Both the enable toggle and target checkboxes live on the **Notification Templates** dashboard page (`/settings/notifications/templates`, see §8) alongside the template editor for that type, so operators see "is it on," "what it looks like," and "where it goes" together.

### 5.4 Templating System

Two render paths exist, selected automatically by the target's `provider_type` at send time:

**A. Plain-text render** (used by any future provider that only accepts a message string, e.g. ntfy, Telegram)
A `text/template`-compatible string, e.g.:
```
🟢 **{PlayerName}** has entered the factory. ({OnlineCount} online)
```

**B. Structured embed render** (used when `provider_type == "discord"`)
A structured object mirroring a Discord embed, each field independently templated:
```json
{
  "title": "🟢 NEW PLAYER DETECTED",
  "description": "**{PlayerName}** has entered the factory.",
  "color": "#57F287",
  "fields": [
    { "name": "Players online", "value": "{OnlineCount}", "inline": true }
  ]
}
```
`title`, `description`, each field's `name`/`value`, and `color` are all independently editable text with variable interpolation. `color` accepts a hex string; a small preset palette (green/red/orange/gold/purple/blue, matching the categories above) is offered in the UI alongside a free-form color picker.

**Defaults:** Every message type ships with a built-in default template stored in `message_types.default_template_json`, a JSON object with independent `plain` and `embed` keys (e.g. `{ "plain": "...", "embed": { "title": ..., "description": ..., "color": ..., "fields": [...] } }`), seeded at first startup and never mutated. `message_templates.template_json` follows the same two-key shape but holds only operator overrides, and the two variants are overridden independently: an admin can customize just the `embed` variant (the one actually used for the group's Discord targets) while leaving `plain` unset, in which case rendering for any future plain-text-only provider falls back to that variant's entry in `default_template_json`. `message_templates` as a whole is absent (no row) if no override exists for either variant. A **"Reset to default"** action is available per variant (reset just the embed template, or just the plain-text template) as well as for the whole message type.

**Validation:** On save, the backend renders the template against a set of placeholder sample values for that message type and rejects the save (with an inline error) if rendering fails (unknown variable reference, invalid template syntax) or if the rendered Discord embed exceeds Discord's limits (title ≤256 chars, description ≤4096 chars, ≤25 fields, field name ≤256 / value ≤1024 chars).

**Live preview:** The template editor renders a live preview using sample data as the operator types, and — for Discord targets — the preview visually approximates a Discord embed card (using shadcn `Card` styled to resemble Discord's embed rendering: colored left border, title, description, fields grid).

---

## 6. Authentication & Authorization

- Username + password login, session cookie (HTTP-only, secure, SameSite=Lax), backed by a server-side session store (in SQLite or in-memory with periodic cleanup — implementer's choice).
- Passwords hashed with bcrypt.
- Two roles:
  - **admin** — full access: settings, notification targets, message templates, target assignment, user management.
  - **viewer** — read-only access to all dashboard pages (status, players, production, power, resource sink, drones, doggos, milestones, research, vehicles, elevator); no access to settings/templates/targets/users pages (these routes 403 for viewers, and are hidden from navigation).
- First-run: if the `users` table is empty, the app serves a one-time setup page to create the first admin account instead of the login page.
- No self-service registration — admins create additional user accounts (viewer or admin) from the Users page.

---

## 7. REST API Reference

All endpoints under `/api`, JSON in/out, session-cookie authenticated unless noted.

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/auth/setup` | none (only when no users exist) | Create first admin account |
| POST | `/api/auth/login` | none | Login, sets session cookie |
| POST | `/api/auth/logout` | session | Clear session |
| GET | `/api/auth/me` | session | Current user + role |
| PUT | `/api/account/password` | session | Change the current user's own password (any role — this is distinct from `/api/users/:id`, which is admin-only and manages *other* users) |
| GET | `/api/status` | session | Current server online state, online player count, active fuse trips |
| GET | `/api/players` | session | Current player list with online status |
| GET | `/api/players/history` | session | Join/leave event log, paginated |
| GET | `/api/power` | session | Current circuit states, including production/consumption/battery detail |
| GET | `/api/power/history` | session | Fuse trip/restore event log, paginated (discrete events, not continuous metrics) |
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
| GET | `/api/elevator/unknown-log` | admin | Unresolved (and recent resolved) `elevator_phase_unknown_log` entries |
| POST | `/api/elevator/unknown-log/:id/resolve` | admin | Mark a diagnostic entry as resolved (after correcting the reference table) |
| GET | `/api/notification-targets` | admin | List targets |
| POST | `/api/notification-targets` | admin | Create target |
| PUT | `/api/notification-targets/:id` | admin | Update target |
| DELETE | `/api/notification-targets/:id` | admin | Delete target |
| POST | `/api/notification-targets/:id/test` | admin | Send a sample notification through this target |
| GET | `/api/message-types` | admin | List message types with `enabled` state, current template (override or default), and assigned target IDs |
| PUT | `/api/message-types/:key/enabled` | admin | Toggle a message type on/off |
| PUT | `/api/message-types/:key/template` | admin | Save template override |
| POST | `/api/message-types/:key/template/reset?variant=plain\|embed\|all` | admin | Delete override for the given variant (or both), revert to default |
| POST | `/api/message-types/:key/template/preview` | admin | Render given (unsaved) template against sample data, return rendered result for live preview |
| PUT | `/api/message-types/:key/targets` | admin | Replace target assignment set for this message type |
| GET | `/api/notification-log?type=&target=&limit=` | admin | Recent sent-notification audit log |
| GET | `/api/settings` | admin | App settings (FRM host/port, poll intervals, retention) |
| PUT | `/api/settings` | admin | Update app settings |
| GET | `/api/users` | admin | List users |
| POST | `/api/users` | admin | Create user |
| PUT | `/api/users/:id` | admin | Update user (role, password reset) |
| DELETE | `/api/users/:id` | admin | Delete user |

---

## 8. Page Inventory (Frontend)

| Route | Access | Purpose | Key Components |
|---|---|---|---|
| `/setup` | public, only if no users exist | First-run: create initial admin account | Form (username, password, confirm) |
| `/login` | public | Login | Form (username, password) |
| `/` (Dashboard Overview) | viewer, admin | At-a-glance status: server online/offline badge, online player avatars/list, active fuse-trip warnings, latest milestone unlocked, elevator phase progress bar | Status cards, badge, progress bar |
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
| `/settings/notifications/targets` | admin only | CRUD for Notification Targets, per-target "Send test" button | Data table, dialog forms |
| `/settings/notifications/templates` | admin only | List of message types; selecting one opens the template editor (plain-text + embed fields, variable picker, live preview, target assignment checkboxes for that type, reset-to-default) | Data table + detail panel, live preview card |
| `/settings/notifications/log` | admin only | Recent sent notifications with success/failure status | Data table |
| `/settings/general` | admin only | Server display name (used as `{ServerName}` in templates), FRM host/port, poll interval, production snapshot interval/retention | Form |
| `/settings/users` | admin only | User management (create/edit role/reset password/delete) | Data table, dialog forms |
| `/account` | viewer, admin | Change own password | Form |

Navigation: a persistent sidebar (shadcn `Sidebar` pattern) with the dashboard pages always visible to both roles, and a "Settings" section only rendered/routable for admins.

### 8.1 Component Mapping (shadcn/ui)

Concrete shadcn/ui components — and, where one exists, a full shadcn **block** (a pre-assembled, literally copy-paste page/section) — per route. `shadcn add <name>` installs each into the project as owned source, not a package dependency.

| Route | shadcn Block | shadcn Components | Custom composition needed |
|---|---|---|---|
| `/setup`, `/login` | `login-01` (centered card login form) as starting point | `Card`, `Form`, `Input`, `Label`, `Button` | None — block covers this directly |
| App shell (all pages) | `sidebar-07` (collapsible sidebar with grouped nav + user menu footer) | `Sidebar`, `Breadcrumb`, `Avatar`, `DropdownMenu`, `Separator` | Nav items/groups (every viewer-accessible route from §8's table, plus a Settings group admin-only) are your own data, block only provides the shell |
| `/` (Overview) | `dashboard-01` as loose layout reference (card grid) | `Card`, `Badge`, `Avatar` (online players), `Progress` (elevator bar), `Alert` (admin diagnostic banner) | The status-card content itself (server online/offline logic, latest milestone) — block gives layout, not your data bindings |
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
| `/settings/notifications/templates` | — | `Table`/list (left pane, with `Switch` per row for `enabled`), `Tabs` (Plain / Embed sub-editor), `Textarea` (plain template, embed description), `Input` (embed title), `Command`+`Popover` (variable-insert picker), `Checkbox`/`Toggle Group` (target assignment), `Card`+`Separator` (embed live-preview) | **Three real gaps here, no stock component:** (1) a repeatable "Fields" array editor for embed fields (build with `react-hook-form`'s `useFieldArray` + `Input` pairs + an "Add field" `Button`); (2) a hex color picker (shadcn has none — pair a `Popover` with a small custom swatch grid, or a plain `Input type="color"`); (3) the Discord-embed-style preview card itself (colored left border, title/description/fields layout) is a custom composition of `Card`+`Separator`, not a stock look |
| `/settings/notifications/log` | — | `Table`, `Badge` (success/fail) | None |
| `/settings/general` | — | `Form`, `Input`, `Label` | None |
| `/settings/users` | — | `Table`, `Dialog`, `AlertDialog`, `Select` (role) | None |
| `/account` | — | `Card`, `Form`, `Input`, `Button` | None |
| Global | — | `Sonner` (toast: save confirmations, test-send results, errors) | None |

**Net effect:** most of the app — auth, shell/navigation, tables, dialogs, forms, charts, badges — is genuinely copy-paste from shadcn's registry with no custom styling work. The one page that needs real hand-built UI is the notification template editor (`/settings/notifications/templates`), specifically the repeatable embed-fields array, the color picker, and the Discord-style preview card — all straightforward compositions of existing primitives, just not single-command installs.

---

## 9. Configuration / Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Backend HTTP port |
| `DATABASE_PATH` | `/data/factorymate.db` | SQLite file path (mounted volume) |
| `SESSION_SECRET` | — (required) | Cookie signing secret |
| `FRM_HOST` | — (required, also editable via `/settings/general`) | Initial value; runtime value lives in `app_settings` table once set via UI |
| `FRM_PORT` | `8080` | Initial value; same as above |

Notification target credentials (Discord webhook URLs) are **not** environment variables — they live in the `notification_targets` table, configured via the UI, so they can be managed without redeploying the container.

---

## 10. Deferred Decisions & Defaults

Everything below was genuinely open earlier in this project's design discussion and has since been resolved with a deliberate default, not left dangling — each can be revisited if the group's actual usage later calls for it, but v1 ships without them.

- **`hard_drive_ready` follow-up notification on recipe selection** (`Purchased` transitioning to `true` after the fact): **decided against for v1.** The moment worth notifying on is "a choice is now available" — the actual selection is a single-person, low-stakes action with little group-relevant signal. `schematic_state` already captures `purchased`/`locked` either way, so adding this later is a small, additive change to §4.2's diff table, not a schema migration.
- **Additional notification providers** (ntfy, Telegram, generic webhook): **decided against for v1.** Discord is the group's actual, confirmed destination; building providers with no real target to point them at is speculative work. The `Provider` interface (§2.2) exists specifically so this is a contained addition later — a new struct implementing `Send`/`Type`, no changes to the poller, templating, or dispatch layers.
- **Per-message-type polling cadence** (e.g. faster polling for player join/leave than for schematics): **decided against for v1.** A single shared `poll_interval_seconds` (default 20s, §4.1) is well under any latency the group would notice for a 5–6 player casual server, and per-type scheduling would meaningfully complicate M3's poll loop for no observed benefit.
- **Confirming Phases 3–5's Space Elevator ClassName mapping against live data**: not a decision to make, just not yet possible — the group hasn't reached those phases. The self-correcting `elevator_phase_unknown_log` mechanism (§4.2) closes this automatically as the save progresses; no action needed until an entry actually appears there.
