import { ConnectionDetailsView } from "@/components/connection/connection-details-view"
import { serverApiFetch } from "@/lib/api-server"
import type { ConnectionDetails } from "@/lib/api-types"

export default async function ConnectionPage() {
  const details = await serverApiFetch<ConnectionDetails>("/connection-details")

  return <ConnectionDetailsView details={details} />
}
