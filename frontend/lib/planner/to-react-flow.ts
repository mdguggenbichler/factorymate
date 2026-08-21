import type { Edge, Node } from "@xyflow/react"

import type { BalanceResult } from "@/lib/planner/balance"
import type { IndexedCatalog } from "@/lib/planner/catalog-types"
import type { PlanEdge, PlanGraph, PlanNode } from "@/lib/planner/graph-types"

export type ProcessNodeData = {
  role: "process"
  recipeClass: string
  buildingClass: string
  count: number
  clockPercent: number
  somersloopCount: number
  unterminatedOutputs: string[]
  starvedInputs: string[]
  powerMW: number
}

export type SourceNodeData = {
  role: "source"
  itemClass: string
  ratePerMin: number
  count: number
  unterminatedOutputs: string[]
  starvedInputs: string[]
}

export type SinkNodeData = {
  role: "sink"
  itemClass: string
  ratePerMin: number
  unterminatedOutputs: string[]
  starvedInputs: string[]
}

export type PlannerNodeData = ProcessNodeData | SourceNodeData | SinkNodeData

export type FlowEdgeData = {
  itemClass: string
  flowPerMin: number
  recommendedMk: number
  unit: string
  exceedsMax: boolean
  overPerMin: number
  underPerMin: number
}

export const PLANNER_NODE_TYPES = {
  process: "process",
  source: "source",
  sink: "sink",
} as const

export const PLANNER_EDGE_TYPE = "flow"

export function graphToReactFlow(
  graph: PlanGraph,
  catalog: IndexedCatalog,
  balance: BalanceResult,
  portStatusByNode: Record<string, { unterminatedOutputs: string[]; starvedInputs: string[] }>
): { nodes: Node<PlannerNodeData>[]; edges: Edge<FlowEdgeData>[] } {
  const nodes: Node<PlannerNodeData>[] = graph.nodes.map((node) => {
    const ports = portStatusByNode[node.id] ?? {
      unterminatedOutputs: [],
      starvedInputs: [],
    }
    const rates = balance.nodes[node.id]

    if (node.role === "source") {
      const data: SourceNodeData = {
        role: "source",
        itemClass: node.itemClass ?? "",
        ratePerMin: node.ratePerMin ?? 0,
        count: node.count ?? 1,
        unterminatedOutputs: ports.unterminatedOutputs,
        starvedInputs: ports.starvedInputs,
      }
      return {
        id: node.id,
        type: PLANNER_NODE_TYPES.source,
        position: { x: node.x, y: node.y },
        data,
      }
    }

    if (node.role === "sink") {
      const data: SinkNodeData = {
        role: "sink",
        itemClass: node.itemClass ?? "",
        ratePerMin: node.ratePerMin ?? 0,
        unterminatedOutputs: ports.unterminatedOutputs,
        starvedInputs: ports.starvedInputs,
      }
      return {
        id: node.id,
        type: PLANNER_NODE_TYPES.sink,
        position: { x: node.x, y: node.y },
        data,
      }
    }

    const data: ProcessNodeData = {
      role: "process",
      recipeClass: node.recipeClass ?? "",
      buildingClass: node.buildingClass ?? "",
      count: node.count ?? 1,
      clockPercent: node.clockPercent ?? 100,
      somersloopCount: node.somersloopCount ?? 0,
      unterminatedOutputs: ports.unterminatedOutputs,
      starvedInputs: ports.starvedInputs,
      powerMW: rates?.powerMW ?? 0,
    }

    return {
      id: node.id,
      type: PLANNER_NODE_TYPES.process,
      position: { x: node.x, y: node.y },
      data,
    }
  })

  const edges: Edge<FlowEdgeData>[] = graph.edges.map((edge) => {
    const eb = balance.edges[edge.id]
    return {
      id: edge.id,
      type: PLANNER_EDGE_TYPE,
      source: edge.sourceNodeId,
      target: edge.targetNodeId,
      sourceHandle: edge.sourcePort,
      targetHandle: edge.targetPort,
      data: {
        itemClass: edge.itemClass,
        flowPerMin: eb?.flowPerMin ?? 0,
        recommendedMk: eb?.recommendedMk ?? 1,
        unit: eb?.unit ?? "items/min",
        exceedsMax: eb?.exceedsMax ?? false,
        overPerMin: eb?.overPerMin ?? 0,
        underPerMin: eb?.underPerMin ?? 0,
      },
    }
  })

  return { nodes, edges }
}

export function reactFlowToGraph(
  nodes: Node<PlannerNodeData>[],
  edges: Edge<FlowEdgeData>[],
  viewport: PlanGraph["viewport"]
): PlanGraph {
  const planNodes: PlanNode[] = nodes.map((node) => {
    const base: PlanNode = {
      id: node.id,
      role: node.data.role,
      x: node.position.x,
      y: node.position.y,
    }

    if (node.data.role === "source") {
      return {
        ...base,
        itemClass: node.data.itemClass,
        ratePerMin: node.data.ratePerMin,
        count: node.data.count,
      }
    }

    if (node.data.role === "sink") {
      return {
        ...base,
        itemClass: node.data.itemClass,
        ratePerMin: node.data.ratePerMin,
      }
    }

    return {
      ...base,
      role: "process",
      recipeClass: node.data.recipeClass,
      buildingClass: node.data.buildingClass,
      count: node.data.count,
      clockPercent: node.data.clockPercent,
      somersloopCount: node.data.somersloopCount,
    }
  })

  const planEdges: PlanEdge[] = edges.map((edge) => ({
    id: edge.id,
    sourceNodeId: edge.source,
    sourcePort: edge.sourceHandle ?? "",
    targetNodeId: edge.target,
    targetPort: edge.targetHandle ?? "",
    itemClass: edge.data?.itemClass ?? "",
  }))

  return {
    viewport,
    nodes: planNodes,
    edges: planEdges,
  }
}

export function newNodeId(): string {
  return `n_${crypto.randomUUID().slice(0, 8)}`
}

export function newEdgeId(): string {
  return `e_${crypto.randomUUID().slice(0, 8)}`
}

export function isValidPlannerConnection(
  sourceHandle: string | null,
  targetHandle: string | null
): boolean {
  if (!sourceHandle || !targetHandle) {
    return false
  }
  if (!sourceHandle.startsWith("out:") || !targetHandle.startsWith("in:")) {
    return false
  }
  const sourceItem = sourceHandle.slice(4)
  const targetItem = targetHandle.slice(3)
  return sourceItem === targetItem && sourceItem.length > 0
}
