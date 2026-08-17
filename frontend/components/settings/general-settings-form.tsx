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
import { apiFetch } from "@/lib/api"
import type { AppSettings } from "@/lib/api-types"

type GeneralSettingsFormProps = {
  initialSettings: AppSettings
}

export function GeneralSettingsForm({ initialSettings }: GeneralSettingsFormProps) {
  const t = useTranslations("settings.general")
  const tCommon = useTranslations("common")
  const [settings, setSettings] = useState(initialSettings)
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    try {
      const updated = await apiFetch<AppSettings>("/settings", {
        method: "PUT",
        body: JSON.stringify({
          serverName: settings.serverName,
          frmHost: settings.frmHost,
          frmPort: Number(settings.frmPort),
          frmAuthToken: settings.frmAuthToken,
          pollIntervalSeconds: Number(settings.pollIntervalSeconds),
          productionSnapshotIntervalSeconds: Number(
            settings.productionSnapshotIntervalSeconds
          ),
          productionSnapshotRetentionDays: Number(
            settings.productionSnapshotRetentionDays
          ),
        }),
      })
      setSettings(updated)
      toast.success(t("saved"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card className="max-w-2xl">
        <CardHeader>
          <CardTitle>{t("formTitle")}</CardTitle>
          <CardDescription>{t("formDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="server-name">{t("serverName")}</FieldLabel>
                <Input
                  id="server-name"
                  value={settings.serverName}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      serverName: event.target.value,
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="frm-host">{t("frmHost")}</FieldLabel>
                <Input
                  id="frm-host"
                  value={settings.frmHost}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      frmHost: event.target.value,
                    }))
                  }
                  placeholder={t("frmHostPlaceholder")}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="frm-port">{t("frmPort")}</FieldLabel>
                <Input
                  id="frm-port"
                  type="number"
                  min={1}
                  value={settings.frmPort}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      frmPort: Number(event.target.value),
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="frm-auth-token">{t("frmAuthToken")}</FieldLabel>
                <Input
                  id="frm-auth-token"
                  type="password"
                  value={settings.frmAuthToken}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      frmAuthToken: event.target.value,
                    }))
                  }
                  placeholder={t("frmAuthTokenPlaceholder")}
                />
                <p className="text-sm text-muted-foreground">
                  {t("frmAuthTokenHint")}
                </p>
              </Field>
              <Field>
                <FieldLabel htmlFor="poll-interval">
                  {t("pollIntervalSeconds")}
                </FieldLabel>
                <Input
                  id="poll-interval"
                  type="number"
                  min={1}
                  value={settings.pollIntervalSeconds}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      pollIntervalSeconds: Number(event.target.value),
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="snapshot-interval">
                  {t("productionSnapshotIntervalSeconds")}
                </FieldLabel>
                <Input
                  id="snapshot-interval"
                  type="number"
                  min={1}
                  value={settings.productionSnapshotIntervalSeconds}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      productionSnapshotIntervalSeconds: Number(
                        event.target.value
                      ),
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="snapshot-retention">
                  {t("productionSnapshotRetentionDays")}
                </FieldLabel>
                <Input
                  id="snapshot-retention"
                  type="number"
                  min={1}
                  value={settings.productionSnapshotRetentionDays}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      productionSnapshotRetentionDays: Number(
                        event.target.value
                      ),
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <Button type="submit" disabled={isSubmitting}>
                  {tCommon("save")}
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
