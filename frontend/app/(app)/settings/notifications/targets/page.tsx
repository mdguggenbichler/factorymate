import { NotificationTargetsView } from "@/components/settings/notification-targets-view"
import { serverApiFetch } from "@/lib/api-server"
import type {
  MessageTypesResponse,
  NotificationTargetsResponse,
} from "@/lib/api-types"

export default async function NotificationTargetsPage() {
  const [targetsData, messageTypesData] = await Promise.all([
    serverApiFetch<NotificationTargetsResponse>("/notification-targets"),
    serverApiFetch<MessageTypesResponse>("/message-types"),
  ])

  return (
    <NotificationTargetsView
      initialTargets={targetsData.targets}
      messageTypes={messageTypesData.messageTypes}
    />
  )
}
