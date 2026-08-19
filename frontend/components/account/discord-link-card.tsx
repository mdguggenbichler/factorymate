"use client"

import { useTranslations } from "next-intl"

import { apiUrl } from "@/lib/api"
import type { User } from "@/lib/auth-types"
import { cn } from "@/lib/utils"
import { buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

type DiscordLinkCardProps = {
  user: User
}

export function DiscordLinkCard({ user }: DiscordLinkCardProps) {
  const t = useTranslations("auth")

  const linked = Boolean(user.externalUserId)
  const linkHref = apiUrl("/account/discord/link")

  return (
    <Card className="max-w-lg">
      <CardHeader>
        <CardTitle>{t("discordLinkTitle")}</CardTitle>
        <CardDescription>
          {linked ? t("discordLinkedDescription") : t("discordLinkDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {linked ? (
          <p className="text-sm text-muted-foreground">
            {t("discordLinkedAs", {
              name: user.externalDisplayName ?? user.externalUsername ?? user.externalUserId ?? "",
            })}
          </p>
        ) : (
          <a href={linkHref} className={cn(buttonVariants())}>
            {t("discordLinkButton")}
          </a>
        )}
      </CardContent>
    </Card>
  )
}
