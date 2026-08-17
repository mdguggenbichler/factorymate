import { UsersView } from "@/components/settings/users-view"
import { serverApiFetch } from "@/lib/api-server"
import type {
  InvitesResponse,
  PendingRegistrationsResponse,
  UnmappedPlayersResponse,
  UsersResponse,
} from "@/lib/api-types"

export default async function SettingsUsersPage() {
  const [usersData, invitesData, pendingData, unmappedData] = await Promise.all([
    serverApiFetch<UsersResponse>("/users"),
    serverApiFetch<InvitesResponse>("/invites"),
    serverApiFetch<PendingRegistrationsResponse>("/registrations/pending"),
    serverApiFetch<UnmappedPlayersResponse>("/players/unmapped"),
  ])

  return (
    <UsersView
      initialUsers={usersData.users}
      initialInvites={invitesData.invites}
      initialPending={pendingData.registrations}
      initialUnmapped={unmappedData.players}
    />
  )
}
