"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { CategoryPrefFields } from "@/components/notifications/category-pref-fields"
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
import type { AdminNotificationDefaults } from "@/lib/api-types"

type NotificationDefaultsFormProps = {
  initialDefaults: AdminNotificationDefaults
}

export function NotificationDefaultsForm({
  initialDefaults,
}: NotificationDefaultsFormProps) {
  const t = useTranslations("settings.notificationDefaults")
  const tCommon = useTranslations("common")
  const [defaults, setDefaults] = useState(initialDefaults)
  const [isSubmitting, setIsSubmitting] = useState(false)

  function handleCategoryChange(category: string, enabled: boolean) {
    setDefaults((current) => ({
      ...current,
      categories: {
        ...current.categories,
        [category]: enabled,
      },
    }))
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    try {
      const updated = await apiFetch<AdminNotificationDefaults>(
        "/settings/notification-defaults",
        {
          method: "PUT",
          body: JSON.stringify(defaults),
        }
      )
      setDefaults(updated)
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
            <CategoryPrefFields
              categories={defaults.categories}
              onCategoryChange={handleCategoryChange}
              personalEnabled={defaults.dmPlayerPersonalDefault}
              onPersonalChange={(enabled) =>
                setDefaults((current) => ({
                  ...current,
                  dmPlayerPersonalDefault: enabled,
                }))
              }
              personalFieldId="dm-player-personal-default"
              personalLabelKey="defaultLabel"
              personalDescriptionKey="defaultDescription"
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
