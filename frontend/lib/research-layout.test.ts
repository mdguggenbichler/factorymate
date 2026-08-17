import { describe, expect, it } from "vitest"

import type { ResearchNode } from "@/lib/api-types"
import {
  RESEARCH_CELL_HEIGHT,
  RESEARCH_CELL_WIDTH,
  buildCoordMap,
  computeBounds,
  computeEdges,
  coordKey,
  countResearchProgress,
  resolveParentCoords,
} from "@/lib/research-layout"

function node(
  id: string,
  x: number,
  y: number,
  state: string,
  parents: { x: number; y: number }[] = []
): ResearchNode {
  return {
    id,
    name: id,
    category: null,
    state,
    techTier: 1,
    cost: [],
    coordinates: { x, y },
    parents,
    updatedAt: "2026-08-17T00:00:00Z",
  }
}

/** Live FRM Quartz tree shape (orphan parent coords at 3,1 and 3,5). */
function quartzNodes(): ResearchNode[] {
  return [
    node("q0", 3, 0, "Purchased"),
    node("q1", 2, 1, "Purchased", [{ x: 3, y: 0 }]),
    node("q2", 4, 1, "Purchased", [{ x: 3, y: 0 }]),
    node("q3", 4, 2, "Purchased", [{ x: 4, y: 1 }]),
    node("q4", 1, 3, "Available", [{ x: 2, y: 1 }]),
    node("q5", 3, 3, "Available", [
      { x: 2, y: 1 },
      { x: 3, y: 1 },
      { x: 2, y: 1 },
    ]),
    node("q6", 2, 4, "Available", [{ x: 3, y: 3 }]),
    node("q7", 5, 4, "Purchased", [{ x: 4, y: 1 }]),
    node("q8", 4, 5, "Available", [{ x: 3, y: 3 }]),
    node("q9", 1, 6, "Available", [{ x: 3, y: 3 }]),
    node("q10", 2, 6, "Available", [
      { x: 3, y: 5 },
      { x: 3, y: 3 },
    ]),
    node("q11", 3, 7, "Available", [
      { x: 3, y: 5 },
      { x: 3, y: 3 },
    ]),
  ]
}

/** Live FRM Sulfur tree shape (orphan parent coords at 3,2 and 5,2). */
function sulfurNodes(): ResearchNode[] {
  return [
    node("s0", 3, 0, "Purchased"),
    node("s1", 3, 1, "Available", [{ x: 3, y: 0 }]),
    node("s2", 4, 1, "Available", [{ x: 3, y: 0 }]),
    node("s3", 2, 2, "Available", [{ x: 3, y: 1 }]),
    node("s4", 4, 2, "Available", [{ x: 4, y: 1 }]),
    node("s5", 1, 3, "Available", [{ x: 3, y: 1 }]),
    node("s6", 3, 3, "Available", [
      { x: 3, y: 2 },
      { x: 3, y: 2 },
      { x: 3, y: 1 },
    ]),
    node("s7", 5, 3, "Available", [
      { x: 4, y: 1 },
      { x: 4, y: 1 },
      { x: 5, y: 2 },
    ]),
    node("s8", 2, 4, "Available", [{ x: 3, y: 3 }]),
    node("s9", 3, 4, "Available", [{ x: 3, y: 3 }]),
    node("s10", 4, 4, "Available", [{ x: 3, y: 3 }]),
    node("s11", 5, 4, "Available", [{ x: 5, y: 3 }]),
    node("s12", 6, 4, "Available", [{ x: 5, y: 3 }]),
    node("s13", 1, 6, "Available", [{ x: 2, y: 4 }]),
    node("s14", 5, 6, "Available", [{ x: 4, y: 4 }]),
    node("s15", 3, 8, "Available", [
      { x: 2, y: 4 },
      { x: 4, y: 4 },
      { x: 5, y: 6 },
      { x: 2, y: 4 },
      { x: 4, y: 4 },
      { x: 1, y: 6 },
    ]),
  ]
}

function edgeParentKeys(edges: ReturnType<typeof computeEdges>): string[] {
  return edges.map((edge) => edge.key.split("->")[0])
}

function assertEdgesOnlyUseNodeCells(nodes: ResearchNode[]) {
  const coordMap = buildCoordMap(nodes)
  const bounds = computeBounds(nodes)!
  const edges = computeEdges(
    nodes,
    bounds,
    RESEARCH_CELL_WIDTH,
    RESEARCH_CELL_HEIGHT
  )

  for (const edge of edges) {
    const [parentKey, childKey] = edge.key.split("->")
    expect(coordMap.has(parentKey)).toBe(true)
    expect(coordMap.has(childKey)).toBe(true)
  }

  return edges
}

describe("computeBounds", () => {
  it("uses node positions only and ignores orphan parent coordinates", () => {
    const nodes = [
      node("parachute", 0, 3, "Purchased", [{ x: 2, y: 1 }]),
      node("fabric-child", 3, 4, "Available", [{ x: 2, y: 1 }]),
      node("medical", 5, 2, "Available", [{ x: 3, y: 0 }]),
    ]

    const bounds = computeBounds(nodes)
    expect(bounds).toEqual({ minX: 0, maxX: 5, minY: 2, maxY: 4 })
  })

  it("sizes nutrients tree to full grid span", () => {
    const nodes = [
      node("beryl", 2, 0, "Purchased"),
      node("paleberry", 3, 0, "Purchased"),
      node("bacon", 4, 0, "Purchased"),
      node("processor", 3, 2, "Available", [
        { x: 2, y: 0 },
        { x: 4, y: 0 },
        { x: 3, y: 0 },
      ]),
      node("inhaler", 3, 3, "Available", [{ x: 3, y: 2 }]),
    ]

    const bounds = computeBounds(nodes)
    expect(bounds).toEqual({ minX: 2, maxX: 4, minY: 0, maxY: 3 })
  })

  it("does not expand quartz bounds for phantom parent row y=5", () => {
    const bounds = computeBounds(quartzNodes())
    expect(bounds).toEqual({ minX: 1, maxX: 5, minY: 0, maxY: 7 })
  })
})

describe("resolveParentCoords", () => {
  it("resolves orphan junction to adjacent nodes", () => {
    const coordMap = buildCoordMap([
      node("nobelisk", 2, 2, "Available"),
      node("compact", 4, 2, "Available"),
      node("black", 3, 1, "Available"),
      node("smokeless", 3, 3, "Available"),
    ])

    const resolved = resolveParentCoords({ x: 3, y: 2 }, { x: 3, y: 3 }, coordMap)
    const keys = resolved.map((p) => coordKey(p.x, p.y)).sort()

    expect(keys).toEqual(["2,2", "3,1", "4,2"].sort())
  })
})

describe("computeEdges", () => {
  it("keeps all edge coordinates within canvas bounds", () => {
    const nodes = quartzNodes()
    const bounds = computeBounds(nodes)!
    const edges = computeEdges(
      nodes,
      bounds,
      RESEARCH_CELL_WIDTH,
      RESEARCH_CELL_HEIGHT
    )

    const width = (bounds.maxX - bounds.minX + 1) * RESEARCH_CELL_WIDTH
    const height = (bounds.maxY - bounds.minY + 1) * RESEARCH_CELL_HEIGHT

    for (const edge of edges) {
      expect(edge.y1).toBeGreaterThanOrEqual(0)
      expect(edge.y2).toBeGreaterThanOrEqual(0)
      expect(edge.x1).toBeGreaterThanOrEqual(0)
      expect(edge.x2).toBeGreaterThanOrEqual(0)
      expect(edge.y1).toBeLessThanOrEqual(height)
      expect(edge.y2).toBeLessThanOrEqual(height)
      expect(edge.x1).toBeLessThanOrEqual(width)
      expect(edge.x2).toBeLessThanOrEqual(width)
    }
  })

  it("deduplicates repeated parent coordinates", () => {
    const nodes = [
      node("parent", 2, 1, "Purchased"),
      node("child", 3, 4, "Available", [
        { x: 2, y: 1 },
        { x: 2, y: 1 },
      ]),
    ]

    const bounds = computeBounds(nodes)!
    const edges = computeEdges(
      nodes,
      bounds,
      RESEARCH_CELL_WIDTH,
      RESEARCH_CELL_HEIGHT
    )

    expect(edges).toHaveLength(1)
  })

  it("quartz tree edges only connect node cells", () => {
    const nodes = quartzNodes()
    const edges = assertEdgesOnlyUseNodeCells(nodes)
    const parentKeys = edgeParentKeys(edges)

    expect(parentKeys).not.toContain("3,1")
    expect(parentKeys).not.toContain("3,5")
    expect(parentKeys).toContain("4,5")
  })

  it("sulfur tree edges only connect node cells", () => {
    const nodes = sulfurNodes()
    const edges = assertEdgesOnlyUseNodeCells(nodes)
    const parentKeys = edgeParentKeys(edges)

    expect(parentKeys).not.toContain("3,2")
    expect(parentKeys).not.toContain("5,2")
    expect(parentKeys).toContain("2,2")
    expect(parentKeys).toContain("4,2")
  })
})

describe("countResearchProgress", () => {
  it("counts purchased nodes against total", () => {
    const nodes = [
      node("a", 0, 0, "Purchased"),
      node("b", 1, 0, "Available"),
      node("c", 2, 0, "Purchased"),
    ]

    expect(countResearchProgress(nodes)).toEqual({ purchased: 2, total: 3 })
  })
})
