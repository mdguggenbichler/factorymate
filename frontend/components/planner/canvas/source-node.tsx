"use client"

import Image from "next/image"
import { useTranslations } from "next-intl"
import { memo } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"

import {
  useCatalogDisplayName,
  usePlannerCatalog,
} from "@/components/planner/planner-catalog-context"
import { plannerIconUrl } from "@/lib/planner/catalog-types"
import type { SourceNodeData } from "@/lib/planner/to-react-flow"
import { cn } from "@/lib/utils"

export const SourceNode = memo(function SourceNode({
  data,
  selected,
}: NodeProps & { data: SourceNodeData }) {
  const t = useTranslations("planner")
  const catalog = usePlannerCatalog()
  const itemName = useCatalogDisplayName(data.itemClass)
  const item = catalog.itemsByClass.get(data.itemClass)
  const isFluid = item?.form === "liquid" || item?.form === "gas"
  const rate = data.ratePerMin * (data.count || 1)
  const warn = data.unterminatedOutputs.length > 0

  return (
    <div
      className={cn(
        "min-w-[140px] rounded-lg border bg-card p-2 text-sm shadow-sm",
        selected && "ring-2 ring-primary",
        warn && "border-amber-500/60"
      )}
    >
      <Handle
        id={`out:${data.itemClass}`}
        type="source"
        position={Position.Right}
        className={cn(
          "!h-2.5 !w-2.5 !border-2 !bg-background",
          isFluid ? "!border-sky-500" : "!border-muted-foreground",
          warn && "!border-amber-500"
        )}
      />
      <div className="flex items-center gap-2">
        <Image
          src={plannerIconUrl(data.itemClass)}
          alt=""
          width={28}
          height={28}
          className="size-7 shrink-0 rounded"
          unoptimized
        />
        <div className="min-w-0">
          <div className="truncate font-medium">{itemName}</div>
          <div className="text-muted-foreground text-xs">
            {t("node.sourceRate", {
              rate: rate.toFixed(1),
              unit: isFluid ? "m³/min" : "items/min",
            })}
          </div>
        </div>
      </div>
    </div>
  )
})
