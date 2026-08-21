"use client"

import Image from "next/image"
import { useTranslations } from "next-intl"
import { memo } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"

import { useCatalogDisplayName } from "@/components/planner/planner-catalog-context"
import { plannerIconUrl } from "@/lib/planner/catalog-types"
import type { SinkNodeData } from "@/lib/planner/to-react-flow"
import { cn } from "@/lib/utils"

export const SinkNode = memo(function SinkNode({
  data,
  selected,
}: NodeProps & { data: SinkNodeData }) {
  const t = useTranslations("planner")
  const itemName = useCatalogDisplayName(data.itemClass)
  const warn = data.starvedInputs.length > 0

  return (
    <div
      className={cn(
        "min-w-[120px] rounded-lg border border-dashed bg-card p-2 text-sm shadow-sm",
        selected && "ring-2 ring-primary",
        warn && "border-amber-500"
      )}
    >
      <Handle
        id={`in:${data.itemClass}`}
        type="target"
        position={Position.Left}
        className={cn(
          "!h-2.5 !w-2.5 !border-2 !bg-background !border-muted-foreground",
          warn && "!border-amber-500"
        )}
      />
      <div className="flex items-center gap-2">
        <Image
          src={plannerIconUrl(data.itemClass)}
          alt=""
          width={28}
          height={28}
          className="size-7 shrink-0 rounded opacity-70"
          unoptimized
        />
        <div className="min-w-0">
          <div className="truncate font-medium">{itemName}</div>
          <div className="text-muted-foreground text-xs">{t("node.sink")}</div>
        </div>
      </div>
    </div>
  )
})
