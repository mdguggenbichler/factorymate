"use client"

import { useTranslations } from "next-intl"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

type SaveStatus = "saved" | "saving" | "dirty" | "readonly" | "error"

type PlannerToolbarProps = {
  saveStatus: SaveStatus
  readOnly: boolean
  hasBaseline: boolean
  totalPowerMW: number
  onSuggest: () => void
  onReset: () => void
  onLayout: () => void
  onAddNode: () => void
}

export function PlannerToolbar({
  saveStatus,
  readOnly,
  hasBaseline,
  totalPowerMW,
  onSuggest,
  onReset,
  onLayout,
  onAddNode,
}: PlannerToolbarProps) {
  const t = useTranslations("planner")

  const statusLabel = {
    saved: t("toolbar.saved"),
    saving: t("toolbar.saving"),
    dirty: t("toolbar.unsaved"),
    readonly: t("toolbar.readOnly"),
    error: t("toolbar.saveError"),
  }[saveStatus]

  return (
    <div className="flex flex-wrap items-center gap-2 border-b bg-background px-3 py-2">
      <Button size="sm" onClick={onSuggest} disabled={readOnly}>
        {t("toolbar.suggest")}
      </Button>
      <Button size="sm" variant="outline" onClick={onAddNode} disabled={readOnly}>
        {t("toolbar.addNode")}
      </Button>
      <Button size="sm" variant="outline" onClick={onLayout} disabled={readOnly}>
        {t("toolbar.layout")}
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={onReset}
        disabled={readOnly || !hasBaseline}
      >
        {t("toolbar.reset")}
      </Button>
      <div className="ml-auto flex items-center gap-2">
        <Badge variant="secondary">
          {t("toolbar.power", { mw: totalPowerMW.toFixed(1) })}
        </Badge>
        <Badge
          variant={
            saveStatus === "error"
              ? "destructive"
              : saveStatus === "readonly"
                ? "outline"
                : "secondary"
          }
        >
          {statusLabel}
        </Badge>
      </div>
    </div>
  )
}
