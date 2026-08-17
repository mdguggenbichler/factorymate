"use client"

import { useTranslations } from "next-intl"

import { ResearchTreeCanvas } from "@/components/research/research-tree-canvas"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import type { ResearchTree } from "@/lib/api-types"

type ResearchViewProps = {
  trees: ResearchTree[]
}

export function ResearchView({ trees }: ResearchViewProps) {
  const t = useTranslations("research")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      {trees.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-sm text-muted-foreground">
            {t("empty")}
          </CardContent>
        </Card>
      ) : (
        <Tabs defaultValue={trees[0].name}>
          <TabsList className="h-auto flex-wrap">
            {trees.map((tree) => (
              <TabsTrigger key={tree.name} value={tree.name} className="gap-2">
                <span>{tree.name}</span>
                <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                  {t("nodeCount", { count: tree.nodes.length })}
                </Badge>
              </TabsTrigger>
            ))}
          </TabsList>

          {trees.map((tree) => (
            <TabsContent key={tree.name} value={tree.name}>
              <ResearchTreeCanvas nodes={tree.nodes} treeName={tree.name} />
            </TabsContent>
          ))}
        </Tabs>
      )}
    </div>
  )
}
