import { ConnectionSettingsForm } from "@/components/settings/connection-settings-form"
import { serverApiFetch } from "@/lib/api-server"
import type { ConnectionDetails } from "@/lib/api-types"

export default async function SettingsConnectionPage() {
  const details = await serverApiFetch<ConnectionDetails>("/connection-details")

  return <ConnectionSettingsForm initialDetails={details} />
}
