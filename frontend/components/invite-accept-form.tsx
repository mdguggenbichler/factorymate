"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { cn } from "@/lib/utils"
import { ApiError } from "@/lib/api"
import { acceptInvite } from "@/lib/auth-client"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
import { formatDateTime } from "@/lib/format"

type InviteAcceptFormProps = {
  token: string
  role?: string
  expiresAt?: string
  errorStatus?: number
}

export function InviteAcceptForm({
  token,
  role,
  expiresAt,
  errorStatus,
}: InviteAcceptFormProps) {
  const t = useTranslations("invite")
  const tAuth = useTranslations("auth")
  const router = useRouter()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (errorStatus != null) {
    const message =
      errorStatus === 404
        ? t("notFound")
        : errorStatus === 410
          ? t("invalid")
          : t("genericError")
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
          <CardDescription>{message}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (password !== confirmPassword) {
      toast.error(tAuth("passwordMismatch"))
      return
    }

    setIsSubmitting(true)

    try {
      await acceptInvite(token, username.trim(), password)
      router.push("/")
      router.refresh()
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        toast.error(t("usernameTaken"))
      } else {
        toast.error(t("genericError"))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className={cn("flex flex-col gap-6")}>
      <Alert>
        <AlertTitle>{t("title")}</AlertTitle>
        <AlertDescription>{t("deprecatedNotice")}</AlertDescription>
      </Alert>
      <Card>
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
          <CardDescription>
            {t("description", {
              role:
                role === "admin" ? tAuth("roleAdmin") : tAuth("roleViewer"),
              expiresAt: expiresAt ? formatDateTime(expiresAt) : "",
            })}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="invite-username">
                  {tAuth("username")}
                </FieldLabel>
                <Input
                  id="invite-username"
                  autoComplete="username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder={tAuth("usernamePlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="invite-password">
                  {tAuth("password")}
                </FieldLabel>
                <Input
                  id="invite-password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={tAuth("passwordPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="invite-confirm-password">
                  {tAuth("confirmPassword")}
                </FieldLabel>
                <Input
                  id="invite-confirm-password"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  placeholder={tAuth("confirmPasswordPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <Button type="submit" disabled={isSubmitting} className="w-full">
                  {t("acceptButton")}
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
