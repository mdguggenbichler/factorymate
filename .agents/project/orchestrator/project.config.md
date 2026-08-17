# Project config — FactoryMate

> Lives under `.agents/project/` — safe from `npx skills update` wiping skill directories.

## Repository

| Field | Value |
| ----- | ----- |
| Project name | FactoryMate |
| Repo path | /home/mdguggenbichler/projects/factorymate |
| Integration branch | `main` |
| Plan file | `docs/factorymate-roadmap.md` |
| Spec doc | `docs/factorymate-spec.md` |
| FRM docs (reference) | `docs/frm-docs/` |

## Milestone IDs

Roadmap milestones: `M0` … `M13` (autonomous loop). **M14** is deferred backlog — no checkboxes, not dispatched.

Commit subject pattern: `feat(M3): implement fast-poll diff engine` (conventional commits + milestone ref).

## Live vs fixture testing

See `docs/testing.md`. Summary for verifiers:

| Dependency | CI / autonomous PASS | Opt-in live |
| --- | --- | --- |
| Discord webhook | Go `httptest` mock server | `DISCORD_TEST_WEBHOOK_URL` |
| FRM API | JSON fixtures in `backend/testdata/frm/`; **live read-only** `http://192.168.178.42:8889` (rule: `.cursor/rules/04-frm-live.mdc`) | `go test -tags=integration` when `FRM_TEST_HOST` set |
| GuggiRaid deploy | `docker compose build` | Human smoke test at M13 DoD |

## Milestone scopes

Per-milestone READ/WRITE paths and scoped CI: `.agents/project/orchestrator/milestone-scopes.md`

Verifier checklist: `.agents/project/orchestrator/verifier-checklist.md`

## Checkbox authority

**Verifier only** marks roadmap checkboxes on PASS (`- [x]`) or FAIL (`- [!] failed — <reason>`). Granularity: **all task checkboxes under the milestone section** (not milestone-level summary only). Orchestrator does not edit checkboxes during execution.

## Verification commands

| Scope | Command |
| ----- | ------- |
| Backend (all) | `cd backend && go test ./... && go vet ./...` |
| Backend (package) | `cd backend && go test ./internal/poller/...` |
| Frontend (all) | `cd frontend && npm run lint && npm run build` |

Scoped gate: run tests for packages touched in WRITE SCOPE only.

## Optional

| Field | Value |
| ----- | ----- |
| Slack session-end | not configured |
| External issue tracker | none — roadmap checkboxes only |
