"use client"

import { useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { cn } from "@/lib/utils"
import { ApiError } from "@/lib/api"
import { completeRegistration } from "@/lib/auth-client"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"

export function RegisterCompleteForm({
  className,
  ...props
}: React.ComponentProps<"div">) {
  const t = useTranslations("auth")
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = searchParams.get("token") ?? ""

  const [username, setUsername] = useState("")
  const [playerName, setPlayerName] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  useEffect(() => {
    if (!token) {
      toast.error(t("registerComplete.missingToken"))
    }
  }, [token, t])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!token) {
      return
    }

    setIsSubmitting(true)
    try {
      const result = await completeRegistration(token, username.trim(), playerName.trim())
      if (result.pendingApproval) {
        toast.success(t("registerComplete.pendingSubmitted"))
        router.push("/awaiting-approval")
      } else {
        toast.success(t("registerComplete.success"))
        router.push("/")
      }
      router.refresh()
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        toast.error(t("registerComplete.alreadyRegistered"))
      } else {
        toast.error(t("genericError"))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!token) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("registerComplete.title")}</CardTitle>
          <CardDescription>{t("registerComplete.missingToken")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card>
        <CardHeader>
          <CardTitle>{t("registerComplete.title")}</CardTitle>
          <CardDescription>{t("registerComplete.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="register-username">{t("username")}</FieldLabel>
                <Input
                  id="register-username"
                  autoComplete="username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder={t("usernamePlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="register-player">{t("registerComplete.playerName")}</FieldLabel>
                <Input
                  id="register-player"
                  value={playerName}
                  onChange={(event) => setPlayerName(event.target.value)}
                  placeholder={t("registerComplete.playerNamePlaceholder")}
                  required
                />
              </Field>
              <Field>
                <Button type="submit" disabled={isSubmitting} className="w-full">
                  {t("registerComplete.submit")}
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
