import { ResourceSinkView } from "@/components/resource-sink/resource-sink-view"
import { buildDateRangeQuery, presetToDateRange } from "@/lib/date-range"
import { serverApiFetch } from "@/lib/api-server"
import type {
  ResourceSinkHistoryResponse,
  ResourceSinkResponse,
} from "@/lib/api-types"

export default async function ResourceSinkPage() {
  const rangeQuery = buildDateRangeQuery(presetToDateRange("7d"))

  const [statusData, historyData] = await Promise.all([
    serverApiFetch<ResourceSinkResponse>("/resource-sink"),
    serverApiFetch<ResourceSinkHistoryResponse>(
      `/resource-sink/history${rangeQuery}`
    ),
  ])

  return (
    <ResourceSinkView
      initialStatus={statusData}
      initialHistory={historyData.items}
    />
  )
}
