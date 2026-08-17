"use client"

import { useMemo } from "react"
import { useTranslations } from "next-intl"

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import type { MilestoneGroup } from "@/lib/api-types"

type MilestonesViewProps = {
  groups: MilestoneGroup[]
}

const TAB_TYPES = ["Milestone", "Hard Drive", "Alternate"] as const

export function MilestonesView({ groups }: MilestonesViewProps) {
  const t = useTranslations("milestones")

  const groupsByType = useMemo(() => {
    const map = new Map<string, MilestoneGroup[]>()
    for (const group of groups) {
      const existing = map.get(group.type) ?? []
      existing.push(group)
      map.set(group.type, existing)
    }
    return map
  }, [groups])

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Tabs defaultValue="Milestone">
        <TabsList>
          {TAB_TYPES.map((type) => (
            <TabsTrigger key={type} value={type}>
              {t(`tabs.${typeToKey(type)}`)}
            </TabsTrigger>
          ))}
        </TabsList>

        {TAB_TYPES.map((type) => {
          const typeGroups = groupsByType.get(type) ?? []
          return (
            <TabsContent key={type} value={type}>
              {typeGroups.length === 0 ? (
                <Card>
                  <CardContent className="py-8 text-sm text-muted-foreground">
                    {t("empty")}
                  </CardContent>
                </Card>
              ) : (
                <Accordion multiple className="space-y-2">
                  {typeGroups.map((group) => (
                    <AccordionItem
                      key={`${group.type}-${group.techTier}`}
                      value={`${group.type}-${group.techTier}`}
                      className="rounded-lg border px-4"
                    >
                      <AccordionTrigger>
                        <div className="flex items-center gap-2">
                          <span>{t("tierGroup", { tier: group.techTier })}</span>
                          <Badge variant="outline">
                            {t("schematicCount", { count: group.schematics.length })}
                          </Badge>
                        </div>
                      </AccordionTrigger>
                      <AccordionContent>
                        <div className="space-y-3">
                          {group.schematics.map((schematic) => (
                            <Card key={schematic.id}>
                              <CardHeader className="pb-2">
                                <div className="flex flex-wrap items-center gap-2">
                                  <CardTitle className="text-base">
                                    {schematic.name}
                                  </CardTitle>
                                  <Badge
                                    variant={
                                      schematic.purchased ? "default" : "secondary"
                                    }
                                  >
                                    {schematic.purchased
                                      ? t("status.unlocked")
                                      : schematic.locked
                                        ? t("status.locked")
                                        : t("status.available")}
                                  </Badge>
                                </div>
                              </CardHeader>
                              {schematic.recipes.length > 0 ? (
                                <CardContent>
                                  <p className="mb-2 text-sm text-muted-foreground">
                                    {t("recipes")}
                                  </p>
                                  <div className="flex flex-wrap gap-1">
                                    {schematic.recipes.map((recipe) => (
                                      <Badge
                                        key={recipe.className}
                                        variant="outline"
                                      >
                                        {recipe.name}
                                      </Badge>
                                    ))}
                                  </div>
                                </CardContent>
                              ) : null}
                            </Card>
                          ))}
                        </div>
                      </AccordionContent>
                    </AccordionItem>
                  ))}
                </Accordion>
              )}
            </TabsContent>
          )
        })}
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
