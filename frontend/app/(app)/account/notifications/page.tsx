import { NotificationPrefsForm } from "@/components/account/notification-prefs-form"
import { serverApiFetch } from "@/lib/api-server"
import type { UserNotificationPrefs } from "@/lib/api-types"

export default async function AccountNotificationsPage() {
  const prefs = await serverApiFetch<UserNotificationPrefs>(
    "/account/notifications"
  )

  return <NotificationPrefsForm initialPrefs={prefs} />
}
