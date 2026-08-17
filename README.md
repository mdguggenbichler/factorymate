# FactoryMate

Self-hosted Satisfactory server monitoring and Discord notification sidecar for the CBC group.

- **Spec:** [`docs/factorymate-spec.md`](docs/factorymate-spec.md) — schemas, APIs, UI, behavior
- **Roadmap:** [`docs/factorymate-roadmap.md`](docs/factorymate-roadmap.md) — milestone sequence M0–M14
- **Agents:** [`AGENTS.md`](AGENTS.md) — how Cursor agents should work in this repo

## Repository layout

```
backend/          Go service (chi, SQLite, FRM poller, REST API)
frontend/         Next.js App Router dashboard (shadcn/ui, next-intl)
docs/             Product spec, roadmap, vendored FRM API reference
.agents/           Orchestrator skill, prompt templates, milestone scopes
```

## Quick start (local dev)

1. Copy env template: `cp .env.example .env` and set `SESSION_SECRET`.
2. **Backend** (once M0+ is implemented): `cd backend && go run ./cmd/server`
3. **Frontend:** `cd frontend && npm install && npm run dev`
4. Open `http://localhost:3000`

See [`docs/development.md`](docs/development.md) for full setup and [`docs/testing.md`](docs/testing.md) for integration-test strategy (FRM fixtures, Discord webhook mocks).

**Live FRM (read-only):** `http://192.168.178.42:8889` — agents may `GET` the 12 polled endpoints for real example data. See [`.cursor/rules/04-frm-live.mdc`](.cursor/rules/04-frm-live.mdc).

## Milestone workflow

Development follows the roadmap **in order** (M0 → M13). Use the orchestrator skill (`.agents/skills/orchestrator/SKILL.md`) for autonomous milestone runs: execution sub-agent implements, verifier sub-agent checks, roadmap checkboxes updated on PASS.

M14 is a deferred backlog — not part of the autonomous loop.

## Verification

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm run lint && npm run build
```

CI runs the same gates on push to `main`.
