export type ItemForm = "solid" | "liquid" | "gas"

export type CatalogItem = {
  className: string
  displayName: string
  form: ItemForm
}

export type CatalogItemAmount = {
  className: string
  amount: number
}

export type CatalogRecipe = {
  className: string
  displayName: string
  durationSec: number
  ingredients: CatalogItemAmount[]
  products: CatalogItemAmount[]
  producedIn: string[]
  isAlternate: boolean
}

export type CatalogBuilding = {
  className: string
  displayName: string
  iconClassName: string
  powerBaseMW: number
  powerExponent: number
  manufacturingSpeed: number
  somersloopSlots: number
  somersloopBoostMultiplier: number
  canChangeProductionBoost: boolean
}

export type CatalogBelt = {
  className: string
  displayName: string
  itemsPerMin: number
}

export type CatalogPipe = {
  className: string
  displayName: string
  cubicMetersPerMin: number
}

export type PlannerCatalog = {
  items: CatalogItem[]
  recipes: CatalogRecipe[]
  buildings: CatalogBuilding[]
  belts: CatalogBelt[]
  pipes: CatalogPipe[]
  iconMap: Record<string, string>
}

export type IndexedCatalog = PlannerCatalog & {
  itemsByClass: Map<string, CatalogItem>
  recipesByClass: Map<string, CatalogRecipe>
  buildingsByClass: Map<string, CatalogBuilding>
}

export function indexCatalog(catalog: PlannerCatalog): IndexedCatalog {
  const itemsByClass = new Map<string, CatalogItem>()
  for (const item of catalog.items) {
    itemsByClass.set(item.className, item)
  }

  const recipesByClass = new Map<string, CatalogRecipe>()
  for (const recipe of catalog.recipes) {
    recipesByClass.set(recipe.className, recipe)
  }

  const buildingsByClass = new Map<string, CatalogBuilding>()
  for (const building of catalog.buildings) {
    buildingsByClass.set(building.className, building)
  }

  return {
    ...catalog,
    itemsByClass,
    recipesByClass,
    buildingsByClass,
  }
}

export function resolveIconClassName(
  catalog: PlannerCatalog,
  className: string
): string {
  return catalog.iconMap[className] ?? className
}

export function plannerIconUrl(className: string): string {
  return `/api/planner/icons/${encodeURIComponent(className)}`
}
