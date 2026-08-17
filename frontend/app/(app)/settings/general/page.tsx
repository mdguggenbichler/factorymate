import { GeneralSettingsForm } from "@/components/settings/general-settings-form"
import { serverApiFetch } from "@/lib/api-server"
import type { AppSettings } from "@/lib/api-types"

export default async function SettingsGeneralPage() {
  const settings = await serverApiFetch<AppSettings>("/settings")

  return <GeneralSettingsForm initialSettings={settings} />
}
