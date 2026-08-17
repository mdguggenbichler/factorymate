"use client"

import { useTranslations } from "next-intl"

import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Switch } from "@/components/ui/switch"
import { NOTIFICATION_DM_CATEGORIES } from "@/lib/api-types"

type CategoryPrefFieldsProps = {
  categories: Record<string, boolean>
  onCategoryChange: (category: string, enabled: boolean) => void
  personalEnabled: boolean
  onPersonalChange: (enabled: boolean) => void
  personalFieldId: string
  personalLabelKey: string
  personalDescriptionKey: string
}

export function CategoryPrefFields({
  categories,
  onCategoryChange,
  personalEnabled,
  onPersonalChange,
  personalFieldId,
  personalLabelKey,
  personalDescriptionKey,
}: CategoryPrefFieldsProps) {
  const tCategories = useTranslations("notifications.categories")
  const tPersonal = useTranslations("notifications.personalDm")

  return (
    <FieldGroup>
      {NOTIFICATION_DM_CATEGORIES.map((category) => (
        <Field key={category} className="flex items-start gap-3">
          <Switch
            id={`category-${category}`}
            checked={categories[category] ?? false}
            onCheckedChange={(checked) => onCategoryChange(category, checked)}
            className="mt-0.5"
          />
          <div className="grid gap-1">
            <FieldLabel htmlFor={`category-${category}`} className="mb-0">
              {tCategories(`${category}.label`)}
            </FieldLabel>
            <p className="text-sm text-muted-foreground">
              {tCategories(`${category}.description`)}
            </p>
          </div>
        </Field>
      ))}
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
