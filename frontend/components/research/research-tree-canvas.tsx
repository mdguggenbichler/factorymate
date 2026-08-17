"use client"

import { useMemo } from "react"
import { useTranslations } from "next-intl"

import { ResearchNodeCard } from "@/components/research/research-node"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import type { ResearchNode } from "@/lib/api-types"
import {
  RESEARCH_CELL_HEIGHT,
  RESEARCH_CELL_WIDTH,
  computeBounds,
  computeEdges,
  gridColumn,
  gridRow,
  hasLayoutData,
} from "@/lib/research-layout"

type ResearchTreeCanvasProps = {
  nodes: ResearchNode[]
  treeName: string
}

export function ResearchTreeCanvas({ nodes, treeName }: ResearchTreeCanvasProps) {
  const t = useTranslations("research")

  const layout = useMemo(() => {
    if (!hasLayoutData(nodes)) {
      return null
    }
    const bounds = computeBounds(nodes)
    if (!bounds) {
      return null
    }
    const cols = bounds.maxX - bounds.minX + 1
    const rows = bounds.maxY - bounds.minY + 1
    const width = cols * RESEARCH_CELL_WIDTH
    const height = rows * RESEARCH_CELL_HEIGHT
    const edges = computeEdges(nodes, bounds, RESEARCH_CELL_WIDTH, RESEARCH_CELL_HEIGHT)
    return { bounds, cols, rows, width, height, edges }
  }, [nodes])

  if (!layout) {
    return (
      <div className="space-y-4">
        <Card>
          <CardContent className="py-6 text-sm text-muted-foreground">
            {t("layoutPending")}
          </CardContent>
        </Card>
        <ul className="space-y-2">
          {nodes.map((node) => (
            <li
              key={node.id}
              className="flex items-center justify-between rounded-lg border px-3 py-2 text-sm"
            >
              <span className="font-medium">{node.name}</span>
              <Badge variant="outline">{node.state}</Badge>
            </li>
          ))}
        </ul>
      </div>
    )
  }

  return (
    <div className="overflow-auto rounded-lg border bg-muted/20 p-4">
      <div
        className="relative"
        style={{ width: layout.width, height: layout.height, minWidth: layout.width }}
        aria-label={treeName}
      >
        <svg
          className="pointer-events-none absolute inset-0 text-muted-foreground"
          width={layout.width}
          height={layout.height}
          aria-hidden
        >
          {layout.edges.map((edge) => (
            <line
              key={edge.key}
              x1={edge.x1}
              y1={edge.y1}
              x2={edge.x2}
              y2={edge.y2}
              stroke="currentColor"
              strokeWidth={2}
              strokeOpacity={0.45}
            />
          ))}
        </svg>

        <div
          className="relative grid"
          style={{
            gridTemplateColumns: `repeat(${layout.cols}, ${RESEARCH_CELL_WIDTH}px)`,
            gridTemplateRows: `repeat(${layout.rows}, ${RESEARCH_CELL_HEIGHT}px)`,
            width: layout.width,
            height: layout.height,
          }}
        >
          {nodes.map((node) => {
            if (!node.coordinates) {
              return null
            }
            return (
              <div
                key={node.id}
                className="flex items-center justify-center p-2"
                style={{
                  gridColumn: gridColumn(node.coordinates.x, layout.bounds),
                  gridRow: gridRow(node.coordinates.y, layout.bounds),
                }}
              >
                <ResearchNodeCard node={node} />
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
