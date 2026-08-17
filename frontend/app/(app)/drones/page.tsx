import { DronesTable } from "@/components/drones/drones-table"
import { serverApiFetch } from "@/lib/api-server"
import type { DronesResponse } from "@/lib/api-types"

export default async function DronesPage() {
  const data = await serverApiFetch<DronesResponse>("/drones")

  return <DronesTable drones={data.drones} />
}
