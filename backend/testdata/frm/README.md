# FRM JSON fixtures

Offline JSON samples for `internal/frm` and poller unit tests.

## Live server (preferred source for fresh captures)

| Field | Value |
| --- | --- |
| URL | `http://192.168.178.42:8889` |
| Access | **GET read endpoints only** — see `.cursor/rules/04-frm-live.mdc` |

```bash
# Example: refresh a fixture from live FRM
curl -sS "http://192.168.178.42:8889/getPower" -o backend/testdata/frm/getPower.json
```

Polled endpoints: spec §4.1 (12 endpoints). Do **not** call Write endpoints.

## Files in this directory

| File | Source | Notes |
| --- | --- | --- |
| `getPower.json` | Live capture | Small; safe to commit |
| `getSpaceElevator.json` | Live capture | Phase/progress shape |
| `getResourceSink.json` | Live capture | Sink status |
| `getDoggo.json` | Live capture | Inventory array |
| `getTrains.json` | Live capture | Empty array `[]` on this save |
| `getDrone.json` | Live capture | Empty array `[]` on this save |

Large endpoints (`getSchematics`, `getFactory`, `getResearchTrees`, `getPlayer`, `getProdStats`, `getVehicles`) — query live when needed; avoid committing full multi‑KB/MB dumps unless trimmed.

Captured: 2026-08-17 from `192.168.178.42:8889`.
