# Factory Planner

Design production chains as a node graph: buildings (process nodes), resource sources, optional sinks, and belt/pipe connections with live balance feedback.

## Quick start

1. Open **Planner** in the sidebar.
2. Click **New plan**, name it, choose **Private** or **Shared**.
3. Open the plan → **Start editing** to acquire the edit lock (others see read-only until you release or the lock expires).
4. **Suggest** — pick a target item and rate (items/min or m³/min for fluids). Apply replaces the graph with a solver-generated chain.
5. Or **Add building** — place a recipe node and wire handles manually (`out:Item` → `in:Item`, same item class only).

## Editing

- **Inspector** — click a node to change recipe (same building), machine count, clock (0–250%), Somersloops.
- **Connections** — edge labels show flow rate and recommended belt/pipe Mk; amber/orange/red indicates over/underproduction or exceeding max Mk.
- **Unterminated outputs / starved inputs** — amber handles on nodes when a port has no matching edge.
- **Re-layout** — runs Dagre left-to-right without changing recipes.
- **Reset to balanced** — restores the last applied Suggest snapshot (requires a baseline from Suggest apply).

Changes save automatically (~800 ms debounce) while you hold the lock.

## Sharing and status

| Visibility | Who can open |
| --- | --- |
| Private | Owner and admins |
| Shared | All active users |

| Status | Behavior |
| --- | --- |
| Planning / In progress / Completed | Editable with lock |
| Archived | Read-only; omitted from default list filter |

Owners and admins can change visibility, status, or delete plans from the list (metadata does not require the edit lock).

## Docker / data files

The backend loads game data from `PLANNER_CATALOG_PATH` (slim JSON) or `PLANNER_DOCS_PATH` (full dump). Icons are served from `PLANNER_ICONS_DIR`. See `.env.example` and `docs/development.md`.

Game item and recipe **display names** come from the catalog API — not from UI locale files.
