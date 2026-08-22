import { ConnectionSettingsForm } from "@/components/settings/connection-settings-form"
import { serverApiFetch } from "@/lib/api-server"
import type { AppSettings, ConnectionDetails } from "@/lib/api-types"

export default async function SettingsConnectionPage() {
  const [details, settings] = await Promise.all([
    serverApiFetch<ConnectionDetails>("/connection-details"),
    serverApiFetch<AppSettings>("/settings"),
  ])

  return (
    <ConnectionSettingsForm initialDetails={details} initialSettings={settings} />
  )
}
