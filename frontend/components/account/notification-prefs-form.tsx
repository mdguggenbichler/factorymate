"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { TypePrefFields } from "@/components/notifications/type-pref-fields"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field } from "@/components/ui/field"
import { apiFetch } from "@/lib/api"
import type { UserNotificationPrefs } from "@/lib/api-types"

type NotificationPrefsFormProps = {
  initialPrefs: UserNotificationPrefs
}

export function NotificationPrefsForm({
  initialPrefs,
}: NotificationPrefsFormProps) {
  const t = useTranslations("account.notifications")
  const tLayers = useTranslations("notifications.layers")
  const tCommon = useTranslations("common")
  const [prefs, setPrefs] = useState(initialPrefs)
  const [isSubmitting, setIsSubmitting] = useState(false)

  function handleTypeChange(key: string, enabled: boolean) {
    setPrefs((current) => ({
      ...current,
      types: {
        ...current.types,
        [key]: enabled,
      },
    }))
  }

  function handleCategorySet(keys: string[], enabled: boolean) {
    setPrefs((current) => {
      const types = { ...current.types }
      for (const key of keys) {
        types[key] = enabled
      }
      return { ...current, types }
    })
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    try {
      const updated = await apiFetch<UserNotificationPrefs>(
        "/account/notifications",
        {
          method: "PUT",
          body: JSON.stringify({
            types: prefs.types,
            dmPlayerPersonal: prefs.dmPlayerPersonal,
          }),
        }
      )
      setPrefs(updated)
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

      <Alert className="max-w-2xl">
        <AlertTitle>{tLayers("title")}</AlertTitle>
        <AlertDescription>
          <p>{tLayers("intro")}</p>
          <p className="mt-2">{tLayers("channel")}</p>
          <p>{tLayers("dm")}</p>
        </AlertDescription>
      </Alert>

      <Card className="max-w-2xl">
        <CardHeader>
          <CardTitle>{t("formTitle")}</CardTitle>
          <CardDescription>{t("formDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <TypePrefFields
              types={prefs.types}
              catalog={prefs.catalog ?? []}
              onTypeChange={handleTypeChange}
              onCategorySet={handleCategorySet}
              personalEnabled={prefs.dmPlayerPersonal}
              onPersonalChange={(enabled) =>
                setPrefs((current) => ({
                  ...current,
                  dmPlayerPersonal: enabled,
                }))
              }
              personalFieldId="dm-player-personal"
              personalLabelKey="label"
              personalDescriptionKey="description"
            />
            <Field className="mt-4">
              <Button type="submit" disabled={isSubmitting}>
                {tCommon("save")}
              </Button>
            </Field>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
