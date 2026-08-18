"use client"

import { useFormatDateTime } from "@/hooks/use-format-datetime"
import { cn } from "@/lib/utils"

type FormattedDateTimeProps = {
  iso: string | null | undefined
  className?: string
}

/** Renders an ISO timestamp in the viewer's browser timezone (client-only). */
export function FormattedDateTime({ iso, className }: FormattedDateTimeProps) {
  const { formatDateTime } = useFormatDateTime()

  if (!iso) {
    return <span className={cn(className)}>—</span>
  }

  return (
    <time className={cn(className)} dateTime={iso}>
      {formatDateTime(iso)}
    </time>
  )
}
