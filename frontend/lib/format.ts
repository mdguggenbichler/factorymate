export function formatDateTime(
  iso: string | null | undefined,
  locale?: string
): string {
  if (!iso) {
    return "—"
  }

  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return "—"
  }

  return date.toLocaleString(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  })
}

export function formatLocalDateTime(
  date: Date,
  locale?: string
): string {
  return date.toLocaleString(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  })
}

export function formatNumber(
  value: number | null | undefined,
  maximumFractionDigits = 1
): string {
  if (value == null || Number.isNaN(value)) {
    return "—"
  }

  return value.toLocaleString(undefined, { maximumFractionDigits })
}

export function formatPercent(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) {
    return "—"
  }

  return `${value.toFixed(1)}%`
}

export function formatMw(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) {
    return "—"
  }

  return `${formatNumber(value)} MW`
}

export function phaseProgress(item: {
  remainingCost: number
  totalCost: number
}): number {
  if (item.totalCost <= 0) {
    return 0
  }

  return Math.max(
    0,
    Math.min(100, 100 * (1 - item.remainingCost / item.totalCost))
  )
}

export function elevatorPercentComplete(
  items: { remainingCost: number; totalCost: number }[]
): number | null {
  if (items.length === 0) {
    return null
  }

  let remaining = 0
  let total = 0
  for (const item of items) {
    remaining += item.remainingCost
    total += item.totalCost
  }

  if (total === 0) {
    return null
  }

  return 100 * (1 - remaining / total)
}
