import { OverviewContent } from "@/components/overview/overview-content"
import { serverApiFetch } from "@/lib/api-server"
import type { ConnectionDetails, StatusResponse } from "@/lib/api-types"

export default async function OverviewPage() {
  const [status, connection] = await Promise.all([
    serverApiFetch<StatusResponse>("/status"),
    serverApiFetch<ConnectionDetails>("/connection-details"),
  ])

  return <OverviewContent status={status} connection={connection} />
}
