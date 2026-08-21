# Milestone scopes — FactoryMate

Per-milestone **READ** and **WRITE** scopes for orchestrator dispatch. Fill these into execution/verifier prompts verbatim.

**Rules:**

- M3.1 is bundled with M3 (same dispatch, same verifier PASS).
- Verifier on PASS marks **all** `- [ ]` task checkboxes under that milestone section `- [x]`.
- Verifier on FAIL marks them `- [!] failed — <reason>`.
- M14 has no checkboxes — excluded from autonomous loop.

---

## M0 — Project Scaffolding

**READ:** `docs/factorymate-spec.md` §2.1, §2.4, §7 (`/healthz`), §8.2, §9; `docs/factorymate-roadmap.md` M0; existing `frontend/` (do not duplicate shadcn work already done).

**WRITE:**

```
backend/go.mod
backend/cmd/server/main.go
backend/internal/db/.gitkeep          # empty package stub
backend/internal/frm/.gitkeep
backend/internal/poller/.gitkeep
backend/internal/notify/.gitkeep
backend/internal/template/.gitkeep
backend/internal/api/.gitkeep
backend/internal/auth/.gitkeep
backend/data/                           # message_defaults.json already exists
docker-compose.yml
frontend/next.config.ts                 # API rewrite only if not already correct
frontend/messages/en.json               # only if missing/incomplete
frontend/i18n/                          # next-intl config if missing
frontend/app/layout.tsx                 # i18n provider if missing
frontend/app/page.tsx                   # stub using t()
.env.example                            # if env vars added
docs/development.md                     # if run instructions change
```

**Scoped CI:**

```bash
cd backend && go build ./... && go vet ./...
cd frontend && npm run lint && npm run build
```

**Notes:** Frontend shadcn init and §8.1 component install are **already done** (see roadmap checkboxes). M0 must not reinstall shadcn blocks. Backend `GET /healthz` returns `{"status":"ok"}`.

---

## M1 — Database Layer

**READ:** spec §3, §5.2, §5.5, §9; `backend/data/message_defaults.json`.

**WRITE:**

```
backend/internal/db/**
backend/cmd/server/main.go              # wire migration runner on startup
backend/data/                           # no schema changes
```

**Scoped CI:**

```bash
cd backend && go test ./internal/db/... && go vet ./internal/db/...
```

---

## M2 — FRM Client

**READ:** spec §4.1, §4.1.1; `docs/frm-docs/`; `docs/testing.md`.

**WRITE:**

```
backend/internal/frm/**
backend/testdata/frm/**                 # JSON fixtures for unit tests
```

**Scoped CI:**

```bash
cd backend && go test ./internal/frm/... && go vet ./internal/frm/...
```

**Live test:** `go test -tags=integration ./internal/frm/...` only when `FRM_TEST_HOST` set — not required for verifier PASS.

---

## M3 — Poller / Diff Engine (includes M3.1)

**READ:** spec §4.2, §4.2.1; roadmap M3 + M3.1.

**WRITE:**

```
backend/internal/poller/**
backend/internal/db/**                  # queries only if needed
backend/data/elevator_phases.json
backend/testdata/frm/**                 # poll fixtures
backend/cmd/server/main.go              # wire poller if needed
```

**Scoped CI:**

```bash
cd backend && go test ./internal/poller/... && go vet ./internal/poller/...
```

---

## M4 — Notification Provider + Discord

**READ:** spec §2.3, §5.1, §5.4; `docs/testing.md`.

**WRITE:**

```
backend/internal/notify/**
```

**Scoped CI:**

```bash
cd backend && go test ./internal/notify/... && go vet ./internal/notify/...
```

**DoD:** httptest mock webhook satisfies "reaches Discord correctly formatted" — no real channel required.

---

## M5 — Templating Engine

**READ:** spec §5.4, §5.4.1, §5.5; `backend/data/message_defaults.json`.

**WRITE:**

```
backend/internal/template/**
```

**Scoped CI:**

```bash
cd backend && go test ./internal/template/... && go vet ./internal/template/...
```

---

## M6 — Dispatch Wiring

**READ:** spec §5.3, §3 `notification_log`; M3/M4/M5 code (read only).

**WRITE:**

```
backend/internal/poller/**              # wire dispatch into poll loop
backend/internal/notify/**              # if dispatch helpers live here
backend/cmd/server/main.go              # if wiring only
```

**Scoped CI:**

```bash
cd backend && go test ./internal/poller/... ./internal/notify/... && go vet ./...
```

---

## M7 — Auth

**READ:** spec §6.

**WRITE:**

```
backend/internal/auth/**
backend/internal/api/**                 # auth handlers only
backend/cmd/server/main.go              # middleware wiring
```

**Scoped CI:**

```bash
cd backend && go test ./internal/auth/... ./internal/api/... && go vet ./...
```

---

## M8 — REST API

**READ:** spec §7, §7.1, §7.2; all `internal/*` packages (read).

**WRITE:**

```
backend/internal/api/**
backend/cmd/server/main.go
```

**Scoped CI:**

```bash
cd backend && go test ./internal/api/... && go vet ./internal/api/...
```

---

## M9 — Slow Poll

**READ:** spec §4.1 (slow poll, retention, building_type mapping).

**WRITE:**

```
backend/internal/poller/**              # slow poll loop
backend/internal/frm/**                 # if slow structs extended
backend/testdata/frm/getFactory.json    # capture before implementing building_type
```

**Scoped CI:**

```bash
cd backend && go test ./internal/poller/... && go vet ./internal/poller/...
```

---

## M10 — Frontend Scaffolding & Shell

**READ:** spec §8, §8.1, §8.2, §2.4, §6; existing `frontend/components/**` (shadcn already installed).

**WRITE:**

```
frontend/app/**
frontend/components/**                  # shell, auth forms — repurpose demo blocks
frontend/messages/en.json
frontend/lib/**
frontend/middleware.ts                  # if auth/i18n routing needed
```

**Scoped CI:**

```bash
cd frontend && npm run lint && npm run build
```

**Notes:** Remove or repurpose demo routes (`/dashboard` demo, demo nav). Do **not** re-run bulk `shadcn add` — components already installed.

---

## M11 — Viewer Pages

**READ:** spec §8, §8.1, §8.3 (acceptance criteria), §7.1, §8.2.

**WRITE:**

```
frontend/app/**
frontend/components/**
frontend/messages/en.json
frontend/lib/**
```

**Scoped CI:**

```bash
cd frontend && npm run lint && npm run build
```

---

## M16 — Notifications polish & admin commands

(See roadmap — verified.)

---

## M17 — Discord SSO & secure onboarding

**READ:** spec §2.1, §3, §6, §7, §7.1, §7.2, §8, §8.1, §8.2, §9; `docs/discord-bot-plan.md` §6, command tables, Appendix G; plan `web_password_onboarding`; existing auth/registration/discord handlers.

**WRITE:**

```
docs/factorymate-roadmap.md              # M17 section only (unchecked)
docs/factorymate-spec.md
docs/discord-bot-plan.md
docs/guide/discord/commands.md
docs/guide/managing/users.md
docs/guide/first-run.md
docs/guide/discord/configuration.md
docs/development.md
.env.example
docker-compose.yml
.agents/project/orchestrator/milestone-scopes.md   # this file — M17 block
backend/internal/db/migrations/008_*.sql
backend/internal/auth/**
backend/internal/registration/**
backend/internal/discord/**
backend/internal/api/**
backend/cmd/server/main.go
frontend/app/(auth)/**
frontend/app/(app)/account/**
frontend/components/**
frontend/lib/**
frontend/messages/en.json
```

**Scoped CI:**

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm run lint && npm run build
```

**Notes:** OAuth uses same Discord app as bot (`DISCORD_CLIENT_SECRET` + bot token app ID). `FACTORYMATE_PUBLIC_URL` required for redirect URI. Do not mark roadmap checkboxes `[x]` — verifier only.

---

## M12 — Admin Settings Pages

**READ:** spec §8, §8.1 (template editor gaps), §7.2, §8.2.

**WRITE:**

```
frontend/app/**
frontend/components/**
frontend/messages/en.json
frontend/package.json                   # react-hook-form if added
```

**Scoped CI:**

```bash
cd frontend && npm run lint && npm run build
```

---

## M13 — Docker Packaging & Deployment

**READ:** spec §2.4, §9; `docs/development.md`.

**WRITE:**

```
Dockerfile
scripts/docker-entrypoint.sh
.dockerignore
docker-compose.yml
docs/development.md
```

**Scoped CI:**

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm run lint && npm run build
docker compose build
```

**DoD:** Human deploy to GuggiRaid is manual verification — not required for autonomous verifier PASS if `docker compose build` succeeds.

---

## M18 — Per-type DM prefs & two-layer notification UX

**READ:** spec §3, §5.3, §7, §7.1, §7.2, §8, §8.1, §8.2; `docs/discord-bot-plan.md` §9, §11.2, §12.2; `docs/guide/managing/notifications.md`; existing M16 prefs/dispatch/discord/frontend notification pages; `backend/internal/db/migrations/` (next number).

**WRITE:**

```
docs/factorymate-roadmap.md              # M18 section only (unchecked)
docs/factorymate-spec.md
docs/discord-bot-plan.md
docs/guide/managing/notifications.md
docs/guide/managing/settings.md
docs/guide/discord/commands.md
docs/testing.md
.agents/project/orchestrator/milestone-scopes.md   # this file — M18 block
.agents/project/orchestrator/doc-index.md
backend/internal/db/migrations/**
backend/internal/notifications/**
backend/internal/notify/**
backend/internal/discord/**
backend/internal/api/**
backend/internal/registration/**         # only if SeedUserPrefs signatures change
frontend/app/**
frontend/components/**
frontend/lib/**
frontend/messages/en.json
```

**Scoped CI:**

```bash
cd backend && go test ./internal/notifications/... ./internal/notify/... ./internal/discord/... ./internal/api/... && go vet ./internal/notifications/... ./internal/notify/... ./internal/discord/... ./internal/api/...
cd frontend && npm run lint && npm run build
```

If `backend/internal/registration` was changed, include it in go test/vet.

**Notes:** Do not add per-type Discord slash toggles. Do not put prefs URL on game-event DM footers. Do not mark roadmap checkboxes `[x]` — verifier only.

---

## M19 — Factory Planner: Game Data & Catalog

**READ:** `docs/proposals/factory-planner.md` §4, §7.1, §10; `docs/factorymate-spec.md` §6 (session auth); `docs/FactoryGame-Docs.json`; `assets/icons.json`, `assets/icons/`; `backend/internal/api/` (route patterns); `backend/internal/auth/`; `backend/cmd/server/main.go`.

**WRITE:**

```
backend/go.mod
backend/internal/planner/**
backend/data/factory_catalog.json          # optional generated slim catalog
backend/cmd/generate-catalog/**
backend/testdata/planner/**                   # includes committed factory_catalog.json slim file
backend/internal/api/**                      # planner catalog + icon handlers, routes only
backend/cmd/server/main.go                   # catalog wiring if needed
.env.example                                 # PLANNER_* env vars
.agents/project/orchestrator/milestone-scopes.md
.agents/project/orchestrator/doc-index.md
```

**Scoped CI:**

```bash
cd backend && go vet ./internal/planner/... ./internal/api/...
cd backend && go test ./internal/planner/... ./internal/api/...
```

Regenerate slim catalog when `docs/FactoryGame-Docs.json` changes:

```bash
cd backend && go run ./cmd/generate-catalog
```

**Notes:** Does not touch FRM poller, Discord, notification pipeline, or frontend planner UI. Verifier marks roadmap checkboxes — execution agents do not.

---

## M20 — Factory Planner: Plans, Solver & Edit Lock

**READ:** `docs/proposals/factory-planner.md` §5–§7; `docs/factorymate-spec.md` §3, §6, §7; `backend/internal/planner/**` (M19 catalog/balance — read only); `backend/internal/api/**` (auth patterns); `backend/internal/db/**` (migration runner); `backend/internal/auth/**`.

**WRITE:**

```
backend/internal/db/migrations/011_*.sql
backend/internal/planner/lock.go
backend/internal/planner/solver.go
backend/internal/planner/**                    # lock/solver tests only
backend/internal/api/planner_handlers.go
backend/internal/api/routes.go
backend/internal/api/**                        # API tests only
backend/cmd/server/main.go                     # if wiring needed
.agents/project/orchestrator/milestone-scopes.md
.agents/project/orchestrator/doc-index.md
```

**Scoped CI:**

```bash
cd backend && go vet ./internal/planner/... ./internal/api/... ./internal/db/...
cd backend && go test ./internal/planner/... ./internal/api/... ./internal/db/...
```

**Notes:** No React Flow / frontend planner UI (M21). Verifier marks roadmap checkboxes — execution agents do not.

---
