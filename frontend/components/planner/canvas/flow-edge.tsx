"use client"

import { useTranslations } from "next-intl"
import { memo } from "react"
import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react"

import type { FlowEdgeData } from "@/lib/planner/to-react-flow"
import { cn } from "@/lib/utils"

export const FlowEdge = memo(function FlowEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  selected,
}: EdgeProps & { data?: FlowEdgeData }) {
  const t = useTranslations("planner")
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  })

  const flow = data?.flowPerMin ?? 0
  const mk = data?.recommendedMk ?? 1
  const unit = data?.unit ?? "items/min"
  const isPipe = unit === "m³/min"
  const over = (data?.overPerMin ?? 0) > 0.01
  const under = (data?.underPerMin ?? 0) > 0.01
  const exceeds = data?.exceedsMax ?? false

  let strokeClass = "stroke-muted-foreground"
  if (exceeds) {
    strokeClass = "stroke-destructive"
  } else if (over) {
    strokeClass = "stroke-amber-500"
  } else if (under) {
    strokeClass = "stroke-orange-600"
  }

  const label = isPipe
    ? t("edge.pipeLabel", { rate: flow.toFixed(1), mk })
    : t("edge.beltLabel", { rate: flow.toFixed(1), mk })

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        className={cn(strokeClass, selected && "!stroke-primary")}
        style={{ strokeWidth: selected ? 2.5 : 1.5 }}
      />
      <EdgeLabelRenderer>
        <div
          className={cn(
            "nodrag nopan pointer-events-auto absolute rounded border bg-background px-1.5 py-0.5 text-[10px] shadow-sm",
            exceeds && "border-destructive text-destructive",
            over && !exceeds && "border-amber-500 text-amber-700",
            under && !over && !exceeds && "border-orange-600 text-orange-700"
          )}
          style={{
            transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
          }}
          title={
            exceeds
              ? t("edge.exceedsMax")
              : over
                ? t("edge.overproduction")
                : under
                  ? t("edge.underproduction")
                  : undefined
          }
        >
          {label}
        </div>
      </EdgeLabelRenderer>
    </>
  )
})
