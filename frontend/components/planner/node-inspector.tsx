"use client"

import { useTranslations } from "next-intl"

import {
  useCatalogBuilding,
  useCatalogRecipe,
  usePlannerCatalog,
} from "@/components/planner/planner-catalog-context"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import type { PlanGraph, PlanNode } from "@/lib/planner/graph-types"

type NodeInspectorProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  nodeId: string | null
  graph: PlanGraph
  readOnly: boolean
  onUpdate: (graph: PlanGraph) => void
}

export function NodeInspector({
  open,
  onOpenChange,
  nodeId,
  graph,
  readOnly,
  onUpdate,
}: NodeInspectorProps) {
  const t = useTranslations("planner")
  const catalog = usePlannerCatalog()
  const node = graph.nodes.find((n) => n.id === nodeId)

  const recipe = useCatalogRecipe(node?.recipeClass ?? "")
  const building = useCatalogBuilding(node?.buildingClass ?? "")

  const recipeOptions =
    node?.buildingClass != null
      ? catalog.recipes.filter((r) =>
          r.producedIn.includes(node.buildingClass!)
        )
      : []

  if (!node) {
    return null
  }

  function patch(updates: Partial<PlanNode>) {
    onUpdate({
      ...graph,
      nodes: graph.nodes.map((n) =>
        n.id === node!.id ? { ...n, ...updates } : n
      ),
    })
  }

  if (node.role === "source") {
    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t("inspector.sourceTitle")}</SheetTitle>
            <SheetDescription>
              {catalog.itemsByClass.get(node.itemClass ?? "")?.displayName}
            </SheetDescription>
          </SheetHeader>
          <FieldGroup className="mt-4">
            <Field>
              <FieldLabel>{t("inspector.rate")}</FieldLabel>
              <Input
                type="number"
                min={0}
                step={0.1}
                disabled={readOnly}
                value={node.ratePerMin ?? 0}
                onChange={(e) =>
                  patch({ ratePerMin: Number(e.target.value) })
                }
              />
            </Field>
          </FieldGroup>
        </SheetContent>
      </Sheet>
    )
  }

  if (node.role === "sink") {
    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t("inspector.sinkTitle")}</SheetTitle>
          </SheetHeader>
        </SheetContent>
      </Sheet>
    )
  }

  const maxSloops = building?.somersloopSlots ?? 0

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{t("inspector.processTitle")}</SheetTitle>
          <SheetDescription>
            {recipe?.displayName ?? node.recipeClass}
          </SheetDescription>
        </SheetHeader>
        <FieldGroup className="mt-4">
          <Field>
            <FieldLabel>{t("inspector.recipe")}</FieldLabel>
            <Select
              disabled={readOnly}
              value={node.recipeClass}
              onValueChange={(value) => {
                if (value) patch({ recipeClass: value })
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {recipeOptions.map((r) => (
                  <SelectItem key={r.className} value={r.className}>
                    {r.displayName}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel>{t("inspector.count")}</FieldLabel>
            <Input
              type="number"
              min={0.01}
              step={0.01}
              disabled={readOnly}
              value={node.count ?? 1}
              onChange={(e) => patch({ count: Number(e.target.value) })}
            />
          </Field>
          <Field>
            <FieldLabel>
              {t("inspector.clock", { percent: node.clockPercent ?? 100 })}
            </FieldLabel>
            <input
              type="range"
              min={0}
              max={250}
              step={1}
              disabled={readOnly}
              className="w-full"
              value={node.clockPercent ?? 100}
              onChange={(e) =>
                patch({ clockPercent: Number(e.target.value) })
              }
            />
          </Field>
          {maxSloops > 0 ? (
            <Field>
              <FieldLabel>{t("inspector.sloops")}</FieldLabel>
              <Input
                type="number"
                min={0}
                max={maxSloops}
                step={1}
                disabled={readOnly}
                value={node.somersloopCount ?? 0}
                onChange={(e) =>
                  patch({ somersloopCount: Number(e.target.value) })
                }
              />
            </Field>
          ) : null}
        </FieldGroup>
      </SheetContent>
    </Sheet>
  )
}
