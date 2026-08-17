import { OverviewContent } from "@/components/overview/overview-content"
import { serverApiFetch } from "@/lib/api-server"
import type { StatusResponse } from "@/lib/api-types"

export default async function OverviewPage() {
  const status = await serverApiFetch<StatusResponse>("/status")

  return <OverviewContent status={status} />
}
