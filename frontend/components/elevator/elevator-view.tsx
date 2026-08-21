"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { AlertTriangleIcon } from "lucide-react"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { ItemIcon } from "@/components/item-icon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { apiFetch } from "@/lib/api"
import { useFormatDateTime } from "@/hooks/use-format-datetime"
import {
  elevatorPercentComplete,
  formatPercent,
  phaseProgress,
} from "@/lib/format"
import type {
  ElevatorResponse,
  ElevatorUnknownLogEntry,
} from "@/lib/api-types"
import type { UserRole } from "@/lib/auth-types"

type ElevatorViewProps = {
  elevator: ElevatorResponse
  unknownLog: ElevatorUnknownLogEntry[]
  userRole: UserRole
}

export function ElevatorView({
  elevator,
  unknownLog: initialUnknownLog,
  userRole,
}: ElevatorViewProps) {
  const t = useTranslations("elevator")
  const tCommon = useTranslations("common")
  const { formatDateTime } = useFormatDateTime()
  const [unknownLog, setUnknownLog] = useState(initialUnknownLog)
  const [resolvingId, setResolvingId] = useState<number | null>(null)

  const unresolved = useMemo(
    () => unknownLog.filter((entry) => !entry.resolved),
    [unknownLog]
  )

  const percentComplete = useMemo(
    () => elevatorPercentComplete(elevator.currentPhase),
    [elevator.currentPhase]
  )

  const resolveEntry = useCallback(
    async (id: number) => {
      setResolvingId(id)
      try {
        await apiFetch(`/elevator/unknown-log/${id}/resolve`, {
          method: "POST",
        })
        setUnknownLog((current) =>
          current.map((entry) =>
            entry.id === id
              ? { ...entry, resolved: true, resolvedAt: new Date().toISOString() }
              : entry
          )
        )
        toast.success(t("resolveSuccess"))
      } catch {
        toast.error(tCommon("error"))
      } finally {
        setResolvingId(null)
      }
    },
    [t, tCommon]
  )

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      {userRole === "admin" && unresolved.length > 0 ? (
        <Alert variant="destructive">
          <AlertTriangleIcon />
          <AlertTitle>{t("unknownAlertTitle")}</AlertTitle>
          <AlertDescription className="space-y-3">
            <p>{t("unknownAlertDescription", { count: unresolved.length })}</p>
            {unresolved.map((entry) => (
              <div
                key={entry.id}
                className="rounded-md border border-destructive/30 bg-background/50 p-3 text-foreground"
              >
                <p className="text-sm font-medium">
                  {t("unknownDetectedAt", {
                    date: formatDateTime(entry.detectedAt),
                  })}
                </p>
                <pre className="mt-2 max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                  {JSON.stringify(entry.currentPhase, null, 2)}
                </pre>
                <Button
                  size="sm"
                  className="mt-2"
                  disabled={resolvingId === entry.id}
                  onClick={() => void resolveEntry(entry.id)}
                >
                  {t("resolve")}
                </Button>
              </div>
            ))}
          </AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle>{elevator.name || t("defaultName")}</CardTitle>
              <p className="text-sm text-muted-foreground">
                {elevator.phaseNumber != null
                  ? t("phaseLabel", { phase: elevator.phaseNumber })
                  : t("phaseUnknown")}
              </p>
            </div>
            {elevator.upgradeReady ? <Badge>{t("upgradeReady")}</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span>{t("overallProgress")}</span>
            <span className="tabular-nums">
              {percentComplete != null ? formatPercent(percentComplete) : "—"}
            </span>
          </div>
          <Progress value={percentComplete ?? 0} aria-label={t("overallProgress")} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("itemsTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {elevator.currentPhase.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("itemsEmpty")}</p>
          ) : (
            elevator.currentPhase.map((item) => {
              const progress = phaseProgress(item)
              const delivered = item.totalCost - item.remainingCost
              return (
                <div key={item.className} className="space-y-2">
                  <div className="flex items-center justify-between gap-2 text-sm">
                    <span className="flex items-center gap-2 font-medium">
                      <ItemIcon className={item.className} size={20} />
                      {item.name}
                    </span>
                    <span className="tabular-nums text-muted-foreground">
                      {t("itemProgress", {
                        delivered,
                        total: item.totalCost,
                        remaining: item.remainingCost,
                      })}
                    </span>
                  </div>
                  <Progress value={progress} aria-label={item.name} />
                </div>
              )
            })
          )}
        </CardContent>
      </Card>
    </div>
  )
}
