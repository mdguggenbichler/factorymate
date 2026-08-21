import dagre from "@dagrejs/dagre"

import {
  PLANNER_LAYOUT_X_SPACING,
  PLANNER_LAYOUT_Y_SPACING,
} from "@/lib/planner/constants"
import type { PlanGraph } from "@/lib/planner/graph-types"

const NODE_WIDTH = 200
const NODE_HEIGHT = 100

export function applyDagreLayout(graph: PlanGraph): PlanGraph {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({
    rankdir: "LR",
    nodesep: PLANNER_LAYOUT_Y_SPACING,
    ranksep: PLANNER_LAYOUT_X_SPACING,
  })

  for (const node of graph.nodes) {
    g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT })
  }

  for (const edge of graph.edges) {
    g.setEdge(edge.sourceNodeId, edge.targetNodeId)
  }

  dagre.layout(g)

  const nodes = graph.nodes.map((node) => {
    const positioned = g.node(node.id)
    if (!positioned) {
      return node
    }
    return {
      ...node,
      x: positioned.x - NODE_WIDTH / 2,
      y: positioned.y - NODE_HEIGHT / 2,
    }
  })

  return { ...graph, nodes }
}
