# FactoryMate — Spec doc index

Quick reference for orchestrator **DOC REFERENCE** blocks. Sub-agents read these files — never paste spec bodies into prompts.

## Precedence

1. `docs/factorymate-spec.md` — schemas, API, FRM integration, notifications, auth, pages
2. `docs/factorymate-roadmap.md` — milestone sequencing and DoD (does not redefine spec)
3. `docs/frm-docs/` — FRM API reference (validate against spec §4.1)

## Doc shorthand

| Shorthand | File | Covers |
| --- | --- | --- |
| `spec` | `docs/factorymate-spec.md` | Full product spec |
| `roadmap` | `docs/factorymate-roadmap.md` | Milestones M0–M14 |
| `frm` | `docs/frm-docs/modules/ROOT/pages/json/Read/` | FRM endpoint adocs (read-only vendored copy — see `docs/frm-docs-reference.md`) |
| `frm-ref` | `docs/frm-docs-reference.md` | What `frm-docs/` is and how to refresh it |
| `frm-live` | `.cursor/rules/04-frm-live.mdc` | Live FRM at `192.168.178.42:8889` — read-only GET for fixtures/tests |
| `defaults` | `backend/data/message_defaults.json` | Seeded notification templates |
| `scopes` | `.agents/project/orchestrator/milestone-scopes.md` | Per-milestone READ/WRITE + scoped CI |
| `testing` | `docs/testing.md` | FRM fixtures, Discord httptest mocks |

Reference sections as `§N`, e.g. `spec §4.2`, `spec §7.1`.

## Key spec sections

| Section | Topic |
| --- | --- |
| `spec §2.3` | Provider interface |
| `spec §2.4` | Frontend/backend wiring (two containers) |
| `spec §3` | SQLite schema (copy verbatim for migrations) |
| `spec §4.1` | FRM endpoints + client notes |
| `spec §4.1.1` | FRM → DB field mapping |
| `spec §4.2` | Diff / event detection |
| `spec §4.2.1` | Event variable population + history tables |
| `spec §5.4` | Templating (`{VarName}` syntax) |
| `spec §5.4.1` | Sample data for preview/validation |
| `spec §5.5` | Default templates JSON |
| `spec §6` | Auth + SQLite sessions |
| `spec §7` | REST API table + pagination/date rules |
| `spec §7.1` | Response JSON schemas |
| `spec §7.2` | Mutating request bodies |
| `spec §8` / `§8.1` | Frontend pages + shadcn mapping |
| `spec §8.2` | Frontend i18n (next-intl, `messages/en.json`, no hardcoded UI strings) |
| `spec §9` | Environment variables |

## Milestone → primary spec sections

| Milestone | Primary refs |
| --- | --- |
| M0 | spec §2.1, §2.4, §7 (`/healthz`), §8.2 (next-intl bootstrap) |
| M1 | spec §3, §5.2, §5.5, §9 |
| M2 | spec §4.1, §4.1.1, frm docs |
| M3 | spec §4.2, §4.2.1, roadmap M3.1 |
| M4 | spec §2.3, §5.1, §5.4 |
| M5 | spec §5.4, §5.4.1, defaults |
| M6 | spec §5.3, §3 `notification_log` |
| M7 | spec §6 |
| M8 | spec §7, §7.1, §7.2 |
| M9 | spec §4.1 (slow poll) |
| M10–M12 | spec §8, §8.1, **§8.2** |
| M13 | spec §2.4, §9 |
| M16 | spec §5.3, §7, discord-bot-plan §9 |
| M17 | spec §2.1, §3, §6, §7, §8, §9; discord-bot-plan OAuth |
| M18 | spec §3, §5.3, §7, §7.1, §7.2, §8, §8.1, §8.2; discord-bot-plan §9, §12.2 |

## Default verification (from project.config.md)

Orchestrator sub-agents: **scoped** `go test` / `npm run lint` for WRITE SCOPE packages before handoff.
