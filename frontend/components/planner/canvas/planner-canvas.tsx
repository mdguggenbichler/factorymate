"use client"

import { useCallback, useEffect, useMemo } from "react"
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type Node,
  type OnConnect,
  type OnEdgesChange,
  type OnNodesChange,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"

import { FlowEdge } from "@/components/planner/canvas/flow-edge"
import { ProcessNode } from "@/components/planner/canvas/process-node"
import { SinkNode } from "@/components/planner/canvas/sink-node"
import { SourceNode } from "@/components/planner/canvas/source-node"
import {
  analyzeGraph,
  analyzePortStatus,
} from "@/lib/planner/balance"
import type { IndexedCatalog } from "@/lib/planner/catalog-types"
import type { PlanGraph } from "@/lib/planner/graph-types"
import {
  graphToReactFlow,
  isValidPlannerConnection,
  newEdgeId,
  PLANNER_EDGE_TYPE,
  PLANNER_NODE_TYPES,
  reactFlowToGraph,
  type FlowEdgeData,
  type PlannerNodeData,
} from "@/lib/planner/to-react-flow"

const nodeTypes = {
  [PLANNER_NODE_TYPES.process]: ProcessNode,
  [PLANNER_NODE_TYPES.source]: SourceNode,
  [PLANNER_NODE_TYPES.sink]: SinkNode,
}

const edgeTypes = {
  [PLANNER_EDGE_TYPE]: FlowEdge,
}

type PlannerCanvasProps = {
  catalog: IndexedCatalog
  graph: PlanGraph
  readOnly: boolean
  onGraphChange: (graph: PlanGraph) => void
  onNodeSelect: (nodeId: string | null) => void
}

export function PlannerCanvas({
  catalog,
  graph,
  readOnly,
  onGraphChange,
  onNodeSelect,
}: PlannerCanvasProps) {
  const balance = useMemo(
    () => analyzeGraph(catalog, graph),
    [catalog, graph]
  )

  const portStatusByNode = useMemo(() => {
    const out: Record<
      string,
      { unterminatedOutputs: string[]; starvedInputs: string[] }
    > = {}
    for (const node of graph.nodes) {
      out[node.id] = analyzePortStatus(catalog, graph, node)
    }
    return out
  }, [catalog, graph])

  const initial = useMemo(
    () => graphToReactFlow(graph, catalog, balance, portStatusByNode),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sync when graph identity changes
    [graph]
  )

  const [nodes, setNodes, onNodesChange] = useNodesState<Node<PlannerNodeData>>(
    initial.nodes
  )
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge<FlowEdgeData>>(
    initial.edges
  )

  useEffect(() => {
    const mapped = graphToReactFlow(graph, catalog, balance, portStatusByNode)
    setNodes(mapped.nodes)
    setEdges(mapped.edges)
  }, [graph, catalog, balance, portStatusByNode, setNodes, setEdges])

  const emitChange = useCallback(
    (
      nextNodes: typeof nodes,
      nextEdges: typeof edges,
      viewport = graph.viewport
    ) => {
      onGraphChange(reactFlowToGraph(nextNodes, nextEdges, viewport))
    },
    [graph.viewport, onGraphChange]
  )

  const handleNodesChange: OnNodesChange<Node<PlannerNodeData>> = useCallback(
    (changes) => {
      onNodesChange(changes)
      if (readOnly) return
      const hasPosition = changes.some(
        (c) => c.type === "position" && c.dragging === false
      )
      const hasRemove = changes.some((c) => c.type === "remove")
      if (hasPosition || hasRemove) {
        setNodes((current) => {
          setEdges((currentEdges) => {
            emitChange(current, currentEdges)
            return currentEdges
          })
          return current
        })
      }
    },
    [onNodesChange, readOnly, emitChange, setNodes, setEdges]
  )

  const handleEdgesChange: OnEdgesChange<Edge<FlowEdgeData>> = useCallback(
    (changes) => {
      onEdgesChange(changes)
      if (readOnly) return
      if (changes.some((c) => c.type === "remove")) {
        setEdges((currentEdges) => {
          setNodes((currentNodes) => {
            emitChange(currentNodes, currentEdges)
            return currentNodes
          })
          return currentEdges
        })
      }
    },
    [onEdgesChange, readOnly, emitChange, setNodes, setEdges]
  )

  const onConnect: OnConnect = useCallback(
    (connection: Connection) => {
      if (readOnly) return
      if (
        !isValidPlannerConnection(
          connection.sourceHandle,
          connection.targetHandle
        )
      ) {
        return
      }
      const itemClass = connection.sourceHandle!.slice(4)
      const newEdge = {
        id: newEdgeId(),
        type: PLANNER_EDGE_TYPE,
        source: connection.source!,
        target: connection.target!,
        sourceHandle: connection.sourceHandle!,
        targetHandle: connection.targetHandle!,
        data: {
          itemClass,
          flowPerMin: 0,
          recommendedMk: 1,
          unit: "items/min",
          exceedsMax: false,
          overPerMin: 0,
          underPerMin: 0,
        } satisfies FlowEdgeData,
      }
      setEdges((eds) => {
        const next = [...eds, newEdge]
        setNodes((nds) => {
          emitChange(nds, next)
          return nds
        })
        return next
      })
    },
    [readOnly, emitChange, setEdges, setNodes]
  )

  return (
    <div className="h-full w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={handleEdgesChange}
        onConnect={onConnect}
        isValidConnection={(c) =>
          isValidPlannerConnection(
            c.sourceHandle ?? null,
            c.targetHandle ?? null
          )
        }
        nodesDraggable={!readOnly}
        nodesConnectable={!readOnly}
        elementsSelectable
        deleteKeyCode={readOnly ? null : "Delete"}
        onSelectionChange={({ nodes: selected }) => {
          onNodeSelect(selected[0]?.id ?? null)
        }}
        defaultViewport={{
          x: graph.viewport.x,
          y: graph.viewport.y,
          zoom: graph.viewport.zoom,
        }}
        fitView={graph.nodes.length > 0 && graph.viewport.zoom === 1}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} size={1} />
        <Controls />
        <MiniMap zoomable pannable className="!bg-card" />
      </ReactFlow>
    </div>
  )
}
