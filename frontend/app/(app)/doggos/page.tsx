import { DoggosTable } from "@/components/doggos/doggos-table"
import { serverApiFetch } from "@/lib/api-server"
import type { DoggosResponse } from "@/lib/api-types"

export default async function DoggosPage() {
  const data = await serverApiFetch<DoggosResponse>("/doggos")

  return <DoggosTable doggos={data.doggos} />
}
