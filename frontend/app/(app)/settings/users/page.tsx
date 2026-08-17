import { UsersView } from "@/components/settings/users-view"
import { serverApiFetch } from "@/lib/api-server"
import type { UsersResponse } from "@/lib/api-types"

export default async function SettingsUsersPage() {
  const usersData = await serverApiFetch<UsersResponse>("/users")

  return <UsersView initialUsers={usersData.users} />
}
