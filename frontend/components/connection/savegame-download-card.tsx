"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
import { DownloadIcon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { apiUrl } from "@/lib/api"
import type { SavegameStatus } from "@/lib/api-types"

type SavegameDownloadCardProps = {
  status: SavegameStatus
}

export function SavegameDownloadCard({ status }: SavegameDownloadCardProps) {
  const t = useTranslations("connection.savegame")
  const tCommon = useTranslations("common")
  const [isDownloading, setIsDownloading] = useState(false)

  async function handleDownload() {
    setIsDownloading(true)
    try {
      const response = await fetch(apiUrl("/savegame"), { credentials: "include" })
      if (response.status === 429) {
        toast.error(t("rateLimited"))
        return
      }
      if (!response.ok) {
        toast.error(t("downloadFailed"))
        return
      }
      const blob = await response.blob()
      const disposition = response.headers.get("Content-Disposition")
      const match = disposition?.match(/filename="([^"]+)"/)
      const filename = match?.[1] ?? "server-save.sav"
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = url
      anchor.download = filename
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success(t("downloadStarted"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsDownloading(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        <CardDescription>{t("description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!status.configured ? (
          <p className="text-sm text-muted-foreground">{t("notConfigured")}</p>
        ) : (
          <>
            {status.latestSaveName ? (
              <dl className="grid gap-2 text-sm">
                {status.activeSessionName ? (
                  <div>
                    <dt className="text-muted-foreground">{t("session")}</dt>
                    <dd className="font-medium">{status.activeSessionName}</dd>
                  </div>
                ) : null}
                <div>
                  <dt className="text-muted-foreground">{t("save")}</dt>
                  <dd className="font-medium">{status.latestSaveName}</dd>
                </div>
                {status.saveDateTime ? (
                  <div>
                    <dt className="text-muted-foreground">{t("savedAt")}</dt>
                    <dd className="font-medium tabular-nums">{status.saveDateTime}</dd>
                  </div>
                ) : null}
              </dl>
            ) : null}
            <Button
              type="button"
              variant="outline"
              disabled={isDownloading}
              onClick={handleDownload}
            >
              <DownloadIcon className="size-4" />
              {isDownloading ? t("downloading") : t("downloadButton")}
            </Button>
          </>
        )}
      </CardContent>
    </Card>
  )
}
