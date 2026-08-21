import { describe, expect, it } from "vitest"

import {
  machineClassNameFromId,
  normalizeIconClassName,
  resolveItemIconUrl,
} from "@/lib/item-icon"

describe("normalizeIconClassName", () => {
  it("rewrites Build_ prefixes to Desc_", () => {
    expect(normalizeIconClassName("Build_AssemblerMk1_C")).toBe(
      "Desc_AssemblerMk1_C"
    )
  })

  it("returns Desc_ class names unchanged", () => {
    expect(normalizeIconClassName("Desc_IronPlate_C")).toBe("Desc_IronPlate_C")
  })

  it("returns null for empty values", () => {
    expect(normalizeIconClassName("")).toBeNull()
    expect(normalizeIconClassName("   ")).toBeNull()
    expect(normalizeIconClassName(null)).toBeNull()
  })
})

describe("resolveItemIconUrl", () => {
  it("returns a public icon URL for known items", () => {
    expect(resolveItemIconUrl("Desc_IronPlate_C")).toBe(
      "/icons/Desc_IronPlate_C.png"
    )
  })

  it("resolves Build_ class names via Desc_ rewrite", () => {
    expect(resolveItemIconUrl("Build_AssemblerMk1_C")).toBe(
      "/icons/Desc_AssemblerMk1_C.png"
    )
  })

  it("returns null for unknown class names", () => {
    expect(resolveItemIconUrl("Desc_ModdedItem_C")).toBeNull()
  })
})

describe("machineClassNameFromId", () => {
  it("strips trailing numeric instance suffix", () => {
    expect(machineClassNameFromId("Build_AssemblerMk1_C_2147394722")).toBe(
      "Build_AssemblerMk1_C"
    )
  })

  it("returns the full id when no suffix is present", () => {
    expect(machineClassNameFromId("Build_AssemblerMk1_C")).toBe(
      "Build_AssemblerMk1_C"
    )
  })
})
