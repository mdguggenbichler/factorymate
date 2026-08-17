import { NotificationLogView } from "@/components/settings/notification-log-view"
import { serverApiFetch } from "@/lib/api-server"
import type { NotificationLogResponse } from "@/lib/api-types"

export default async function NotificationLogPage() {
  const logData = await serverApiFetch<NotificationLogResponse>(
    "/notification-log?limit=50"
  )

  return (
    <NotificationLogView entries={logData.items} total={logData.total} />
  )
}
