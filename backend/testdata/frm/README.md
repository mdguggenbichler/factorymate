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
| `getPlayer.json` | Live capture (trimmed) | One player |
| `getPower.json` | Live capture | Small; safe to commit |
| `getSchematics.json` | Live capture (trimmed) | One schematic with recipes |
| `getSpaceElevator.json` | Live capture | Phase/progress shape |
| `getResearchTrees.json` | Live capture (trimmed) | One tree |
| `getTrains.json` | Live capture | Empty array `[]` on this save |
| `getVehicles.json` | Live capture (trimmed) | Two tractors (`FuelInventory`, `Autopilot`) |
| `getProdStats.json` | Live capture (trimmed) | Three items |
| `getResourceSink.json` | Live capture | Sink status; `GraphPoints` as numbers |
| `getFactory.json` | Live capture (trimmed) | One assembler |
| `getDrone.json` | Live capture | Empty array `[]` on this save |
| `getDoggo.json` | Live capture | Inventory array |

Large full responses — query live when needed; committed fixtures are trimmed to one or few elements.

Captured: 2026-08-17 from `192.168.178.42:8889`.
