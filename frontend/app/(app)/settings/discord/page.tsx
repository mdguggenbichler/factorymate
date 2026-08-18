import { DiscordSettingsForm } from "@/components/settings/discord-settings-form"
import { serverApiFetch } from "@/lib/api-server"
import type { DiscordSettings } from "@/lib/api-types"

export default async function SettingsDiscordPage() {
  const settings = await serverApiFetch<DiscordSettings>("/discord/settings")

  let inviteUrl: string | null = null
  try {
    const invite = await serverApiFetch<{ inviteUrl: string }>(
      "/discord/invite-url"
    )
    inviteUrl = invite.inviteUrl
  } catch {
    inviteUrl = null
  }

  return (
    <DiscordSettingsForm
      initialSettings={settings}
      initialInviteUrl={inviteUrl}
    />
  )
}
