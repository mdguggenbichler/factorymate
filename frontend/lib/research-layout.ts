import type { ResearchNode } from "@/lib/api-types"

export const RESEARCH_CELL_WIDTH = 140
export const RESEARCH_CELL_HEIGHT = 100

export type ResearchBounds = {
  minX: number
  maxX: number
  minY: number
  maxY: number
}

export type ResearchEdge = {
  key: string
  x1: number
  y1: number
  x2: number
  y2: number
}

export type ResearchProgress = {
  purchased: number
  total: number
}

export function coordKey(x: number, y: number): string {
  return `${x},${y}`
}

export function hasLayoutData(nodes: ResearchNode[]): boolean {
  return nodes.length > 0 && nodes.every((node) => node.coordinates !== null)
}

export function countResearchProgress(nodes: ResearchNode[]): ResearchProgress {
  const total = nodes.length
  const purchased = nodes.filter((node) => node.state === "Purchased").length
  return { purchased, total }
}

function isValidCoord(x: number | null | undefined, y: number | null | undefined): boolean {
  return x != null && y != null && !Number.isNaN(x) && !Number.isNaN(y)
}

function expandBounds(
  bounds: ResearchBounds,
  x: number,
  y: number
): ResearchBounds {
  return {
    minX: Math.min(bounds.minX, x),
    maxX: Math.max(bounds.maxX, x),
    minY: Math.min(bounds.minY, y),
    maxY: Math.max(bounds.maxY, y),
  }
}

export function computeBounds(nodes: ResearchNode[]): ResearchBounds | null {
  const positioned = nodes.filter((node) => node.coordinates !== null)
  if (positioned.length === 0) {
    return null
  }

  let bounds: ResearchBounds = {
    minX: positioned[0].coordinates!.x,
    maxX: positioned[0].coordinates!.x,
    minY: positioned[0].coordinates!.y,
    maxY: positioned[0].coordinates!.y,
  }

  for (const node of positioned) {
    const { x, y } = node.coordinates!
    bounds = expandBounds(bounds, x, y)

    for (const parent of node.parents) {
      if (isValidCoord(parent.x, parent.y)) {
        bounds = expandBounds(bounds, parent.x, parent.y)
      }
    }
  }

  return bounds
}

export function buildCoordMap(nodes: ResearchNode[]): Map<string, ResearchNode> {
  const map = new Map<string, ResearchNode>()
  for (const node of nodes) {
    if (!node.coordinates) {
      continue
    }
    map.set(coordKey(node.coordinates.x, node.coordinates.y), node)
  }
  return map
}

function cellCenter(
  x: number,
  y: number,
  bounds: ResearchBounds,
  cellWidth: number,
  cellHeight: number
): { x: number; y: number } {
  return {
    x: (x - bounds.minX + 0.5) * cellWidth,
    y: (y - bounds.minY + 0.5) * cellHeight,
  }
}

export function computeEdges(
  nodes: ResearchNode[],
  bounds: ResearchBounds,
  cellWidth: number,
  cellHeight: number
): ResearchEdge[] {
  const edges: ResearchEdge[] = []
  const seen = new Set<string>()

  for (const node of nodes) {
    if (!node.coordinates) {
      continue
    }
    const childCenter = cellCenter(
      node.coordinates.x,
      node.coordinates.y,
      bounds,
      cellWidth,
      cellHeight
    )

    for (const parent of node.parents) {
      if (!isValidCoord(parent.x, parent.y)) {
        continue
      }

      const edgeKey = `${coordKey(parent.x, parent.y)}->${coordKey(node.coordinates.x, node.coordinates.y)}`
      if (seen.has(edgeKey)) {
        continue
      }
      seen.add(edgeKey)

      const parentCenter = cellCenter(parent.x, parent.y, bounds, cellWidth, cellHeight)
      edges.push({
        key: edgeKey,
        x1: parentCenter.x,
        y1: parentCenter.y,
        x2: childCenter.x,
        y2: childCenter.y,
      })
    }
  }

  return edges
}

export function gridColumn(x: number, bounds: ResearchBounds): number {
  return x - bounds.minX + 1
}

export function gridRow(y: number, bounds: ResearchBounds): number {
  return y - bounds.minY + 1
}
