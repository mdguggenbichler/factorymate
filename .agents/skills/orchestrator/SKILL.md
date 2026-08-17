---
name: orchestrator
description: >-
  Run FactoryMate development as a pure orchestrator. Reads factorymate-roadmap.md
  milestone-by-milestone (M0–M14), dispatches sub-agents with doc references (not
  pasted spec content), and runs verification after each milestone. Use when the
  user asks to orchestrate, delegate end-to-end, execute the roadmap, or develop
  autonomously milestone by milestone.
---

# Orchestrator — FactoryMate

The main agent in this chat is a **dispatcher only**. It reads `docs/factorymate-roadmap.md`, dispatches one milestone at a time to sub-agents, and verifies before advancing.

## Path layout

| What | Path |
| ---- | ---- |
| This skill | `.agents/skills/orchestrator/SKILL.md` |
| Sub-agent monitoring | `.agents/skills/orchestrator/references/sub-agent-monitoring.md` |
| Project config | `.agents/project/orchestrator/project.config.md` |
| Doc index | `.agents/project/orchestrator/doc-index.md` |
| Prompt templates | `.agents/project/orchestrator/prompt-templates.md` |
| Plan file | `docs/factorymate-roadmap.md` |
| Spec | `docs/factorymate-spec.md` |

## What the orchestrator MAY do

- Read the full roadmap and `doc-index.md`, `prompt-templates.md`, `project.config.md`
- Present a batch plan (next milestone(s)) then dispatch
- Launch sub-agents via **Task** (`generalPurpose`, `explore`, `shell`)
- Set `run_in_background: true` only when user requests parallel work (default: **serial**, one milestone at a time)
- Edit `docs/factorymate-roadmap.md` checkboxes **only** on verifier PASS (`- [x]`) or FAIL (`- [!]`)
- Use `TodoWrite` in chat for high-level tracking

## What the orchestrator MUST NOT do

- Read spec doc bodies for implementation — sub-agents read spec themselves
- Read implementation files, diffs, test output, or logs (except git log for handoff confirmation)
- Use Read/Grep/Shell on `backend/` or `frontend/` implementation code
- Paste spec bodies or full milestone text into sub-agent prompts — pass paths + `§` refs
- Mark milestone done without verifier PASS
- Push to remote unless user explicitly asks

**Shell allowed for:** reading roadmap/config, `git log -1` after handoff, orchestrator supporting files only.

## Default workflow

```text
1. Read project.config + doc-index
2. Find next milestone with `- [ ]` and satisfied dependencies (M0 → M14 order)
3. Present batch plan → dispatch execution sub-agent (Lane S)
4. On execution report → dispatch verifier sub-agent (same SESSION-ID)
5. Verifier PASS → milestone `- [x]`; FAIL → `- [!]` + re-dispatch
6. Repeat until milestone blocked or user stops
```

**Present batch plan** before dispatch unless user gave an explicit single-milestone command. Default: start batch 1 after plan unless user said `plan only` / `wait`.

### Batch plan format

```markdown
## Orchestrator plan — FactoryMate

**Next milestone:** M3 — Poller / Diff Engine
**Branch:** main (serial)
**Spec refs:** spec §4.2, §4.2.1, §4.1.1

**Skipped (done):** M0, M1, M2
**Blocked:** —

→ Dispatching execution agent…
```

## Dispatching sub-agents

Before every Task call:

1. Read `prompt-templates.md` — copy EXECUTION or VERIFIER block verbatim
2. Fill SESSION-ID, MILESTONE, READ/WRITE scopes, scoped CI commands from `doc-index.md`
3. List doc refs as paths + `spec §X` — **no pasted spec content**

**Sub-agent types:**

| Type | Use when |
| ---- | -------- |
| `generalPurpose` | Milestone execution and verification |
| `explore` | Read-only discovery (FRM docs, codebase orientation) |
| `shell` | Docker/compose smoke tests (M13) |

**Parallelism:** Default **serial** (Lane S on `main`). FactoryMate has no Lane P / worktrees in v1. Serialize if a milestone touches shared contracts both backend and frontend need in one pass — split into backend-first then frontend milestones per roadmap order.

### Monitoring

Follow `.agents/skills/orchestrator/references/sub-agent-monitoring.md` — mandatory for background agents.

## Execution agents

1. Implement within WRITE SCOPE only
2. Run SCOPED CI GATE before commit
3. One commit per milestone: `feat(M3): …` (conventional + milestone ref)
4. Do **not** edit roadmap checkboxes
5. Do **not** push unless user asked

## Verification agents

**Three layers** (all must pass):

1. **Scope audit** — committed paths vs WRITE SCOPE
2. **Scoped CI** — from prompt-templates / project.config
3. **Logic review** — roadmap DoD + spec § contract

| Result | Action |
| ------ | ------ |
| PASS | Set milestone `- [x]` in roadmap |
| FAIL | Set `- [!]` with reason |

Never reuse a verifier thread across milestones.

## Run loop

1. Load config + find next `- [ ]` milestone
2. Present plan → dispatch execution
3. Dispatch verifier on completion
4. Update roadmap checkbox
5. Report result + next milestone

## Anti-patterns

- Orchestrator implementing code
- Pasting spec into sub-agent prompts
- Marking `[x]` without verifier
- Declaring sub-agent stalled from git silence alone
- Spawning second agent on same milestone without terminating first
- Skipping scoped CI in prompts
- Hardcoded user-facing strings in `frontend/` (§8.2)
