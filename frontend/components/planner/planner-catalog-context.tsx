"use client"

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  type ReactNode,
} from "react"

import {
  indexCatalog,
  type IndexedCatalog,
  type PlannerCatalog,
} from "@/lib/planner/catalog-types"

type PlannerCatalogContextValue = {
  catalog: IndexedCatalog
}

const PlannerCatalogContext = createContext<PlannerCatalogContextValue | null>(
  null
)

export function PlannerCatalogProvider({
  catalog: rawCatalog,
  children,
}: {
  catalog: PlannerCatalog
  children: ReactNode
}) {
  const catalog = useMemo(() => indexCatalog(rawCatalog), [rawCatalog])

  const value = useMemo(() => ({ catalog }), [catalog])

  return (
    <PlannerCatalogContext.Provider value={value}>
      {children}
    </PlannerCatalogContext.Provider>
  )
}

export function usePlannerCatalog(): IndexedCatalog {
  const ctx = useContext(PlannerCatalogContext)
  if (!ctx) {
    throw new Error(
      "usePlannerCatalog must be used within PlannerCatalogProvider"
    )
  }
  return ctx.catalog
}

export function useCatalogBuilding(className: string) {
  const catalog = usePlannerCatalog()
  return useMemo(
    () => catalog.buildingsByClass.get(className),
    [catalog, className]
  )
}

export function useCatalogRecipe(className: string) {
  const catalog = usePlannerCatalog()
  return useMemo(
    () => catalog.recipesByClass.get(className),
    [catalog, className]
  )
}

export function useCatalogDisplayName(className: string): string {
  const catalog = usePlannerCatalog()
  return useMemo(() => {
    const item = catalog.itemsByClass.get(className)
    if (item) return item.displayName
    const recipe = catalog.recipesByClass.get(className)
    if (recipe) return recipe.displayName
    const building = catalog.buildingsByClass.get(className)
    if (building) return building.displayName
    return className
  }, [catalog, className])
}

export function useCatalogSearch() {
  const catalog = usePlannerCatalog()
  return useCallback(
    (query: string, limit = 30) => {
      const q = query.trim().toLowerCase()
      if (!q) {
        return catalog.items.slice(0, limit)
      }
      return catalog.items
        .filter((item) => item.displayName.toLowerCase().includes(q))
        .slice(0, limit)
    },
    [catalog]
  )
}
