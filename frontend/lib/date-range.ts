export type DateRangePreset = "7d" | "30d" | "90d" | "all"

export type DateRangeQuery = {
  from?: string
  to?: string
}

export function presetToDateRange(preset: DateRangePreset): DateRangeQuery {
  if (preset === "all") {
    return {}
  }

  const to = new Date()
  const from = new Date()
  const days = preset === "7d" ? 7 : preset === "30d" ? 30 : 90
  from.setDate(from.getDate() - days)

  return {
    from: from.toISOString(),
    to: to.toISOString(),
  }
}

export function buildDateRangeQuery(range: DateRangeQuery): string {
  const params = new URLSearchParams()
  if (range.from) {
    params.set("from", range.from)
  }
  if (range.to) {
    params.set("to", range.to)
  }
  const query = params.toString()
  return query ? `?${query}` : ""
}
