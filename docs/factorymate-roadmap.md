# FactoryMate — Build Roadmap

**Companion document to:** `factorymate-spec.md` (the source of truth for schemas, API contracts, component choices, and every design decision — this document sequences the *work*, it does not redefine anything already specified there)

**How to use this document:** Milestones are ordered by dependency, not by page/feature grouping — each one only requires what was built in a prior milestone. Work through them in order. Each milestone has a **Definition of Done (DoD)** — do not start the next milestone until the current one's DoD is met. Every task that touches a schema, endpoint, or UI choice links to the exact spec section (`§X`) that defines it in full; this roadmap intentionally does not repeat those details, so cross-check the referenced section before implementing, don't guess from the task description alone.

---

## M0 — Project Scaffolding

**Goal:** An empty but runnable skeleton for both backend and frontend, in one repository.

- [x] Create repo with `/backend` (Go module) and `/frontend` (Next.js App Router) directories.
- [x] `backend`: `go mod init`, add dependencies: `modernc.org/sqlite` (§3), `chi` router (§2.1), `golang.org/x/crypto/bcrypt` (§6).
- [x] `backend`: directory layout —
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
- [x] `frontend`: `npx create-next-app` (App Router, TypeScript, Tailwind), then `npx shadcn init`. Configure `next.config` API rewrite to backend for production (§2.4).
- [x] `frontend`: install and wire **next-intl** per **spec §8.2** — `messages/en.json` with `common` namespace stub, `i18n` config, provider in root layout. Default page uses `t()` (no hardcoded UI strings even in skeleton).
- [x] `docker-compose.yml` skeleton with two service stubs (`backend`, `frontend`) — filled in at M13.
- [x] Backend: `GET /healthz` per §7 (liveness only). Frontend: default Next.js page.

**DoD:** `docker-compose up` (or `go run` + `npm run dev` locally) starts both processes cleanly with no business logic yet.

---

## M1 — Database Layer

**Goal:** Every table from spec §3 exists, migratable, with a seed step for the fixed message-type catalog.

- [x] Implement a migration runner (numbered `.sql` files + `schema_migrations` table).
- [x] Migration 001: create every table exactly as defined in **spec §3** — including `sessions`, `player_session_events`, `power_circuit_events`, `schematic_state.purchased_at`, and all extended columns on `schematic_state`, `elevator_state`, `vehicle_state`, `research_node_state`, `factory_machine_state`, `app_settings` (`frm_auth_token`). Copy SQL verbatim from spec §3.
- [x] Seed step (idempotent, every startup): read `backend/data/message_defaults.json` (§5.5) and `INSERT OR IGNORE` the 13 rows into `message_types` per **spec §5.2**, with `default_template_json` from the file and `variables_json` as JSON array of strings per §5.2 (e.g. `["PlayerName","OnlineCount"]`). **Never overwrite** existing rows' `enabled` column.
- [x] Seed `app_settings` with `id=1` if missing — `server_name` default per §3, `frm_host`/`frm_port` from env §9 (empty host allowed). Do **not** seed `notification_targets` or `message_type_targets` (§5.3).
- [x] Unit test: migrations + seed twice on fresh DB — second run is no-op.

**DoD:** Fresh SQLite contains all §3 tables and exactly 13 seeded `message_types` rows with valid templates from `message_defaults.json`.

---

## M2 — FRM Client

**Goal:** Go package calling FRM endpoints from **spec §4.1** with typed structs and flexible JSON handling.

- [x] Fast-poll structs per §4.1.1:
  - `getPlayer` → `[]Player{ID, Name, Online}`
  - `getPower` → `[]Circuit{CircuitGroupID, FuseTriggered, PowerProduction, PowerConsumed, PowerCapacity, PowerMaxConsumed, BatteryDifferential, BatteryPercent, BatteryCapacity, BatteryTimeEmpty, BatteryTimeFull}`
  - `getSchematics` → `[]Schematic{ID, Name, Type, Purchased, Locked, TechTier, Recipes []Recipe{Name, ClassName}}`
  - `getSpaceElevator` → `[]Elevator{ID, Name, CurrentPhase []PhaseItem{...}, UpgradeReady}`
  - `getResearchTrees` → full node struct including `Cost []Item` (§4.1.1)
  - `getTrains` → field names match FRM exactly (§4.1.1 mapping)
  - `getVehicles` → flexible `ID`; include `FollowingPath`, `Fuel []Item{Name, ClassName, Amount}` (§4.1.1, §4.2 fuel detection)
- [x] Slow-poll structs:
  - `getProdStats` → per §3 `prod_stats_state` fields
  - `getResourceSink` → array response; use first element; `GraphPoint` accepts `Value` or `value` JSON key (ignored for history — §4.1)
  - `getFactory` → include `ingredients` and `production` arrays; flexible unmarshal on item `Amount` (string or int)
  - `getDrone` → `FlyingSpeed`/`MaxSpeed` as `float64` with flexible unmarshal (adoc vs live mismatch)
  - `getDoggo` → `Inventory` as-is
- [x] HTTP client: 5s timeout, no retry, config from `app_settings` (host/port/token), `X-FRM-Authorization` when token set (§4.1).
- [x] `Client.GetFast(ctx)` and `Client.GetSlow(ctx)` — partial failure reporting on GetFast for unreachable detection (M3).
- [x] Integration test against live server (host from `app_settings`, not hardcoded) — all 12 endpoints parse without error.

**DoD:** Live FRM server returns populated structs for all 12 endpoints with no parse errors.

---

## M3 — Poller / Diff Engine

**Goal:** Edge-triggered detection per **spec §4.2**, state persistence, event history, variable population per **§4.2.1**.

- [x] Poll loop: `app_settings.poll_interval_seconds` (default 20s), `frm.Client.GetFast`.
- [x] Reachability + **`server_state` updates** per §4.2 (`server_online`/`server_offline`; First Observation when `server_online` IS NULL — write baseline, do not emit).
- [x] Edge-trigger all message types in §4.2 table; set `schematic_state.purchased_at` on `Purchased` false→true; set `player_state.last_seen_at` on leave (§4.1.1). When upserting `player_state`/`circuit_state`/`schematic_state`/`elevator_state`/`research_node_state`/`train_state`/`vehicle_state` and `server_state`, check whether a previous row existed for that entity before this poll. If not, write the baseline row and skip event emission for that entity this cycle — do not treat a missing previous row as an implicit `false` or `off` value when evaluating the §4.2 trigger table, per the First Observation rule in spec §4.2.
- [x] Unit test: seed an empty database, run one poll cycle against a fixture where several entities are already in a "positive" state (a milestone already Purchased, a fuse already tripped, a player already online), and assert zero notifications are emitted on that first cycle, with all state tables nonetheless populated correctly as the baseline.
- [x] On each event: populate variables per **§4.2.1**; INSERT `player_session_events` / `power_circuit_events` as specified (regardless of `enabled`).
- [x] `vehicle_stuck` debounce per §4.2 (3 consecutive polls, edge on `stuck` column).
- [x] Detection runs regardless of `message_types.enabled` — M6 filters at dispatch (§4.2 opening paragraph).

### M3.1 — Elevator Phase Lookup

- [x] `backend/data/elevator_phases.json` — §4.2 reference table (Phases 1–5 ClassName sets).
- [x] Match sorted `CurrentPhase[].ClassName` set → `elevator_state.phase_number`; persist `name`, `current_phase_json` (§3, §4.1.1).
- [x] No match → `phase_number = NULL`, insert `elevator_phase_unknown_log` with **dedup rule** (§4.2).
- [x] Unit tests: Phase 2 payload → `phase_number = 2`; unmatched set → NULL + log row.

**DoD:** Poller produces correct non-duplicated state in all fast-poll tables + `server_state`; elevator phase = 2 on group's save.

---

## M4 — Notification Provider Interface + Discord

**Goal:** `Provider` abstraction from **spec §2.3**, working Discord implementation.

- [x] Implement `internal/notify` types and `Provider` interface per §2.3.
- [x] `RenderedMessage` supports plain + embed shapes (M5 output).
- [x] `DiscordProvider`: webhook POST per §5.1; respect Discord limits (§5.4).
- [x] Manual test: sample embed to real Discord webhook.

**DoD:** Hardcoded sample embed reaches Discord correctly formatted.

---

## M5 — Templating Engine

**Goal:** Render templates per **§5.4** (`{VarName}` syntax, not Go text/template).

- [x] Custom `{VarName}` substitution for plain and per-field embed templating.
- [x] Lookup: `message_templates` override → `default_template_json` fallback (partial override shape §5.4).
- [x] Validation: unknown vars, Discord limits, §5.4.1 sample data.
- [x] Empty optional vars: omit empty embed fields (§5.4).
- [x] Unit tests: all 13 defaults from `message_defaults.json` render with §5.4.1 samples.

**DoD:** Every default template renders without validation errors for both variants.

---

## M6 — Dispatch Wiring

**Goal:** M3 events → M5 renderer → M4 provider, per §5.3.

- [x] Skip dispatch when `message_types.enabled = 0`; lookup `message_type_targets`; render per provider type.
- [x] Dispatch to enabled targets; log to `notification_log` (not a substitute for event history tables — §3).
- [x] Wire into M3 poll loop as final step.
- [x] E2E manual test: with target configured (API insert after M8 or dev seed), real player join → Discord within one poll interval.

**DoD:** Dispatch path verified — integration test with test webhook + `notification_log` row, or full in-game E2E when target exists.

---

## M7 — Auth

**Goal:** Session auth per **§6** (SQLite `sessions` table).

- [x] `POST /api/auth/setup`, login, logout, `GET /api/auth/me` — cookie per §6.
- [x] `PUT /api/account/password` for any logged-in user.
- [x] Middleware: `requireSession`, `requireAdmin`.
- [x] Session cleanup job for expired rows.
- [x] Tests: setup once, login/logout, viewer gets 403 on admin route.

**DoD:** Setup → login → admin route works; second setup rejected.

---

## M8 — REST API

**Goal:** Every endpoint in **spec §7**, response shapes per **§7.1** and §7.2 request bodies.

**DoD:** Every §7 row has a handler + at least one request/response test. Assert full JSON for endpoints in §7.1; for other GETs, assert camelCase mapping from §3 columns per §7.1 intro.

- [x] **Read-only data** (session auth): all GET endpoints from §7 table — verify admin vs session per row. History endpoints use pagination envelope (§7).
- [x] **Notification target CRUD** (admin) + test send.
- [x] **Message type / template** endpoints (admin) including preview (M5, unsaved input).
- [x] **Settings, users, elevator diagnostics, notification log** (admin).
- [x] `GET /healthz` (no auth) — may already exist from M0; ensure §7.1 shape.
- [x] Mutating endpoints: server-side validation (templates via M5 validator); request bodies per §7.2.

---

## M9 — Slow Poll

**Goal:** Slow-poll loop per **spec §4.1** + retention.

- [x] Ticker at `production_snapshot_interval_seconds`, `frm.Client.GetSlow`.
- [x] `getProdStats` → `prod_stats_state` + `production_snapshots`.
- [x] `getResourceSink` → `resource_sink_state` (first array element) + `resource_sink_snapshots`.
- [x] `getFactory` → `factory_machine_state` including **`ingredients_json` and `production_json`** (§3, §4.1.1); derive `building_type` from `ClassName` using the mapping table in **spec §4.1** (several of the 11 types have no ClassName in vendored FRM docs and must be verified against a live `getFactory` response before implementing).
- [x] `getResearchTrees` cost → already on fast poll (`research_node_state.cost_json`); slow poll does not re-poll research.
- [x] `getDrone` → `drone_state`; `getDoggo` → `doggo_state`.
- [x] Append `circuit_snapshots` from current `circuit_state` (no extra FRM call).
- [x] Prune all three history tables after successful cycle (§4.1 retention).
- [x] Slow-poll failures: log + skip affected tables only; never touch `server_state`.

**DoD:** History tables accumulate real data with pruning; snapshot tables reflect live server after one cycle.

---

## M10 — Frontend Scaffolding & Shell

**Goal:** shadcn set, auth pages, shell per **§8.1**; API wiring per **§2.4**; i18n per **§8.2**.

- [x] `shadcn add` all components from §8.1 table.
- [x] `/setup`, `/login` (`login-01`), wired to M7 — **all labels/buttons via `messages/en.json`** (§8.2).
- [x] App shell (`sidebar-07`): viewer nav + admin Settings group — nav item labels from `nav` namespace.
- [x] Auth guard + `/account` password form (M7) — form strings from `auth` namespace.
- [x] `NEXT_PUBLIC_API_URL` for local dev (§9).

**DoD:** Setup → login → shell with correct nav → change password → logout. **No hardcoded user-facing strings** in `frontend/` (§8.2).

---

## M11 — Viewer Pages

**Goal:** 11 viewer pages per **§8**, wired to M8 + **§7.1** shapes. **Every new UI string → `messages/en.json`** (§8.2).

Build order:

- [x] `/players` — `/api/players` + `/api/players/history`
- [x] `/power` — `/api/power` + `/api/power/history` + `/api/power/metrics` chart
- [x] `/drones`, `/doggos` — simple tables
- [x] `/vehicles` — trains + wheeled tabs
- [x] `/resource-sink` — cards + history chart (interval picker end-to-end)
- [x] `/milestones`, `/research` — grouped views with names/costs/recipes
- [x] `/elevator` — progress from `currentPhase`; admin unknown-log alert
- [x] `/production` — Overall + Detailed tabs; Detailed expand uses `ingredients`/`production` from API
- [x] `/` Overview — **primarily `GET /api/status`** (§7.1 includes milestone + elevator summaries)

**DoD:** All 11 pages render real live data; edge cases per **spec §8.3** (e.g. Phase 2 on `/elevator`, fuse detail on `/power`).

---

## M12 — Admin Settings Pages

**Goal:** 5 admin pages per **§8/§8.1**. **Every new UI string → `messages/en.json`** (§8.2).

- [x] `/settings/general` — including optional `frm_auth_token` (§3)
- [x] `/settings/notifications/targets` — CRUD + cascade delete warning
- [x] `/settings/notifications/log`
- [x] `/settings/users`
- [x] `/settings/notifications/templates` — full editor per §8.1 (fields array, color picker, live preview)

**DoD:** Admin can create Discord target, customize `player_joined` embed, assign target, see change on next real join.

---

## M13 — Docker Packaging & Deployment

**Goal:** Ship single-container stack per **§2.4**.

- [x] Multi-stage root `Dockerfile` (static Go binary + Next.js standalone).
- [x] Entrypoint runs Go backend and Next.js in one container; `/api` rewrite to localhost backend.
- [x] Finalize `docker-compose.yml`: §9 env vars, SQLite volume, single `factorymate` service, network with `satisfactory-server`.
- [x] Deploy to GuggiRaid; survive restarts; FRM offline → `server_offline` → auto-recover (M3).

**DoD:** Full stack in production; real Discord notification on real game event without manual intervention.

---

## M15 — Discord Bot, Provider Refactor & Player Onboarding

**Goal:** Unified Discord bot for onboarding, notifications, connection details, and mod list — per `docs/discord-bot-plan.md`.

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

**DoD:** Discord `/register` onboarding works; bot posts game events to configured channels; `/mods` page and connection settings live; web invite is break-glass only; spec and docs updated.

---

## M16 — Notifications polish & admin commands

**Goal:** Per-user DM routing for game events, notification preference APIs, and M16 Discord admin commands — per `docs/discord-bot-plan.md` §9, §11.2, §12.2, §13.

- [x] Migration 005: `user_notification_prefs` table; seed `notifications.dm_defaults_json` and `notifications.dm_player_personal_default`
- [x] Game-event DM fan-out: dispatcher `SendDirect` for users with category `dm_enabled`
- [x] Personal player event DMs: `player_joined` / `player_left` when event player matches linked user and `dm_player_personal=1`
- [x] API: `GET/PUT /api/account/notifications`, `GET/PUT /api/settings/notification-defaults` (admin)
- [x] Discord admin commands: `/status`, `/players`, `/broadcast`, `/sync-roles`, `/notifications`
- [x] Auto-link DM when poller resolves `pending_player_name` → `player_id`
- [x] Guild member role change listener (`/sync-roles` mapping)
- [x] Optional: `connection_details_changed` message type in `message_defaults.json` + seed
- [x] Tests: DM fan-out respects prefs; personal player DM; dispatcher regression
- [x] Phase 2 (frontend): Account → Notifications UI + Settings → Notifications → Defaults

**DoD (backend):** Category DM fan-out and personal player DMs work; preference APIs return/update prefs; M16 slash commands respond; auto-link and role-sync paths covered by tests.

---

## M17 — Discord SSO & secure onboarding

**Goal:** Discord OAuth for Discord-origin users (no FM password). Local password only for `/setup` and break-glass invites. Remove Discord password collection. Fix guild-save slash command registration bug.

- [x] Spec §2.1/§3/§6/§7/§7.1/§7.2/§8/§9 — Discord OAuth, nullable `password_hash`, `oauth_states`; discord-bot-plan §6 + command tables + Appendix G
- [x] Migration 008: nullable `password_hash`, `oauth_states` table (token_hash SHA-256, purpose, TTL, single-use)
- [x] OAuth: `GET /api/auth/discord` (login — existing users only), callback; register flow via `/register` slash → OAuth state=register → web complete form (username + in-game name, no password); Account → Link Discord (logged-in OAuth link)
- [x] `LinkExternal`; `Register` without password when Discord external present; password login rejects NULL hash
- [x] Discord: `/register` and `/register-user` DM OAuth URLs (no modals); remove `/link` command; remove or replace `/password-reset` (no DM secrets — admin sets password on web)
- [x] Re-register slash commands when guild ID saved in Settings → Discord (fix bug: only registered on bot Start today); clear commands on old guild if ID changed
- [x] Rewrite `/help` and all slash command `Description` strings in commands.go
- [x] Frontend: `/login` Continue with Discord (when OAuth configured); register-complete page; Account Link Discord; hide Discord UI when not configured; i18n in messages/en.json
- [x] `.env.example`, docker-compose, guides (commands.md, users.md, first-run.md, development.md, discord configuration)
- [x] Add M17 to `.agents/project/orchestrator/milestone-scopes.md` with READ/WRITE + scoped CI
- [x] Tests: SSO login vs provision separation; link while logged in; no Discord password fields; guild-save re-registers commands (mock discordgo if needed)

**DoD:** Discord `/register` finishes on web via OAuth (no password modals); login with Discord for linked users; setup/invite users link Discord from Account; slash commands appear after saving guild ID without container restart; `/help` and descriptions current; spec and docs updated.

---

## M18 — Per-type DM prefs & two-layer notification UX

**Goal:** Per-message-type personal DM preferences on the dashboard; Discord `/notifications` stays category-level coarse control with a dashboard link; clearly communicate independent guild-channel vs personal-DM layers; informational channel-overlap hints (never block enabling DMs).

- [x] Spec §3/§7/§7.1/§7.2/§8/§8.1: per-type user prefs + catalog on GET /api/account/notifications; admin defaults per-type; Account and Defaults pages described as two-layer + type checkboxes + overlap hints. Reverse “categories only / no per-type DM toggles” in docs/discord-bot-plan.md §9.3, §9.6, §11.3. Guide copy in docs/guide/managing/notifications.md (and related settings/commands docs if they still say categories-only). Add M18 to docs/factorymate-roadmap.md (unchecked tasks) and `.agents/project/orchestrator/milestone-scopes.md` (M18 WRITE/READ + scoped CI). Update `.agents/project/orchestrator/doc-index.md` milestone table if needed.
- [x] SQLite migration (next numbered file after existing migrations): rebuild `user_notification_prefs` to PK (user_id, message_type_key) FK message_types(key). Expand old category rows to all types in that category. Expand `notifications.dm_defaults_json` from five category booleans to per-type booleans. Exclude `connection_details` and `connection_details_changed` from prefs catalog.
- [x] API: GET/PUT /api/account/notifications uses `types` map + `dmPlayerPersonal` + `catalog` (key, label, category, globallyEnabled, channelTargets [{id, name}]). PUT may be partial. GET/PUT /api/settings/notification-defaults uses per-type `types` map + dmPlayerPersonalDefault. Viewers must not need admin GET /api/message-types.
- [x] Dispatcher: DM fan-out by message type key (not category). `message_types.enabled` remains global kill switch (no channel and no DM). Channel delivery still needs assigned enabled targets. Personal player DMs unchanged (`dm_player_personal` for player_joined/left matching linked player). Tests updated.
- [x] Frontend: Account → Notifications and Settings → Notifications → Defaults: two-layer intro copy; types grouped by category; per-type Switch/checkbox using API labels; category enable-all/none optional; overlap hint when channelTargets nonempty (informational, not AlertDialog); disabled row when globallyEnabled false; personal-player toggle kept. All chrome via frontend/messages/en.json (spec §8.2). Short callout on Templates page about two layers.
- [x] Discord `/notifications`: keep action view|category|personal. Category on/off sets ALL types in that category (overwrites mixed). View shows two-layer one-liner, category summary on/off/mixed (n/m), personal toggle. ALWAYS append dashboard URL `{FACTORYMATE_PUBLIC_URL}/account/notifications` when set; prefer Discord Link button (style 5) plus URL in body. If public URL empty, say dashboard URL is not configured. After category enable, overlap hint if any of those types have channel targets. Do NOT add per-type Discord toggles. Do NOT put prefs URL on every game-event DM footer.

**DoD:** Per-type DM prefs persist and dispatch; catalog is available to viewers on the account API; Discord `/notifications` remains category-level with a dashboard link; two-layer copy and overlap hints never block enabling DMs; scoped tests and frontend lint/build pass.

---

## M19 — Factory Planner: Game Data & Catalog

**Goal:** Parse Coffee Stain `FactoryGame-Docs.json` into an in-memory catalog and expose slim read-only planner APIs — per `docs/proposals/factory-planner.md` §4, §7.1 (catalog/icons only). Does **not** touch FRM poller, Discord, or notification pipeline.

**Design doc:** `docs/proposals/factory-planner.md` (source of truth for planner schemas, formulas, and API shapes until spec §2–§8 are updated in M21).

- [x] Add `golang.org/x/text` to `backend/go.mod` for UTF-16 LE BOM decode (`docs/FactoryGame-Docs.json` is **not** UTF-8 — see proposal §4.1).
- [x] `backend/internal/planner/parse_ue.go` — parse Unreal `((ItemClass="...",Amount=N))` ingredient/product strings; extract `Desc_*_C` / `Build_*_C` ClassNames.
- [x] `backend/internal/planner/catalog.go` — load dump via `unicode.UTF16(LittleEndian, UseBOM)` decoder; ingest `FGRecipe`, item/resource descriptors, manufacturers, extractors/pumps, belts (`mSpeed`), pipes (`mFlowLimit`); fluid/gas amounts **÷1000** to game units; filter `mProducedIn` (drop workbench/workshop/build gun/automated workbench); alternate detection (`/AlternateRecipes/` or `Recipe_Alternate_*`); **Smelter-only** somersloop slot override `{ Build_SmelterMk1_C: 1 }` (Packager stays 0); `Build_*` → `Desc_*` icon ClassName map from `assets/icons.json`.
- [x] Optional generated slim catalog: `backend/data/factory_catalog.json` (UTF-8) via small `go generate` / cmd — version in repo so tests/Docker need not ship the 10 MiB dump at runtime if slim file present; keep `docs/FactoryGame-Docs.json` as source of truth.
- [x] `backend/internal/planner/balance.go` — per-node rates from clock (0–250%) + Somersloops; edge flow, proportional split, over/underproduction, belt/pipe Mk recommendation with numeric rate + `exceedsMax`; power formula `P = P_base × (clock/100)^N × (1 + sloops/slots)²` using dump `mPowerConsumptionExponent`.
- [x] Golden fixtures: `backend/testdata/planner/power_examples.json` (four wiki rows in proposal §4.1) + balance in/out JSON for Vitest parity in M21.
- [x] `GET /api/planner/catalog` — session + active user; slim JSON (items, recipes, buildings, belts, pipes); mount in `routes.go` like other session routes.
- [x] `GET /api/planner/icons/{className}` — `image/png` from `assets/icons/` with mapping fallback; 404 when missing (frontend uses placeholder).
- [x] Env defaults: `PLANNER_DOCS_PATH`, `PLANNER_ICONS_DIR` (proposal §10); local dev paths relative to repo root.
- [x] Tests: `catalog_test.go` (UTF-16 fixture + dump BOM signature); fluid scaling; alternates; belt/pipe tables; Smelter override; `balance_test.go` power within ±0.01 MW of golden file.
- [x] Add M19 to `.agents/project/orchestrator/milestone-scopes.md` (READ/WRITE + scoped CI) and update `.agents/project/orchestrator/doc-index.md`.

**DoD:** `go test ./internal/planner/...` passes; authenticated `GET /api/planner/catalog` returns parseable slim catalog; icon route serves PNG or 404; no planner UI yet.

---

## M20 — Factory Planner: Plans, Solver & Edit Lock

**Goal:** Persist factory plans in SQLite; greedy suggest solver; edit-lock concurrency; full REST surface except canvas — per proposal §5–§7. **Viewer write exception:** active viewers may create/edit **shared** plans when holding the lock (proposal §6; spec update in M21).

- [x] Migration `011_factory_plans.sql`: `factory_plans` table per proposal §5.3 (`graph_json`, `baseline_json`, `solver_options_json`, visibility, **status** lifecycle, edit-lock columns, indexes). Follow existing migration runner; `CREATE TABLE` only (no rebuild).
- [x] `backend/internal/planner/lock.go` — acquire (409 if held), heartbeat (~45 s client / **5 min** expiry), release, force-release (owner or admin); archived plans reject lock acquire and graph writes (403).
- [x] `backend/internal/planner/solver.go` — greedy recursive tree (not LP): target item + rate → machine counts at default clock/sloops; byproducts as extra output ports **without** auto-sink/edges; raw resources → `source` nodes; cycle detection → 400 with cycle ClassNames; depth-based x/y layout; **Plastic 60/min** fixture (3 refineries, 90 m³/min oil, HOR unterminated).
- [x] `backend/internal/api/planner_handlers.go` — full API per proposal §7.4: plan CRUD; list with default omit `archived`, `status` filter, `includeArchived`; `canEdit` / `canManage` / `lock` on list+detail; `PUT .../graph` optimistic concurrency (`updatedAt`); suggest preview vs apply (`apply-suggest` sets graph **and** baseline atomically); `reset-baseline`; optional `POST /api/planner/analyze`.
- [x] Authorization per proposal §6: owner/admin manage metadata without lock; admin lists others’ private plans; shared readable by all active users; graph/suggest/reset require lock + non-archived status.
- [x] Wire all routes under `/api/planner/...` with `RequireSession` + `RequireActiveUser` (not admin wrapper).
- [x] Tests: Iron Plate suggest fixture; cycle error; lock 409 + expiry steal + force-release; visibility (viewer cannot GET others’ private); graph PUT without lock → 409; archived → 403 on graph/lock/suggest; PATCH status to archived clears lock; admin private list.
- [x] Add M20 to `.agents/project/orchestrator/milestone-scopes.md` and `doc-index.md`.

**DoD:** Plan CRUD + suggest + reset + lock verifiable via API tests or curl; solver fixtures pass; no React Flow yet.

---

## M21 — Factory Planner: Canvas UI & Shipping

**Goal:** Hybrid node-graph editor (Suggest + freehand), client-side balance for instant clock/sloop UX, list page, nav, i18n, Docker data assets, spec/guide updates — per proposal §8–§10, §13 success criteria.

- [x] Dependencies: `@xyflow/react` (MIT, client-only `@xyflow/react/dist/style.css`), `@dagrejs/dagre` for suggest apply + optional re-layout. **Do not** use React Flow Pro.
- [x] `frontend/lib/planner/` — `graph-types.ts`, `catalog-types.ts`, `constants.ts` (`PLANNER_GRAPH_SAVE_DEBOUNCE_MS` default 800), `balance.ts` (mirror Go; shared golden fixtures with `balance.test.ts`), `layout.ts`, `to-react-flow.ts`; add planner types to `frontend/lib/api-types.ts`.
- [x] Routes: `/planner` (RSC list), `/planner/[id]` (RSC load plan → client editor). **Do not** extend read-only `research-tree-canvas.tsx`.
- [x] `frontend/components/planner/` — list (`Table`, create `Dialog`, status/visibility `Badge`, status filter default hides archived); editor (lock heartbeat 45 s, debounced save, read-only gate); toolbar (suggest, reset, layout, save status); lock banner; suggest dialog; node inspector (`Sheet`: recipe, clock 0–250, sloops, count); add-node popover; canvas (`dynamic(..., { ssr: false })`) with `process-node`, `source-node`, `sink-node`, `flow-edge` (rate + Mk + imbalance colors).
- [x] Canvas UX per proposal §8.6: typed handles (`out:Desc_*` / `in:Desc_*`), `isValidConnection` same itemClass; unterminated outputs / starved inputs on nodes; reset via `AlertDialog`; empty plan CTA (suggest or add node).
- [x] Nav: add **Planner** to `viewerItems` in `app-sidebar.tsx`; fix `NavMain` `isActive` for nested routes (`pathname.startsWith(url + "/")` when `url !== "/"`).
- [x] i18n: full `planner` namespace in `messages/en.json` (spec §8.2); game display names from catalog API only.
- [x] Spec updates: §2.1 React Flow exception; §3 `factory_plans`; §6 viewer planner write exception; §7 planner routes + §7.1/§7.2 shapes; §8 page inventory + §8.1 mapping.
- [x] Docker: copy `docs/FactoryGame-Docs.json` (or slim catalog) + `assets/icons/` + `assets/icons.json` into image; document env in `.env.example` / `docs/development.md`.
- [x] User guide: `docs/guide/planner.md` (short how-to when shipping).
- [x] Add M21 to `.agents/project/orchestrator/milestone-scopes.md` and `doc-index.md`.
- [x] Proposal status line: mark `docs/proposals/factory-planner.md` as on-roadmap (M19–M21) once M21 DoD met.

**DoD (proposal §13):** Suggest Iron Plate at fixture rate → draggable graph; Plastic 60/min → 3 refineries, HOR unconnected; clock/sloop on one node does not rewrite others — edges show under/over + Mk **and numeric rate**; reset restores last applied suggest; private/shared visibility + edit lock work; status filter hides archived by default, archived graph read-only; no hardcoded UI strings; `go test ./internal/planner/...`, Vitest `lib/planner/balance.test.ts`, `npm run lint` + `npm run build` pass.

---

## M14 — Deferred Backlog

Per **spec §10** — not v1:

- `hard_drive_ready` follow-up on recipe selection
- Additional notification providers (Provider interface §2.3 ready)
- Per-message-type polling cadence
- Live confirmation of elevator Phases 3–5 via `elevator_phase_unknown_log`
- Additional UI languages + locale switcher (i18n infra ready — §8.2, §10)
