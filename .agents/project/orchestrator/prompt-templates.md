# Prompt templates — FactoryMate orchestrator

> Copy blocks **verbatim** into sub-agent prompts. Fill `<placeholders>` per milestone.

## Execution prompt assembly

1. **Header** — SESSION-ID, MILESTONE, READ/WRITE scope (absolute paths)
2. **DOC REFERENCES** — paths + `spec §X` only (no pasted spec bodies)
3. **ACCEPTANCE** — verbatim from roadmap DoD + milestone tasks
4. **SCOPED CI GATE**
5. **HANDOFF** — commit message format

## Verifier prompt assembly

1. Same header + AC + DOC REFERENCES
2. **Three-layer verification** checklist
3. **SCOPED CI GATE**
4. On PASS: mark roadmap checkbox `- [x]` for this milestone only

---

## EXECUTION — template block

```text
SESSION-ID: <MILESTONE>-<YYYYMMDD>-<4hex>
MILESTONE: <M0–M14 title from roadmap>
LANE: S (serial on integration branch: main)

ACCEPTANCE (from roadmap DoD + tasks):
<paste milestone checkbox tasks and DoD bullets verbatim>

DOC REFERENCES (read yourself — do not expect pasted content):
- docs/factorymate-roadmap.md — <MILESTONE>
- docs/factorymate-spec.md — <list spec § sections from doc-index.md>
- .agents/project/orchestrator/doc-index.md

READ SCOPE:
- docs/factorymate-spec.md (reference only unless AC requires doc edit)
- <read paths for dependencies only>

WRITE SCOPE:
- <absolute paths for this milestone only>

FORBIDDEN:
- Work outside WRITE SCOPE
- Starting the next milestone
- git push (unless user explicitly asked)
- Editing roadmap checkboxes (verifier only)

SCOPED CI GATE (run before commit):
<from project.config.md — scoped to WRITE SCOPE packages>
Example:
  cd backend && go test ./internal/poller/...
  cd backend && go vet ./internal/poller/...

HANDOFF:
- One commit: <type>(<MILESTONE>): <imperative summary>
  Example: feat(M3): implement fast-poll diff engine
- Report: changed files, test output, any spec ambiguities
```

---

## VERIFIER — template block

```text
SESSION-ID: <same as execution>
MILESTONE: <M0–M14>
LANE: S

ACCEPTANCE: <same verbatim AC as execution>

READ SCOPE:
- Same as execution WRITE SCOPE (verify committed diff)
- docs/factorymate-roadmap.md — milestone section only

WRITE SCOPE:
- docs/factorymate-roadmap.md — change only this milestone's `- [ ]` → `- [x]` or `- [!]` on PASS/FAIL

SCOPED CI GATE: <same scoped commands as execution>

THREE-LAYER VERIFICATION:

Layer 1 — Scope audit: committed paths ⊆ execution WRITE SCOPE (+ roadmap checkbox on PASS only).

Layer 2 — Scoped automated checks from project.config.md. Mark n/a only if no code changed.

Layer 3 — Logic review:
  3a. Each acceptance criterion / DoD bullet met?
  3b. Spec contract deviations with file:line + fix hint (spec § refs)
  3c. Frontend milestones (M0/M10–M12): no hardcoded user-facing UI strings — all via `messages/en.json` per spec §8.2 and `.cursor/rules/03-i18n.mdc`
  3d. Migrations: numbered .sql only — hand-written SQL outside migrations → FAIL

RESULT:
- PASS → set milestone checkboxes to `- [x]` in roadmap; report PASS summary
- FAIL → set `- [!]` with reason; report FAIL with fix hints for re-dispatch
```

---

## SCOPED CI GATE — backend

```text
SCOPED CI GATE:
Before commit, from repo root:
  cd backend && go vet ./<package>/...
  cd backend && go test ./<package>/...
Replace <package> with paths under WRITE SCOPE (e.g. internal/poller).
If WRITE SCOPE includes multiple packages, test each.
If only docs changed, gate is n/a — state n/a in report.
```

## SCOPED CI GATE — frontend

```text
SCOPED CI GATE:
  cd frontend && npm run lint
  cd frontend && npm run build
Run only when frontend/ is in WRITE SCOPE.
```

## SCOPED CI GATE — full stack milestone

```text
SCOPED CI GATE:
  cd backend && go test ./... && go vet ./...
  cd frontend && npm run lint && npm run build
```
