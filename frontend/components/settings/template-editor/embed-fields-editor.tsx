"use client"

import { PlusIcon, Trash2Icon } from "lucide-react"
import { useTranslations } from "next-intl"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { EmbedField } from "@/lib/api-types"

type EmbedFieldsEditorProps = {
  fields: EmbedField[]
  onChange: (fields: EmbedField[]) => void
}

export function EmbedFieldsEditor({
  fields,
  onChange,
}: EmbedFieldsEditorProps) {
  const t = useTranslations("settings.templates")

  function updateField(index: number, patch: Partial<EmbedField>) {
    onChange(
      fields.map((field, fieldIndex) =>
        fieldIndex === index ? { ...field, ...patch } : field
      )
    )
  }

  function removeField(index: number) {
    onChange(fields.filter((_, fieldIndex) => fieldIndex !== index))
  }

  function addField() {
    onChange([...fields, { name: "", value: "", inline: true }])
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <FieldLabel>{t("embedFields")}</FieldLabel>
        <Button type="button" variant="outline" size="sm" onClick={addField}>
          <PlusIcon data-icon="inline-start" />
          {t("addField")}
        </Button>
      </div>

      {fields.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("fieldsEmpty")}</p>
      ) : (
        <div className="flex flex-col gap-4">
          {fields.map((field, index) => (
            <div
              key={index}
              className="rounded-lg border p-4 flex flex-col gap-3"
            >
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">
                  {t("fieldNumber", { number: index + 1 })}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => removeField(index)}
                >
                  <Trash2Icon />
                  {t("removeField")}
                </Button>
              </div>
              <Field>
                <FieldLabel htmlFor={`field-name-${index}`}>
                  {t("fieldName")}
                </FieldLabel>
                <Input
                  id={`field-name-${index}`}
                  value={field.name}
                  onChange={(event) =>
                    updateField(index, { name: event.target.value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor={`field-value-${index}`}>
                  {t("fieldValue")}
                </FieldLabel>
                <Input
                  id={`field-value-${index}`}
                  value={field.value}
                  onChange={(event) =>
                    updateField(index, { value: event.target.value })
                  }
                />
              </Field>
              <div className="flex items-center gap-2">
                <Checkbox
                  id={`field-inline-${index}`}
                  checked={field.inline}
                  onCheckedChange={(checked) =>
                    updateField(index, { inline: Boolean(checked) })
                  }
                />
                <FieldLabel htmlFor={`field-inline-${index}`} className="mb-0">
                  {t("fieldInline")}
                </FieldLabel>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
