"use client"

import Image from "next/image"
import { useTranslations } from "next-intl"
import { memo } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"

import {
  useCatalogBuilding,
  usePlannerCatalog,
} from "@/components/planner/planner-catalog-context"
import { plannerIconUrl } from "@/lib/planner/catalog-types"
import type { ProcessNodeData } from "@/lib/planner/to-react-flow"
import { cn } from "@/lib/utils"

function PortHandle({
  id,
  type,
  warn,
  fluid,
}: {
  id: string
  type: "source" | "target"
  warn?: boolean
  fluid?: boolean
}) {
  return (
    <Handle
      id={id}
      type={type}
      position={type === "target" ? Position.Left : Position.Right}
      className={cn(
        "!h-2.5 !w-2.5 !border-2 !bg-background",
        fluid ? "!border-sky-500" : "!border-muted-foreground",
        warn && "!border-amber-500 !bg-amber-500/20"
      )}
    />
  )
}

export const ProcessNode = memo(function ProcessNode({
  data,
  selected,
}: NodeProps & { data: ProcessNodeData }) {
  const t = useTranslations("planner")
  const catalog = usePlannerCatalog()
  const recipe = catalog.recipesByClass.get(data.recipeClass)
  const building = useCatalogBuilding(data.buildingClass)
  const recipeName = recipe?.displayName ?? data.recipeClass
  const buildingName = building?.displayName ?? data.buildingClass
  const iconClass = building?.iconClassName ?? data.buildingClass

  const warnSet = new Set([
    ...data.unterminatedOutputs.map((c) => `out:${c}`),
    ...data.starvedInputs.map((c) => `in:${c}`),
  ])

  return (
    <div
      className={cn(
        "min-w-[180px] rounded-lg border bg-card p-2 text-sm shadow-sm",
        selected && "ring-2 ring-primary",
        (data.unterminatedOutputs.length > 0 || data.starvedInputs.length > 0) &&
          "border-amber-500/60"
      )}
    >
      {recipe?.ingredients.map((ing) => (
        <PortHandle
          key={ing.className}
          id={`in:${ing.className}`}
          type="target"
          warn={warnSet.has(`in:${ing.className}`)}
          fluid={
            ing.className.includes("Liquid") || ing.className.includes("Gas")
          }
        />
      ))}

      <div className="flex items-start gap-2 px-1">
        <Image
          src={plannerIconUrl(iconClass)}
          alt=""
          width={32}
          height={32}
          className="size-8 shrink-0 rounded"
          unoptimized
        />
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium">{recipeName}</div>
          <div className="text-muted-foreground text-xs">
            {t("node.machineCount", {
              count: data.count,
              building: buildingName,
            })}
          </div>
          <div className="mt-1 flex flex-wrap gap-1 text-xs">
            <span className="rounded bg-muted px-1">
              {t("node.clock", { percent: data.clockPercent })}
            </span>
            {data.somersloopCount > 0 ? (
              <span className="rounded bg-muted px-1">
                {t("node.sloops", { count: data.somersloopCount })}
              </span>
            ) : null}
            <span className="rounded bg-muted px-1">
              {t("node.power", { mw: data.powerMW.toFixed(1) })}
            </span>
          </div>
        </div>
      </div>

      {recipe?.products.map((prod) => (
        <PortHandle
          key={prod.className}
          id={`out:${prod.className}`}
          type="source"
          warn={warnSet.has(`out:${prod.className}`)}
          fluid={
            prod.className.includes("Liquid") || prod.className.includes("Gas")
          }
        />
      ))}
    </div>
  )
})
