import { AccountForm } from "@/components/account-form"
import { DiscordLinkCard } from "@/components/account/discord-link-card"
import { apiFetch } from "@/lib/api"
import type { AuthConfig } from "@/lib/auth-client"
import { getCurrentUser } from "@/lib/auth-server"

export default async function AccountPage() {
  const user = await getCurrentUser()
  let discordOAuthEnabled = false
  try {
    const config = await apiFetch<AuthConfig>("/auth/config", { server: true })
    discordOAuthEnabled = config.discordOAuthEnabled
  } catch {
    discordOAuthEnabled = false
  }

  if (!user) {
    return null
  }

  return (
    <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
      {discordOAuthEnabled ? <DiscordLinkCard user={user} /> : null}
      {user.hasPassword !== false ? <AccountForm /> : null}
    </div>
  )
}
