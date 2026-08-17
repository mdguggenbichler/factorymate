"use client"

import { useTranslations } from "next-intl"

import type { DateRangePreset } from "@/lib/date-range"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

type IntervalPickerProps = {
  value: DateRangePreset
  onChange: (value: DateRangePreset) => void
}

export function IntervalPicker({ value, onChange }: IntervalPickerProps) {
  const t = useTranslations("charts")

  return (
    <Select
      value={value}
      onValueChange={(next) => {
        if (next) {
          onChange(next as DateRangePreset)
        }
      }}
    >
      <SelectTrigger className="w-[180px]" size="sm" aria-label={t("intervalLabel")}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="6h">{t("interval6h")}</SelectItem>
        <SelectItem value="12h">{t("interval12h")}</SelectItem>
        <SelectItem value="24h">{t("interval24h")}</SelectItem>
        <SelectItem value="7d">{t("interval7d")}</SelectItem>
        <SelectItem value="30d">{t("interval30d")}</SelectItem>
        <SelectItem value="90d">{t("interval90d")}</SelectItem>
        <SelectItem value="all">{t("intervalAll")}</SelectItem>
      </SelectContent>
    </Select>
  )
}
