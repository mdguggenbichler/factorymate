"use client"

import { useTranslations } from "next-intl"

import { FormattedDateTime } from "@/components/formatted-datetime"

type MilestoneUnlockMetaProps = {
  techTier: number
  unlockedAt: string
}

export function MilestoneUnlockMeta({
  techTier,
  unlockedAt,
}: MilestoneUnlockMetaProps) {
  const t = useTranslations("home")

  return (
    <span className="text-sm text-muted-foreground">
      {t("milestoneMetaPrefix", { tier: techTier })}{" "}
      <FormattedDateTime iso={unlockedAt} />
    </span>
  )
}
