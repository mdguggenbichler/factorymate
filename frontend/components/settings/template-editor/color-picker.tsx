"use client"

import { useTranslations } from "next-intl"

import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

type ColorPickerProps = {
  value: string
  onChange: (value: string) => void
}

function normalizeHex(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) {
    return "#5865F2"
  }
  return trimmed.startsWith("#") ? trimmed : `#${trimmed}`
}

export function ColorPicker({ value, onChange }: ColorPickerProps) {
  const t = useTranslations("settings.templates")
  const hex = normalizeHex(value)

  return (
    <div className="flex items-center gap-3">
      <Label htmlFor="embed-color" className="sr-only">
        {t("embedColor")}
      </Label>
      <Input
        id="embed-color-picker"
        type="color"
        value={hex}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-12 shrink-0 p-1"
      />
      <Input
        id="embed-color"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="#5865F2"
        className="font-mono"
      />
    </div>
  )
}
