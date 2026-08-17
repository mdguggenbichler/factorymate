"use client"

import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"

import { logout } from "@/lib/auth-client"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

export function AwaitingApprovalContent() {
  const t = useTranslations("auth.awaitingApproval")
  const router = useRouter()

  async function handleLogout() {
    await logout()
    router.push("/login")
    router.refresh()
  }

  return (
    <div className="flex flex-1 flex-col items-center justify-center p-4 md:p-6">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
          <CardDescription>{t("description")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">{t("hint")}</p>
          <Button type="button" variant="outline" onClick={() => void handleLogout()}>
            {t("logout")}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
