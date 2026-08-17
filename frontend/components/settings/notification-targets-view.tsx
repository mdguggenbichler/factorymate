"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { AlertTriangleIcon, PencilIcon, PlusIcon, SendIcon, Trash2Icon } from "lucide-react"
import { toast } from "sonner"

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { apiFetch } from "@/lib/api"
import type {
  DiscordChannel,
  DiscordTargetConfig,
  MessageType,
  NotificationTarget,
} from "@/lib/api-types"

type TargetFormState = {
  name: string
  channelId: string
  threadId: string
  enabled: boolean
}

const emptyForm: TargetFormState = {
  name: "",
  channelId: "",
  threadId: "",
  enabled: true,
}

function isLegacyWebhookTarget(target: NotificationTarget): boolean {
  return Boolean(target.config.webhook_url?.trim())
}

type NotificationTargetsViewProps = {
  initialTargets: NotificationTarget[]
  messageTypes: MessageType[]
}

export function NotificationTargetsView({
  initialTargets,
  messageTypes,
}: NotificationTargetsViewProps) {
  const t = useTranslations("settings.targets")
  const tCommon = useTranslations("common")
  const [targets, setTargets] = useState(initialTargets)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<TargetFormState>(emptyForm)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<NotificationTarget | null>(
    null
  )
  const [channels, setChannels] = useState<DiscordChannel[]>([])
  const [channelsLoading, setChannelsLoading] = useState(false)
  const [channelsError, setChannelsError] = useState(false)

  const hasLegacyTargets = useMemo(
    () => targets.some(isLegacyWebhookTarget),
    [targets]
  )

  const assignmentCounts = useMemo(() => {
    const counts = new Map<number, number>()
    for (const messageType of messageTypes) {
      for (const targetId of messageType.targetIds) {
        counts.set(targetId, (counts.get(targetId) ?? 0) + 1)
      }
    }
    return counts
  }, [messageTypes])

  const loadChannels = useCallback(async () => {
    setChannelsLoading(true)
    setChannelsError(false)
    try {
      const data = await apiFetch<{ channels: DiscordChannel[] }>(
        "/discord/channels"
      )
      setChannels(data.channels)
    } catch {
      setChannels([])
      setChannelsError(true)
    } finally {
      setChannelsLoading(false)
    }
  }, [])

  const openCreate = useCallback(() => {
    setEditingId(null)
    setForm(emptyForm)
    setDialogOpen(true)
    void loadChannels()
  }, [loadChannels])

  const openEdit = useCallback((target: NotificationTarget) => {
    setEditingId(target.id)
    setForm({
      name: target.name,
      channelId: target.config.channel_id ?? "",
      threadId: target.config.thread_id ?? "",
      enabled: target.enabled,
    })
    setDialogOpen(true)
    void loadChannels()
  }, [loadChannels])

  function channelLabel(target: NotificationTarget): string {
    if (target.config.channel_id) {
      const match = channels.find((ch) => ch.id === target.config.channel_id)
      return match ? `#${match.name}` : target.config.channel_id
    }
    if (target.config.webhook_url) {
      return t("legacyWebhook")
    }
    return "—"
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    const config: DiscordTargetConfig = {
      channel_id: form.channelId,
    }
    if (form.threadId.trim()) {
      config.thread_id = form.threadId.trim()
    }

    const body = {
      name: form.name,
      providerType: "discord",
      config,
      enabled: form.enabled,
    }

    try {
      if (editingId != null) {
        const updated = await apiFetch<NotificationTarget>(
          `/notification-targets/${editingId}`,
          {
            method: "PUT",
            body: JSON.stringify(body),
          }
        )
        setTargets((current) =>
          current.map((target) =>
            target.id === editingId ? { ...target, ...updated } : target
          )
        )
        toast.success(t("updated"))
      } else {
        const created = await apiFetch<NotificationTarget>(
          "/notification-targets",
          {
            method: "POST",
            body: JSON.stringify(body),
          }
        )
        setTargets((current) => [...current, created])
        toast.success(t("created"))
      }
      setDialogOpen(false)
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleTest(targetId: number) {
    setTestingId(targetId)
    try {
      await apiFetch(`/notification-targets/${targetId}/test`, {
        method: "POST",
      })
      toast.success(t("testSent"))
    } catch {
      toast.error(t("testFailed"))
    } finally {
      setTestingId(null)
    }
  }

  async function handleDelete() {
    if (!deleteTarget) {
      return
    }

    try {
      await apiFetch(`/notification-targets/${deleteTarget.id}`, {
        method: "DELETE",
      })
      setTargets((current) =>
        current.filter((target) => target.id !== deleteTarget.id)
      )
      toast.success(t("deleted"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setDeleteTarget(null)
    }
  }

  const channelSelectItems = useMemo(
    () =>
      channels.map((channel) => ({
        label: `#${channel.name}`,
        value: channel.id,
      })),
    [channels]
  )

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
          <p className="text-muted-foreground">{t("description")}</p>
        </div>
        <Button onClick={openCreate}>
          <PlusIcon data-icon="inline-start" />
          {t("create")}
        </Button>
      </div>

      {hasLegacyTargets ? (
        <Alert>
          <AlertTriangleIcon />
          <AlertTitle>{t("legacyBannerTitle")}</AlertTitle>
          <AlertDescription>{t("legacyBannerDescription")}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("tableTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {targets.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.name")}</TableHead>
                  <TableHead>{t("columns.channel")}</TableHead>
                  <TableHead>{t("columns.provider")}</TableHead>
                  <TableHead>{t("columns.enabled")}</TableHead>
                  <TableHead className="text-right">{t("columns.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((target) => (
                  <TableRow key={target.id}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2">
                        {target.name}
                        {isLegacyWebhookTarget(target) ? (
                          <Badge variant="secondary">{t("legacyBadge")}</Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {channelLabel(target)}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{target.providerType}</Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={target.enabled ? "default" : "secondary"}>
                        {target.enabled ? t("enabled") : t("disabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleTest(target.id)}
                          disabled={
                            testingId === target.id ||
                            isLegacyWebhookTarget(target)
                          }
                        >
                          <SendIcon />
                          {t("test")}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => openEdit(target)}
                        >
                          <PencilIcon />
                          {t("edit")}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setDeleteTarget(target)}
                        >
                          <Trash2Icon />
                          {t("delete")}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editingId != null ? t("editTitle") : t("createTitle")}
            </DialogTitle>
            <DialogDescription>
              {editingId != null ? t("editDescription") : t("createDescription")}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="target-name">{t("fields.name")}</FieldLabel>
                <Input
                  id="target-name"
                  value={form.name}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="target-channel">
                  {t("fields.channel")}
                </FieldLabel>
                {channelsError ? (
                  <p className="text-sm text-destructive">{t("channelsLoadFailed")}</p>
                ) : null}
                <Select
                  value={form.channelId}
                  onValueChange={(value) =>
                    setForm((current) => ({
                      ...current,
                      channelId: value ?? "",
                    }))
                  }
                  items={channelSelectItems}
                  disabled={channelsLoading || channels.length === 0}
                >
                  <SelectTrigger id="target-channel" className="w-full">
                    <SelectValue placeholder={t("fields.channelPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {channelSelectItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="target-thread">
                  {t("fields.threadId")}
                </FieldLabel>
                <Input
                  id="target-thread"
                  value={form.threadId}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      threadId: event.target.value,
                    }))
                  }
                  placeholder={t("fields.threadIdPlaceholder")}
                />
              </Field>
              <Field className="flex items-center gap-3">
                <Switch
                  id="target-enabled"
                  checked={form.enabled}
                  onCheckedChange={(checked) =>
                    setForm((current) => ({
                      ...current,
                      enabled: checked,
                    }))
                  }
                />
                <FieldLabel htmlFor="target-enabled" className="mb-0">
                  {t("fields.enabled")}
                </FieldLabel>
              </Field>
            </FieldGroup>
            <DialogFooter className="mt-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
              >
                {tCommon("cancel")}
              </Button>
              <Button
                type="submit"
                disabled={isSubmitting || !form.channelId}
              >
                {tCommon("save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget != null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDescription", { name: deleteTarget?.name ?? "" })}
              {deleteTarget &&
                assignmentCounts.get(deleteTarget.id) != null &&
                assignmentCounts.get(deleteTarget.id)! > 0 && (
                  <span className="mt-2 block font-medium text-foreground">
                    {t("deleteCascadeWarning", {
                      count: assignmentCounts.get(deleteTarget.id)!,
                    })}
                  </span>
                )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>
              {t("deleteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
