"use client"

import { useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { cn } from "@/lib/utils"
import { apiUrl } from "@/lib/api"
import { ApiError } from "@/lib/api"
import { getAuthConfig, login } from "@/lib/auth-client"
import { Button, buttonVariants } from "@/components/ui/button"
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

export function LoginForm({
  className,
  ...props
}: React.ComponentProps<"div">) {
  const t = useTranslations("auth")
  const router = useRouter()
  const searchParams = useSearchParams()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [discordOAuthEnabled, setDiscordOAuthEnabled] = useState(false)

  useEffect(() => {
    getAuthConfig()
      .then((config) => setDiscordOAuthEnabled(config.discordOAuthEnabled))
      .catch(() => setDiscordOAuthEnabled(false))
  }, [])

  useEffect(() => {
    const error = searchParams.get("error")
    if (error === "not_registered") {
      toast.error(t("discordLoginNotRegistered"))
    } else if (error === "already_registered") {
      toast.error(t("discordLoginAlreadyRegistered"))
    }
  }, [searchParams, t])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    try {
      await login(username.trim(), password)
      router.push("/")
      router.refresh()
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        toast.error(t("loginFailed"))
      } else if (
        error instanceof ApiError &&
        error.status === 403 &&
        error.message === "account pending approval"
      ) {
        toast.error(t("pendingApproval"))
        router.push("/awaiting-approval")
        router.refresh()
      } else {
        toast.error(t("genericError"))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const discordHref = apiUrl("/auth/discord")

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card>
        <CardHeader>
          <CardTitle>{t("loginTitle")}</CardTitle>
          <CardDescription>{t("loginDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {discordOAuthEnabled ? (
            <div className="mb-6 flex flex-col gap-3">
              <a
                href={discordHref}
                className={cn(buttonVariants({ variant: "outline" }), "w-full")}
              >
                {t("discordLoginButton")}
              </a>
              <p className="text-center text-sm text-muted-foreground">
                {t("loginOrDivider")}
              </p>
            </div>
          ) : null}
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="username">{t("username")}</FieldLabel>
                <Input
                  id="username"
                  name="username"
                  autoComplete="username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder={t("usernamePlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="password">{t("password")}</FieldLabel>
                <Input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={t("passwordPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <Button type="submit" disabled={isSubmitting} className="w-full">
                  {t("loginButton")}
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
