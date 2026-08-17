import { NotificationDefaultsForm } from "@/components/settings/notification-defaults-form"
import { serverApiFetch } from "@/lib/api-server"
import type { AdminNotificationDefaults } from "@/lib/api-types"

export default async function NotificationDefaultsPage() {
  const defaults = await serverApiFetch<AdminNotificationDefaults>(
    "/settings/notification-defaults"
  )

  return <NotificationDefaultsForm initialDefaults={defaults} />
}
