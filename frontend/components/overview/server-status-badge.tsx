"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"

import { Badge } from "@/components/ui/badge"
import { apiFetch } from "@/lib/api"
import type { RecoveryPhase, StatusResponse } from "@/lib/api-types"

type ServerStatusBadgeProps = {
  initialPhase: RecoveryPhase
  initialServerOnline: boolean
}

const POLL_MS = 5000

function badgeVariant(
  phase: RecoveryPhase,
  serverOnline: boolean
): "default" | "destructive" | "secondary" {
  if (phase === "recovering") {
    return "secondary"
  }
  if (serverOnline && phase === "healthy") {
    return "default"
  }
  return "destructive"
}

function labelKey(phase: RecoveryPhase, serverOnline: boolean): string {
  if (phase === "recovering") {
    return "serverRecovering"
  }
  if (serverOnline && phase === "healthy") {
    return "serverOnline"
  }
  return "serverOffline"
}

export function ServerStatusBadge({
  initialPhase,
  initialServerOnline,
}: ServerStatusBadgeProps) {
  const t = useTranslations("home")
  const [phase, setPhase] = useState(initialPhase)
  const [serverOnline, setServerOnline] = useState(initialServerOnline)

  useEffect(() => {
    if (phase === "healthy" && serverOnline) {
      return
    }

    let cancelled = false

    async function poll() {
      try {
        const status = await apiFetch<StatusResponse>("/status")
        if (cancelled) {
          return
        }
        setPhase(status.recoveryPhase)
        setServerOnline(status.serverOnline)
      } catch {
        // Keep last known state on fetch errors.
      }
    }

    void poll()
    const id = window.setInterval(() => {
      void poll()
    }, POLL_MS)

    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [phase, serverOnline])

  return (
    <Badge variant={badgeVariant(phase, serverOnline)}>
      {t(labelKey(phase, serverOnline))}
    </Badge>
  )
}
