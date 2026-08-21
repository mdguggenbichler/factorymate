import type { IndexedCatalog } from "@/lib/planner/catalog-types"
import type { PlanGraph, PlanNode } from "@/lib/planner/graph-types"

const POWER_TOLERANCE_MW = 0.01

export type NodeRates = {
  outputs: Record<string, number>
  inputs: Record<string, number>
  powerMW: number
}

export type EdgeBalance = {
  itemClass: string
  flowPerMin: number
  demandPerMin: number
  supplyPerMin: number
  overPerMin: number
  underPerMin: number
  recommendedMk: number
  capacityPerMin: number
  exceedsMax: boolean
  unit: string
}

export type MkRecommendation = {
  mk: number
  ratePerMin: number
  capacityPerMin: number
  exceedsMax: boolean
  unit: string
}

export type BalanceResult = {
  nodes: Record<string, NodeRates>
  edges: Record<string, EdgeBalance>
  totalPowerMW: number
}

export type PortStatus = {
  unterminatedOutputs: string[]
  starvedInputs: string[]
}

export function computePowerMW(
  powerBaseMW: number,
  clockPercent: number,
  powerExponent: number,
  somersloopCount: number,
  slotCount: number
): number {
  if (powerBaseMW <= 0) {
    return 0
  }
  const clockFactor = (clockPercent / 100) ** powerExponent
  let power = powerBaseMW * clockFactor
  if (slotCount > 0 && somersloopCount > 0) {
    const sloopFactor = (1 + somersloopCount / slotCount) ** 2
    power *= sloopFactor
  }
  return power
}

export function buildingPowerMW(
  catalog: IndexedCatalog,
  buildingClass: string,
  clockPercent: number,
  somersloopCount: number
): number | null {
  const building = catalog.buildingsByClass.get(buildingClass)
  if (!building) {
    return null
  }
  let sloops = somersloopCount
  if (!building.canChangeProductionBoost) {
    sloops = 0
  }
  return computePowerMW(
    building.powerBaseMW,
    clockPercent,
    building.powerExponent,
    sloops,
    building.somersloopSlots
  )
}

export function somersloopOutputMultiplier(
  building: IndexedCatalog["buildings"][number] | undefined,
  somersloopCount: number
): number {
  if (
    !building ||
    !building.canChangeProductionBoost ||
    building.somersloopSlots <= 0
  ) {
    return 1
  }
  if (somersloopCount <= 0) {
    return 1
  }
  return 1 + somersloopCount * building.somersloopBoostMultiplier
}

function nodeCount(count: number | undefined): number {
  if (!count || count <= 0) {
    return 1
  }
  return count
}

function beltMkNumber(className: string): number {
  if (className.includes("Mk6")) return 6
  if (className.includes("Mk5")) return 5
  if (className.includes("Mk4")) return 4
  if (className.includes("Mk3")) return 3
  if (className.includes("Mk2")) return 2
  return 1
}

function pipeMkNumber(className: string): number {
  if (className.includes("MK2") || className.includes("Mk2")) {
    return 2
  }
  return 1
}

export function recommendBeltMk(
  catalog: IndexedCatalog,
  ratePerMin: number
): MkRecommendation {
  const unit = "items/min"
  const sorted = [...catalog.belts].sort(
    (a, b) => beltMkNumber(a.className) - beltMkNumber(b.className)
  )
  if (sorted.length === 0) {
    return {
      mk: 0,
      ratePerMin,
      capacityPerMin: 0,
      exceedsMax: ratePerMin > 0,
      unit,
    }
  }
  const maxCap = sorted[sorted.length - 1].itemsPerMin
  for (const belt of sorted) {
    if (belt.itemsPerMin >= ratePerMin) {
      return {
        mk: beltMkNumber(belt.className),
        ratePerMin,
        capacityPerMin: belt.itemsPerMin,
        exceedsMax: false,
        unit,
      }
    }
  }
  return {
    mk: beltMkNumber(sorted[sorted.length - 1].className),
    ratePerMin,
    capacityPerMin: maxCap,
    exceedsMax: true,
    unit,
  }
}

export function recommendPipeMk(
  catalog: IndexedCatalog,
  ratePerMin: number
): MkRecommendation {
  const unit = "m³/min"
  const sorted = [...catalog.pipes].sort(
    (a, b) => pipeMkNumber(a.className) - pipeMkNumber(b.className)
  )
  if (sorted.length === 0) {
    return {
      mk: 0,
      ratePerMin,
      capacityPerMin: 0,
      exceedsMax: ratePerMin > 0,
      unit,
    }
  }
  const maxCap = sorted[sorted.length - 1].cubicMetersPerMin
  for (const pipe of sorted) {
    if (pipe.cubicMetersPerMin >= ratePerMin) {
      return {
        mk: pipeMkNumber(pipe.className),
        ratePerMin,
        capacityPerMin: pipe.cubicMetersPerMin,
        exceedsMax: false,
        unit,
      }
    }
  }
  return {
    mk: pipeMkNumber(sorted[sorted.length - 1].className),
    ratePerMin,
    capacityPerMin: maxCap,
    exceedsMax: true,
    unit,
  }
}

export function recommendTransportMk(
  catalog: IndexedCatalog,
  itemClass: string,
  ratePerMin: number
): MkRecommendation {
  const item = catalog.itemsByClass.get(itemClass)
  if (item && (item.form === "liquid" || item.form === "gas")) {
    return recommendPipeMk(catalog, ratePerMin)
  }
  return recommendBeltMk(catalog, ratePerMin)
}

export function analyzeGraph(
  catalog: IndexedCatalog,
  graph: PlanGraph
): BalanceResult {
  const nodeRates: Record<string, NodeRates> = {}
  let totalPower = 0

  for (const node of graph.nodes) {
    const rates: NodeRates = { outputs: {}, inputs: {}, powerMW: 0 }

    switch (node.role) {
      case "source": {
        if (node.itemClass && (node.ratePerMin ?? 0) > 0) {
          rates.outputs[node.itemClass] =
            (node.ratePerMin ?? 0) * nodeCount(node.count)
        }
        break
      }
      case "sink": {
        if (node.itemClass) {
          rates.inputs[node.itemClass] = node.ratePerMin ?? 0
        }
        break
      }
      case "process":
      default: {
        if (node.recipeClass && node.buildingClass) {
          const recipe = catalog.recipesByClass.get(node.recipeClass)
          const building = catalog.buildingsByClass.get(node.buildingClass)
          if (recipe && building) {
            const mult = nodeCount(node.count)
            const clock = node.clockPercent && node.clockPercent > 0
              ? node.clockPercent
              : 100
            const sloopMult = somersloopOutputMultiplier(
              building,
              node.somersloopCount ?? 0
            )
            const cyclesPerMin =
              (60 / recipe.durationSec) *
              building.manufacturingSpeed *
              (clock / 100) *
              mult *
              sloopMult
            for (const product of recipe.products) {
              rates.outputs[product.className] = cyclesPerMin * product.amount
            }
            for (const ingredient of recipe.ingredients) {
              rates.inputs[ingredient.className] =
                cyclesPerMin * ingredient.amount
            }
            const power = buildingPowerMW(
              catalog,
              node.buildingClass,
              clock,
              node.somersloopCount ?? 0
            )
            if (power !== null) {
              rates.powerMW = power * mult
              totalPower += rates.powerMW
            }
          }
        }
        break
      }
    }

    nodeRates[node.id] = rates
  }

  const edgeFlows: Record<string, EdgeBalance> = {}

  for (const edge of graph.edges) {
    const src = nodeRates[edge.sourceNodeId] ?? {
      outputs: {},
      inputs: {},
      powerMW: 0,
    }
    const dst = nodeRates[edge.targetNodeId] ?? {
      outputs: {},
      inputs: {},
      powerMW: 0,
    }
    const supply = src.outputs[edge.itemClass] ?? 0
    const demand = dst.inputs[edge.itemClass] ?? 0

    let sameItemOutgoing = 0
    for (const other of graph.edges) {
      if (
        other.sourceNodeId === edge.sourceNodeId &&
        other.itemClass === edge.itemClass
      ) {
        sameItemOutgoing++
      }
    }

    const available =
      sameItemOutgoing > 0 ? supply / sameItemOutgoing : 0
    let flow = available
    if (demand > 0 && flow > demand) {
      flow = demand
    }

    const rec = recommendTransportMk(catalog, edge.itemClass, flow)

    edgeFlows[edge.id] = {
      itemClass: edge.itemClass,
      flowPerMin: flow,
      demandPerMin: demand,
      supplyPerMin: supply,
      overPerMin: Math.max(0, available - demand),
      underPerMin: Math.max(0, demand - flow),
      recommendedMk: rec.mk,
      capacityPerMin: rec.capacityPerMin,
      exceedsMax: rec.exceedsMax,
      unit: rec.unit,
    }
  }

  return {
    nodes: nodeRates,
    edges: edgeFlows,
    totalPowerMW: totalPower,
  }
}

export function analyzePortStatus(
  catalog: IndexedCatalog,
  graph: PlanGraph,
  node: PlanNode
): PortStatus {
  const unterminatedOutputs: string[] = []
  const starvedInputs: string[] = []

  const connectedOut = new Set<string>()
  const connectedIn = new Set<string>()

  for (const edge of graph.edges) {
    if (edge.sourceNodeId === node.id) {
      connectedOut.add(edge.sourcePort)
    }
    if (edge.targetNodeId === node.id) {
      connectedIn.add(edge.targetPort)
    }
  }

  if (node.role === "source" && node.itemClass) {
    const port = `out:${node.itemClass}`
    if (!connectedOut.has(port)) {
      unterminatedOutputs.push(node.itemClass)
    }
    return { unterminatedOutputs, starvedInputs }
  }

  if (node.role === "sink" && node.itemClass) {
    const port = `in:${node.itemClass}`
    if (!connectedIn.has(port)) {
      starvedInputs.push(node.itemClass)
    }
    return { unterminatedOutputs, starvedInputs }
  }

  if (node.role === "process" && node.recipeClass) {
    const recipe = catalog.recipesByClass.get(node.recipeClass)
    if (recipe) {
      for (const product of recipe.products) {
        const port = `out:${product.className}`
        if (!connectedOut.has(port)) {
          unterminatedOutputs.push(product.className)
        }
      }
      for (const ingredient of recipe.ingredients) {
        const port = `in:${ingredient.className}`
        if (!connectedIn.has(port)) {
          starvedInputs.push(ingredient.className)
        }
      }
    }
  }

  return { unterminatedOutputs, starvedInputs }
}

export function withinPowerTolerance(got: number, want: number): boolean {
  const got2 = Math.round(got * 100) / 100
  const want2 = Math.round(want * 100) / 100
  if (Math.abs(got2 - want2) <= POWER_TOLERANCE_MW) {
    return true
  }
  return Math.abs(Math.round(got * 10) / 10 - want) <= POWER_TOLERANCE_MW
}
