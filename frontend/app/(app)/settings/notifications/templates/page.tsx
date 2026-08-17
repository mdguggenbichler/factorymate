import { TemplatesView } from "@/components/settings/templates-view"
import { serverApiFetch } from "@/lib/api-server"
import type {
  MessageTypesResponse,
  NotificationTargetsResponse,
} from "@/lib/api-types"

export default async function NotificationTemplatesPage() {
  const [messageTypesData, targetsData] = await Promise.all([
    serverApiFetch<MessageTypesResponse>("/message-types"),
    serverApiFetch<NotificationTargetsResponse>("/notification-targets"),
  ])

  return (
    <TemplatesView
      initialMessageTypes={messageTypesData.messageTypes}
      targets={targetsData.targets}
    />
  )
}
