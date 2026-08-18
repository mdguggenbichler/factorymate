import type { MilestoneGroup, MilestoneSchematic } from "@/lib/api-types"

export type MilestoneProgress = {
  purchased: number
  total: number
}

export type TierLadderRow = {
  techTier: number
  schematics: MilestoneSchematic[]
  allPurchased: boolean
  anyPurchased: boolean
}

export function filterGroupsByType(
  groups: MilestoneGroup[],
  type: string
): MilestoneGroup[] {
  return groups.filter((group) => group.type === type)
}

export function flattenSchematics(groups: MilestoneGroup[]): MilestoneSchematic[] {
  return groups.flatMap((group) => group.schematics)
}

export function countMilestoneProgress(
  schematics: MilestoneSchematic[]
): MilestoneProgress {
  const total = schematics.length
  const purchased = schematics.filter((s) => s.purchased).length
  return { purchased, total }
}

export function buildTierLadder(groups: MilestoneGroup[]): TierLadderRow[] {
  const byTier = new Map<number, MilestoneSchematic[]>()

  for (const group of groups) {
    const existing = byTier.get(group.techTier) ?? []
    existing.push(...group.schematics)
    byTier.set(group.techTier, existing)
  }

  const rows: TierLadderRow[] = []
  for (const [techTier, schematics] of byTier) {
    const sorted = [...schematics].sort((a, b) => a.name.localeCompare(b.name))
    const allPurchased =
      sorted.length > 0 && sorted.every((s) => s.purchased)
    const anyPurchased = sorted.some((s) => s.purchased)
    rows.push({ techTier, schematics: sorted, allPurchased, anyPurchased })
  }

  return rows.sort((a, b) => b.techTier - a.techTier)
}

export function highestCompleteTier(ladder: TierLadderRow[]): number | null {
  const complete = ladder
    .filter((row) => row.allPurchased)
    .map((row) => row.techTier)
  if (complete.length === 0) {
    return null
  }
  return Math.max(...complete)
}

export function nextPartialTier(ladder: TierLadderRow[]): number | null {
  const ascending = [...ladder].sort((a, b) => a.techTier - b.techTier)
  for (const row of ascending) {
    if (row.anyPurchased && !row.allPurchased) {
      return row.techTier
    }
    if (!row.anyPurchased) {
      return row.techTier
    }
  }
  return null
}

export function readyHardDrives(
  schematics: MilestoneSchematic[]
): MilestoneSchematic[] {
  return schematics.filter((s) => !s.purchased && !s.locked)
}

export type SchematicStatus = "unlocked" | "locked" | "available"

export function schematicStatus(schematic: MilestoneSchematic): SchematicStatus {
  if (schematic.purchased) {
    return "unlocked"
  }
  if (schematic.locked) {
    return "locked"
  }
  return "available"
}
