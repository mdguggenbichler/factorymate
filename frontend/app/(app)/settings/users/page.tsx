import { UsersView } from "@/components/settings/users-view"
import { serverApiFetch } from "@/lib/api-server"
import type { InvitesResponse, UsersResponse } from "@/lib/api-types"

export default async function SettingsUsersPage() {
  const [usersData, invitesData] = await Promise.all([
    serverApiFetch<UsersResponse>("/users"),
    serverApiFetch<InvitesResponse>("/invites"),
  ])

  return (
    <UsersView
      initialUsers={usersData.users}
      initialInvites={invitesData.invites}
    />
  )
}
