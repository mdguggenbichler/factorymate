import { ModsView } from "@/components/mods/mods-view"
import { getCurrentUser } from "@/lib/auth-server"
import { serverApiFetch } from "@/lib/api-server"
import type { ModsResponse } from "@/lib/api-types"

export default async function ModsPage() {
  const [user, data] = await Promise.all([
    getCurrentUser(),
    serverApiFetch<ModsResponse>("/mods"),
  ])

  return <ModsView initialData={data} isAdmin={user?.role === "admin"} />
}
