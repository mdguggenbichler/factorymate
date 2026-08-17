"use client"

import { useTranslations } from "next-intl"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import type { EmbedTemplate } from "@/lib/api-types"

type EmbedPreviewProps = {
  embed: EmbedTemplate | undefined
  title?: string
  footerText?: string
  showTimestamp?: boolean
}

function hexToBorderColor(hex: string): string {
  const normalized = hex.trim().replace("#", "")
  if (!/^[0-9a-fA-F]{6}$/.test(normalized)) {
    return "#5865F2"
  }
  return `#${normalized}`
}

export function EmbedPreview({
  embed,
  title,
  footerText,
  showTimestamp,
}: EmbedPreviewProps) {
  const t = useTranslations("settings.templates")

  if (!embed) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{title ?? t("previewTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t("previewEmpty")}</p>
        </CardContent>
      </Card>
    )
  }

  const borderColor = hexToBorderColor(embed.color)
  const footer = footerText ?? embed.footer
  const displayTimestamp = showTimestamp ?? embed.show_timestamp

  return (
    <Card>
      <CardHeader>
        <CardTitle>{title ?? t("previewTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        <div
          className="rounded-lg border bg-muted/30 p-4"
          style={{ borderLeftWidth: 4, borderLeftColor: borderColor }}
        >
          {embed.title ? (
            <p className="font-semibold">{embed.title}</p>
          ) : null}
          {embed.description ? (
            <p className="mt-1 text-sm text-muted-foreground whitespace-pre-wrap">
              {embed.description}
            </p>
          ) : null}
          {embed.fields.length > 0 ? (
            <>
              <Separator className="my-3" />
              <div className="grid gap-2 sm:grid-cols-2">
                {embed.fields.map((field, index) => (
                  <div
                    key={`${field.name}-${index}`}
                    className={field.inline ? "" : "sm:col-span-2"}
                  >
                    <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      {field.name}
                    </p>
                    <p className="text-sm whitespace-pre-wrap">{field.value}</p>
                  </div>
                ))}
              </div>
            </>
          ) : null}
          {footer || displayTimestamp ? (
            <>
              <Separator className="my-3" />
              <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                {footer ? <span>{footer}</span> : <span />}
                {displayTimestamp ? (
                  <span className="shrink-0">{t("previewTimestampSample")}</span>
                ) : null}
              </div>
            </>
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}
