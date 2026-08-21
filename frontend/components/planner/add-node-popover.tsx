"use client"

import { useMemo, useState } from "react"
import { useTranslations } from "next-intl"

import { usePlannerCatalog } from "@/components/planner/planner-catalog-context"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
} from "@/components/ui/popover"
import type { PlanGraph, PlanNode } from "@/lib/planner/graph-types"
import { newNodeId } from "@/lib/planner/to-react-flow"

type AddNodePopoverProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdd: (graph: PlanGraph) => void
  graph: PlanGraph
}

export function AddNodePopover({
  open,
  onOpenChange,
  onAdd,
  graph,
}: AddNodePopoverProps) {
  const t = useTranslations("planner")
  const catalog = usePlannerCatalog()
  const [query, setQuery] = useState("")

  const recipes = useMemo(() => {
    const q = query.trim().toLowerCase()
    const list = catalog.recipes.filter((r) => !r.isAlternate)
    if (!q) return list.slice(0, 40)
    return list
      .filter((r) => r.displayName.toLowerCase().includes(q))
      .slice(0, 40)
  }, [catalog.recipes, query])

  function addRecipe(recipeClass: string) {
    const recipe = catalog.recipesByClass.get(recipeClass)
    if (!recipe || recipe.producedIn.length === 0) return
    const buildingClass = recipe.producedIn[0]
    const node: PlanNode = {
      id: newNodeId(),
      role: "process",
      recipeClass,
      buildingClass,
      count: 1,
      clockPercent: 100,
      somersloopCount: 0,
      x: 100 + graph.nodes.length * 30,
      y: 100 + graph.nodes.length * 30,
    }
    onAdd({
      ...graph,
      nodes: [...graph.nodes, node],
    })
    onOpenChange(false)
    setQuery("")
  }

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverContent className="w-80 p-0" align="start">
        <Command>
          <CommandInput
            placeholder={t("addNode.searchPlaceholder")}
            value={query}
            onValueChange={setQuery}
          />
          <CommandList>
            <CommandEmpty>{t("addNode.noRecipes")}</CommandEmpty>
            <CommandGroup>
              {recipes.map((recipe) => (
                <CommandItem
                  key={recipe.className}
                  value={recipe.displayName}
                  onSelect={() => addRecipe(recipe.className)}
                >
                  {recipe.displayName}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
