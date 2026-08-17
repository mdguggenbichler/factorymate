"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { cn } from "@/lib/utils"
import { ApiError } from "@/lib/api"
import { setupAdmin } from "@/lib/auth-client"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"

export function SetupForm({
  className,
  ...props
}: React.ComponentProps<"div">) {
  const t = useTranslations("auth")
  const router = useRouter()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (password !== confirmPassword) {
      toast.error(t("passwordMismatch"))
      return
    }

    setIsSubmitting(true)

    try {
      await setupAdmin(username.trim(), password)
      router.push("/")
      router.refresh()
    } catch (error) {
      if (error instanceof ApiError && error.status === 403) {
        toast.error(t("setupFailed"))
        router.push("/login")
        router.refresh()
      } else {
        toast.error(t("genericError"))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card>
        <CardHeader>
          <CardTitle>{t("setupTitle")}</CardTitle>
          <CardDescription>{t("setupDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="setup-username">{t("username")}</FieldLabel>
                <Input
                  id="setup-username"
                  name="username"
                  autoComplete="username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder={t("usernamePlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="setup-password">{t("password")}</FieldLabel>
                <Input
                  id="setup-password"
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={t("passwordPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="setup-confirm-password">
                  {t("confirmPassword")}
                </FieldLabel>
                <Input
                  id="setup-confirm-password"
                  name="confirmPassword"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  placeholder={t("confirmPasswordPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <Button type="submit" disabled={isSubmitting} className="w-full">
                  {t("setupButton")}
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
