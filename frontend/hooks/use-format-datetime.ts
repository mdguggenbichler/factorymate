"use client"

import { useLocale } from "next-intl"
import { useCallback } from "react"

import { formatDateTime, formatLocalDateTime } from "@/lib/format"

export function useFormatDateTime() {
  const locale = useLocale()

  const formatDateTimeLocalized = useCallback(
    (iso: string | null | undefined) => formatDateTime(iso, locale),
    [locale]
  )

  const formatLocalDateTimeLocalized = useCallback(
    (date: Date) => formatLocalDateTime(date, locale),
    [locale]
  )

  return {
    formatDateTime: formatDateTimeLocalized,
    formatLocalDateTime: formatLocalDateTimeLocalized,
  }
}
