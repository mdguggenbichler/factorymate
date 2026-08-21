import { ITEM_ICON_KEYS } from "@/lib/generated/item-icon-keys"

export function normalizeIconClassName(
  className: string | null | undefined
): string | null {
  if (!className) {
    return null
  }
  const trimmed = className.trim()
  if (!trimmed) {
    return null
  }
  if (trimmed.startsWith("Build_")) {
    return `Desc_${trimmed.slice("Build_".length)}`
  }
  return trimmed
}

export function resolveItemIconUrl(
  className: string | null | undefined
): string | null {
  const normalized = normalizeIconClassName(className)
  if (!normalized || !ITEM_ICON_KEYS.has(normalized)) {
    return null
  }
  return `/icons/${normalized}.png`
}

export function machineClassNameFromId(
  machineId: string | null | undefined
): string | null {
  if (!machineId) {
    return null
  }
  const trimmed = machineId.trim()
  if (!trimmed) {
    return null
  }
  const match = trimmed.match(/^(.*)_\d+$/)
  return match ? match[1] : trimmed
}
