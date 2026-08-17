import { ElevatorView } from "@/components/elevator/elevator-view"
import { getCurrentUser } from "@/lib/auth-server"
import { serverApiFetch } from "@/lib/api-server"
import type {
  ElevatorResponse,
  ElevatorUnknownLogResponse,
} from "@/lib/api-types"

export default async function ElevatorPage() {
  const user = await getCurrentUser()
  const elevator = await serverApiFetch<ElevatorResponse>("/elevator")

  let unknownLog: ElevatorUnknownLogResponse["items"] = []
  if (user?.role === "admin") {
    try {
      const logData = await serverApiFetch<ElevatorUnknownLogResponse>(
        "/elevator/unknown-log"
      )
      unknownLog = logData.items
    } catch {
      unknownLog = []
    }
  }

  return (
    <ElevatorView
      elevator={elevator}
      unknownLog={unknownLog}
      userRole={user?.role ?? "viewer"}
    />
  )
}
