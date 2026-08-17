"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"

import { ColorPicker } from "@/components/settings/template-editor/color-picker"
import { EmbedFieldsEditor } from "@/components/settings/template-editor/embed-fields-editor"
import { EmbedPreview } from "@/components/settings/template-editor/embed-preview"
import { VariablePicker } from "@/components/settings/template-editor/variable-picker"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { apiFetch } from "@/lib/api"
import type {
  EmbedTemplate,
  MessageTemplate,
  MessageType,
  NotificationTarget,
  RenderedPreview,
} from "@/lib/api-types"

type TemplatesViewProps = {
  initialMessageTypes: MessageType[]
  targets: NotificationTarget[]
}

const defaultEmbed: EmbedTemplate = {
  title: "",
  description: "",
  color: "#5865F2",
  fields: [],
}

function cloneTemplate(template: MessageTemplate): MessageTemplate {
  return {
    plain: template.plain ?? "",
    embed: template.embed
      ? {
          ...template.embed,
          fields: template.embed.fields.map((field) => ({ ...field })),
        }
      : { ...defaultEmbed, fields: [] },
  }
}

export function TemplatesView({
  initialMessageTypes,
  targets,
}: TemplatesViewProps) {
  const t = useTranslations("settings.templates")
  const tCommon = useTranslations("common")
  const [messageTypes, setMessageTypes] = useState(initialMessageTypes)
  const initialSelected = initialMessageTypes[0]
  const [selectedKey, setSelectedKey] = useState<string | null>(
    initialSelected?.key ?? null
  )
  const [editorTab, setEditorTab] = useState<"plain" | "embed">("embed")
  const [draft, setDraft] = useState<MessageTemplate | null>(
    initialSelected ? cloneTemplate(initialSelected.template) : null
  )
  const [targetIds, setTargetIds] = useState<number[]>(
    initialSelected ? [...initialSelected.targetIds] : []
  )
  const [preview, setPreview] = useState<RenderedPreview | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [isResetting, setIsResetting] = useState(false)
  const plainRef = useRef<HTMLTextAreaElement>(null)
  const descriptionRef = useRef<HTMLTextAreaElement>(null)

  const selected = useMemo(
    () => messageTypes.find((item) => item.key === selectedKey) ?? null,
    [messageTypes, selectedKey]
  )

  function selectMessageType(key: string) {
    const item = messageTypes.find((messageType) => messageType.key === key)
    if (!item) {
      return
    }
    setSelectedKey(key)
    setDraft(cloneTemplate(item.template))
    setTargetIds([...item.targetIds])
    setPreview(null)
  }

  const enabledTargets = useMemo(
    () => targets.filter((target) => target.enabled),
    [targets]
  )

  const insertVariable = useCallback(
    (variable: string, ref: React.RefObject<HTMLTextAreaElement | null>) => {
      const token = `{${variable}}`
      if (!draft) {
        return
      }

      const element = ref.current
      if (element) {
        const start = element.selectionStart ?? element.value.length
        const end = element.selectionEnd ?? start
        const current = element.value
        const next = `${current.slice(0, start)}${token}${current.slice(end)}`
        if (editorTab === "plain") {
          setDraft((currentDraft) =>
            currentDraft ? { ...currentDraft, plain: next } : currentDraft
          )
        } else {
          setDraft((currentDraft) =>
            currentDraft
              ? {
                  ...currentDraft,
                  embed: {
                    ...(currentDraft.embed ?? defaultEmbed),
                    description: next,
                  },
                }
              : currentDraft
          )
        }
        requestAnimationFrame(() => {
          element.focus()
          const cursor = start + token.length
          element.setSelectionRange(cursor, cursor)
        })
        return
      }

      if (editorTab === "plain") {
        setDraft((currentDraft) =>
          currentDraft
            ? { ...currentDraft, plain: `${currentDraft.plain ?? ""}${token}` }
            : currentDraft
        )
      }
    },
    [draft, editorTab]
  )

  useEffect(() => {
    if (!selected || !draft) {
      return
    }

    const timeout = window.setTimeout(async () => {
      try {
        const body = {
          variant: editorTab,
          template: {
            ...(draft.plain ? { plain: draft.plain } : {}),
            ...(draft.embed ? { embed: draft.embed } : {}),
          },
        }
        const rendered = await apiFetch<RenderedPreview>(
          `/message-types/${selected.key}/template/preview`,
          {
            method: "POST",
            body: JSON.stringify(body),
          }
        )
        setPreview(rendered)
      } catch {
        setPreview(null)
      }
    }, 400)

    return () => window.clearTimeout(timeout)
  }, [draft, editorTab, selected])

  async function toggleEnabled(key: string, enabled: boolean) {
    try {
      await apiFetch(`/message-types/${key}/enabled`, {
        method: "PUT",
        body: JSON.stringify({ enabled }),
      })
      setMessageTypes((current) =>
        current.map((item) =>
          item.key === key ? { ...item, enabled } : item
        )
      )
    } catch {
      toast.error(tCommon("error"))
    }
  }

  async function saveTargets(key: string, ids: number[]) {
    try {
      const response = await apiFetch<{ targetIds: number[] }>(
        `/message-types/${key}/targets`,
        {
          method: "PUT",
          body: JSON.stringify({ targetIds: ids }),
        }
      )
      setTargetIds(response.targetIds)
      setMessageTypes((current) =>
        current.map((item) =>
          item.key === key ? { ...item, targetIds: response.targetIds } : item
        )
      )
      toast.success(t("targetsSaved"))
    } catch {
      toast.error(tCommon("error"))
    }
  }

  async function handleSaveTemplate() {
    if (!selected || !draft) {
      return
    }

    setIsSaving(true)
    try {
      const body =
        editorTab === "plain"
          ? { plain: draft.plain ?? "" }
          : { embed: draft.embed ?? defaultEmbed }

      const updated = await apiFetch<MessageTemplate>(
        `/message-types/${selected.key}/template`,
        {
          method: "PUT",
          body: JSON.stringify(body),
        }
      )
      setDraft(cloneTemplate(updated))
      setMessageTypes((current) =>
        current.map((item) =>
          item.key === selected.key ? { ...item, template: updated } : item
        )
      )
      toast.success(t("templateSaved"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSaving(false)
    }
  }

  async function handleReset() {
    if (!selected) {
      return
    }

    setIsResetting(true)
    try {
      const variant = editorTab === "plain" ? "plain" : "embed"
      const updated = await apiFetch<MessageTemplate>(
        `/message-types/${selected.key}/template/reset?variant=${variant}`,
        { method: "POST" }
      )
      setDraft(cloneTemplate(updated))
      setMessageTypes((current) =>
        current.map((item) =>
          item.key === selected.key ? { ...item, template: updated } : item
        )
      )
      toast.success(t("templateReset"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsResetting(false)
    }
  }

  function toggleTargetAssignment(targetId: number, checked: boolean) {
    if (!selected) {
      return
    }

    const next = checked
      ? [...targetIds, targetId]
      : targetIds.filter((id) => id !== targetId)
    setTargetIds(next)
    saveTargets(selected.key, next)
  }

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <Card>
          <CardHeader>
            <CardTitle>{t("listTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.label")}</TableHead>
                  <TableHead>{t("columns.category")}</TableHead>
                  <TableHead>{t("columns.enabled")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {messageTypes.map((item) => (
                  <TableRow
                    key={item.key}
                    data-state={item.key === selectedKey ? "selected" : undefined}
                    className="cursor-pointer"
                    onClick={() => selectMessageType(item.key)}
                  >
                    <TableCell className="font-medium">{item.label}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{item.category}</Badge>
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={item.enabled}
                        onCheckedChange={(checked) =>
                          toggleEnabled(item.key, Boolean(checked))
                        }
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {selected && draft ? (
          <div className="flex flex-col gap-6">
            <Card>
              <CardHeader>
                <CardTitle>{selected.label}</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-6">
                <div>
                  <FieldLabel className="mb-3">{t("targetAssignment")}</FieldLabel>
                  {enabledTargets.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      {t("noTargets")}
                    </p>
                  ) : (
                    <div className="flex flex-col gap-2">
                      {enabledTargets.map((target) => (
                        <div
                          key={target.id}
                          className="flex items-center gap-2"
                        >
                          <Checkbox
                            id={`target-${target.id}`}
                            checked={targetIds.includes(target.id)}
                            onCheckedChange={(checked) =>
                              toggleTargetAssignment(target.id, Boolean(checked))
                            }
                          />
                          <FieldLabel
                            htmlFor={`target-${target.id}`}
                            className="mb-0 font-normal"
                          >
                            {target.name}
                          </FieldLabel>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <Separator />

                <Tabs
                  value={editorTab}
                  onValueChange={(value) =>
                    setEditorTab(value as "plain" | "embed")
                  }
                >
                  <TabsList>
                    <TabsTrigger value="plain">{t("tabPlain")}</TabsTrigger>
                    <TabsTrigger value="embed">{t("tabEmbed")}</TabsTrigger>
                  </TabsList>

                  <TabsContent value="plain" className="mt-4 flex flex-col gap-4">
                    <div className="flex items-center justify-between gap-2">
                      <FieldLabel>{t("plainTemplate")}</FieldLabel>
                      <VariablePicker
                        variables={selected.variables}
                        onSelect={(variable) =>
                          insertVariable(variable, plainRef)
                        }
                      />
                    </div>
                    <Textarea
                      ref={plainRef}
                      value={draft.plain ?? ""}
                      onChange={(event) =>
                        setDraft((currentDraft) =>
                          currentDraft
                            ? { ...currentDraft, plain: event.target.value }
                            : currentDraft
                        )
                      }
                      rows={6}
                    />
                    {preview?.plain ? (
                      <Card>
                        <CardHeader>
                          <CardTitle>{t("plainPreviewTitle")}</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <p className="text-sm whitespace-pre-wrap">
                            {preview.plain}
                          </p>
                        </CardContent>
                      </Card>
                    ) : null}
                  </TabsContent>

                  <TabsContent value="embed" className="mt-4 flex flex-col gap-4">
                    <FieldGroup>
                      <Field>
                        <FieldLabel htmlFor="embed-title">
                          {t("embedTitle")}
                        </FieldLabel>
                        <Input
                          id="embed-title"
                          value={draft.embed?.title ?? ""}
                          onChange={(event) =>
                            setDraft((currentDraft) =>
                              currentDraft
                                ? {
                                    ...currentDraft,
                                    embed: {
                                      ...(currentDraft.embed ?? defaultEmbed),
                                      title: event.target.value,
                                    },
                                  }
                                : currentDraft
                            )
                          }
                        />
                      </Field>
                      <Field>
                        <div className="flex items-center justify-between gap-2">
                          <FieldLabel htmlFor="embed-description">
                            {t("embedDescription")}
                          </FieldLabel>
                          <VariablePicker
                            variables={selected.variables}
                            onSelect={(variable) =>
                              insertVariable(variable, descriptionRef)
                            }
                          />
                        </div>
                        <Textarea
                          ref={descriptionRef}
                          id="embed-description"
                          value={draft.embed?.description ?? ""}
                          onChange={(event) =>
                            setDraft((currentDraft) =>
                              currentDraft
                                ? {
                                    ...currentDraft,
                                    embed: {
                                      ...(currentDraft.embed ?? defaultEmbed),
                                      description: event.target.value,
                                    },
                                  }
                                : currentDraft
                            )
                          }
                          rows={4}
                        />
                      </Field>
                      <Field>
                        <FieldLabel>{t("embedColor")}</FieldLabel>
                        <ColorPicker
                          value={draft.embed?.color ?? defaultEmbed.color}
                          onChange={(color) =>
                            setDraft((currentDraft) =>
                              currentDraft
                                ? {
                                    ...currentDraft,
                                    embed: {
                                      ...(currentDraft.embed ?? defaultEmbed),
                                      color,
                                    },
                                  }
                                : currentDraft
                            )
                          }
                        />
                      </Field>
                    </FieldGroup>

                    <EmbedFieldsEditor
                      fields={draft.embed?.fields ?? []}
                      onChange={(fields) =>
                        setDraft((currentDraft) =>
                          currentDraft
                            ? {
                                ...currentDraft,
                                embed: {
                                  ...(currentDraft.embed ?? defaultEmbed),
                                  fields,
                                },
                              }
                            : currentDraft
                        )
                      }
                    />

                    <EmbedPreview embed={preview?.embed ?? draft.embed} />
                  </TabsContent>
                </Tabs>

                <div className="flex flex-wrap gap-2">
                  <Button onClick={handleSaveTemplate} disabled={isSaving}>
                    {tCommon("save")}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleReset}
                    disabled={isResetting}
                  >
                    {t("resetToDefault")}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        ) : (
          <Card>
            <CardContent className="py-8 text-sm text-muted-foreground">
              {t("selectMessageType")}
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
