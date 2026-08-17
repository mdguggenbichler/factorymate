# FactoryMate — Build Roadmap

**Companion document to:** `factorymate-spec.md` (the source of truth for schemas, API contracts, component choices, and every design decision — this document sequences the *work*, it does not redefine anything already specified there)

**How to use this document:** Milestones are ordered by dependency, not by page/feature grouping — each one only requires what was built in a prior milestone. Work through them in order. Each milestone has a **Definition of Done (DoD)** — do not start the next milestone until the current one's DoD is met. Every task that touches a schema, endpoint, or UI choice links to the exact spec section (`§X`) that defines it in full; this roadmap intentionally does not repeat those details, so cross-check the referenced section before implementing, don't guess from the task description alone.

---

## M0 — Project Scaffolding

**Goal:** An empty but runnable skeleton for both backend and frontend, in one repository.

- [ ] Create repo with `/backend` (Go module) and `/frontend` (Next.js App Router) directories.
- [ ] `backend`: `go mod init`, add dependencies: `modernc.org/sqlite` (§3), `chi` router (§2.1), `golang.org/x/crypto/bcrypt` (§6).
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
    data/                 -- static reference data: elevator phases (M3.1), message_defaults.json (M1)
  ```
- [ ] `frontend`: `npx create-next-app` (App Router, TypeScript, Tailwind), then `npx shadcn init`. Configure `next.config` API rewrite to backend for production (§2.4).
- [ ] `frontend`: install and wire **next-intl** per **spec §8.2** — `messages/en.json` with `common` namespace stub, `i18n` config, provider in root layout. Default page uses `t()` (no hardcoded UI strings even in skeleton).
- [ ] `docker-compose.yml` skeleton with two service stubs (`backend`, `frontend`) — filled in at M13.
- [ ] Backend: `GET /healthz` per §7 (liveness only). Frontend: default Next.js page.

**DoD:** `docker-compose up` (or `go run` + `npm run dev` locally) starts both processes cleanly with no business logic yet.

---

## M1 — Database Layer

**Goal:** Every table from spec §3 exists, migratable, with a seed step for the fixed message-type catalog.

- [ ] Implement a migration runner (numbered `.sql` files + `schema_migrations` table).
- [ ] Migration 001: create every table exactly as defined in **spec §3** — including `sessions`, `player_session_events`, `power_circuit_events`, `schematic_state.purchased_at`, and all extended columns on `schematic_state`, `elevator_state`, `vehicle_state`, `research_node_state`, `factory_machine_state`, `app_settings` (`frm_auth_token`). Copy SQL verbatim from spec §3.
- [ ] Seed step (idempotent, every startup): read `backend/data/message_defaults.json` (§5.5) and `INSERT OR IGNORE` the 13 rows into `message_types` per **spec §5.2**, with `default_template_json` from the file and `variables_json` as JSON array of strings per §5.2 (e.g. `["PlayerName","OnlineCount"]`). **Never overwrite** existing rows' `enabled` column.
- [ ] Seed `app_settings` with `id=1` if missing — `server_name` default per §3, `frm_host`/`frm_port` from env §9 (empty host allowed). Do **not** seed `notification_targets` or `message_type_targets` (§5.3).
- [ ] Unit test: migrations + seed twice on fresh DB — second run is no-op.

**DoD:** Fresh SQLite contains all §3 tables and exactly 13 seeded `message_types` rows with valid templates from `message_defaults.json`.

---

## M2 — FRM Client

**Goal:** Go package calling FRM endpoints from **spec §4.1** with typed structs and flexible JSON handling.

- [ ] Fast-poll structs per §4.1.1:
  - `getPlayer` → `[]Player{ID, Name, Online}`
  - `getPower` → `[]Circuit{CircuitGroupID, FuseTriggered, PowerProduction, PowerConsumed, PowerCapacity, PowerMaxConsumed, BatteryDifferential, BatteryPercent, BatteryCapacity, BatteryTimeEmpty, BatteryTimeFull}`
  - `getSchematics` → `[]Schematic{ID, Name, Type, Purchased, Locked, TechTier, Recipes []Recipe{Name, ClassName}}`
  - `getSpaceElevator` → `[]Elevator{ID, Name, CurrentPhase []PhaseItem{...}, UpgradeReady}`
  - `getResearchTrees` → full node struct including `Cost []Item` (§4.1.1)
  - `getTrains` → field names match FRM exactly (§4.1.1 mapping)
  - `getVehicles` → flexible `ID`; include `FollowingPath`, `Fuel []Item{Name, ClassName, Amount}` (§4.1.1, §4.2 fuel detection)
- [ ] Slow-poll structs:
  - `getProdStats` → per §3 `prod_stats_state` fields
  - `getResourceSink` → array response; use first element; `GraphPoint` accepts `Value` or `value` JSON key (ignored for history — §4.1)
  - `getFactory` → include `ingredients` and `production` arrays; flexible unmarshal on item `Amount` (string or int)
  - `getDrone` → `FlyingSpeed`/`MaxSpeed` as `float64` with flexible unmarshal (adoc vs live mismatch)
  - `getDoggo` → `Inventory` as-is
- [ ] HTTP client: 5s timeout, no retry, config from `app_settings` (host/port/token), `X-FRM-Authorization` when token set (§4.1).
- [ ] `Client.GetFast(ctx)` and `Client.GetSlow(ctx)` — partial failure reporting on GetFast for unreachable detection (M3).
- [ ] Integration test against live server (host from `app_settings`, not hardcoded) — all 12 endpoints parse without error.

**DoD:** Live FRM server returns populated structs for all 12 endpoints with no parse errors.

---

## M3 — Poller / Diff Engine

**Goal:** Edge-triggered detection per **spec §4.2**, state persistence, event history, variable population per **§4.2.1**.

- [ ] Poll loop: `app_settings.poll_interval_seconds` (default 20s), `frm.Client.GetFast`.
- [ ] Reachability + **`server_state` updates** per §4.2 (`server_online`/`server_offline`; First Observation when `server_online` IS NULL — write baseline, do not emit).
- [ ] Edge-trigger all message types in §4.2 table; set `schematic_state.purchased_at` on `Purchased` false→true; set `player_state.last_seen_at` on leave (§4.1.1). When upserting `player_state`/`circuit_state`/`schematic_state`/`elevator_state`/`research_node_state`/`train_state`/`vehicle_state` and `server_state`, check whether a previous row existed for that entity before this poll. If not, write the baseline row and skip event emission for that entity this cycle — do not treat a missing previous row as an implicit `false` or `off` value when evaluating the §4.2 trigger table, per the First Observation rule in spec §4.2.
- [ ] Unit test: seed an empty database, run one poll cycle against a fixture where several entities are already in a "positive" state (a milestone already Purchased, a fuse already tripped, a player already online), and assert zero notifications are emitted on that first cycle, with all state tables nonetheless populated correctly as the baseline.
- [ ] On each event: populate variables per **§4.2.1**; INSERT `player_session_events` / `power_circuit_events` as specified (regardless of `enabled`).
- [ ] `vehicle_stuck` debounce per §4.2 (3 consecutive polls, edge on `stuck` column).
- [ ] Detection runs regardless of `message_types.enabled` — M6 filters at dispatch (§4.2 opening paragraph).

### M3.1 — Elevator Phase Lookup

- [ ] `backend/data/elevator_phases.json` — §4.2 reference table (Phases 1–5 ClassName sets).
- [ ] Match sorted `CurrentPhase[].ClassName` set → `elevator_state.phase_number`; persist `name`, `current_phase_json` (§3, §4.1.1).
- [ ] No match → `phase_number = NULL`, insert `elevator_phase_unknown_log` with **dedup rule** (§4.2).
- [ ] Unit tests: Phase 2 payload → `phase_number = 2`; unmatched set → NULL + log row.

**DoD:** Poller produces correct non-duplicated state in all fast-poll tables + `server_state`; elevator phase = 2 on group's save.

---

## M4 — Notification Provider Interface + Discord

**Goal:** `Provider` abstraction from **spec §2.3**, working Discord implementation.

- [ ] Implement `internal/notify` types and `Provider` interface per §2.3.
- [ ] `RenderedMessage` supports plain + embed shapes (M5 output).
- [ ] `DiscordProvider`: webhook POST per §5.1; respect Discord limits (§5.4).
- [ ] Manual test: sample embed to real Discord webhook.

**DoD:** Hardcoded sample embed reaches Discord correctly formatted.

---

## M5 — Templating Engine

**Goal:** Render templates per **§5.4** (`{VarName}` syntax, not Go text/template).

- [ ] Custom `{VarName}` substitution for plain and per-field embed templating.
- [ ] Lookup: `message_templates` override → `default_template_json` fallback (partial override shape §5.4).
- [ ] Validation: unknown vars, Discord limits, §5.4.1 sample data.
- [ ] Empty optional vars: omit empty embed fields (§5.4).
- [ ] Unit tests: all 13 defaults from `message_defaults.json` render with §5.4.1 samples.

**DoD:** Every default template renders without validation errors for both variants.

---

## M6 — Dispatch Wiring

**Goal:** M3 events → M5 renderer → M4 provider, per §5.3.

- [ ] Skip dispatch when `message_types.enabled = 0`; lookup `message_type_targets`; render per provider type.
- [ ] Dispatch to enabled targets; log to `notification_log` (not a substitute for event history tables — §3).
- [ ] Wire into M3 poll loop as final step.
- [ ] E2E manual test: with target configured (API insert after M8 or dev seed), real player join → Discord within one poll interval.

**DoD:** Dispatch path verified — integration test with test webhook + `notification_log` row, or full in-game E2E when target exists.

---

## M7 — Auth

**Goal:** Session auth per **§6** (SQLite `sessions` table).

- [ ] `POST /api/auth/setup`, login, logout, `GET /api/auth/me` — cookie per §6.
- [ ] `PUT /api/account/password` for any logged-in user.
- [ ] Middleware: `requireSession`, `requireAdmin`.
- [ ] Session cleanup job for expired rows.
- [ ] Tests: setup once, login/logout, viewer gets 403 on admin route.

**DoD:** Setup → login → admin route works; second setup rejected.

---

## M8 — REST API

**Goal:** Every endpoint in **spec §7**, response shapes per **§7.1** and §7.2 request bodies.

**DoD:** Every §7 row has a handler + at least one request/response test. Assert full JSON for endpoints in §7.1; for other GETs, assert camelCase mapping from §3 columns per §7.1 intro.

- [ ] **Read-only data** (session auth): all GET endpoints from §7 table — verify admin vs session per row. History endpoints use pagination envelope (§7).
- [ ] **Notification target CRUD** (admin) + test send.
- [ ] **Message type / template** endpoints (admin) including preview (M5, unsaved input).
- [ ] **Settings, users, elevator diagnostics, notification log** (admin).
- [ ] `GET /healthz` (no auth) — may already exist from M0; ensure §7.1 shape.
- [ ] Mutating endpoints: server-side validation (templates via M5 validator); request bodies per §7.2.

---

## M9 — Slow Poll

**Goal:** Slow-poll loop per **spec §4.1** + retention.

- [ ] Ticker at `production_snapshot_interval_seconds`, `frm.Client.GetSlow`.
- [ ] `getProdStats` → `prod_stats_state` + `production_snapshots`.
- [ ] `getResourceSink` → `resource_sink_state` (first array element) + `resource_sink_snapshots`.
- [ ] `getFactory` → `factory_machine_state` including **`ingredients_json` and `production_json`** (§3, §4.1.1); derive `building_type` from `ClassName` using the mapping table in **spec §4.1** (several of the 11 types have no ClassName in vendored FRM docs and must be verified against a live `getFactory` response before implementing).
- [ ] `getResearchTrees` cost → already on fast poll (`research_node_state.cost_json`); slow poll does not re-poll research.
- [ ] `getDrone` → `drone_state`; `getDoggo` → `doggo_state`.
- [ ] Append `circuit_snapshots` from current `circuit_state` (no extra FRM call).
- [ ] Prune all three history tables after successful cycle (§4.1 retention).
- [ ] Slow-poll failures: log + skip affected tables only; never touch `server_state`.

**DoD:** History tables accumulate real data with pruning; snapshot tables reflect live server after one cycle.

---

## M10 — Frontend Scaffolding & Shell

**Goal:** shadcn set, auth pages, shell per **§8.1**; API wiring per **§2.4**; i18n per **§8.2**.

- [ ] `shadcn add` all components from §8.1 table.
- [ ] `/setup`, `/login` (`login-01`), wired to M7 — **all labels/buttons via `messages/en.json`** (§8.2).
- [ ] App shell (`sidebar-07`): viewer nav + admin Settings group — nav item labels from `nav` namespace.
- [ ] Auth guard + `/account` password form (M7) — form strings from `auth` namespace.
- [ ] `NEXT_PUBLIC_API_URL` for local dev (§9).

**DoD:** Setup → login → shell with correct nav → change password → logout. **No hardcoded user-facing strings** in `frontend/` (§8.2).

---

## M11 — Viewer Pages

**Goal:** 11 viewer pages per **§8**, wired to M8 + **§7.1** shapes. **Every new UI string → `messages/en.json`** (§8.2).

Build order:

- [ ] `/players` — `/api/players` + `/api/players/history`
- [ ] `/power` — `/api/power` + `/api/power/history` + `/api/power/metrics` chart
- [ ] `/drones`, `/doggos` — simple tables
- [ ] `/vehicles` — trains + wheeled tabs
- [ ] `/resource-sink` — cards + history chart (interval picker end-to-end)
- [ ] `/milestones`, `/research` — grouped views with names/costs/recipes
- [ ] `/elevator` — progress from `currentPhase`; admin unknown-log alert
- [ ] `/production` — Overall + Detailed tabs; Detailed expand uses `ingredients`/`production` from API
- [ ] `/` Overview — **primarily `GET /api/status`** (§7.1 includes milestone + elevator summaries)

**DoD:** All 11 pages render real live data; edge cases per `factorymate-spec.md` §8 DoD notes (e.g. Phase 2 on `/elevator`, fuse detail on `/power`).

---

## M12 — Admin Settings Pages

**Goal:** 5 admin pages per **§8/§8.1**. **Every new UI string → `messages/en.json`** (§8.2).

- [ ] `/settings/general` — including optional `frm_auth_token` (§3)
- [ ] `/settings/notifications/targets` — CRUD + cascade delete warning
- [ ] `/settings/notifications/log`
- [ ] `/settings/users`
- [ ] `/settings/notifications/templates` — full editor per §8.1 (fields array, color picker, live preview)

**DoD:** Admin can create Discord target, customize `player_joined` embed, assign target, see change on next real join.

---

## M13 — Docker Packaging & Deployment

**Goal:** Ship two-container stack per **§2.4**.

- [ ] Multi-stage `backend` Dockerfile (static Go binary).
- [ ] `frontend` Dockerfile (Next.js standalone + `/api` rewrite to backend).
- [ ] Finalize `docker-compose.yml`: §9 env vars, SQLite volume, `backend` + `frontend` services, network with `satisfactory-server`.
- [ ] Deploy to GuggiRaid; survive restarts; FRM offline → `server_offline` → auto-recover (M3).

**DoD:** Full stack in production; real Discord notification on real game event without manual intervention.

---

## M14 — Deferred Backlog

Per **spec §10** — not v1:

- `hard_drive_ready` follow-up on recipe selection
- Additional notification providers (Provider interface §2.3 ready)
- Per-message-type polling cadence
- Live confirmation of elevator Phases 3–5 via `elevator_phase_unknown_log`
- Additional UI languages + locale switcher (i18n infra ready — §8.2, §10)
