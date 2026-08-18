"use client"

import { useTranslations } from "next-intl"

import { MilestoneSchematicCard } from "@/components/milestones/milestone-schematic-card"
import { Badge } from "@/components/ui/badge"
import type { TierLadderRow } from "@/lib/milestone-layout"
import { cn } from "@/lib/utils"

type MilestoneTierLadderProps = {
  ladder: TierLadderRow[]
}

export function MilestoneTierLadder({ ladder }: MilestoneTierLadderProps) {
  const t = useTranslations("milestones")

  if (ladder.length === 0) {
    return null
  }

  return (
    <div className="relative space-y-6">
      {ladder.map((row, index) => {
        const isLast = index === ladder.length - 1
        const tierProgress = row.schematics.filter((s) => s.purchased).length

        return (
          <div key={row.techTier} className="relative flex gap-4">
            <div className="flex w-8 shrink-0 flex-col items-center">
              <div
                className={cn(
                  "z-10 flex size-8 shrink-0 items-center justify-center rounded-full border-2 text-xs font-semibold",
                  row.allPurchased
                    ? "border-emerald-500 bg-emerald-950 text-emerald-300"
                    : row.anyPurchased
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-muted-foreground/40 bg-muted text-muted-foreground"
                )}
              >
                {row.techTier}
              </div>
              {!isLast ? (
                <div
                  className={cn(
                    "mt-1 w-0.5 flex-1 min-h-8",
                    row.allPurchased
                      ? "bg-emerald-500/60"
                      : "bg-muted-foreground/25"
                  )}
                  aria-hidden
                />
              ) : null}
            </div>

            <div className="min-w-0 flex-1 space-y-3 pb-2">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="text-sm font-semibold">
                  {t("tierLabel", { tier: row.techTier })}
                </h3>
                <Badge variant="outline" className="text-[10px]">
                  {t("tabProgress", {
                    purchased: tierProgress,
                    total: row.schematics.length,
                  })}
                </Badge>
                {row.allPurchased ? (
                  <Badge className="bg-emerald-950 text-[10px] text-emerald-300">
                    {t("tierComplete")}
                  </Badge>
                ) : row.anyPurchased ? (
                  <Badge variant="secondary" className="text-[10px]">
                    {t("tierPartial")}
                  </Badge>
                ) : null}
              </div>

              <div className="flex flex-wrap gap-3">
                {row.schematics.map((schematic) => (
                  <MilestoneSchematicCard
                    key={schematic.id}
                    schematic={schematic}
                    techTier={row.techTier}
                  />
                ))}
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
