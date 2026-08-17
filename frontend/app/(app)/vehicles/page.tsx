import { VehiclesView } from "@/components/vehicles/vehicles-view"
import { serverApiFetch } from "@/lib/api-server"
import type { VehiclesResponse } from "@/lib/api-types"

export default async function VehiclesPage() {
  const data = await serverApiFetch<VehiclesResponse>("/vehicles")

  return (
    <VehiclesView trains={data.trains} vehicles={data.vehicles} />
  )
}
