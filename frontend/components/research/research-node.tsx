"use client"

import { useTranslations } from "next-intl"

import { ItemIcon } from "@/components/item-icon"
import { Badge } from "@/components/ui/badge"
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover"
import { formatNumber } from "@/lib/format"
import type { ResearchNode } from "@/lib/api-types"
import { cn } from "@/lib/utils"

type ResearchNodeCardProps = {
  node: ResearchNode
}

function stateLabelKey(state: string): string | null {
  switch (state) {
    case "Purchased":
      return "state.purchased"
    case "Available":
      return "state.available"
    case "Hidden":
      return "state.hidden"
    default:
      return null
  }
}

function nodeStateClasses(state: string): string {
  switch (state) {
    case "Purchased":
      return "border-emerald-500/60 bg-emerald-950 text-emerald-100 dark:bg-emerald-950 dark:text-emerald-300"
    case "Available":
      return "border-primary/70 bg-card text-foreground"
    case "Hidden":
      return "border-dashed border-muted-foreground/40 bg-muted text-muted-foreground"
    default:
      return "border-border bg-card text-foreground"
  }
}

export function ResearchNodeCard({ node }: ResearchNodeCardProps) {
  const t = useTranslations("research")
  const labelKey = stateLabelKey(node.state)
  const stateLabel = labelKey ? t(labelKey) : node.state
  const thumbnailClassName = node.cost[0]?.className

  return (
    <Popover>
      <PopoverTrigger
        className={cn(
          "flex h-20 w-full max-w-[120px] cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border px-2 py-1.5 text-center text-xs font-medium transition-colors hover:brightness-95",
          nodeStateClasses(node.state)
        )}
      >
        <ItemIcon className={thumbnailClassName} size={32} />
        <span className="line-clamp-2 leading-tight">{node.name}</span>
        <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
          {stateLabel}
        </Badge>
      </PopoverTrigger>
      <PopoverContent align="center" className="w-72">
        <PopoverHeader>
          <PopoverTitle>{node.name}</PopoverTitle>
          <PopoverDescription>
            {t("nodeDetails.tier", { tier: node.techTier ?? "—" })}
          </PopoverDescription>
        </PopoverHeader>
        <div className="flex flex-wrap gap-1">
          <Badge variant="outline">{stateLabel}</Badge>
          {node.category ? (
            <Badge variant="secondary">{node.category}</Badge>
          ) : null}
        </div>
        {node.cost.length > 0 ? (
          <div className="flex flex-col gap-1.5">
            <p className="text-xs font-medium text-muted-foreground">
              {t("nodeDetails.cost")}
            </p>
            <div className="flex flex-wrap gap-1">
              {node.cost.map((item) => (
                <Badge
                  key={`${node.id}-${item.name}`}
                  variant="secondary"
                  className="gap-1"
                >
                  <ItemIcon className={item.className} size={14} />
                  {t("costItem", {
                    name: item.name,
                    amount: formatNumber(item.amount, 0),
                  })}
                </Badge>
              ))}
            </div>
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}
