"use client"

import { useTranslations } from "next-intl"

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatNumber } from "@/lib/format"
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
        <Accordion multiple className="space-y-2">
          {trees.map((tree) => (
            <AccordionItem
              key={tree.name}
              value={tree.name}
              className="rounded-lg border px-4"
            >
              <AccordionTrigger>
                <div className="flex items-center gap-2">
                  <span>{tree.name}</span>
                  <Badge variant="outline">
                    {t("nodeCount", { count: tree.nodes.length })}
                  </Badge>
                </div>
              </AccordionTrigger>
              <AccordionContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.state")}</TableHead>
                      <TableHead>{t("columns.tier")}</TableHead>
                      <TableHead>{t("columns.cost")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tree.nodes.map((node) => (
                      <TableRow key={node.id}>
                        <TableCell className="font-medium">{node.name}</TableCell>
                        <TableCell>
                          <Badge variant="outline">{node.state}</Badge>
                        </TableCell>
                        <TableCell>{node.techTier ?? "—"}</TableCell>
                        <TableCell>
                          {node.cost.length === 0 ? (
                            "—"
                          ) : (
                            <div className="flex flex-wrap gap-1">
                              {node.cost.map((item) => (
                                <Badge key={`${node.id}-${item.name}`} variant="secondary">
                                  {t("costItem", {
                                    name: item.name,
                                    amount: formatNumber(item.amount, 0),
                                  })}
                                </Badge>
                              ))}
                            </div>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      )}
    </div>
  )
}
