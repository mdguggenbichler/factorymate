import { describe, expect, it } from "vitest"

import type { MilestoneGroup, MilestoneSchematic } from "@/lib/api-types"
import {
  buildTierLadder,
  countMilestoneProgress,
  filterGroupsByType,
  flattenSchematics,
  highestCompleteTier,
  nextPartialTier,
  readyHardDrives,
  schematicStatus,
} from "@/lib/milestone-layout"

function schematic(
  id: string,
  purchased: boolean,
  locked = false
): MilestoneSchematic {
  return {
    id,
    name: id,
    purchased,
    locked,
    recipes: [],
    purchasedAt: purchased ? "2026-08-17T00:00:00Z" : null,
    updatedAt: "2026-08-17T00:00:00Z",
  }
}

function group(
  type: string,
  techTier: number,
  schematics: MilestoneSchematic[]
): MilestoneGroup {
  return { type, techTier, schematics }
}

describe("filterGroupsByType", () => {
  it("returns only matching type groups", () => {
    const groups = [
      group("Milestone", 1, [schematic("a", true)]),
      group("Hard Drive", 0, [schematic("b", false)]),
      group("Milestone", 2, [schematic("c", false)]),
    ]
    expect(filterGroupsByType(groups, "Milestone")).toHaveLength(2)
  })
})

describe("countMilestoneProgress", () => {
  it("counts purchased schematics", () => {
    const schematics = [
      schematic("a", true),
      schematic("b", false),
      schematic("c", true),
    ]
    expect(countMilestoneProgress(schematics)).toEqual({ purchased: 2, total: 3 })
  })
})

describe("buildTierLadder", () => {
  it("sorts tiers descending and aggregates schematics", () => {
    const groups = [
      group("Milestone", 1, [schematic("t1", true)]),
      group("Milestone", 3, [schematic("t3a", true), schematic("t3b", false)]),
      group("Milestone", 2, [schematic("t2", true)]),
    ]

    const ladder = buildTierLadder(groups)
    expect(ladder.map((row) => row.techTier)).toEqual([3, 2, 1])
    expect(ladder[0].allPurchased).toBe(false)
    expect(ladder[0].anyPurchased).toBe(true)
    expect(ladder[1].allPurchased).toBe(true)
  })
})

describe("highestCompleteTier", () => {
  it("returns highest tier where all schematics are purchased", () => {
    const ladder = buildTierLadder([
      group("Milestone", 1, [schematic("a", true)]),
      group("Milestone", 2, [schematic("b", true)]),
      group("Milestone", 3, [schematic("c", true), schematic("d", false)]),
    ])
    expect(highestCompleteTier(ladder)).toBe(2)
  })

  it("returns null when no tier is fully complete", () => {
    const ladder = buildTierLadder([
      group("Milestone", 1, [schematic("a", false)]),
    ])
    expect(highestCompleteTier(ladder)).toBeNull()
  })
})

describe("nextPartialTier", () => {
  it("returns first tier with partial or no progress in ascending order", () => {
    const ladder = buildTierLadder([
      group("Milestone", 1, [schematic("a", true)]),
      group("Milestone", 2, [schematic("b", true)]),
      group("Milestone", 3, [schematic("c", true), schematic("d", false)]),
    ])
    expect(nextPartialTier(ladder)).toBe(3)
  })
})

describe("readyHardDrives", () => {
  it("filters drives ready for recipe selection", () => {
    const drives = [
      schematic("ready", false, false),
      schematic("locked", false, true),
      schematic("done", true, false),
    ]
    expect(readyHardDrives(drives)).toHaveLength(1)
    expect(readyHardDrives(drives)[0].id).toBe("ready")
  })
})

describe("schematicStatus", () => {
  it("maps purchased and locked flags to status", () => {
    expect(schematicStatus(schematic("a", true))).toBe("unlocked")
    expect(schematicStatus(schematic("b", false, true))).toBe("locked")
    expect(schematicStatus(schematic("c", false, false))).toBe("available")
  })
})

describe("flattenSchematics", () => {
  it("collects all schematics from groups", () => {
    const groups = [
      group("Milestone", 1, [schematic("a", true)]),
      group("Milestone", 2, [schematic("b", false)]),
    ]
    expect(flattenSchematics(groups)).toHaveLength(2)
  })
})
