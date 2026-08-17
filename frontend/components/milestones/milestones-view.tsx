"use client"

import { useMemo } from "react"
import { useTranslations } from "next-intl"

import { MilestoneTierLadder } from "@/components/milestones/milestone-tier-ladder"
import { MilestoneTypeSections } from "@/components/milestones/milestone-type-sections"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import type { MilestoneGroup } from "@/lib/api-types"
import {
  buildTierLadder,
  countMilestoneProgress,
  filterGroupsByType,
  flattenSchematics,
  highestCompleteTier,
  nextPartialTier,
} from "@/lib/milestone-layout"

type MilestonesViewProps = {
  groups: MilestoneGroup[]
}

const TAB_TYPES = ["Milestone", "Hard Drive", "Alternate"] as const

export function MilestonesView({ groups }: MilestonesViewProps) {
  const t = useTranslations("milestones")

  const groupsByType = useMemo(() => {
    const map = new Map<string, MilestoneGroup[]>()
    for (const type of TAB_TYPES) {
      map.set(type, filterGroupsByType(groups, type))
    }
    return map
  }, [groups])

  const milestoneGroups = useMemo(
    () => groupsByType.get("Milestone") ?? [],
    [groupsByType]
  )
  const milestoneLadder = useMemo(
    () => buildTierLadder(milestoneGroups),
    [milestoneGroups]
  )
  const milestoneProgress = useMemo(
    () => countMilestoneProgress(flattenSchematics(milestoneGroups)),
    [milestoneGroups]
  )
  const highestTier = useMemo(
    () => highestCompleteTier(milestoneLadder),
    [milestoneLadder]
  )
  const nextTier = useMemo(
    () => nextPartialTier(milestoneLadder),
    [milestoneLadder]
  )

  function tabProgress(type: (typeof TAB_TYPES)[number]) {
    const typeGroups = groupsByType.get(type) ?? []
    return countMilestoneProgress(flattenSchematics(typeGroups))
  }

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Tabs defaultValue="Milestone">
        <TabsList className="h-auto flex-wrap">
          {TAB_TYPES.map((type) => {
            const { purchased, total } = tabProgress(type)
            return (
              <TabsTrigger key={type} value={type} className="gap-2">
                <span>{t(`tabs.${typeToKey(type)}`)}</span>
                {total > 0 ? (
                  <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                    {t("tabProgress", { purchased, total })}
                  </Badge>
                ) : null}
              </TabsTrigger>
            )
          })}
        </TabsList>

        <TabsContent value="Milestone" className="space-y-6">
          {milestoneGroups.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-sm text-muted-foreground">
                {t("empty")}
              </CardContent>
            </Card>
          ) : (
            <>
              <Card>
                <CardContent className="flex flex-col gap-2 py-4 sm:flex-row sm:flex-wrap sm:items-center sm:gap-6">
                  <p className="text-sm">
                    {t("progressSummary", {
                      purchased: milestoneProgress.purchased,
                      total: milestoneProgress.total,
                    })}
                  </p>
                  {highestTier != null ? (
                    <p className="text-sm text-muted-foreground">
                      {t("highestTierComplete", { tier: highestTier })}
                    </p>
                  ) : null}
                  {nextTier != null &&
                  (highestTier == null || nextTier > highestTier) ? (
                    <p className="text-sm text-muted-foreground">
                      {t("nextTierHint", { tier: nextTier })}
                    </p>
                  ) : null}
                </CardContent>
              </Card>
              <MilestoneTierLadder ladder={milestoneLadder} />
            </>
          )}
        </TabsContent>

        <TabsContent value="Hard Drive">
          <MilestoneTypeSections
            groups={groupsByType.get("Hard Drive") ?? []}
            variant="hardDrive"
          />
        </TabsContent>

        <TabsContent value="Alternate">
          <MilestoneTypeSections
            groups={groupsByType.get("Alternate") ?? []}
            variant="alternate"
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function typeToKey(type: string): "milestone" | "hardDrive" | "alternate" {
  if (type === "Hard Drive") {
    return "hardDrive"
  }
  if (type === "Alternate") {
    return "alternate"
  }
  return "milestone"
}
