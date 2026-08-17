import { PowerView } from "@/components/power/power-view"
import { buildDateRangeQuery, presetToDateRange } from "@/lib/date-range"
import { serverApiFetch } from "@/lib/api-server"
import type {
  PaginatedResponse,
  PowerHistoryEvent,
  PowerMetricsResponse,
  PowerResponse,
} from "@/lib/api-types"

export default async function PowerPage() {
  const powerData = await serverApiFetch<PowerResponse>("/power")
  const firstCircuit = powerData.circuits[0]?.circuitId
  const rangeQuery = buildDateRangeQuery(presetToDateRange("7d"))

  const [historyData, metricsData] = await Promise.all([
    serverApiFetch<PaginatedResponse<PowerHistoryEvent>>(
      "/power/history?limit=50"
    ),
    firstCircuit != null
      ? serverApiFetch<PowerMetricsResponse>(
          `/power/metrics?circuit=${firstCircuit}${rangeQuery ? rangeQuery.replace("?", "&") : ""}`
        )
      : Promise.resolve({ items: [] }),
  ])

  return (
    <PowerView
      initialPower={powerData}
      initialHistory={historyData.items}
      initialMetrics={metricsData.items}
    />
  )
}
