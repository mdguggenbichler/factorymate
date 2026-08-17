import { describe, expect, it } from "vitest"

import type { ResearchNode } from "@/lib/api-types"
import {
  RESEARCH_CELL_HEIGHT,
  RESEARCH_CELL_WIDTH,
  computeBounds,
  computeEdges,
  countResearchProgress,
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

describe("computeBounds", () => {
  it("includes parent coordinates outside node positions", () => {
    const nodes = [
      node("parachute", 0, 3, "Purchased", [{ x: 2, y: 1 }]),
      node("fabric-child", 3, 4, "Available", [{ x: 2, y: 1 }]),
      node("medical", 5, 2, "Available", [{ x: 3, y: 0 }]),
    ]

    const bounds = computeBounds(nodes)
    expect(bounds).toEqual({ minX: 0, maxX: 5, minY: 0, maxY: 4 })
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
})

describe("computeEdges", () => {
  it("keeps all edge coordinates within canvas bounds", () => {
    const nodes = [
      node("parachute", 0, 3, "Purchased", [{ x: 2, y: 1 }]),
      node("fabric-child", 3, 4, "Available", [{ x: 2, y: 1 }]),
      node("medical", 5, 2, "Available", [{ x: 3, y: 0 }]),
    ]

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
      node("fabric", 3, 4, "Available", [
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
