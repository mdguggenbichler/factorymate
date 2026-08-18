import { getTranslations } from "next-intl/server"
import { AlertTriangleIcon, ServerIcon, UsersIcon, ZapIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { MilestoneUnlockMeta } from "@/components/overview/milestone-unlock-meta"
import { ConnectionJoinCard } from "@/components/connection/connection-details-view"
import { formatPercent } from "@/lib/format"
import type { ConnectionDetails, StatusResponse } from "@/lib/api-types"

type OverviewContentProps = {
  status: StatusResponse
  connection: ConnectionDetails
}

export async function OverviewContent({ status, connection }: OverviewContentProps) {
  const t = await getTranslations("home")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{status.serverName}</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="flex items-center gap-2">
              <ServerIcon className="size-4" />
              {t("serverStatus")}
            </CardDescription>
            <CardTitle className="text-xl">
              <Badge variant={status.serverOnline ? "default" : "destructive"}>
                {status.serverOnline ? t("serverOnline") : t("serverOffline")}
              </Badge>
            </CardTitle>
          </CardHeader>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="flex items-center gap-2">
              <UsersIcon className="size-4" />
              {t("playersOnline")}
            </CardDescription>
            <CardTitle className="text-3xl tabular-nums">
              {status.onlinePlayerCount}
            </CardTitle>
          </CardHeader>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="flex items-center gap-2">
              <ZapIcon className="size-4" />
              {t("fuseTrips")}
            </CardDescription>
            <CardTitle className="text-xl">
              {status.trippedCircuits.length > 0 ? (
                <Badge variant="destructive">
                  {t("trippedCount", { count: status.trippedCircuits.length })}
                </Badge>
              ) : (
                <Badge variant="outline">{t("allClear")}</Badge>
              )}
            </CardTitle>
          </CardHeader>
          {status.trippedCircuits.length > 0 ? (
            <CardContent className="text-sm text-muted-foreground">
              {t("trippedCircuits", {
                circuits: status.trippedCircuits.join(", "),
              })}
            </CardContent>
          ) : null}
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("latestMilestone")}</CardDescription>
            <CardTitle className="text-lg">
              {status.latestMilestone?.name ?? t("noMilestone")}
            </CardTitle>
          </CardHeader>
          {status.latestMilestone ? (
            <CardContent>
              <MilestoneUnlockMeta
                techTier={status.latestMilestone.techTier}
                unlockedAt={status.latestMilestone.unlockedAt}
              />
            </CardContent>
          ) : null}
        </Card>

        <ConnectionJoinCard details={connection} />
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle>{status.elevator.name || t("elevatorTitle")}</CardTitle>
              <CardDescription>
                {status.elevator.phaseNumber != null
                  ? t("elevatorPhase", { phase: status.elevator.phaseNumber })
                  : t("elevatorPhaseUnknown")}
              </CardDescription>
            </div>
            {status.elevator.upgradeReady ? (
              <Badge>{t("upgradeReady")}</Badge>
            ) : null}
          </div>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span>{t("phaseProgress")}</span>
            <span className="tabular-nums">
              {status.elevator.percentComplete != null
                ? formatPercent(status.elevator.percentComplete)
                : "—"}
            </span>
          </div>
          <Progress
            value={status.elevator.percentComplete ?? 0}
            aria-label={t("phaseProgress")}
          />
        </CardContent>
      </Card>

      {status.trippedCircuits.length > 0 ? (
        <Card className="border-destructive/50">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangleIcon className="size-5" />
              {t("powerAlertTitle")}
            </CardTitle>
            <CardDescription>{t("powerAlertDescription")}</CardDescription>
          </CardHeader>
        </Card>
      ) : null}
    </div>
  )
}
