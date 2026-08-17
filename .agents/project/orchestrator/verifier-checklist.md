# Verifier checklist — FactoryMate

Quick reference for verifier sub-agents. Full template: `prompt-templates.md` VERIFIER block.

## Three layers (all required)

### Layer 1 — Scope audit

- [ ] Every committed file path ⊆ execution WRITE SCOPE (from `milestone-scopes.md`)
- [ ] No unrelated refactors or drive-by fixes
- [ ] Roadmap edited only by verifier (checkbox updates on PASS/FAIL)

### Layer 2 — Scoped CI

- [ ] Run commands from `milestone-scopes.md` for this milestone
- [ ] All commands exit 0
- [ ] Mark `n/a` only if milestone is docs-only with no code gate

### Layer 3 — Logic review

- [ ] Each roadmap task checkbox under this milestone is satisfied
- [ ] DoD bullets met
- [ ] Spec contract deviations listed with `file:line` + fix hint + `spec §` ref
- [ ] **Frontend (M0/M10–M12):** no hardcoded user-facing strings — all via `messages/en.json`
- [ ] **Migrations (M1+):** DDL only in numbered `.sql` under `internal/db/migrations/`
- [ ] **M4/M6:** Discord tests use httptest mock (see `docs/testing.md`) — real webhook not required

## Checkbox updates

On **PASS:** change every `- [ ]` under this milestone's task list to `- [x]`. Do not mark other milestones.

On **FAIL:** change them to `- [!] failed — <one-line reason>`.

To retry after FAIL: orchestrator re-dispatches execution; verifier clears `[!]` back to `[ ]` or leaves until PASS replaces with `[x]`.

## Common FAIL reasons

| Symptom | Typical fix |
| --- | --- |
| Hardcoded JSX string | Move to `messages/en.json`, use `t()` |
| Migration SQL outside `migrations/` | Move to next numbered file |
| shadcn component hand-written | Use MCP `shadcn add` instead |
| M2/M4 requires live Discord/FRM | Use fixtures/httptest per `docs/testing.md` |
| Scope creep into next milestone | Re-dispatch with tighter WRITE SCOPE |
