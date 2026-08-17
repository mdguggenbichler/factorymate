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

Roadmap milestones: `M0` … `M14` (checkboxes in plan file).

Commit subject pattern: `feat(M3): implement fast-poll diff engine` (conventional commits + milestone ref).

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
