"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { PencilIcon, PlusIcon, SendIcon, Trash2Icon } from "lucide-react"
import { toast } from "sonner"

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
  MessageType,
  NotificationTarget,
} from "@/lib/api-types"

type TargetFormState = {
  name: string
  webhookUrl: string
  usernameOverride: string
  avatarUrlOverride: string
  enabled: boolean
}

const emptyForm: TargetFormState = {
  name: "",
  webhookUrl: "",
  usernameOverride: "",
  avatarUrlOverride: "",
  enabled: true,
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

  const assignmentCounts = useMemo(() => {
    const counts = new Map<number, number>()
    for (const messageType of messageTypes) {
      for (const targetId of messageType.targetIds) {
        counts.set(targetId, (counts.get(targetId) ?? 0) + 1)
      }
    }
    return counts
  }, [messageTypes])

  const openCreate = useCallback(() => {
    setEditingId(null)
    setForm(emptyForm)
    setDialogOpen(true)
  }, [])

  const openEdit = useCallback((target: NotificationTarget) => {
    setEditingId(target.id)
    setForm({
      name: target.name,
      webhookUrl: target.config.webhook_url ?? "",
      usernameOverride: target.config.username_override ?? "",
      avatarUrlOverride: target.config.avatar_url_override ?? "",
      enabled: target.enabled,
    })
    setDialogOpen(true)
  }, [])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    const body = {
      name: form.name,
      providerType: "discord",
      config: {
        webhook_url: form.webhookUrl,
        ...(form.usernameOverride
          ? { username_override: form.usernameOverride }
          : {}),
        ...(form.avatarUrlOverride
          ? { avatar_url_override: form.avatarUrlOverride }
          : {}),
      },
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
                  <TableHead>{t("columns.provider")}</TableHead>
                  <TableHead>{t("columns.enabled")}</TableHead>
                  <TableHead className="text-right">{t("columns.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((target) => (
                  <TableRow key={target.id}>
                    <TableCell className="font-medium">{target.name}</TableCell>
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
                          disabled={testingId === target.id}
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
                <FieldLabel htmlFor="webhook-url">
                  {t("fields.webhookUrl")}
                </FieldLabel>
                <Input
                  id="webhook-url"
                  type="url"
                  value={form.webhookUrl}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      webhookUrl: event.target.value,
                    }))
                  }
                  required={editingId == null}
                  placeholder={t("fields.webhookUrlPlaceholder")}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="username-override">
                  {t("fields.usernameOverride")}
                </FieldLabel>
                <Input
                  id="username-override"
                  value={form.usernameOverride}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      usernameOverride: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="avatar-url">
                  {t("fields.avatarUrlOverride")}
                </FieldLabel>
                <Input
                  id="avatar-url"
                  type="url"
                  value={form.avatarUrlOverride}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      avatarUrlOverride: event.target.value,
                    }))
                  }
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
              <Button type="submit" disabled={isSubmitting}>
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
