-- Users with dashboard access
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    created_at TEXT NOT NULL
);

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
-- Startup seeding only INSERTs keys missing from the table (new message
-- types introduced by a later app version); it never overwrites `enabled`
-- on an existing row, so an admin's on/off choice survives upgrades.
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

-- Audit log of sent notifications (dispatch outcomes only — NOT a substitute for
-- player_session_events or power_circuit_events; disabled message types produce
-- no rows here even when the underlying game event occurred)
CREATE TABLE notification_log (
    id INTEGER PRIMARY KEY,
    message_type_key TEXT NOT NULL,
    target_id INTEGER NOT NULL,         -- intentionally no FK / ON DELETE CASCADE: audit history must survive target deletion; this value may reference a deleted notification_targets row (UI: §8.1; GET /api/notification-log LEFT JOIN: §7)
    rendered_preview TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    sent_at TEXT NOT NULL
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
    server_name TEXT NOT NULL DEFAULT 'Satisfactory Server',  -- free-text label, used as {ServerName} in templates
    frm_host TEXT NOT NULL DEFAULT '',
    frm_port INTEGER NOT NULL DEFAULT 8080,
    frm_auth_token TEXT,                -- optional; sent as X-FRM-Authorization when set (see §4.1)
    poll_interval_seconds INTEGER NOT NULL DEFAULT 20,
    production_snapshot_interval_seconds INTEGER NOT NULL DEFAULT 300,
    production_snapshot_retention_days INTEGER NOT NULL DEFAULT 30
);
