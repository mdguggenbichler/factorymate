# FRM docs reference (`docs/frm-docs`)

The `docs/frm-docs/` directory is a **read-only local copy** of the documentation from the [FicsIt Remote Monitoring (FRM)](https://github.com/porisius/FicsitRemoteMonitoring) repository — essentially the mod’s `docs/` tree (Antora/Asciidoc sources), vendored into FactoryMate for offline lookup.

**Do not edit files under `frm-docs/` in this repo.** They are reference material, not FactoryMate product docs. Integration decisions, schemas, and polling contracts live in `factorymate-spec.md` (especially §4.1). Use `frm-docs` to verify endpoint names, response fields, and FRM-specific behaviour when implementing the client (roadmap M2) or debugging parse mismatches.

## What it contains

| Path | Contents |
|------|----------|
| `modules/ROOT/pages/json/Read/` | Per-endpoint docs (`getPlayer.adoc`, `getPower.adoc`, …) |
| `modules/ROOT/pages/json/Write/` | Write endpoints (not used by FactoryMate v1) |
| `modules/ROOT/pages/json/Models/` | Shared response models (`_inventoryItem.adoc`, `_powerInfo.adoc`, …) |
| `modules/ROOT/pages/webserver.adoc`, `webhook.adoc`, `config/` | Web server setup, webhooks, configuration |
| `antora.yml` | Module metadata (`ficsitremotemonitoring`) |

Published docs (same content, rendered): [FicsIt Remote Monitoring on docs.ficsit.app](https://docs.ficsit.app/ficsitremotemonitoring/latest/index.html)

## Relationship to FactoryMate

- **Spec wins** on what FactoryMate polls, stores, and exposes — see `factorymate-spec.md` §4.1 and §4.1.1.
- **frm-docs wins** on raw FRM API shape when the spec points here or when validating structs against the adoc field tables and examples.
- FactoryMate uses a **subset** of FRM endpoints (12 read endpoints on fast + slow poll schedules). The rest of the adoc catalog exists in `frm-docs` but is intentionally out of scope for v1.

Orchestrator shorthand: `frm` → `docs/frm-docs/modules/ROOT/pages/json/Read/` (see `.agents/project/orchestrator/doc-index.md`).

## Updating this copy

When FRM releases change API fields or add endpoints:

1. Pull the latest `docs/` (or equivalent Antora module) from [porisius/FicsitRemoteMonitoring](https://github.com/porisius/FicsitRemoteMonitoring).
2. Replace the contents of `docs/frm-docs/` with that snapshot (or merge selectively).
3. Re-run any spec/roadmap validation if FactoryMate’s polled endpoints or field mappings need to change.

No submodule or automated sync in v1 — manual refresh when someone notices drift or before a major FRM upgrade on the game server.

## Format note

Sources are **AsciiDoc** (`.adoc`), not Markdown. Read them in the editor or render via Antora (`package.json` / `antora-playbook*.yml` are from the original docs build setup and are not used by FactoryMate’s own build).
