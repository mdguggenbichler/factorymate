"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { PlusIcon, Trash2Icon } from "lucide-react"
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
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { apiFetch } from "@/lib/api"
import type {
  PlannerPlanStatus,
  PlannerPlanSummary,
  PlannerPlansListResponse,
} from "@/lib/api-types"
import { useFormatDateTime } from "@/hooks/use-format-datetime"

type StatusFilter = "all" | PlannerPlanStatus

type PlannerListProps = {
  initialPlans: PlannerPlanSummary[]
}

export function PlannerList({ initialPlans }: PlannerListProps) {
  const t = useTranslations("planner")
  const { formatDateTime } = useFormatDateTime()
  const router = useRouter()

  const [plans, setPlans] = useState(initialPlans)
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all")
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState("")
  const [visibility, setVisibility] = useState<"private" | "shared">("private")
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [creating, setCreating] = useState(false)

  const filtered = useMemo(() => {
    if (statusFilter === "all") {
      return plans.filter((p) => p.status !== "archived")
    }
    return plans.filter((p) => p.status === statusFilter)
  }, [plans, statusFilter])

  const reload = useCallback(async (filter: StatusFilter) => {
    const params = new URLSearchParams()
    if (filter === "archived") {
      params.set("includeArchived", "true")
      params.set("status", "archived")
    } else if (filter !== "all") {
      params.set("status", filter)
    }
    const qs = params.toString()
    const data = await apiFetch<PlannerPlansListResponse>(
      `/planner/plans${qs ? `?${qs}` : ""}`
    )
    setPlans(data.plans)
  }, [])

  async function handleCreate() {
    const trimmed = name.trim()
    if (!trimmed) return
    setCreating(true)
    try {
      const plan = await apiFetch<PlannerPlanSummary>("/planner/plans", {
        method: "POST",
        body: JSON.stringify({ name: trimmed, visibility }),
      })
      router.push(`/planner/${plan.id}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("list.createFailed"))
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete() {
    if (deleteId === null) return
    try {
      await apiFetch(`/planner/plans/${deleteId}`, { method: "DELETE" })
      setPlans((prev) => prev.filter((p) => p.id !== deleteId))
      toast.success(t("list.deleted"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("list.deleteFailed"))
    } finally {
      setDeleteId(null)
    }
  }

  function statusBadge(status: PlannerPlanStatus) {
    const variants: Record<PlannerPlanStatus, "default" | "secondary" | "outline" | "destructive"> = {
      planning: "secondary",
      inProgress: "default",
      completed: "outline",
      archived: "destructive",
    }
    return (
      <Badge variant={variants[status]}>{t(`status.${status}`)}</Badge>
    )
  }

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-2xl font-semibold">{t("list.title")}</h1>
          <p className="text-muted-foreground text-sm">{t("list.description")}</p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <PlusIcon className="size-4" />
          {t("list.create")}
        </Button>
      </div>

      <Tabs
        value={statusFilter}
        onValueChange={(v) => {
          const next = v as StatusFilter
          setStatusFilter(next)
          void reload(next)
        }}
      >
        <TabsList>
          <TabsTrigger value="all">{t("list.filterAll")}</TabsTrigger>
          <TabsTrigger value="planning">{t("status.planning")}</TabsTrigger>
          <TabsTrigger value="inProgress">{t("status.inProgress")}</TabsTrigger>
          <TabsTrigger value="completed">{t("status.completed")}</TabsTrigger>
          <TabsTrigger value="archived">{t("status.archived")}</TabsTrigger>
        </TabsList>
      </Tabs>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("list.columnName")}</TableHead>
            <TableHead>{t("list.columnOwner")}</TableHead>
            <TableHead>{t("list.columnVisibility")}</TableHead>
            <TableHead>{t("list.columnStatus")}</TableHead>
            <TableHead>{t("list.columnLock")}</TableHead>
            <TableHead>{t("list.columnUpdated")}</TableHead>
            <TableHead className="w-[80px]" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {filtered.length === 0 ? (
            <TableRow>
              <TableCell colSpan={7} className="text-muted-foreground text-center">
                {t("list.empty")}
              </TableCell>
            </TableRow>
          ) : (
            filtered.map((plan) => (
              <TableRow key={plan.id}>
                <TableCell>
                  <Link
                    href={`/planner/${plan.id}`}
                    className="font-medium hover:underline"
                  >
                    {plan.name}
                  </Link>
                </TableCell>
                <TableCell>{plan.owner.username}</TableCell>
                <TableCell>
                  <Badge variant="outline">
                    {plan.visibilityLabel ??
                      t(`visibility.${plan.visibility}`)}
                  </Badge>
                </TableCell>
                <TableCell>{statusBadge(plan.status)}</TableCell>
                <TableCell>
                  {plan.lock.held
                    ? t("list.lockHeld", {
                        username: plan.lock.username ?? "",
                      })
                    : "—"}
                </TableCell>
                <TableCell>{formatDateTime(plan.updatedAt)}</TableCell>
                <TableCell>
                  {plan.canManage ? (
                    <Button
                      size="icon"
                      variant="ghost"
                      aria-label={t("list.delete")}
                      onClick={() => setDeleteId(plan.id)}
                    >
                      <Trash2Icon className="size-4" />
                    </Button>
                  ) : null}
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("list.createTitle")}</DialogTitle>
            <DialogDescription>{t("list.createDescription")}</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel>{t("list.nameLabel")}</FieldLabel>
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field>
              <FieldLabel>{t("list.visibilityLabel")}</FieldLabel>
              <Select
                value={visibility}
                onValueChange={(v) =>
                  setVisibility(v as "private" | "shared")
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="private">
                    {t("visibility.private")}
                  </SelectItem>
                  <SelectItem value="shared">
                    {t("visibility.shared")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t("list.cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={creating || !name.trim()}>
              {t("list.createConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteId !== null}
        onOpenChange={(open) => !open && setDeleteId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("list.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("list.deleteDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("list.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>
              {t("list.deleteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
