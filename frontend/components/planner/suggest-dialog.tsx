"use client"

import { useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import {
  useCatalogSearch,
  usePlannerCatalog,
} from "@/components/planner/planner-catalog-context"
import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { apiFetch } from "@/lib/api"
import type { PlannerSuggestResponse } from "@/lib/api-types"
import type { PlanGraph } from "@/lib/planner/graph-types"
import { applyDagreLayout } from "@/lib/planner/layout"

type SuggestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  planId: number
  onApplied: (graph: PlanGraph) => void
}

export function SuggestDialog({
  open,
  onOpenChange,
  planId,
  onApplied,
}: SuggestDialogProps) {
  const t = useTranslations("planner")
  const catalog = usePlannerCatalog()
  const searchItems = useCatalogSearch()
  const [query, setQuery] = useState("")
  const [itemClass, setItemClass] = useState("")
  const [rate, setRate] = useState("60")
  const [submitting, setSubmitting] = useState(false)

  const items = useMemo(() => searchItems(query, 40), [searchItems, query])
  const selectedName =
    catalog.itemsByClass.get(itemClass)?.displayName ?? itemClass

  async function handleApply() {
    const rateNum = Number(rate)
    if (!itemClass || !Number.isFinite(rateNum) || rateNum <= 0) {
      toast.error(t("suggest.invalid"))
      return
    }
    setSubmitting(true)
    try {
      const res = await apiFetch<PlannerSuggestResponse>(
        `/planner/plans/${planId}/suggest`,
        {
          method: "POST",
          body: JSON.stringify({
            itemClass,
            ratePerMin: rateNum,
            apply: true,
          }),
        }
      )
      const laidOut = applyDagreLayout(res.graph)
      onApplied(laidOut)
      onOpenChange(false)
      toast.success(t("suggest.applied"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("suggest.failed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("suggest.title")}</DialogTitle>
          <DialogDescription>{t("suggest.description")}</DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel>{t("suggest.item")}</FieldLabel>
            <Command className="rounded-md border">
              <CommandInput
                placeholder={t("suggest.searchPlaceholder")}
                value={query}
                onValueChange={setQuery}
              />
              <CommandList>
                <CommandEmpty>{t("suggest.noItems")}</CommandEmpty>
                <CommandGroup>
                  {items.map((item) => (
                    <CommandItem
                      key={item.className}
                      value={item.displayName}
                      onSelect={() => {
                        setItemClass(item.className)
                        setQuery(item.displayName)
                      }}
                    >
                      {item.displayName}
                    </CommandItem>
                  ))}
                </CommandGroup>
              </CommandList>
            </Command>
            {itemClass ? (
              <p className="text-muted-foreground mt-1 text-xs">
                {selectedName}
              </p>
            ) : null}
          </Field>
          <Field>
            <FieldLabel>{t("suggest.rate")}</FieldLabel>
            <Input
              type="number"
              min={0.1}
              step={0.1}
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("suggest.cancel")}
          </Button>
          <Button onClick={handleApply} disabled={submitting || !itemClass}>
            {t("suggest.apply")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
