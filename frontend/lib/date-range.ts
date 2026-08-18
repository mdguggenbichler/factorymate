export type DateRangePreset =
  | "6h"
  | "12h"
  | "24h"
  | "7d"
  | "30d"
  | "90d"
  | "all"

export type DateRangeQuery = {
  from?: string
  to?: string
}

const HOUR_PRESETS: Record<"6h" | "12h" | "24h", number> = {
  "6h": 6,
  "12h": 12,
  "24h": 24,
}

const DAY_PRESETS: Record<"7d" | "30d" | "90d", number> = {
  "7d": 7,
  "30d": 30,
  "90d": 90,
}

export function isShortTimePreset(preset: DateRangePreset): boolean {
  return preset === "6h" || preset === "12h" || preset === "24h"
}

export function presetToDateRange(preset: DateRangePreset): DateRangeQuery {
  if (preset === "all") {
    return {}
  }

  const to = new Date()
  const from = new Date()

  if (preset in HOUR_PRESETS) {
    from.setHours(from.getHours() - HOUR_PRESETS[preset as keyof typeof HOUR_PRESETS])
  } else {
    from.setDate(from.getDate() - DAY_PRESETS[preset as keyof typeof DAY_PRESETS])
  }

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
