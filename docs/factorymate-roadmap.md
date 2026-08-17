# FactoryMate — Build Roadmap

**Companion document to:** `factorymate-spec.md` (the source of truth for schemas, API contracts, component choices, and every design decision — this document sequences the *work*, it does not redefine anything already specified there)

**How to use this document:** Milestones are ordered by dependency, not by page/feature grouping — each one only requires what was built in a prior milestone. Work through them in order. Each milestone has a **Definition of Done (DoD)** — do not start the next milestone until the current one's DoD is met. Every task that touches a schema, endpoint, or UI choice links to the exact spec section (`§X`) that defines it in full; this roadmap intentionally does not repeat those details, so cross-check the referenced section before implementing, don't guess from the task description alone.

---

## M0 — Project Scaffolding

**Goal:** An empty but runnable skeleton for both backend and frontend, in one repository.

- [ ] Create repo with `/backend` (Go module) and `/frontend` (Next.js App Router) directories.
- [ ] `backend`: `go mod init`, add dependencies: `modernc.org/sqlite` (§3), a router (`chi` per §2.1), `golang.org/x/crypto/bcrypt` (§6).
- [ ] `backend`: directory layout —
  ```
  backend/
    cmd/server/main.go
    internal/db/          -- migrations + queries (M1)
    internal/frm/         -- FRM HTTP client (M2)
    internal/poller/      -- diff engine (M3)
    internal/notify/      -- Provider interface + DiscordProvider (M4)
    internal/template/    -- templating engine (M5)
    internal/api/         -- REST handlers (M8)
    internal/auth/        -- sessions, bcrypt (M7)
    data/                 -- static reference data, e.g. elevator phase table (M3.1)
  ```
- [ ] `frontend`: `npx create-next-app` (App Router, TypeScript, Tailwind), then `npx shadcn init`.
- [ ] `docker-compose.yml` skeleton with two empty service stubs (`backend`, `frontend`) — filled in at M12.
- [ ] Both processes start and respond to a trivial health check (`GET /healthz` on backend; default Next.js page on frontend).

**DoD:** `docker-compose up` (or `go run` + `npm run dev` locally) starts both processes cleanly with no business logic yet.

---

## M1 — Database Layer

**Goal:** Every table from spec §3 exists, migratable, with a seed step for the fixed message-type catalog.

- [ ] Implement a migration runner (a minimal hand-rolled one is fine — e.g. numbered `.sql` files applied in order and tracked in a `schema_migrations` table; no need for a heavyweight migration framework at this scale).
- [ ] Migration 001: create every table exactly as defined in **spec §3** — `users`, `notification_targets`, `message_types`, `message_templates`, `message_type_targets`, `notification_log`, `elevator_phase_unknown_log`, `server_state`, `player_state`, `circuit_state` (now with full power/battery columns, not just `tripped`), `schematic_state`, `elevator_state`, `train_state`, `vehicle_state`, `research_node_state`, `production_snapshots` (+ its index), `resource_sink_snapshots` (+ its index), `circuit_snapshots` (+ its index), `app_settings`, and the five slow-poll dashboard tables: `resource_sink_state`, `prod_stats_state`, `factory_machine_state`, `drone_state`, `doggo_state`. Copy the SQL verbatim from spec §3, do not paraphrase it — every column comment in that SQL documents a decision made elsewhere in this conversation.
- [ ] Seed step (idempotent, run on every startup): insert the 13 rows into `message_types` per **spec §5.2**'s table (`server_online`, `server_offline`, `player_joined`, `player_left`, `fuse_tripped`, `power_restored`, `milestone_unlocked`, `hard_drive_ready`, `elevator_phase_complete`, `research_unlocked`, `train_derailed`, `vehicle_out_of_fuel`, `vehicle_stuck`), each with its `default_template_json` (build this now per **spec §5.4**'s two-key `{plain, embed}` shape — the example embed JSON in §5.4 is the `player_joined` default, use it verbatim and author sensible equivalents for the other 12 types) and `variables_json` (array of variable names from the §5.2 table). **Critical:** the seed must only `INSERT OR IGNORE` — never overwrite an existing row's `enabled` column, per the comment in spec §3's `message_types` definition.
- [ ] Seed `app_settings` with `id=1` if no row exists yet — `server_name` uses the schema's own `DEFAULT 'Satisfactory Server'` (§3), `frm_host`/`frm_port` come from env vars at first boot (see §9). The group renames it to their actual server name via `/settings/general` post-setup — don't hardcode this group's specific server name into the seed step, since the project (FactoryMate) is deliberately named/designed to be reusable by other groups too, not just this one.
- [ ] Unit test: run migrations + seed twice in a row against a fresh DB file; second run must be a no-op that doesn't error or duplicate rows.

**DoD:** A fresh SQLite file, after running the app once, contains all tables and exactly 13 seeded `message_types` rows with valid default templates.

---

## M2 — FRM Client

**Goal:** A Go package that calls the FRM endpoints from **spec §4.1** (fast-poll and slow-poll sets) and returns typed structs.

- [ ] Define Go structs for each fast-poll endpoint's response shape:
  - `getPlayer` → `[]Player{ID, Name, Online bool, ...}`
  - `getPower` → `[]Circuit{CircuitGroupID int, FuseTriggered bool, PowerProduction, PowerConsumed, PowerCapacity, BatteryDifferential, BatteryPercent, BatteryCapacity float64, BatteryTimeEmpty, BatteryTimeFull string, ...}` — the battery/power fields are needed now for `/power`'s expanded dashboard, not just `FuseTriggered`.
  - `getSchematics` → `[]Schematic{ID, Name, Type, Purchased, Locked bool, TechTier int, Recipes []Recipe{Name, ClassName}}`
  - `getSpaceElevator` → `[]Elevator{ID, Name string, CurrentPhase []PhaseItem{Name, ClassName string, Amount, MaxAmount, RemainingCost, TotalCost int}, UpgradeReady bool}`
  - `getResearchTrees` → `[]ResearchTree{Name string, Nodes []ResearchNode{ID, Name, ClassName, Description, Category, State string, TechTier, TimeToComplete int, Cost []Item{Name, ClassName string, Amount, MaxAmount int}}}` — confirmed via the full response schema (including `Coordinates`/`Parents`/`UnhiddenBy`, not needed for v1's flat `/research` view but present in the raw response if a future tree-graph UI wants them).
  - `getTrains` → `[]Train{ID, Name string, Derailed, PendingDerail bool, Status, TrainStation, SelfDriving, Docking, Path string}`
  - `getVehicles` → `[]Vehicle{ID string, VehicleType, Status, Driver string, AutoPilot, FollowingPath bool, ForwardSpeed float64, Fuel []Item{Name, ClassName string, Amount int}}`
- [ ] Define Go structs for each slow-poll endpoint's response shape — **all five confirmed via this project's own documentation lookup, safe to implement directly, no live-guessing needed the way `getPower` initially went wrong**:
  - `getProdStats` → `[]ProdStat{Name, ClassName, ProdPerMin, Type string, ProdPercent, ConsPercent, CurrentProd, MaxProd, CurrentConsumed, MaxConsumed float64}`
  - `getResourceSink` → `[]ResourceSink{Name string, NumCoupon int, Percent float64, GraphPoints []GraphPoint{Index, Value int}, PointsToCoupon, TotalPoints int}`
  - `getFactory` → `[]FactoryMachine{ID, Name, ClassName, Recipe, RecipeClassName string, ManuSpeed float64, IsConfigured, IsProducing, IsPaused bool, PowerInfo struct{CircuitGroupID int, PowerConsumed, MaxPowerConsumed float64}, ...}` — the full response also includes per-machine `production`/`ingredients`/inventory arrays; only map the fields `factory_machine_state` (§3) actually stores, the rest can be ignored rather than fully modeled.
  - `getDrone` → `[]Drone{ID, Name, ClassName, HomeStation, PairedStation string, HasPairedStation bool, CurrentDestination, CurrentFlyingMode string, FlyingSpeed, MaxSpeed float64}`
  - `getDoggo` → `[]Doggo{ID, Name, ClassName string, Inventory []Item{Name, ClassName string, Amount, MaxAmount int}}` — store `Inventory` as-is in `doggo_state.inventory_json`; each item already carries its own display `Name`, so there's nothing to derive or match against.
- [ ] HTTP client with: 5s timeout, no retry-on-failure (a failed call is a valid signal — see M3's server-offline detection), config sourced from `app_settings` (host/port), not hardcoded.
- [ ] A `Client.GetFast(ctx) (*FastSnapshot, error)` method for the 7 fast-poll endpoints, and a separate `Client.GetSlow(ctx) (*SlowSnapshot, error)` for the 5 slow-poll endpoints (§4.1) — keep them as two distinct methods since M3 and M9 call them on different schedules. `GetFast` should return a partial result + error indicating which calls failed (needed for the "unreachable" detection in M3 — a single connection failure should be enough to mark the server offline, not require all 7 to fail); `GetSlow` failures just log, per §4.1's note that slow-poll failures don't affect `server_state`.
- [ ] Manual integration test against the group's real server (`192.168.178.42:8889` per this conversation, though this should come from `app_settings`, not be hardcoded even in tests) confirming real responses parse into the structs without error, for all 12 endpoints.

**DoD:** Running the client against the live FRM server returns populated, correctly-typed structs for all 12 endpoints (7 fast-poll, 5 slow-poll) with no parse errors.

---

## M3 — Poller / Diff Engine

**Goal:** Port the working edge-triggered detection logic (validated extensively in this project's n8n prototype) into Go, against the real DB state tables from M1.

- [ ] Implement the poll loop: ticks every `app_settings.poll_interval_seconds` (default 20s per §4.1), calls `frm.Client.GetFast`.
- [ ] Reachability: if `GetFast` indicates any endpoint failure, treat as unreachable for this poll — see **spec §4.2**'s `server_online`/`server_offline` trigger conditions and the "state left untouched on unreachable" rule at the end of §4.2.
- [ ] For each of `player_state`, `circuit_state`, `schematic_state`, `elevator_state`, `research_node_state`, `train_state`, `vehicle_state`: implement the exact edge-triggered comparisons from the **spec §4.2 table** (previous DB row vs. new FRM value), emitting a list of `(message_type_key, variables map[string]string)` events for each transition, then writing the new state back to the DB row (upsert).
- [ ] **`vehicle_stuck` needs its own sub-step, not a simple one-line comparison** — see spec §4.2's heuristic note, and note the `vehicle_state.stuck` column is the actual thing to edge-trigger on, not `low_speed_since` directly. Per vehicle, per poll: compute the raw candidate `(AutoPilot || FollowingPath) && ForwardSpeed < 0.5`. If true and `low_speed_since` is currently `NULL`, set it to this poll's timestamp (don't overwrite on later polls). If false, immediately reset both `low_speed_since` and `stuck` to their zero values. Once `low_speed_since` has been continuously set for ≥3 consecutive polls, set `stuck = true` — the emitted event is the `stuck` column's `false → true` transition specifically, handled by the same generic edge-trigger mechanism as every other message type (§4.2), so it naturally fires exactly once per stuck episode without any extra bookkeeping.
- [ ] **Critical ordering point from spec §4.2's opening paragraph:** detection must run and update state on every poll regardless of a message type's `enabled` flag — do not let M6's dispatch-layer `enabled` check leak backward into skipping detection. Structure the poller so it always produces the full event list; M6 filters it at send time, not here.

### M3.1 — Elevator Phase Lookup

- [ ] Create `backend/data/elevator_phases.json` (or `.go` data file) containing the **spec §4.2 reference table** verbatim — all 5 phases, their required `ClassName` sets, and the verification status column (Phase 1–2 live-confirmed, 3–5 wiki-sourced) as a code comment, not a runtime field (verification status doesn't affect matching logic, it's documentation for whoever edits this file next).
- [ ] Matching logic: sort the current poll's `CurrentPhase[].ClassName` values, compare as a set (not amount-sensitive) against each table row's ClassName set. On match, set `elevator_state.phase_number`. On no match, set it `NULL` and insert a row into `elevator_phase_unknown_log` with the raw `CurrentPhase` JSON — see **spec §4.2**'s "Self-correcting verification" paragraph and the table definition in §3.
- [ ] Unit test with the real captured payload from this conversation (Phase 2: `Desc_SpaceElevatorPart_1_C`, `_2_C`, `_3_C`) — must resolve to `phase_number = 2`.
- [ ] Unit test with a deliberately unmatched set — must produce `phase_number = NULL` and a new `elevator_phase_unknown_log` row.

**DoD:** Running the poller against the live server for a few cycles produces correct, non-duplicated state in all 7 fast-poll state tables, and the elevator phase resolves to `2` against the group's actual current save.

---

## M4 — Notification Provider Interface + Discord

**Goal:** The `Provider` abstraction from **spec §2.2**, with a working Discord implementation.

- [ ] Define `internal/notify.Provider` interface: `Send(ctx, target NotificationTarget, msg RenderedMessage) error` and `Type() string` — see spec §2.2's proposed shape.
- [ ] `RenderedMessage` type must support both plain-text and structured-embed shapes (output of M5's templating), since the Discord provider needs the embed object, not a flattened string.
- [ ] `DiscordProvider`: POSTs to the target's `webhook_url` (from `notification_targets.config_json`, shape per **spec §5.1**) with a JSON body containing `embeds: [...]` and, if set, `username`/`avatar_url` overrides from the target config.
- [ ] Respect Discord's payload limits from **spec §5.4**'s Validation paragraph at the provider level too (defense in depth — M5 should already reject invalid templates at save time, but the provider should not silently truncate or crash on an oversized payload either; log and mark the send failed in `notification_log` instead).
- [ ] Manual test: send a real message to a real Discord webhook (reuse a target the group already has from the n8n phase of this project, or create a fresh test one) and confirm it renders with title/description/color/fields/footer as expected.

**DoD:** A hardcoded sample embed reaches Discord correctly formatted via the `DiscordProvider`.

---

## M5 — Templating Engine

**Goal:** Render `message_templates`/`message_types.default_template_json` (§5.4) against event variables (§5.2) into a `RenderedMessage`.

- [ ] Implement variable substitution for both the plain-text (`text/template`) and structured-embed (per-field templating) render paths described in **spec §5.4**.
- [ ] Lookup order: `message_templates` row for this key/variant if present, else `message_types.default_template_json` for that variant — per the "Defaults" paragraph in §5.4.
- [ ] Validation function (usable both at save-time via API and at render-time): rejects unknown variables, template syntax errors, and Discord limit violations (title/description/field length, field count) — exact limits are in §5.4's Validation paragraph.
- [ ] Unit tests: render each of the 13 message types' default templates against sample variable sets (use the values already exercised in this project's n8n prototype — e.g. a player name, a circuit ID, a tech tier, a recipe list — plus new ones for the vehicle/research types: a train name, a vehicle type, a research node name) and confirm valid output for both variants.

**DoD:** Every default template for every message type renders without validation errors against representative sample data, for both plain and embed variants.

---

## M6 — Dispatch Wiring

**Goal:** Connect M3's event list → M5's renderer → M4's provider, respecting `enabled` and target assignment.

- [ ] For each event emitted by the poller: skip if `message_types.enabled = 0` for that key (§5.3). Otherwise, look up assigned targets via `message_type_targets`, render the message once per target's provider type (a mixed set of Discord + future-provider targets would need both variants rendered — not a concern yet with Discord-only, but don't hardcode "always render embed only").
- [ ] Dispatch to each assigned, enabled (`notification_targets.enabled = 1`) target via its provider; record the outcome (success/error + rendered preview) in `notification_log` per §3's schema.
- [ ] Wire this into the M3 poll loop as the final step of each cycle.
- [ ] End-to-end manual test: with the real FRM server running and a real Discord target configured, trigger an actual player join/leave in-game and confirm the message arrives in Discord with correct content — this is the same validation already done manually during the n8n phase of this project (worth repeating here to confirm the Go port behaves identically).

**DoD:** A real, in-game-triggered event (e.g. joining the server) results in a correctly formatted Discord message within one poll interval, and a corresponding row in `notification_log`.

---

## M7 — Auth

**Goal:** Session-based auth per **spec §6**.

- [ ] `POST /api/auth/setup`: only reachable while `users` table is empty; creates the first `admin` account (bcrypt-hashed password).
- [ ] `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`: session cookie handling per §6 (HTTP-only, secure, SameSite=Lax).
- [ ] `PUT /api/account/password`: any logged-in user changes their own password — build this here alongside the other identity endpoints, not in M8/M12, since it's what backs M10's `/account` page and is functionally part of the auth surface, not admin user management (`/api/users/:id`, which is a separate, admin-only endpoint for managing *other* accounts).
- [ ] Middleware: `requireSession` (any logged-in user) and `requireAdmin` (role check) for use across M8's route table.
- [ ] Unit/integration tests: setup flow blocked once a user exists; login/logout cycle; admin-only route returns 403 for a viewer-role session.

**DoD:** Setup → login → an admin-only test route works; a second setup attempt after the first user exists is rejected.

---

## M8 — REST API

**Goal:** Every endpoint in **spec §7**'s table, backed by M1–M7.

Build in this sub-order (each group is independently testable):

- [ ] **Read-only data endpoints** (any logged-in user): `/api/status`, `/api/players`, `/api/players/history`, `/api/power`, `/api/power/history`, `/api/power/metrics`, `/api/production`, `/api/production/items`, `/api/production/current`, `/api/production/machines`, `/api/resource-sink`, `/api/resource-sink/history`, `/api/drones`, `/api/doggos`, `/api/milestones`, `/api/research`, `/api/vehicles`, `/api/elevator`, `/api/elevator/unknown-log` — wait, that last one is admin-only per §7, double-check auth level against the table for every route while implementing, don't assume from this list.
- [ ] **Notification target CRUD** (admin): full set from §7, including `POST /api/notification-targets/:id/test` (send a placeholder message through M4's provider without a real trigger).
- [ ] **Message type / template endpoints** (admin): `GET /api/message-types`, `PUT .../template`, `POST .../template/reset?variant=...`, `POST .../template/preview` (renders unsaved input via M5 without persisting — needed for the frontend's live preview in M11), `PUT .../targets`, `PUT .../enabled`.
- [ ] **Notification log, settings, users, elevator diagnostics**: remaining admin endpoints from §7.
- [ ] Every mutating endpoint validates input server-side even though the frontend (M11) will also validate — never trust client-side validation alone, especially for template content (M5's validation function must run here, not just in the browser).

**DoD:** Every row in spec §7's table has a working handler, covered by at least one request/response test.

---

## M9 — Slow Poll: Production, Resource Sink, Factory, Drones

**Goal:** The slow-poll loop from **spec §4.1** — five endpoints, all dashboard-only, none notification-driving — plus the `production_snapshots` retention job.

- [ ] Separate ticker (not the M3 fast-poll loop) at `app_settings.production_snapshot_interval_seconds` (default 5 min), calling `frm.Client.GetSlow` (M2) once per tick.
- [ ] `getProdStats` result: upsert `prod_stats_state` (current snapshot, keyed by `item_class_name`) **and** append a row to `production_snapshots` (historical trend) — both from the same call, per §4.1's note that these two tables are populated together.
- [ ] `getResourceSink` result: upsert the single `resource_sink_state` row (id=1) with current values, **and** append a row to `resource_sink_snapshots` (history, for the `/resource-sink` chart) — do not use FRM's own `GraphPoints` field for this, it's a fixed non-interval-selectable window (see spec §4.1); this project builds its own proper time series instead.
- [ ] `getFactory` result: upsert `factory_machine_state` per machine ID. Derive `building_type` from `ClassName` (e.g. `Build_AssemblerMk1_C` → "Assembler") — a small mapping table/function, not a guess per machine.
- [ ] `getDrone` result: upsert `drone_state` per drone ID.
- [ ] `getDoggo` result: upsert `doggo_state` per doggo ID, storing `Inventory` as-is (JSON) — no derived flag or ClassName filtering needed, the item's own `Name` field already shows what's being carried. This is the one slow-poll table with no notification behind it at all, purely for the `/doggos` fun page.
- [ ] After updating `resource_sink_state`/`prod_stats_state`/etc., also append a `circuit_snapshots` row per circuit, reading from the already-current `circuit_state` (kept fresh by the fast poll's `getPower` call) — this is a DB read + insert, **not** an additional FRM API call, per spec §4.1's note on how this data gets into the slow-poll cycle without duplicating polling.
- [ ] Immediately after each successful cycle, delete rows older than `production_snapshot_retention_days` from **all three** history tables — `production_snapshots`, `resource_sink_snapshots`, `circuit_snapshots` — sharing the one retention setting per spec §4.1. The other five slow-poll tables are current-snapshot upserts, not history — nothing to prune there.
- [ ] A single failed slow-poll call logs an error and skips that tick's update for the affected table(s) only — it must not touch `server_state` (that's the fast poll's job exclusively, per §4.1).

**DoD:** `production_snapshots`, `resource_sink_snapshots`, and `circuit_snapshots` all accumulate real data over several cycles against the live server with automatic shared-retention pruning; `resource_sink_state`, `prod_stats_state`, `factory_machine_state`, `drone_state`, and `doggo_state` each reflect current real data from the group's server after one successful cycle.

---

## M10 — Frontend Scaffolding & Shell

**Goal:** Installed component set, auth pages, and app shell from **spec §8.1**.

- [ ] Run `shadcn add` for every component listed in the §8.1 table (not just the ones needed for the first page — get the full set installed once).
- [ ] Implement `/setup` and `/login` from the `login-01` block per §8.1, wired to M7's endpoints.
- [ ] Implement the app shell from the `sidebar-07` block per §8.1: nav items for all viewer pages always visible, "Settings" group only rendered when `GET /api/auth/me`'s role is `admin` — per the navigation note at the end of spec §8.
- [ ] Auth guard: unauthenticated users redirected to `/login`; a logged-in viewer hitting an admin route gets redirected or shown a 403 state, not a broken page.
- [ ] `/account` — simple `Card`+`Form`+`Input`+`Button` per §8.1, change-own-password form for any logged-in user (viewer or admin), backed by `PUT /api/account/password` (M7). Build it here rather than in M11/M12 since it's auth-adjacent, not a dashboard or admin-settings page — neither of those milestones' page counts (11 and 5 respectively) include it.

**DoD:** A fresh browser session can complete setup, log in, see the shell with correct nav for their role, change their password via `/account`, and log out.

---

## M11 — Viewer Pages

**Goal:** The 11 viewer-accessible dashboard pages from **spec §8** (routes) and **§8.1** (components), each wired to its M8 endpoint(s).

Build in this order (roughly simplest-to-most-complex):

- [ ] `/players` — `Table`/`Avatar`/`Badge` per §8.1, backed by `/api/players` + `/api/players/history`. The join/leave timeline has no stock component per §8.1 — build the simple `Card`+`Separator` list described there.
- [ ] `/power` — `Table` + `Progress` per §8.1, backed by `/api/power` + `/api/power/history`; render the full column set now available (capacity/production/consumption/battery), not just the tripped indicator. Add the historical `Chart` (per-circuit, interval-selectable) backed by `/api/power/metrics`, same date-range UX as `/production`'s chart.
- [ ] `/drones` — `Table` per §8.1, backed by `/api/drones`. Simplest new page, good warm-up before the more involved `/resource-sink` and `/production` rebuild below.
- [ ] `/doggos` — `Table` per §8.1, backed by `/api/doggos`, showing each doggo's `Inventory` items directly (no filtering logic needed). Trivial page, build alongside `/drones`.
- [ ] `/vehicles` — `Tabs` (Trains / Wheeled Vehicles) + `Table` per §8.1, backed by `/api/vehicles`. Straightforward, similar complexity to `/drones`.
- [ ] `/resource-sink` — `Card`s for current values (backed by `/api/resource-sink`) plus a historical `Chart` backed by `/api/resource-sink/history`, per §8.1. Confirm the interval picker actually changes the query range end-to-end — this page exists specifically because FRM's own native graph doesn't support that, so it's the one thing to double-check works before calling this page done.
- [ ] `/milestones` — `Tabs`/`Accordion`/`Badge` per §8.1, backed by `/api/milestones`, grouped by `Type` and `TechTier` as described in the page's purpose column in §8.
- [ ] `/research` — `Accordion`/`Table`/`Badge` per §8.1, backed by `/api/research`, grouped by tree name. Same composition pattern as `/milestones` — build them back-to-back.
- [ ] `/elevator` — `Card`/`Progress`/`Alert` per §8.1, backed by `/api/elevator`; the admin-only unresolved-diagnostics alert reads `/api/elevator/unknown-log` and offers the resolve action from `/api/elevator/unknown-log/:id/resolve`.
- [ ] `/production` — the most involved viewer page, two `Tabs`: **Overall** (`Table` from `/api/production/current`, row-click expands a `Chart` backed by `/api/production` filtered to that item) and **Detailed** (`Table` from `/api/production/machines`, row-click expands ingredient/output detail). This is the page most directly modeled on FRM's own web UI per §8's description — cross-check the built result against that UI's actual behavior on the group's server, not just against this spec.
- [ ] `/` (Dashboard Overview) — build last since it's a summary of everything above; `dashboard-01`-inspired card grid per §8.1, backed by `/api/status` primarily, pulling latest-milestone and elevator-phase summaries from the endpoints already wired above.

**DoD:** All 11 pages render real data from the live server correctly, including at least one manually-verified edge case per page (e.g. a tripped fuse and its battery detail showing correctly on `/power`, the group's actual Phase 2 progress on `/elevator`, real machine data on `/production`'s Detailed tab). For `/vehicles` specifically, the DoD only requires confirming trains/vehicles render correctly with real data — deliberately triggering an actual derailment or fuel-out event just to test the page isn't expected; that's covered incidentally whenever it happens for real.

---

## M12 — Admin Settings Pages

**Goal:** The 5 admin-only pages from spec §8/§8.1, the most custom-UI-heavy part of the project.

- [ ] `/settings/general` — form for `server_name`, `frm_host`, `frm_port`, poll/snapshot intervals, retention — backed by `/api/settings`.
- [ ] `/settings/notifications/targets` — `Table`+`Dialog`+`AlertDialog` per §8.1, backed by the target CRUD endpoints, including the "N message types assigned" count in the delete confirmation per spec §5.1's cascade-delete note.
- [ ] `/settings/notifications/log` — `Table`+`Badge` per §8.1, backed by `/api/notification-log`.
- [ ] `/settings/users` — `Table`+`Dialog`+`AlertDialog`+`Select` per §8.1, backed by the user CRUD endpoints.
- [ ] `/settings/notifications/templates` — build last, it's the one page with real custom components per §8.1:
  - [ ] List pane with `enabled` `Switch` per row.
  - [ ] Detail panel: `Tabs` for Plain/Embed variant.
  - [ ] The repeatable embed-fields array editor (`useFieldArray`, no stock component — see §8.1's note).
  - [ ] The color picker (`Popover` + swatch grid or `Input type="color"` — see §8.1's note).
  - [ ] The Discord-style live preview card (`Card`+`Separator` composition — see §8.1's note), updating on keystroke via `/api/message-types/:key/template/preview`.
  - [ ] Target-assignment checkboxes and the reset-to-default action (per-variant, per §5.4).

**DoD:** An admin can, entirely through the UI: create a Discord target, disable all message types except `player_joined`/`player_left`, customize the join notification's embed color and add a field, save, and see the change reflected in the next real in-game join event's Discord message.

---

## M13 — Docker Packaging & Deployment

**Goal:** Ship it.

- [ ] Multi-stage `Dockerfile` for the Go backend (static binary, minimal final image).
- [ ] `Dockerfile` (or static export served by the backend, per spec §2.1's "1–2 containers" note — pick one and be consistent) for the Next.js frontend.
- [ ] Finalize `docker-compose.yml`: env vars from **spec §9** (`PORT`, `DATABASE_PATH`, `SESSION_SECRET`, `FRM_HOST`, `FRM_PORT`), a named volume for the SQLite file, network alongside the existing `satisfactory-server` container.
- [ ] Deploy to the group's actual host (GuggiRaid, per this conversation) alongside the Satisfactory server; confirm it survives a container restart with state intact (SQLite volume persists) and a Satisfactory server restart (FRM's own well-documented autostart flakiness from earlier in this project should not affect FactoryMate's own reachability handling — it should just correctly report `server_offline` and recover automatically per M3).

**DoD:** The full stack runs in production on the group's server, survives restarts, and delivers a real Discord notification for a real game event without any manual intervention.

---

## M14 — Deferred Backlog

Not required for v1 launch; tracked here so they aren't lost. Each corresponds to a decided-default entry in **spec §10** ("Deferred Decisions & Defaults") — these are things the group already made a deliberate call on, not unresolved design questions:

- `hard_drive_ready` follow-up notification on actual recipe selection (schema already supports it, per §10 — deliberately deferred, not forgotten).
- Additional notification providers (ntfy, Telegram, generic webhook) — `Provider` interface from M4 already supports this without touching the dispatcher.
- Per-message-type polling cadence — deliberately kept simple for v1; revisit only if the group actually notices latency.
- Confirming Phases 3–5's elevator ClassName mapping against live data as the group naturally reaches them — check `elevator_phase_unknown_log` (M3.1) periodically; this isn't a task to schedule, it resolves itself as entries appear.
