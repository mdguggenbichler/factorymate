"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

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
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { apiFetch } from "@/lib/api"
import type { AppSettings, ConnectionDetails, GameAPITestResponse } from "@/lib/api-types"
import { useFormatDateTime } from "@/hooks/use-format-datetime"

type ConnectionSettingsFormProps = {
  initialDetails: ConnectionDetails
  initialSettings: AppSettings
}

export function ConnectionSettingsForm({
  initialDetails,
  initialSettings,
}: ConnectionSettingsFormProps) {
  const t = useTranslations("settings.connection")
  const { formatDateTime } = useFormatDateTime()
  const tCommon = useTranslations("common")
  const [details, setDetails] = useState(initialDetails)
  const [gameHost, setGameHost] = useState(initialDetails.gameHost ?? "")
  const [gamePort, setGamePort] = useState(
    initialDetails.gamePort ? String(initialDetails.gamePort) : ""
  )
  const [gamePassword, setGamePassword] = useState(initialDetails.gamePassword ?? "")
  const [notes, setNotes] = useState(initialDetails.notes ?? "")
  const [smmProfileName, setSmmProfileName] = useState(
    initialDetails.smmProfileName ?? ""
  )
  const [clearPassword, setClearPassword] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [gameApiSettings, setGameApiSettings] = useState(initialSettings)
  const [gameApiToken, setGameApiToken] = useState("")
  const [clearGameApiToken, setClearGameApiToken] = useState(false)
  const [isSavingGameApi, setIsSavingGameApi] = useState(false)
  const [isTestingGameApi, setIsTestingGameApi] = useState(false)
  const [gameApiTestResult, setGameApiTestResult] = useState<GameAPITestResponse | null>(
    null
  )

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    try {
      const updated = await apiFetch<ConnectionDetails>("/connection-details", {
        method: "PUT",
        body: JSON.stringify({
          gameHost,
          gamePort: Number(gamePort),
          gamePassword: gamePassword || undefined,
          notes,
          clearPassword,
          smmProfileName: smmProfileName || undefined,
        }),
      })
      setDetails(updated)
      setSmmProfileName(updated.smmProfileName ?? smmProfileName)
      setClearPassword(false)
      toast.success(t("saved"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleSaveGameApi(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSavingGameApi(true)
    try {
      const body: Record<string, unknown> = {
        gameApiHost: gameApiSettings.gameApiHost,
        gameApiPort: Number(gameApiSettings.gameApiPort) || 7777,
      }
      if (gameApiToken.trim()) {
        body.gameApiToken = gameApiToken.trim()
      }
      if (clearGameApiToken) {
        body.clearGameApiToken = true
      }
      const updated = await apiFetch<AppSettings>("/settings", {
        method: "PUT",
        body: JSON.stringify(body),
      })
      setGameApiSettings(updated)
      setGameApiToken("")
      setClearGameApiToken(false)
      setGameApiTestResult(null)
      toast.success(t("gameApi.saved"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSavingGameApi(false)
    }
  }

  async function handleTestGameApi() {
    setIsTestingGameApi(true)
    try {
      const result = await apiFetch<GameAPITestResponse>("/settings/game-api/test", {
        method: "POST",
        body: JSON.stringify({
          gameApiHost: gameApiSettings.gameApiHost,
          gameApiPort: Number(gameApiSettings.gameApiPort) || 7777,
          gameApiToken: gameApiToken.trim() || undefined,
        }),
      })
      setGameApiTestResult(result)
      toast.success(t("gameApi.testOk"))
    } catch {
      setGameApiTestResult(null)
      toast.error(t("gameApi.testFailed"))
    } finally {
      setIsTestingGameApi(false)
    }
  }

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("formTitle")}</CardTitle>
          <CardDescription>{t("formDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="game-host">{t("fields.gameHost")}</FieldLabel>
                <Input
                  id="game-host"
                  value={gameHost}
                  onChange={(event) => setGameHost(event.target.value)}
                  placeholder={t("fields.gameHostPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="game-port">{t("fields.gamePort")}</FieldLabel>
                <Input
                  id="game-port"
                  type="number"
                  min={1}
                  value={gamePort}
                  onChange={(event) => setGamePort(event.target.value)}
                  placeholder={t("fields.gamePortPlaceholder")}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="game-password">
                  {t("fields.gamePassword")}
                </FieldLabel>
                <Input
                  id="game-password"
                  type="password"
                  value={gamePassword}
                  onChange={(event) => setGamePassword(event.target.value)}
                  placeholder={t("fields.gamePasswordPlaceholder")}
                />
              </Field>
              <Field className="flex items-center gap-3">
                <Switch
                  id="clear-password"
                  checked={clearPassword}
                  onCheckedChange={setClearPassword}
                />
                <FieldLabel htmlFor="clear-password" className="mb-0">
                  {t("fields.clearPassword")}
                </FieldLabel>
              </Field>
              <Field>
                <FieldLabel htmlFor="connection-notes">{t("fields.notes")}</FieldLabel>
                <Textarea
                  id="connection-notes"
                  value={notes}
                  onChange={(event) => setNotes(event.target.value)}
                  placeholder={t("fields.notesPlaceholder")}
                  rows={3}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="smm-profile-name">
                  {t("fields.smmProfileName")}
                </FieldLabel>
                <Input
                  id="smm-profile-name"
                  value={smmProfileName}
                  onChange={(event) => setSmmProfileName(event.target.value)}
                  placeholder={t("fields.smmProfileNamePlaceholder")}
                />
                <p className="text-sm text-muted-foreground">
                  {t("fields.smmProfileNameHint")}
                </p>
              </Field>
            </FieldGroup>
            {details.updatedAt ? (
              <p className="mt-4 text-sm text-muted-foreground">
                {t("lastUpdated", { date: formatDateTime(details.updatedAt) })}
              </p>
            ) : null}
            <div className="mt-6 flex justify-end">
              <Button type="submit" disabled={isSubmitting}>
                {tCommon("save")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("gameApi.title")}</CardTitle>
          <CardDescription>{t("gameApi.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSaveGameApi}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="game-api-host">{t("gameApi.fields.host")}</FieldLabel>
                <Input
                  id="game-api-host"
                  value={gameApiSettings.gameApiHost}
                  onChange={(event) =>
                    setGameApiSettings((current) => ({
                      ...current,
                      gameApiHost: event.target.value,
                    }))
                  }
                  placeholder={t("gameApi.fields.hostPlaceholder")}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="game-api-port">{t("gameApi.fields.port")}</FieldLabel>
                <Input
                  id="game-api-port"
                  type="number"
                  min={1}
                  value={gameApiSettings.gameApiPort || 7777}
                  onChange={(event) =>
                    setGameApiSettings((current) => ({
                      ...current,
                      gameApiPort: Number(event.target.value),
                    }))
                  }
                  placeholder="7777"
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="game-api-token">{t("gameApi.fields.token")}</FieldLabel>
                <Input
                  id="game-api-token"
                  type="password"
                  value={gameApiToken}
                  onChange={(event) => setGameApiToken(event.target.value)}
                  placeholder={
                    gameApiSettings.gameApiTokenConfigured
                      ? t("gameApi.fields.tokenConfiguredPlaceholder")
                      : t("gameApi.fields.tokenPlaceholder")
                  }
                />
                <p className="text-sm text-muted-foreground">
                  {t("gameApi.fields.tokenHint")}
                </p>
              </Field>
              <Field className="flex items-center gap-3">
                <Switch
                  id="clear-game-api-token"
                  checked={clearGameApiToken}
                  onCheckedChange={setClearGameApiToken}
                />
                <FieldLabel htmlFor="clear-game-api-token" className="mb-0">
                  {t("gameApi.fields.clearToken")}
                </FieldLabel>
              </Field>
            </FieldGroup>
            {gameApiTestResult ? (
              <p className="mt-4 text-sm text-muted-foreground">
                {t("gameApi.testResult", {
                  session: gameApiTestResult.activeSessionName,
                  save: gameApiTestResult.latestSaveName,
                })}
              </p>
            ) : null}
            <div className="mt-6 flex flex-wrap justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={isTestingGameApi}
                onClick={handleTestGameApi}
              >
                {t("gameApi.testButton")}
              </Button>
              <Button type="submit" disabled={isSavingGameApi}>
                {tCommon("save")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
