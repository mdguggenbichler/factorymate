"use client"

import { useTranslations } from "next-intl"

import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Switch } from "@/components/ui/switch"
import {
  NOTIFICATION_DM_CATEGORIES,
  type NotificationCatalogEntry,
} from "@/lib/api-types"

type TypePrefFieldsProps = {
  types: Record<string, boolean>
  catalog: NotificationCatalogEntry[]
  onTypeChange: (key: string, enabled: boolean) => void
  onCategorySet: (keys: string[], enabled: boolean) => void
  personalEnabled: boolean
  onPersonalChange: (enabled: boolean) => void
  personalFieldId: string
  personalLabelKey: string
  personalDescriptionKey: string
}

export function TypePrefFields({
  types,
  catalog,
  onTypeChange,
  onCategorySet,
  personalEnabled,
  onPersonalChange,
  personalFieldId,
  personalLabelKey,
  personalDescriptionKey,
}: TypePrefFieldsProps) {
  const tCategories = useTranslations("notifications.categories")
  const tPersonal = useTranslations("notifications.personalDm")
  const tTypes = useTranslations("notifications.types")

  return (
    <FieldGroup>
      {NOTIFICATION_DM_CATEGORIES.map((category) => {
        const entries = catalog.filter((entry) => entry.category === category)
        if (entries.length === 0) {
          return null
        }
        const keys = entries.map((entry) => entry.key)
        return (
          <div key={category} className="grid gap-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <p className="text-sm font-medium leading-none">
                  {tCategories(`${category}.label`)}
                </p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {tCategories(`${category}.description`)}
                </p>
              </div>
              <div className="flex gap-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => onCategorySet(keys, true)}
                >
                  {tTypes("enableAll")}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => onCategorySet(keys, false)}
                >
                  {tTypes("enableNone")}
                </Button>
              </div>
            </div>
            {entries.map((entry) => {
              const fieldId = `type-${entry.key}`
              const targetNames = entry.channelTargets
                .map((target) => target.name)
                .filter(Boolean)
                .join(", ")
              return (
                <Field key={entry.key} className="flex items-start gap-3 pl-1">
                  <Switch
                    id={fieldId}
                    checked={types[entry.key] ?? false}
                    disabled={!entry.globallyEnabled}
                    onCheckedChange={(checked) =>
                      onTypeChange(entry.key, checked)
                    }
                    className="mt-0.5"
                  />
                  <div className="grid gap-1">
                    <FieldLabel htmlFor={fieldId} className="mb-0">
                      {entry.label}
                    </FieldLabel>
                    {!entry.globallyEnabled ? (
                      <p className="text-sm text-muted-foreground">
                        {tTypes("globallyDisabled")}
                      </p>
                    ) : null}
                    {targetNames ? (
                      <p className="text-sm text-muted-foreground">
                        {tTypes("overlapHint", { targets: targetNames })}
                      </p>
                    ) : null}
                  </div>
                </Field>
              )
            })}
          </div>
        )
      })}
      <Field className="flex items-start gap-3 border-t pt-4">
        <Switch
          id={personalFieldId}
          checked={personalEnabled}
          onCheckedChange={onPersonalChange}
          className="mt-0.5"
        />
        <div className="grid gap-1">
          <FieldLabel htmlFor={personalFieldId} className="mb-0">
            {tPersonal(personalLabelKey)}
          </FieldLabel>
          <p className="text-sm text-muted-foreground">
            {tPersonal(personalDescriptionKey)}
          </p>
        </div>
      </Field>
    </FieldGroup>
  )
}
