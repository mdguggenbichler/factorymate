"use client"

import { useFormatter, useTranslations } from "next-intl"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import type { PlannerLockState } from "@/lib/api-types"

type PlannerLockBannerProps = {
  lock: PlannerLockState
  canManage: boolean
  onAcquire: () => void
  onRelease: () => void
  onForceRelease: () => void
  acquiring?: boolean
}

export function PlannerLockBanner({
  lock,
  canManage,
  onAcquire,
  onRelease,
  onForceRelease,
  acquiring,
}: PlannerLockBannerProps) {
  const t = useTranslations("planner")
  const format = useFormatter()

  if (lock.mine) {
    return (
      <Alert className="rounded-none border-x-0 border-t-0">
        <AlertTitle>{t("lock.editing")}</AlertTitle>
        <AlertDescription className="flex flex-wrap items-center gap-2">
          <span>
            {lock.expiresAt
              ? t("lock.expires", {
                  time: format.relativeTime(new Date(lock.expiresAt)),
                })
              : null}
          </span>
          <Button size="sm" variant="outline" onClick={onRelease}>
            {t("lock.release")}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  if (lock.held) {
    return (
      <Alert variant="default" className="rounded-none border-x-0 border-t-0">
        <AlertTitle>{t("lock.heldBy", { username: lock.username ?? "" })}</AlertTitle>
        <AlertDescription className="flex flex-wrap items-center gap-2">
          <span>
            {lock.expiresAt
              ? t("lock.expires", {
                  time: format.relativeTime(new Date(lock.expiresAt)),
                })
              : null}
          </span>
          {canManage ? (
            <Button size="sm" variant="destructive" onClick={onForceRelease}>
              {t("lock.forceRelease")}
            </Button>
          ) : null}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <Alert className="rounded-none border-x-0 border-t-0">
      <AlertDescription className="flex flex-wrap items-center gap-2">
        <span>{t("lock.readOnlyHint")}</span>
        <Button size="sm" onClick={onAcquire} disabled={acquiring}>
          {t("lock.acquire")}
        </Button>
      </AlertDescription>
    </Alert>
  )
}
