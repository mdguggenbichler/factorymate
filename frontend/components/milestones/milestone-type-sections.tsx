"use client"

import { useTranslations } from "next-intl"

import { MilestoneSchematicCard } from "@/components/milestones/milestone-schematic-card"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import type { MilestoneGroup, MilestoneSchematic } from "@/lib/api-types"
import { buildTierLadder, readyHardDrives } from "@/lib/milestone-layout"

type MilestoneTypeSectionsProps = {
  groups: MilestoneGroup[]
  variant: "hardDrive" | "alternate"
}

export function MilestoneTypeSections({
  groups,
  variant,
}: MilestoneTypeSectionsProps) {
  const t = useTranslations("milestones")
  const ladder = buildTierLadder(groups)

  if (ladder.length === 0) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-muted-foreground">
          {t("empty")}
        </CardContent>
      </Card>
    )
  }

  const allSchematics = groups.flatMap((g) => g.schematics)
  const ready = variant === "hardDrive" ? readyHardDrives(allSchematics) : []

  return (
    <div className="space-y-6">
      {variant === "hardDrive" && ready.length > 0 ? (
        <Card className="border-primary/50 bg-primary/5">
          <CardHeader className="pb-2">
            <CardTitle className="text-base">{t("hardDriveReady")}</CardTitle>
            <CardDescription>{t("hardDriveReadyDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-3">
              {ready.map((schematic) => (
                <HardDriveReadyCard
                  key={schematic.id}
                  schematic={schematic}
                  techTier={
                    groups.find((g) =>
                      g.schematics.some((s) => s.id === schematic.id)
                    )?.techTier ?? 0
                  }
                />
              ))}
            </div>
          </CardContent>
        </Card>
      ) : null}

      {ladder.map((row) => (
        <section key={row.techTier} className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-semibold">
              {t("tierLabel", { tier: row.techTier })}
            </h3>
            <Badge variant="outline" className="text-[10px]">
              {t("schematicCount", { count: row.schematics.length })}
            </Badge>
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
        </section>
      ))}
    </div>
  )
}

function HardDriveReadyCard({
  schematic,
  techTier,
}: {
  schematic: MilestoneSchematic
  techTier: number
}) {
  const t = useTranslations("milestones")

  return (
    <div className="space-y-2 rounded-lg border border-primary/40 bg-card p-3">
      <MilestoneSchematicCard schematic={schematic} techTier={techTier} />
      {schematic.recipes.length > 0 ? (
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">
            {t("recipes")}
          </p>
          <div className="flex flex-wrap gap-1">
            {schematic.recipes.map((recipe) => (
              <Badge key={recipe.className} variant="secondary">
                {recipe.name}
              </Badge>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}
