export type PlanViewport = {
  x: number
  y: number
  zoom: number
}

export type PlanNodeRole = "process" | "source" | "sink"

export type PlanNode = {
  id: string
  role: PlanNodeRole
  recipeClass?: string
  buildingClass?: string
  itemClass?: string
  count?: number
  clockPercent?: number
  somersloopCount?: number
  ratePerMin?: number
  x: number
  y: number
}

export type PlanEdge = {
  id: string
  sourceNodeId: string
  sourcePort: string
  targetNodeId: string
  targetPort: string
  itemClass: string
}

export type PlanGraph = {
  viewport: PlanViewport
  nodes: PlanNode[]
  edges: PlanEdge[]
}

export type SolverOptions = {
  recipeByProductClass?: Record<string, string>
  defaultClockPercent?: number
  defaultSomersloopCount?: number
}

export function emptyPlanGraph(): PlanGraph {
  return {
    viewport: { x: 0, y: 0, zoom: 1 },
    nodes: [],
    edges: [],
  }
}

export function portId(direction: "in" | "out", itemClass: string): string {
  return `${direction}:${itemClass}`
}

export function parsePortId(
  port: string
): { direction: "in" | "out"; itemClass: string } | null {
  const match = port.match(/^(in|out):(.+)$/)
  if (!match) {
    return null
  }
  return {
    direction: match[1] as "in" | "out",
    itemClass: match[2],
  }
}
