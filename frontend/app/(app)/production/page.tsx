import { ProductionView } from "@/components/production/production-view"
import { serverApiFetch } from "@/lib/api-server"
import type {
  ProductionCurrentResponse,
  ProductionMachinesResponse,
} from "@/lib/api-types"

export default async function ProductionPage() {
  const [current, machines] = await Promise.all([
    serverApiFetch<ProductionCurrentResponse>("/production/current"),
    serverApiFetch<ProductionMachinesResponse>("/production/machines"),
  ])

  return (
    <ProductionView
      overallItems={current.items}
      machines={machines.machines}
    />
  )
}
