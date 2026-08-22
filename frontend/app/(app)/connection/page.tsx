import { ConnectionDetailsView } from "@/components/connection/connection-details-view"
import { serverApiFetch } from "@/lib/api-server"
import type { ConnectionDetails, SavegameStatus } from "@/lib/api-types"

export default async function ConnectionPage() {
  const [details, savegameStatus] = await Promise.all([
    serverApiFetch<ConnectionDetails>("/connection-details"),
    serverApiFetch<SavegameStatus>("/savegame/status"),
  ])

  return (
    <ConnectionDetailsView details={details} savegameStatus={savegameStatus} />
  )
}
