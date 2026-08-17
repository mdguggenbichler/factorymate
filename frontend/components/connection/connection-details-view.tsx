"use client"

import Link from "next/link"
import { useTranslations } from "next-intl"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import type { ConnectionDetails } from "@/lib/api-types"
import { useFormatDateTime } from "@/hooks/use-format-datetime"

type ConnectionDetailsViewProps = {
  details: ConnectionDetails
  showModsLink?: boolean
}

export function ConnectionDetailsView({
  details,
  showModsLink = true,
}: ConnectionDetailsViewProps) {
  const t = useTranslations("connection")
  const { formatDateTime } = useFormatDateTime()
  const tMods = useTranslations("mods")

  const configured = Boolean(details.gameHost?.trim())

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("joinTitle")}</CardTitle>
          <CardDescription>{t("joinDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!configured ? (
            <p className="text-sm text-muted-foreground">{t("notConfigured")}</p>
          ) : (
            <dl className="grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-muted-foreground">{t("fields.gameHost")}</dt>
                <dd className="font-medium">{details.gameHost}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">{t("fields.gamePort")}</dt>
                <dd className="font-medium tabular-nums">{details.gamePort}</dd>
              </div>
              {details.gamePassword ? (
                <div className="sm:col-span-2">
                  <dt className="text-muted-foreground">{t("fields.gamePassword")}</dt>
                  <dd className="font-medium">{details.gamePassword}</dd>
                </div>
              ) : null}
              {details.notes?.trim() ? (
                <div className="sm:col-span-2">
                  <dt className="text-muted-foreground">{t("fields.notes")}</dt>
                  <dd>{details.notes}</dd>
                </div>
              ) : null}
            </dl>
          )}
          {details.updatedAt ? (
            <p className="text-sm text-muted-foreground">
              {t("lastUpdated", { date: formatDateTime(details.updatedAt) })}
            </p>
          ) : null}
          {showModsLink && configured ? (
            <Button variant="outline" render={<Link href="/mods" />}>
              {tMods("title")}
            </Button>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}

export function ConnectionJoinCard({
  details,
}: {
  details: ConnectionDetails
}) {
  const t = useTranslations("home")
  const configured = Boolean(details.gameHost?.trim())

  if (!configured) {
    return null
  }

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{t("joinTitle")}</CardDescription>
        <CardTitle className="text-lg">
          {details.gameHost}:{details.gamePort}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 text-sm text-muted-foreground">
        {details.gamePassword ? (
          <p>{t("joinPasswordHint")}</p>
        ) : null}
        <Button variant="link" className="h-auto p-0" render={<Link href="/connection" />}>
          {t("joinViewDetails")}
        </Button>
      </CardContent>
    </Card>
  )
}
