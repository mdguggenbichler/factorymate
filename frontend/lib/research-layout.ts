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

export function coordKey(x: number, y: number): string {
  return `${x},${y}`
}

export function hasLayoutData(nodes: ResearchNode[]): boolean {
  return nodes.length > 0 && nodes.every((node) => node.coordinates !== null)
}

export function computeBounds(nodes: ResearchNode[]): ResearchBounds | null {
  const positioned = nodes.filter((node) => node.coordinates !== null)
  if (positioned.length === 0) {
    return null
  }

  let minX = positioned[0].coordinates!.x
  let maxX = minX
  let minY = positioned[0].coordinates!.y
  let maxY = minY

  for (const node of positioned) {
    const { x, y } = node.coordinates!
    minX = Math.min(minX, x)
    maxX = Math.max(maxX, x)
    minY = Math.min(minY, y)
    maxY = Math.max(maxY, y)
  }

  return { minX, maxX, minY, maxY }
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
      const parentCenter = cellCenter(parent.x, parent.y, bounds, cellWidth, cellHeight)
      edges.push({
        key: `${coordKey(parent.x, parent.y)}->${coordKey(node.coordinates.x, node.coordinates.y)}`,
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
