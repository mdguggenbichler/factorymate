# Proposal: Satisfactory Factory Calculator / Planner

**Status:** on-roadmap — implemented as **M19–M21** in `docs/factorymate-roadmap.md`.  
**Related:** spec §2.1 (stack), §3 (SQLite + numbered migrations), §6 (admin/viewer), §7 (REST + camelCase JSON), §8 / §8.1 / §8.2 (pages, shadcn, i18n); `backend/internal/db/migrate.go`; existing custom canvas at `frontend/components/research/research-tree-canvas.tsx` (read-only — not a starting point for this feature).

FactoryMate today is a **live-server sidecar** (FRM poller + Discord + dashboard). This feature is a **second product surface** in the same app: an offline production-chain planner that uses Coffee Stain’s recipe/building dump, not FRM factory state. Interaction should match what players know from the [wiki planner list](https://satisfactory.wiki.gg/wiki/Online_tools) (Suggest like Satisfactory Tools / SC simple mode; canvas like Ferrumium Planner), while avoiding calculator-only UIs, auto-rewriting solvers, and edges that hide Mk and rate (§2).

---

## 1. Goals and non-goals

### Goals

- Players build production chains as a **node graph**: a node is a recipe running in a building (with a machine count); an edge is item (or fluid) flow between ports.
- **Hybrid editing:** auto-suggest from a target item + rate **and** freehand add/connect at any time. Solver output and manual nodes share **one** graph model — no “solver node” vs “user node” type.
- **Overrides** on any node: clock speed **0–250%**, Somersloop count **0…building max**. Changing them must **not** auto-rewire or auto-rescale the rest of the graph.
- **Over-/underproduction** shown on the **affected connections** (color + numbers). Never auto-corrected.
- **Reset to balanced** restores the last solver snapshot (see §5.4).
- **Belt / pipe Mk recommendation** on every connection, including the **actual transfer rate** (items/min or m³/min), not Mk alone.
- Plans **persist** in SQLite, reopen later, **private** or **shared** among FactoryMate users.
- Shared-plan concurrency: **edit lock** (one writer, others read-only until release or timeout). Not CRDT / WebSocket collab.

### Non-goals (v1 of this feature)

- Importing the live FRM `getFactory` layout or syncing to the dedicated-server save.
- 3D / isometric factory modeling (Satisfactory Modeler / SaLT territory).
- First-class splitter/merger/manifold geometry (fan-in / fan-out on item ports is enough; buildings stay “logical,” not foundation-accurate).
- Linear-programming “optimal alternate mix” / weighted mega-base solvers (FactorioLab, YAFP, SFTools HiGHS). Default recipe walk + user recipe picks.
- Power-grid simulation beyond **summed building power** (clock + Somersloop formulas).
- Multi-factory logistics networks, train/truck export calculators, Discover/community publishing (Satisfactory Factories, logistics.xyz, Ferrumium social).
- AI prompt-to-layout (SatisfiedVisual2).
- Real-time multi-user cursors.
- Public internet sharing / unauthenticated links (this stays behind FactoryMate sessions).

---

## 2. Existing tools (wiki list) — source, UX, what we take

The [wiki Online tools](https://satisfactory.wiki.gg/wiki/Online_tools) list is the right survey. These products fall into three jobs:

1. **Production calculator** — target item + rate → machine counts / raws / power (tree or table). Fast to start, weak as a factory you can poke.
2. **Node-graph / visual planner** — place buildings, draw belts/pipes, inspect flow. Familiar in-game metaphor, often unusable at scale or missing Mk/clock/sloop.
3. **Macro logistics** — several factories, I/O contracts, trains. Out of scope for us (one dedicated-server group already has a live world).

**Closest product analog to our hybrid:** Ferrumium’s split of **Drafts** (target → generated plan) vs **Planner** (place/connect/inspect), except we keep **one graph** so a draft is just nodes you can immediately drag. Manifolder’s “calculator mode vs layout mode” is the same split; we should not make the player pick a mode.

**Do not copy code** from these projects. Several are source-available under licenses that would infect FactoryMate (AGPL) or that forbid reuse. Steal **interaction conventions** and **feature gaps**, not implementations.

### 2.1 Per-tool notes

| Tool | Source? | What it is | Take / skip for FactoryMate |
|---|---|---|---|
| [Satisfactory Tools](https://www.satisfactorytools.com) | **Yes (partial).** Classic: [greeny/SatisfactoryTools](https://github.com/greeny/SatisfactoryTools) (PHP + TS, custom license — game assets copyrighted). Next-gen: [SatisfactoryTools/SFTools](https://github.com/SatisfactoryTools/SFTools) (Angular, **AntV X6** graph, **HiGHS LP** solver). | The default “pick item, get a chain” players know. New graph is a planner overlay on an LP, not a freeform factory. | **Take:** suggest dialog, alternate toggles, codex-like recipe names. **Skip:** HiGHS/LP, Angular, X6 (we stay on React). |
| [Satisfactory Calculator production planner](https://satisfactory-calculator.com/en/production-planner) | **Source visible, reuse restricted.** [AnthorNet/SC-ProductionPlanner](https://github.com/AnthorNet/SC-ProductionPlanner) is public; the related [SCIM map](https://github.com/AnthorNet/SC-InteractiveMap) README forbids reuse/deploy. Treat SCPP the same: **read, don’t vendor**. | Wiki: **simple mode** = calculator tree; simple mode **off** = micro/spreadsheet planning. Extremely well known; diagram UX is not the selling point. | **Take:** simple-mode mental model for Suggest (item, rate, recipe list). **Skip:** their canvas; public blueprint library; save-file map. |
| [Calculatory](https://calculatory.ovh/) | **No public repo found.** | Minimal “what buildings do I need.” | **Take:** Suggest dialog should be this simple on first open. **Skip:** stopping at a static tree. |
| [FactorioLab / Satisfactory](https://factoriolab.github.io/satisfactory) | **Yes, MIT.** [factoriolab/factoriolab](https://github.com/factoriolab/factoriolab) (Angular + Redux). Kirk McDonald lineage. Wiki still tags U6/7/8 in places — verify game version before trusting numbers. | **List / Flow (Sankey) / Data** views — a calculator with visualization, **not** a building graph you wire by hand. Belt/pipe Mk as **display settings**, optimization weights, surplus machines. | **Take:** summary columns (machines, power, belts) in an inspector/sidebar; optional later Sankey is nice-to-have. **Skip:** Angular, full LP objective UI, wagon/train columns. |
| [v1p1.satisfactory.dev](https://v1p1.satisfactory.dev/) | **Guts yes, UI no.** [@satisfactory-dev/docs.json.ts](https://github.com/satisfactory-dev/Docs.json.ts) (Apache-2.0) + [Satisfactory-Production-Calculator](https://github.com/satisfactory-dev/Satisfactory-Production-Calculator). [UI repo](https://github.com/satisfactory-dev/Satisfactory-Production-Calculator-UI) is **issue tracker only**. | Typed Docs.json → planner. Same dump we already vendor as `docs/FactoryGame-Docs.json`. | **Take:** Unreal `ItemClass`/`Amount` parsing ideas and fluid scaling tests (reimplement in **Go**). **Skip:** npm in the Next app; we do not add a TS catalog duplicate. |
| [satisfactory-logistics.xyz](https://satisfactory-logistics.xyz) | **No public repo found.** | Calculator **plus** logistics between factory I/O; 1.0 Somersloops. | **Take:** sloops as a first-class rate multiplier. **Skip:** inter-factory tracking (v1 is one graph). |
| [Satisfactory Factories](https://satisfactory-factories.app) | **Yes, AGPL-3.0.** [satisfactory-factories/application](https://github.com/satisfactory-factories/application) (Vue + TS). **Do not copy** into this MIT/unspecified FactoryMate tree. | Macro chain: bottlenecks between factories, clock/sloop, building **groups**, Quantum Converter, export-rate (trains/trucks). Closest “highlight the problem, don’t silently fix it.” | **Take:** bottleneck highlighting on connections; clock + sloop on lines; recipe picker that can **inject** a product into the plan. **Skip:** multi-factory manager, export-rate calculator, Vue, any AGPL source. |
| [satisfactoryplanner.net](https://satisfactoryplanner.net) | **No public repo found.** Marketing visual designer (drag buildings, belt colors, 1–250% shards, Mk.1–6). **Not** the same as YAFP’s old `satisfactory-planner.net` hostname. | Closest **marketing** match to our canvas (green/yellow/red belts). Quality of the actual graph is the usual risk (pretty landing page ≠ usable editor). | **Take:** edge color + Mk. **Verify** in the live app that rate is shown, not only color. **Skip:** cloning their UI chrome or i18n set (we have next-intl `en` only). |
| [SatisfiedVisual2](https://madeinnewyork87.github.io/SatisfiedVisual2/SatisfiedPlanner2.html) | **Likely the GitHub Pages tree** under that user; no well-known library repo. Treat as closed unless we confirm a license file. | Node-based DnD, **auto-balancing**, auto-build, **AI Architect**, 1.0 sloops/SAM/alts, blueprint batch sizes. | **Take:** drag-and-drop node graph, sloops, alts. **Skip:** auto-rebalance on every edit (conflicts with our override highlighting); AI; blueprint modules. |
| [Ferrumium](https://ferrumium.com) | **No public repo found.** | **Planner** (place, connect belts/pipes, groups, inspect flow) + **Drafts** (target + recipe rules → generated plan → promote to factory) + splitter-ratio tools + public Discover. | **Take:** Drafts ≡ Suggest; Planner ≡ canvas; inspect-how-materials-move. **Skip:** community publish, splitter-tree utility pages (link a wiki later). |
| [Manifolder](https://manifolder.app/) | **No public repo found.** | Calculator (divide lines, **modules**) vs **layout mode** (drag buildings). | **Take:** modules later as optional grouping. **Skip:** two separate apps/modes. |
| Yet Another Factory Planner | **Yes.** [lunafoxfire/yet-another-factory-planner](https://github.com/lunafoxfire/yet-another-factory-planner); maintained fork [Greven145/…](https://github.com/Greven145/yet-another-factory-planner). Live often at [yafp.game.gottselig.ca](https://yafp.game.gottselig.ca/). In-browser solver, URL-encoded factories, **no account**. | Weighted optimization (resource vs power vs building count), belt/pipe **caps**, recipe copies vs items/min vs max-from-raws, nuclear recipes, underclock copy-from-recipe, 1.2 cost/power multipliers. **Not a node editor.** | **Take:** power + build-cost summary sidebar; belt/pipe cap as a **display/warning** (we already want Mk). **Skip:** WASM/LP solver, URL-as-database, maximize-from-world-nodes, mobile-first. |

Related open visual planners (not on your list, but they confirm the canvas stack): several community apps use **`@xyflow/react`** (e.g. React Flow Satisfactory clones). SFTools went **AntV X6** instead. FactorioLab uses **Sankey**, not nodes. That spread is why we pick React Flow for **our** Next.js app, not because every Satisfactory tool uses it.

### 2.2 Conventions players already know (adopt)

| Convention | Where it shows up | FactoryMate |
|---|---|---|
| Target item + items/min → generate a chain | Tools, SC simple, Calculatory, Ferrumium Drafts, v1p1 | Suggest dialog |
| One node ≈ **recipe + building + machine count** | Almost every calculator | Yes — not one sprite per constructor |
| Alternate recipes as per-product choices | All serious calculators | `recipeByProductClass` + inspector |
| Clock 1–250% and Somersloops | Factories.app, logistics.xyz, SatisfiedVisual2, Ferrumium, planner.net | First-class node fields |
| Bottlenecks **highlighted**, not auto-fixed | Factories.app; our explicit requirement | Edge color + numbers |
| Ports / connect matching items | Visual planners, Ferrumium | Typed React Flow handles |
| Left-to-right raw → product, then free drag | Visual planners | Dagre on suggest only |
| Summary: power, raws, building counts | FactorioLab list, YAFP report, Tools | Inspector + plan header |
| Source nodes for miners / water with a rate | Tools, visual planners | `role: source` |

### 2.3 Shortcomings to avoid (this list)

- **Calculator-only UX** (Calculatory, v1p1, YAFP, FactorioLab, SC simple mode): players cannot freely add a Constructor and wire it. We need the Ferrumium **Planner** half.
- **Unusable or non-existent diagrams:** SC’s planner is not a factory editor; FactorioLab’s flow is a Sankey of the **solution**, not an editable graph. Our research SVG (`research-tree-canvas.tsx`) is the same class of mistake — read-only boxes. Do not extend it.
- **Missing Mk + numeric rate on the wire:** most calculators stop at “3.2 constructors.” Edges must show **rate + recommended belt/pipe Mk**.
- **Auto-balance / solver-owned graph** (SatisfiedVisual2 auto-balancing; some Tools flows): overclocking one step silently rescales the tree. We highlight imbalance and offer **Reset to balanced**.
- **LP as the only way to add a building:** YAFP/FactorioLab/SFTools HiGHS are great at “optimal mix,” terrible at “I already placed this Refinery.” Hybrid graph first; LP is a later optional Suggest strategy.
- **URL-only persistence / public Discover:** fine for anonymous web tools; we persist in SQLite with private/shared + edit lock.
- **Mode split** (Manifolder calculator vs layout; Ferrumium Draft vs Planner as separate artifacts): one graph, two **actions** (Suggest vs Add node).

### 2.4 License / reuse (hard rule)

| Source | Use in FactoryMate |
|---|---|
| Coffee Stain `FactoryGame-Docs.json` + extracted icons | Already in-repo; game data/assets, not third-party planner code |
| `@satisfactory-dev/docs.json.ts` (Apache-2.0) | Optional **reference** for Unreal-string parsing; reimplement in Go. Do not add the npm package to `frontend/` |
| FactorioLab, YAFP (check each repo’s LICENSE) | Ideas only unless we explicitly vendor a file and keep attribution — not planned |
| Satisfactory Factories (**AGPL-3.0**) | **No code, no derived Vue components** |
| AnthorNet SCIM / treat SCPP similarly | **No reuse of source or data assets** |
| Closed sites (Ferrumium, Manifolder, Calculatory, logistics.xyz, planner.net) | UX observation only |

---

## 3. How this fits FactoryMate architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Next.js (App Router)                                        │
│  /planner            list (shadcn Table + Dialog)           │
│  /planner/[id]       client canvas: React Flow + shadcn     │
│                      inspector / toolbar / lock banner      │
└─────────────┬───────────────────────────────────────────────┘
              │ session cookie, same /api rewrite as today
              ▼
┌─────────────────────────────────────────────────────────────┐
│ Go chi API                                                  │
│  plan CRUD, visibility, edit lock, suggest (solver)         │
│  GET catalog (slim JSON) + GET icon by ClassName            │
└─────────────┬──────────────────────────┬────────────────────┘
              │                          │
              ▼                          ▼
     SQLite factory_plans        in-memory catalog parsed from
     graph_json + baseline       FactoryGame-Docs.json (+ icons)
```

- **Does not touch** the poller, FRM client, or notification pipeline.
- **New Go package:** `backend/internal/planner` (catalog + solver + optional analyze). Handlers stay in `internal/api` like other features (`mods_handlers.go`, `connection_handlers.go`).
- **Auth:** `RequireSession` + `RequireActiveUser` (same as `/api/mods`). **Not** admin-only. Viewers can create and edit plans — this is the one intentional exception to “viewer = read-only dashboard” (spec §6). Settings remain admin-only.
- **JSON:** camelCase API, snake_case columns (spec §7.1). Errors `{ "error": "..." }` via `writeError`.
- **i18n:** all chrome in `frontend/messages/en.json` namespace `planner`. Item/building/recipe **display names** come from game data, not locale files (same rule as FRM names today).
- **Docker:** the current image copies `backend/data` but **not** `docs/FactoryGame-Docs.json` or `assets/icons/`. Implementation must add those (or a generated slim catalog + icon dir) to the image; see §10.

---

## 4. Game data (already in repo)

### 4.1 `docs/FactoryGame-Docs.json`

Coffee Stain Docs.json dump (same family as `@satisfactory-dev/docs.json.ts`, which we **do not** add as an npm dependency — parse in Go). Top-level array of `{ NativeClass, Classes[] }`.

**File encoding (boot-critical):** `docs/FactoryGame-Docs.json` is **UTF-16 LE with BOM**, not UTF-8. Verified 2026-08-21: file starts `FF FE 5B 00 0D 00 0A 00` (`ÿþ` BOM + `[` as UTF-16LE); size ~10.6 MiB; `encoding=utf-8` raises `UnicodeDecodeError` on byte `0xFF`. `encoding/json` only consumes UTF-8. **`catalog.go` must transcode before unmarshal**, or process start dies on the first catalog load.

Implementation (use this, not `os.ReadFile` + `json.Unmarshal` on the raw bytes):

```go
import (
    "encoding/json"
    "os"

    "golang.org/x/text/encoding/unicode"
    "golang.org/x/text/transform"
)

f, err := os.Open(path)
// ...
dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
err = json.NewDecoder(transform.NewReader(f, dec)).Decode(&docs)
```

Add `golang.org/x/text` to `backend/go.mod`. Tests: a tiny UTF-16 LE BOM fixture that parses, and an assertion that the real dump’s first two bytes are `0xFF 0xFE` (or that decode yields `'['`). Do **not** commit a UTF-8 re-save of the dump as the source of truth — keep Coffee Stain’s encoding.

Planner-relevant classes:

| NativeClass | Use |
|---|---|
| `FGRecipe` | Ingredients, products, duration, `mProducedIn` |
| `FGItemDescriptor`, `FGResourceDescriptor`, biomass / nuclear / etc. | Item form (`RF_LIQUID` / `RF_GAS` / solid), display name |
| `FGBuildableManufacturer`, `FGBuildableManufacturerVariablePower` | Clock, Somersloop slots, power, manufacturing speed |
| `FGBuildableResourceExtractor`, `FGBuildableWaterPump` | Miner / pump source nodes |
| `FGBuildableConveyorBelt`, `FGBuildableConveyorLift` | Belt Mk from `mSpeed` |
| `FGBuildablePipeline` | Pipe Mk from `mFlowLimit` (m³/s) |

**Recipes** look like Unreal property strings, not JSON arrays:

```text
mIngredients: ((ItemClass=".../Desc_IronIngot.Desc_IronIngot_C",Amount=3))
mProduct:     ((ItemClass=".../Desc_IronPlate.Desc_IronPlate_C",Amount=2))
mManufactoringDuration: 6.000000   // typo is in the dump
mProducedIn: (".../Build_ConstructorMk1.Build_ConstructorMk1_C", "...WorkBench...", ...)
```

Parser must extract `Desc_*_C` / `Build_*_C` ClassNames from those paths.

**Alternate recipes:** `FullName` contains `/AlternateRecipes/` **or** `ClassName` starts with `Recipe_Alternate_`. Default solver picks the **non-alternate** recipe when several produce the same item; the UI exposes alternates per product.

**Fluids/gases:** `Amount` in this file is **×1000** relative to in-game m³. Planner catalog must store **canonical per-minute amounts in game units** (divide fluid/gas amounts by 1000 when ingesting). Solids stay as-is.

**Filter `mProducedIn`:** drop workbench, workshop, build gun, `FGBuildableAutomatedWorkBench`. Only automated factory buildings belong on the canvas.

**Clock:** dump `mMaxPotential` is `1.0` (100%) even though the game UI goes to 250% with power shards. Treat **clock as 0–250%** in the planner (game min is 1%; 0% can mean “this node is off” — either clamp to 1–250 or allow 0 as paused). Implied shards = `ceil((clockPercent - 100) / 50)` for display only; do not persist shard count separately.

**Somersloop:** `mCanChangeProductionBoost`, `mProductionShardSlotSize`, `mProductionShardBoostMultiplier`. Output multiplier:

`1 + somersloopCount * boostMultiplier`

Constructor in the dump: 1 slot, multiplier `1.0` → 2× at 1 loop. Assembler: 2 slots, `0.5` each. Manufacturer: 4 slots, `0.25` each. Equivalent form used by the wiki: `1 + filled/totalSlots`.

**Dump gap (enumerated 2026-08-21):** among every class in this dump that has `mCanChangeProductionBoost` / `mProductionShardSlotSize` (62 classes), **exactly one** has boost `True` and slot size `0`:

| ClassName | Display | Dump `mProductionShardSlotSize` | Dump `mCanChangeProductionBoost` | Dump `mProductionShardBoostMultiplier` | Wiki slots |
|---|---|---|---|---|---|
| `Build_SmelterMk1_C` | Smelter | `0` (`mOverrideProductionShardSlotSize` = False) | True | `1.000000` (already correct for 1 slot) | **1** ([Production amplifier](https://satisfactory.wiki.gg/wiki/Production_amplifier) treats a fully slooped Smelter like a 1-slot building; same 53.7 MW at 250% as a Constructor) |

All other automated manufacturers in this dump already match the wiki slot table, so the curated override is **this one row only** — do not add a generic “if slots==0 invent a count” rule:

| Building | Dump slots | Wiki slots |
|---|---|---|
| Constructor `Build_ConstructorMk1_C` | 1 | 1 |
| Smelter `Build_SmelterMk1_C` | **0 (wrong)** | **1** |
| Assembler `Build_AssemblerMk1_C` | 2 | 2 |
| Foundry `Build_FoundryMk1_C` | 2 | 2 |
| Refinery `Build_OilRefinery_C` | 2 | 2 |
| Manufacturer `Build_ManufacturerMk1_C` | 4 | 4 |
| Blender `Build_Blender_C` | 4 | 4 |
| Converter `Build_Converter_C` | 2 | 2 |
| Particle Accelerator `Build_HadronCollider_C` | 4 | 4 |
| Quantum Encoder `Build_QuantumEncoder_C` | 4 | 4 |
| Packager `Build_Packager_C` | 0 and boost **False** | 0 (cannot amplify) — **no override** |

Ingest: apply `{ "Build_SmelterMk1_C": 1 }` after parse; keep dump multiplier `1.0`. If boost is false, leave slots at 0.

**Power (display / summary) — formulas verified against wiki, not TBD:**

Canonical (wiki [Clock speed](https://satisfactory.wiki.gg/wiki/Clock_speed) + [Production amplifier](https://satisfactory.wiki.gg/wiki/Production_amplifier)):

```text
P = P_base × (clockPercent/100)^N × (1 + somersloopCount/slotCount)^2
```

- `N` = `mPowerConsumptionExponent` from the dump (`1.321929` on Constructor/Assembler). Wiki publishes `1.321928` = log₂(2.5) after Patch 0.7.0.0. Use the **per-building dump field**. At Constructor 150% the two exponents differ by less than 0.00001 MW; published wiki MW is rounded to 2 decimals.
- Somersloop exponent **2.0** is the wiki `(1 + filled/total)²` and matches dump `mProductionBoostPowerConsumptionExponent` = `2.000000`. Full slots → 4× power.
- Base: `mPowerConsumption` (variable-power buildings: recipe `mVariablePowerConsumption*` / estimated min–max — not in the golden fixtures below).
- When `slotCount` is 0 (Packager, miners): omit the sloop term (treat multiplier as 1); UI must not offer sloops.

Worked examples (wiki-rounded MW are the **golden expected values** for `balance_test.go` / `testdata/planner/power_examples.json`; compute with dump `N` then compare with ±0.01 MW, which is how the wiki tables round):

| Fixture id | Building | P_base (dump) | Clock | Sloops / slots | Wiki / formula expected | Source |
|---|---|---|---|---|---|---|
| `constructor_150` | Constructor | 4 MW | 150% | 0 / 1 | **6.84 MW** | [Constructor](https://satisfactory.wiki.gg/wiki/Constructor) overclock table; [Clock speed](https://satisfactory.wiki.gg/wiki/Clock_speed) 150% = 170.91% of base → 4 × 1.7091 ≈ 6.84. Raw: `4 × 1.5^1.321928 ≈ 6.8366` |
| `assembler_1_sloop` | Assembler | 15 MW | 100% | **1 / 2** | **33.75 MW** | Wiki: 1 of 2 slots → 2.25× power. `15 × (1+1/2)² = 15 × 2.25 = 33.75` |
| `constructor_1_sloop` | Constructor | 4 MW | 100% | 1 / 1 | **16 MW** | [Production amplifier](https://satisfactory.wiki.gg/wiki/Production_amplifier): “4 MW … 4 × 4 = 16 MW for an amplified constructor” |
| `constructor_250_sloop` | Constructor | 4 MW | 250% | 1 / 1 | **53.7 MW** | Same page: amplified fully overclocked constructor. `4 × 4 × 2.5^1.321928 ≈ 53.72` |

Cross-check (already implied by the formula): Assembler 150% no sloop = **25.64 MW** on the [Assembler](https://satisfactory.wiki.gg/wiki/Assembler) table (`15 × 1.5^N ≈ 25.637`). Clock-only table: 50% → 40% of base (Constructor **1.6 MW**), 200% → 250% of base (**10 MW**), 250% → 335.77% (**13.43 MW**) — all match `P_base × (clock/100)^N` within rounding.

**Belts:** `mSpeed` is Unreal uu/s; **items/min = mSpeed / 2** (Mk.1 `120` → 60/min; Mk.5 `1560` → 780/min; Mk.6 `2400` → 1200/min). Lifts share the same Mk speeds.

**Pipes:** `mFlowLimit` is m³/s → **m³/min = flowLimit * 60** (Mk.1 `5` → 300; Mk.2 `10` → 600).

### 4.2 `assets/icons/` + `assets/icons.json`

~188 PNGs named `{ClassName}.png` (mostly `Desc_*_C`). `icons.json` maps ClassName → `fs_path` / `asset_name`.

**Gap:** buildings in recipes are `Build_ConstructorMk1_C`; icons are `Desc_ConstructorMk1_C.png`. Catalog must map `Build_*` → descriptor icon ClassName (strip `Build_` / use `FGBuildingDescriptor` if present). Missing icons: placeholder lucide / muted box, never a broken image.

Serve icons from the **backend** (`GET /api/planner/icons/{className}`) so Docker does not need to duplicate files into `frontend/public`, and so mapping stays in one place. Frontend `<img src={apiUrl(...)} />` with session cookie (`credentials: include`).

---

## 5. Data model

### 5.1 Principle: persist the graph, compute the flows

**Persisted (user intent):**

- Plan metadata (name, visibility, owner).
- Graph: nodes (id, recipe, building, count, clock, somersloops, x/y) and edges (id, from node+port, to node+port, item ClassName).
- Last solver request (target item, target rate, recipe choices) and **baseline snapshot** of the graph after that suggest.
- Viewport (optional) so reopen matches last pan/zoom.

**Not persisted (derived):**

- Per-edge actual throughput, surplus/deficit, recommended Mk, “over belt capacity.”
- Totals: power, raw remaining, building counts.
- Lock **holder display name** (join at read time).

That split is what makes “don’t auto-correct overrides” tractable: the client (and optionally the server) **recomputes** flows whenever the graph or overrides change.

### 5.2 Node and edge identity (hybrid model)

There is **no** `origin: solver | manual` column on a live node. A node is just a node.

Reset works because we store a **second copy** of the graph (`baseline_json`) taken when the user last applied a suggestion — not because live nodes are tagged.

Optional node **roles** (not origin) so the canvas can render different bodies:

- `process` — manufacturer / extractor with a recipe (default).
- `source` — raw input with item + rate (miner, well, “bought from storage”).
- `sink` — dump / AWESOME sink / “export” for byproducts.

Roles are still the same table/JSON shape; recipe/building may be null on source/sink.

**Machine count** is a field on the node (`count`, float allowed). That matches community calculators. Players who want “one constructor = one box” set `count = 1` and duplicate nodes. The solver emits fractional counts (e.g. 3.33); the inspector can “round up” as a one-click override (which then shows as overproduction upstream unless they also change clock).

### 5.3 SQLite (new tables)

Follow existing style: numbered SQL in `backend/internal/db/migrations/`, applied in a transaction, recorded in `schema_migrations` (`migrate.go`). **New feature → `CREATE TABLE`**, no rebuild. If a later milestone must change primary keys or CHECKs, use the **table-recreation** pattern from `006_research_node_composite_key.sql` (`*_new` → `INSERT` → `DROP` → `RENAME`), not `ALTER` for those cases. Additive columns may use `ALTER TABLE` like `002_research_layout.sql`.

Suggested next file: `011_factory_plans.sql` (renumber if 011 is taken by then).

```sql
CREATE TABLE factory_plans (
    id INTEGER PRIMARY KEY,
    owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private'
        CHECK (visibility IN ('private', 'shared')),
    -- Player-facing lifecycle; independent of visibility (who can see it).
    status TEXT NOT NULL DEFAULT 'planning'
        CHECK (status IN ('planning', 'in_progress', 'completed', 'archived')),
    -- Last applied suggest (nullable until the user runs one)
    target_item_class TEXT,
    target_rate REAL,
    solver_options_json TEXT NOT NULL DEFAULT '{}',
    -- Current graph (nodes, edges, viewport)
    graph_json TEXT NOT NULL,
    -- Graph immediately after last successful suggest; NULL = never suggested
    baseline_json TEXT,
    locked_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    lock_expires_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_factory_plans_owner ON factory_plans(owner_user_id);
CREATE INDEX idx_factory_plans_visibility ON factory_plans(visibility);
CREATE INDEX idx_factory_plans_status ON factory_plans(status);

-- Optional: no extra table required for heartbeat; lock_expires_at is enough.
```

`ON DELETE CASCADE` for owner matches “user gone → their private plans go.” Shared plans owned by a deleted user: either cascade or `ON DELETE RESTRICT` and require transfer — **recommend CASCADE** for v1 (small group); document it.

**`status` (not visibility):** player-facing lifecycle of the *idea*, independent of who can see or edit it.

| SQL | JSON (`status`) | Meaning |
|---|---|---|
| `planning` | `"planning"` | Sketching / not built yet (default on `POST`) |
| `in_progress` | `"inProgress"` | Being built in-game |
| `completed` | `"completed"` | Design is done and built in-game |
| `archived` | `"archived"` | No longer relevant; keep for reference |

`archived` specials: omitted from the **default** list; graph **read-only** (no lock acquire, `PUT /graph` 403) even for the owner; still `GET`able and still `PATCH`able so status/visibility/name can change (un-archive). Not a delete.

Do **not** normalize nodes/edges into SQL rows in v1. This codebase already stores structured blobs as TEXT JSON (`cost_json`, `parents_json`, templates). Graph queries (“all plans using Recipe X”) are not needed yet.

### 5.4 `graph_json` / `baseline_json` shape (conceptual)

```json
{
  "viewport": { "x": 0, "y": 0, "zoom": 1 },
  "nodes": [
    {
      "id": "n_uuid",
      "role": "process",
      "recipeClass": "Recipe_IronPlate_C",
      "buildingClass": "Build_ConstructorMk1_C",
      "count": 3.0,
      "clockPercent": 100,
      "somersloopCount": 0,
      "x": 120,
      "y": 80
    }
  ],
  "edges": [
    {
      "id": "e_uuid",
      "sourceNodeId": "n1",
      "sourcePort": "out:Desc_IronIngot_C",
      "targetNodeId": "n2",
      "targetPort": "in:Desc_IronIngot_C",
      "itemClass": "Desc_IronIngot_C"
    }
  ]
}
```

Port ids are **item ClassName + direction**, so reconnecting after a recipe change can drop invalid edges instead of silently carrying Iron into a Copper input.

`solver_options_json`: `{ "recipeByProductClass": { "Desc_Wire_C": "Recipe_Alternate_Wire_1_C" }, "defaultClockPercent": 100, "defaultSomersloopCount": 0 }`.

**Reset to balanced:** `graph_json := baseline_json` (keep `updated_at` / lock). If `baseline_json` is NULL, 400 with a clear error (“no solver baseline”). This **does** remove nodes the user added after the last suggest — that is the point of “revert to solver baseline.” A weaker “reset clocks/sloops only” can be a second toolbar action later; v1 is the full snapshot restore.

Positions in the baseline are whatever the solver laid out (or the graph at apply time). Reset restores those positions too, so the diagram matches the balanced tree. (If that feels harsh, v1 can restore production fields by node id and keep current x/y for ids that still exist; prefer **full snapshot** first — simpler and honest.)

### 5.5 Edit lock

| Field | Behavior |
|---|---|
| `locked_by_user_id` / `lock_expires_at` | Writer must hold an unexpired lock |
| Acquire | `POST .../lock` — succeeds if unlocked, expired, or already held by self; **409** if someone else holds a live lock |
| Heartbeat | `POST .../lock/heartbeat` every ~45s; extends expiry (recommend **5 minutes**) |
| Release | `POST .../lock/release` or tab `visibilitychange` / `pagehide` (`fetch` keepalive or `sendBeacon` to a small POST) |
| Force | Owner **or** admin can `POST .../lock/force-release` |
| Reads | Always allowed if the user may **see** the plan |
| Writes | `PUT` graph/metadata requires holding the lock; **409** `error` if not |

SQLite already uses `MaxOpenConns(1)` (`db.go`) — lock checks are ordinary `UPDATE … WHERE` compare-and-swap, not `SELECT FOR UPDATE`.

Not real-time: a second user sees a banner “Editing: {username}, lock expires {relative}” from GET payload. They can refresh; no websocket.

---

## 6. Authorization (admin / viewer)

Spec §6: viewers cannot hit admin settings APIs. **Planner is a user-content area**, more like Account notifications than Settings.

| Action | Who |
|---|---|
| List plans | Active user: own `private` + all `shared`. **Admin additionally lists others’ private** (support / moderation); UI labels them “private (user)”. Default list **omits `archived`** (see §8.3). |
| Create plan | Any active user (default `private`, status `planning`) |
| Read plan | Owner, or anyone if `shared`, or admin — including archived (open by URL / “show archived” filter) |
| Change visibility / rename / **status** / delete | **Owner** or **admin** (`PATCH` metadata). **Does not require the edit lock** (same as rename/visibility). Setting `archived` **releases any live lock** in the same transaction. |
| Edit graph / suggest / reset | Any user who can **read** it, **and** who holds the edit lock, **and** status is **not** `archived` |
| Shared plan: viewer with lock | **Allowed** if not archived — otherwise “shared” is view-only and the lock model is pointless |

List endpoint should return `canEdit`, `canManage`, `lock` so the UI does not guess.

---

## 7. Backend

### 7.1 Package layout

```
backend/internal/planner/
  catalog.go      -- UTF-16 LE BOM decode (§4.1), then parse Docs.json; belts/pipes; icon map; Smelter slot override
  catalog_test.go
  parse_ue.go     -- Unreal ((ItemClass=...,Amount=)) strings
  solver.go       -- recursive suggest
  solver_test.go
  balance.go      -- given graph + catalog → edge rates, surplus/deficit, Mk
  balance_test.go -- golden JSON fixtures
  lock.go         -- acquire / heartbeat / expiry helper
backend/internal/api/planner_handlers.go
backend/data/factory_catalog.json   -- optional generated slim file (see §10)
backend/testdata/planner/          -- recipe walk fixtures + power_examples.json (§4.1 / §7.5)
```

Keep `FactoryGame-Docs.json` as the **source** in `docs/` (large, **UTF-16 LE + BOM** — see §4.1). At process start, `catalog.go` transcodes with `golang.org/x/text/encoding/unicode` then unmarshals once into memory. Optionally commit a generated slim JSON under `backend/data/` (UTF-8 is fine for that derived file) so tests and Docker do not need the 90k-line dump — regeneration is a `go generate` / small cmd, not a runtime requirement if the slim file is versioned.

### 7.2 Solver (server-side)

**Placement:** Go, not the browser. Recipe walking, alternate defaults, and fluid scaling should have one authoritative implementation and unit tests.

**Algorithm (v1 — greedy tree, not LP):**

1. Input: `itemClass`, `ratePerMin`, `recipeByProductClass`, optional default clock/sloops.
2. Resolve recipe for the item (override map → else unique non-alternate producer → else first automated recipe).
3. Pick the **first automated building** in `mProducedIn` (constructor vs manufacturer is determined by the recipe).
4. Compute `outputPerMin` for **one** building at default clock/sloops from duration, product amount (fluid-corrected), `mManufacturingSpeed`. For multi-product recipes, `outputPerMin` is **only** the rate of the **requested** `itemClass` (the product whose ClassName matches the current recursion target). Other products on the same recipe are ignored for sizing.
5. `count = targetRate / outputPerMinOfRequestedItem`. Do **not** size to satisfy byproduct demand; do **not** start extra solver trees for byproducts; do **not** emit a `sink` node. Extra products are extra **output ports on the same process node**, with **no edges**. After apply, §7.3 flags those ports as **unterminated output** (amber on the node). The player may later add a sink or wire the byproduct by hand. The requested product is also unconnected (no auto goal-sink); that is expected for a suggest leaf — still unterminated under the same rule; the plan’s `targetItemClass` / `targetRate` is metadata, not a graph node.

   **Worked example — `Recipe_Plastic_C` in this dump:** Refinery, 6 s cycle. Ingredients: Crude Oil `Amount=3000` → **3 m³** (fluid ÷1000). Products: Plastic `Amount=2` (solid) + Heavy Oil Residue `Amount=1000` → **1 m³**. At 100% clock, one building: **10 cycles/min** → **20 Plastic/min**, **10 m³/min HOR**, **30 m³/min** Crude Oil.

   Suggest `{ itemClass: "Desc_Plastic_C", ratePerMin: 60 }` (default clock/sloops):

   - `outputPerMin` = 20 (Plastic only). `count` = 60/20 = **3**. HOR falls out at **30 m³/min**; it is **not** a second target.
   - Recurse ingredients only: Crude Oil demand = 3 × 30 = **90 m³/min** → one `source` node.
   - Generated graph (ids illustrative):

```text
nodes:
  n_refinery  role=process  recipeClass=Recipe_Plastic_C
              buildingClass=Build_OilRefinery_C  count=3  clock=100  sloops=0
              ports in:  Desc_LiquidOil_C
              ports out: Desc_Plastic_C, Desc_HeavyOilResidue_C
  n_oil       role=source   itemClass=Desc_LiquidOil_C  (rate 90 m³/min)
edges:
  n_oil.out:Desc_LiquidOil_C → n_refinery.in:Desc_LiquidOil_C
  (no edge from Desc_Plastic_C)
  (no edge from Desc_HeavyOilResidue_C)
  (no sink node)
```

   Balance immediately after apply: oil edge can be balanced; **both** refinery outputs are unterminated (amber on `n_refinery`). HOR is the byproduct the player must sink/reuse; Plastic is the requested 60/min with nothing downstream. Connecting a player-made `sink` to HOR clears only that port’s warning.
6. For each ingredient, recurse with `count * ingredientPerMin` (never with byproduct rates).
7. Raw resources (`FGResourceDescriptor` with no automated recipe, or recipes only in miners): emit a **source** node with the required rate (purity/miner Mk left to the user as an override on that node).
8. Cycles (recycling recipes): detect via a visiting set; **stop and return 400** with the cycle ClassNames rather than looping. User can place those nodes manually.
9. Layout: assign x by depth (raw left, target right), y by sibling index. Client may run dagre again; sending coordinates from Go keeps the first paint stable.

Output: a full `graph` object **plus** a copy for `baseline`. The handler **does not** have to persist until the user confirms (see API).

Solver does **not** read the current graph unless the API is “suggest into existing plan”:

- **Replace mode (v1 default):** applying suggest overwrites `graph_json` and `baseline_json`.
- **Merge mode (optional later):** add a disconnected suggested subgraph; user wires it. Skip in v1 to avoid layout collisions.

### 7.3 Balance / Mk (authoritative tests in Go; hot path on the client)

For slider UX, waiting on HTTP every clock tick is too slow. **Port the same formulas to TypeScript** in `frontend/lib/planner/balance.ts`, driven by the slim catalog.

Contract:

- Go `balance_test.go` and Vitest `balance.test.ts` share **golden fixtures** under `backend/testdata/planner/` (JSON in/out).
- Optional `POST /api/planner/analyze` for debugging / “recompute on save” validation; **not** required on every mouse move.

**Balance rules:**

- Each process node produces/consumes `count * perBuildingRate(clock, sloops)` per item.
- For each `itemClass`, sum production on source ports vs consumption on target ports **along connected edges only**. Unconnected production is “unterminated output” (amber on the **node**, not an edge). Unconnected consumption is “starved input” (amber/red on the node).
- An **edge** carries `min(available from that output, demand on that input)` after distributing a node’s output **proportionally** across outgoing edges of that item (and demand across incoming). Simple proportional split is enough without splitter entities.
- **Overproduction on an edge:** flow offered into that wire exceeds what the consumer can take (consumer already satisfied). **Underproduction:** consumer still hungry after all incoming edges.
- **Mk recommendation:** smallest belt (solid) or pipe (fluid/gas) whose capacity ≥ edge flow. If flow exceeds Mk.6 / Mk.2, recommend that Mk and flag `exceedsMax: true`. Label always includes **numeric rate** with unit (`items/min` vs `m³/min`).

Do **not** write these fields back into `graph_json`.

### 7.4 API surface

Mount next to other session routes in `routes.go` (active user, not admin wrapper).

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/planner/catalog` | session, active | Slim items, recipes, buildings, belts, pipes (cacheable in memory; `ETag` optional) |
| GET | `/api/planner/icons/{className}` | session, active | `image/png` or 404 |
| GET | `/api/planner/plans` | session, active | List visible plans + lock summary + permissions. Default: omit `archived`. Query: `status=planning` (repeatable or comma-separated) and/or `includeArchived=true` |
| POST | `/api/planner/plans` | session, active | `{ name, visibility?, status? }` → empty graph; defaults `private` + `planning` |
| GET | `/api/planner/plans/{id}` | can-read | Full plan including `graph`, `baseline` present?, `status`, `lock`, `canEdit`, `canManage` |
| PATCH | `/api/planner/plans/{id}` | can-manage | `{ name?, visibility?, status? }` — metadata only; **no lock**. Transition to `archived` clears lock. |
| DELETE | `/api/planner/plans/{id}` | can-manage | Delete |
| PUT | `/api/planner/plans/{id}/graph` | can-read + **lock** | `{ graph, updatedAt }` optimistic concurrency — 409 if `updatedAt` mismatch or no lock; **403** if `archived` |
| POST | `/api/planner/plans/{id}/suggest` | lock | `{ itemClass, ratePerMin, recipeByProductClass? }` → `{ graph }` **preview** (no persist) **or** persist if `apply: true`. **403** if `archived` |
| POST | `/api/planner/plans/{id}/apply-suggest` | lock | Persist last preview **or** body graph as current + baseline (prefer one round-trip: suggest with `apply`). **403** if `archived` |
| POST | `/api/planner/plans/{id}/reset-baseline` | lock | `graph_json = baseline_json`. **403** if `archived` |
| POST | `/api/planner/plans/{id}/lock` | can-read | Acquire. **403** if `archived` |
| POST | `/api/planner/plans/{id}/lock/heartbeat` | lock holder | Extend |
| POST | `/api/planner/plans/{id}/lock/release` | lock holder | Release |
| POST | `/api/planner/plans/{id}/lock/force-release` | owner or admin | Kick editor |
| POST | `/api/planner/analyze` | session, active | `{ graph }` → derived flows (optional) |

List/detail camelCase sketch:

```json
{
  "id": 1,
  "name": "Heavy frames 30/min",
  "visibility": "shared",
  "status": "inProgress",
  "owner": { "id": 2, "username": "ada" },
  "targetItemClass": "Desc_ModularFrameHeavy_C",
  "targetRate": 30,
  "updatedAt": "2026-08-21T07:00:00Z",
  "lock": {
    "held": true,
    "userId": 2,
    "username": "ada",
    "expiresAt": "2026-08-21T07:05:00Z",
    "mine": true
  },
  "canEdit": true,
  "canManage": true
}
```

Optimistic concurrency: client sends `updatedAt` from last GET; successful PUT returns new `updatedAt`. Prevents overwriting if the lock expired and someone else saved (belt-and-suspenders with the lock).

Suggest **preview** vs **apply:** keep the dialog on the client; `POST suggest` with `apply: false` returns a graph; user hits Apply → `PUT graph` **and** set baseline in one server transaction (`POST apply-suggest` with that graph) so a failed PUT cannot leave baseline stale.

### 7.5 Tests

- Catalog: UTF-16 LE BOM decode (fixture + dump signature); fluid ÷1000; alternate detection; belt/pipe tables; **Smelter-only** somersloop slot override `0 → 1` (`Build_SmelterMk1_C`); Packager stays 0.
- Power golden file `backend/testdata/planner/power_examples.json` — the four rows in §4.1 (`constructor_150`, `assembler_1_sloop`, `constructor_1_sloop`, `constructor_250_sloop`). `balance_test.go` (and Vitest `balance.test.ts`) must assert computed MW vs those wiki-rounded values within **0.01 MW**. Do not treat the exponent as an unchecked comment.
- Solver: Iron Plate 60/min → expected constructor + smelter + ore source counts (fixture). Plastic 60/min (`Recipe_Plastic_C`) → **3** refineries, oil source **90 m³/min**, **no** HOR sink/edge, two output ports on the refinery node (§7.2 example).
- Cycle recipe → error.
- Lock: second user 409; expiry allows steal; force-release admin.
- Visibility: viewer cannot GET another user’s private plan; can GET shared.
- Graph PUT without lock → 409. Graph PUT / lock / suggest on `archived` → 403. PATCH status does not require a lock.

No live FRM. No Discord.

---

## 8. Frontend

### 8.1 Why shadcn copy-paste is not enough

Spec §8.1 assumes tables, forms, and charts. An interactive directed graph needs:

- Infinite pan/zoom canvas
- Draggable nodes with **multiple handles**
- User-drawn edges with validation
- Custom edge labels (rate + Mk + imbalance)
- Selection, delete, maybe undo

The research tree proves we can draw SVG edges, but it is **not** an editor. Installing a graph library is an **explicit §8.1 exception**, analogous to Recharts for charts.

Surrounding UI (list page, dialogs, inspector, toasts, sidebar nav) stays **shadcn** (`Button`, `Sheet`/`Sidebar`, `Input`, `Select`/`Combobox`, `Slider`, `Badge`, `Alert`, `DropdownMenu`, `Sonner`).

### 8.2 Diagram library: `@xyflow/react` (React Flow 12)

| Option | Verdict |
|---|---|
| **`@xyflow/react` (MIT)** | **Choose.** Custom nodes/edges, handles, MiniMap, Controls, Background, `isValidConnection`. Several community Satisfactory canvases use it; it matches our React 19 / Next App Router stack as a **client** component. |
| AntV **X6** (SFTools next-gen) | Capable graph editor, but Angular-centric in that codebase and another heavy dependency. No benefit over React Flow here. |
| React Flow **Pro** (helpers, paid copy-paste, undo) | **Do not use.** Paid, extra telemetry/branding risk. Implement undo ourselves if needed (`z`/`y` on a small command stack) or ship v1 without undo (confirm destructive reset). |
| Rete.js | More “editor framework,” heavier, worse shadcn fit. |
| Cytoscape.js / JointJS | Graph viz, weaker React node components. |
| GoJS | Commercial. |
| Hand-rolled SVG + pointer events | High cost; we would reimplement React Flow poorly. |
| `@dnd-kit/*` (already in the repo) | Lists/sortables, not node+edge graphs. Keep for other pages; do not force it onto the canvas. |

**Self-hosted / dependency-light:** `@xyflow/react` is MIT, npm-install like `recharts`. No cloud. CSS import `@xyflow/react/dist/style.css` in the planner client module only (do not global-pollute). Theme: override CSS variables to match Tailwind/shadcn neutrals; custom node bodies use `bg-card`, `border`, `text-sm` so nodes look like FactoryMate cards, not the default blue React Flow skin.

**Next.js:** `dynamic(() => import(...), { ssr: false })` for the canvas (needs `window`). Page RSC loads plan JSON via `serverApiFetch`, passes props into the client editor.

**Auto-layout:** `@dagrejs/dagre` (MIT, small) on suggest apply and optional “Re-layout.” Not Elk (EPL, heavier) unless dagre fails on large graphs later.

### 8.3 Routes and nav

| Route | Access | Role |
|---|---|---|
| `/planner` | viewer, admin (active) | Plan list: create dialog, visibility badge, **status badge**, lock indicator, **status filter** (default hides archived) |
| `/planner/[id]` | can-read | Full editor / read-only canvas |

Add **Planner** to `viewerItems` in `app-sidebar.tsx` (with production / milestones). Fix `NavMain` `isActive`: today it is `pathname === url`, which will miss `/planner/3`. Use `pathname === url || (url !== "/" && pathname.startsWith(url + "/"))`.

Middleware / layout: same `(app)` shell as other dashboard pages. No new auth.

**List page (`planner-list.tsx`):** shadcn `Table` + `Badge` for `visibility` and `status` (i18n keys under `planner.status.*`). Filter with `Tabs` or `Select` (same pattern as `/production` tabs): **Planning / In progress / Completed**, plus an **Archived** option that is **not** the default. Default fetch is “all non-archived.” Row action (dropdown) for owner/admin: set status via `PATCH` (no lock). Opening an archived plan still uses `/planner/[id]` but the editor is read-only (`canEdit` false) until status is changed off `archived`.

### 8.4 Component structure

```
frontend/app/(app)/planner/page.tsx              -- RSC list
frontend/app/(app)/planner/[id]/page.tsx         -- RSC load plan + catalog? 
frontend/components/planner/
  planner-list.tsx                               -- Table, Dialog create, delete AlertDialog, status Badge + filter
  planner-editor.tsx                             -- lock heartbeat, save debounce, read-only gate
  planner-toolbar.tsx                            -- suggest, reset, layout, save status
  planner-lock-banner.tsx
  suggest-dialog.tsx                             -- item Combobox, rate Input, alternate picks
  node-inspector.tsx                             -- Sheet: recipe, clock Slider, sloops, count
  add-node-popover.tsx                           -- search recipe/building
  canvas/planner-canvas.tsx                      -- React Flow wrapper (ssr: false)
  canvas/process-node.tsx                        -- custom node
  canvas/source-node.tsx
  canvas/sink-node.tsx
  canvas/flow-edge.tsx                           -- custom edge (Mk + rate + color)
frontend/lib/planner/
  catalog-types.ts
  graph-types.ts
  constants.ts                                   -- PLANNER_GRAPH_SAVE_DEBOUNCE_MS (v1 default 800)
  balance.ts                                     -- mirror Go
  balance.test.ts
  layout.ts                                      -- dagre
  to-react-flow.ts                               -- graph_json ↔ RF nodes/edges
frontend/messages/en.json                        -- "planner" namespace
```

Catalog: fetch once in the editor (`GET /api/planner/catalog`), keep in React context (`PlannerCatalogProvider`). Do not put the 90k-line dump in the browser.

### 8.5 Graph state

No Redux/Zustand in the project today. **v1: React Flow `useNodesState` / `useEdgesState` plus a small React context** for:

- plan metadata, `updatedAt`, lock, permissions
- catalog
- derived balance (recomputed in `useMemo` when nodes/edges/data change)
- dirty flag / save status

Avoid duplicating node position in a second store. Persist by mapping RF nodes/edges → `graph_json` (see `to-react-flow.ts`).

**Node `data`:** recipe/building/count/clock/sloops (serializable). Derived rates injected when mapping to RF so the node UI can show “45/min” without storing it.

If context gets noisy, **Zustand is an optional follow-up** (one store), not a prerequisite.

**Save:** debounce after drag end / inspector change → `PUT /graph`. v1 interval is **800 ms**, but that number is an unvalidated starting point, not a settled product decision. Implement as a named constant `PLANNER_GRAPH_SAVE_DEBOUNCE_MS` in `frontend/lib/planner/constants.ts` (one-line change if 800 ms feels laggy or chatty). Immediate save on Apply suggest / Reset. Show “Saving…” / “Saved” / “Read-only” in the toolbar (i18n). Read-only users (including **archived**): `nodesDraggable={false}`, `nodesConnectable={false}`, `elementsSelectable={true}`.

**Heartbeat:** `setInterval` 45s while `canEdit && lock.mine`; pause when `document.hidden`; release on unload.

### 8.6 Canvas UX (player-familiar)

- **Background** dots + **Controls** (zoom) + **MiniMap**.
- **Process node:** building icon, recipe name, `count ×` building, clock badge, sloop dots, power snippet. **Handles:** left = ingredients, right = products (fluid handles can use a pipe-colored ring).
- **Click** node → inspector Sheet (shadcn). Change recipe (Combobox of recipes for that building), clock slider 0–250, sloop stepper 0…max, count input.
- **Add node:** toolbar “Add building” → pick recipe (or building then recipe). Drop at viewport center. Unconnected until the user draws edges (or they run suggest).
- **Connect:** drag handle to handle; `isValidConnection` requires same `itemClass` and `out → in`.
- **Delete:** selected node/edge, with i18n confirm only for nodes that have many edges.
- **Edges:** bezier or smoothstep; label = `{icon} {rate} · Mk.{n}` (or Pipe Mk). Colors:
  - balanced: default/muted
  - under: destructive/amber
  - over: warning (surplus dumped)
  - exceeds max belt/pipe: destructive + i18n tooltip
- **Suggest dialog:** item search (icons + name), rate, optional alternate table for items in the chain (can be a second step after a first generate). Apply runs dagre, then `apply-suggest`.
- **Reset to balanced:** `AlertDialog`, then `reset-baseline`.
- Empty plan: CTA to suggest **or** add a node — hybrid from the first screen.

### 8.7 Recalculation: client vs server

| Work | Where | When |
|---|---|---|
| Recipe tree walk (suggest) | **Server** | User applies suggest |
| Persist graph / baseline / lock | **Server** | Save, apply, reset |
| Throughput, under/over, Mk, power sum | **Client** (`balance.ts`) | Every graph/override change (`useMemo`) |
| Catalog parse / fluid scaling | **Server** at boot; client uses slim catalog | Once per editor load |
| Analyze endpoint | Optional server | Tests / mismatch debug |

Clock slider must feel instant; that is the whole reason for a TS port of balance.

### 8.8 i18n and types

- Namespace `planner` in `en.json` (list, lock, suggest, inspector, edge status, errors).
- Add TypeScript types to `frontend/lib/api-types.ts` (same file as the rest of the API).
- Client mutations: `apiFetch` + `ApiError` + Sonner, same as settings forms.

---

## 9. Integration summary (persisted vs computed)

```
User edits clock
    → RF node.data.clockPercent changes
    → balance.ts recomputes edge labels/colors (not saved as such)
    → debounced PUT graph_json (clock IS saved; rates are not)

User clicks Suggest + Apply
    → POST suggest/apply
    → server solver + layout
    → DB: graph_json = baseline_json = result
    → client replaces RF state, balance runs locally

User clicks Reset to balanced
    → POST reset-baseline
    → graph_json copied from baseline
    → client reloads graph; imbalances from later overrides disappear

User B opens shared plan
    → GET: canEdit false, lock banner
    → canvas read-only; live numbers still computed client-side from the saved graph
```

---

## 10. Ops, Docker, and docs follow-through

- **Dockerfile:** copy `docs/FactoryGame-Docs.json` (or generated `backend/data/factory_catalog.json`) and `assets/icons/` + `assets/icons.json` next to the Go binary; set paths via env (`PLANNER_DOCS_PATH`, `PLANNER_ICONS_DIR`) with local-dev defaults relative to repo root.
- **Spec updates (after this proposal is accepted, in the implementation milestone):** §2.1 note the React Flow exception; §3 tables; §6 viewer write exception; §7 routes; §8 page inventory + §8.1 mapping (list = shadcn; `[id]` = React Flow + shadcn inspector).
- **Roadmap:** new milestone **after** current work (not M14 backlog trivia) — e.g. M19 Planner, possibly split: M19a catalog+API+list, M19b canvas+solver+lock.
- **Guides:** a short `docs/guide/planner.md` when shipping (not required for this design doc).

---

## 11. Suggested implementation slices

Not a commitment to checkbox wording; a sensible build order:

1. Catalog parser + slim JSON + icon route + tests (fluids, alternates, belts).
2. Migration + CRUD + visibility + lock (no canvas; curl/JSON).
3. Solver + fixtures; suggest API.
4. Frontend list page (shadcn only).
5. React Flow editor: load/save, custom nodes/edges, inspector, client balance, Mk labels.
6. Suggest/reset/layout wired; lock banner + heartbeat; read-only path.
7. Spec/roadmap/i18n/nav polish; Docker copy of data.

---

## 12. Open choices (defaults proposed)

| Topic | Default in this plan | Why |
|---|---|---|
| Fractional machine counts | Yes | Matches every major calculator; count=1 remains available |
| Reset | Full baseline snapshot | Matches “revert to solver”; no origin flags on nodes |
| Admin sees others’ private plans | Yes | Small trusted group; hide later if unwanted |
| Viewer edits shared plans | Yes, with lock | Matches the lock decision |
| Merge-into-existing suggest | No in v1 | Replace graph on apply |
| Undo stack | No in v1 | Reset + debounce save is enough |
| `POST /analyze` | Optional | Client balance is the UX path |
| Graph save debounce | **800 ms** (`PLANNER_GRAPH_SAVE_DEBOUNCE_MS`) | Unvalidated v1 default; tune after real editing, one-line constant |
| Plan `status` | `planning` default; list hides `archived`; archived graph read-only | Org for “sketch vs built vs stale”; independent of visibility |

---

## 13. Success criteria (for a later DoD)

- Suggest Iron Plate (or similar) at a known rate → node counts match fixture; graph is draggable and reconnectable. Suggest Plastic 60/min → 3 refineries, HOR unconnected (unterminated), no auto-sink.
- Changing clock/sloops on one node **does not** rewrite other nodes; edges show under/over and a Mk **plus numeric rate**.
- Reset restores the last applied suggest.
- Private plans invisible to other viewers; shared plans visible; second editor is read-only while lock is live.
- Plan list filters by status; archived plans omitted by default, still openable, graph not editable until un-archived.
- No hardcoded UI strings; catalog names stay as game data.
- Scoped tests: `go test ./internal/planner/...` and Vitest `lib/planner/balance.test.ts`; frontend lint/build.
