import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"

import {
  analyzeGraph,
  buildingPowerMW,
  computePowerMW,
  withinPowerTolerance,
} from "@/lib/planner/balance"
import {
  indexCatalog,
  type PlannerCatalog,
} from "@/lib/planner/catalog-types"
import type { PlanGraph } from "@/lib/planner/graph-types"

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), "../../..")
const fixtureDir = join(repoRoot, "backend/testdata/planner")

function loadCatalog(): ReturnType<typeof indexCatalog> {
  const raw = readFileSync(
    join(fixtureDir, "factory_catalog.json"),
    "utf-8"
  )
  return indexCatalog(JSON.parse(raw) as PlannerCatalog)
}

describe("balance parity with Go golden fixtures", () => {
  const catalog = loadCatalog()

  it("matches power_examples.json within tolerance", () => {
    const raw = readFileSync(join(fixtureDir, "power_examples.json"), "utf-8")
    const examples = JSON.parse(raw) as Array<{
      id: string
      powerBaseMW: number
      clockPercent: number
      somersloopCount: number
      somersloopSlots: number
      powerExponent: number
      expectedMW: number
    }>

    for (const ex of examples) {
      const got = computePowerMW(
        ex.powerBaseMW,
        ex.clockPercent,
        ex.powerExponent,
        ex.somersloopCount,
        ex.somersloopSlots
      )
      expect(
        withinPowerTolerance(got, ex.expectedMW),
        `${ex.id}: got ${got}, want ${ex.expectedMW}`
      ).toBe(true)
    }
  })

  it("matches building power from catalog", () => {
    const cases = [
      { id: "constructor_150", build: "Build_ConstructorMk1_C", clock: 150, sloops: 0, want: 6.84 },
      { id: "assembler_1_sloop", build: "Build_AssemblerMk1_C", clock: 100, sloops: 1, want: 33.75 },
      { id: "constructor_1_sloop", build: "Build_ConstructorMk1_C", clock: 100, sloops: 1, want: 16 },
      { id: "constructor_250_sloop", build: "Build_ConstructorMk1_C", clock: 250, sloops: 1, want: 53.7 },
    ]

    for (const tc of cases) {
      const got = buildingPowerMW(catalog, tc.build, tc.clock, tc.sloops)
      expect(got, tc.id).not.toBeNull()
      expect(
        withinPowerTolerance(got!, tc.want),
        `${tc.id}: got ${got}, want ${tc.want}`
      ).toBe(true)
    }
  })

  it("matches balance_iron_plate.json edge flows", () => {
    const raw = readFileSync(
      join(fixtureDir, "balance_iron_plate.json"),
      "utf-8"
    )
    const fixture = JSON.parse(raw) as {
      graph: PlanGraph
      expectedEdges: Record<
        string,
        { flowPerMin: number; recommendedMk: number; exceedsMax: boolean }
      >
    }

    const result = analyzeGraph(catalog, fixture.graph)

    for (const [edgeId, want] of Object.entries(fixture.expectedEdges)) {
      const got = result.edges[edgeId]
      expect(got, `missing edge ${edgeId}`).toBeDefined()
      expect(
        withinPowerTolerance(got.flowPerMin, want.flowPerMin),
        `edge ${edgeId} flow`
      ).toBe(true)
      expect(got.recommendedMk).toBe(want.recommendedMk)
      expect(got.exceedsMax).toBe(want.exceedsMax)
    }
  })
})
