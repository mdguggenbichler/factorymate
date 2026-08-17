"use client"

import { useCallback, useState } from "react"
import { useTranslations } from "next-intl"
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
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
import {
  Select,
  SelectContent,
  SelectGroup,
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
import { apiFetch } from "@/lib/api"
import type { AppUser } from "@/lib/api-types"
import { formatDateTime } from "@/lib/format"

type UserFormState = {
  username: string
  password: string
  role: "admin" | "viewer"
}

const emptyForm: UserFormState = {
  username: "",
  password: "",
  role: "viewer",
}

type UsersViewProps = {
  initialUsers: AppUser[]
}

export function UsersView({ initialUsers }: UsersViewProps) {
  const t = useTranslations("settings.users")
  const tAuth = useTranslations("auth")
  const tCommon = useTranslations("common")
  const [users, setUsers] = useState(initialUsers)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<AppUser | null>(null)
  const [form, setForm] = useState<UserFormState>(emptyForm)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [deleteUser, setDeleteUser] = useState<AppUser | null>(null)

  const openCreate = useCallback(() => {
    setEditingUser(null)
    setForm(emptyForm)
    setDialogOpen(true)
  }, [])

  const openEdit = useCallback((user: AppUser) => {
    setEditingUser(user)
    setForm({
      username: user.username,
      password: "",
      role: user.role as "admin" | "viewer",
    })
    setDialogOpen(true)
  }, [])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    try {
      if (editingUser) {
        const body: { role?: string; password?: string } = {
          role: form.role,
        }
        if (form.password) {
          body.password = form.password
        }
        const updated = await apiFetch<AppUser>(`/users/${editingUser.id}`, {
          method: "PUT",
          body: JSON.stringify(body),
        })
        setUsers((current) =>
          current.map((user) =>
            user.id === editingUser.id ? { ...user, ...updated } : user
          )
        )
        toast.success(t("updated"))
      } else {
        const created = await apiFetch<AppUser>("/users", {
          method: "POST",
          body: JSON.stringify({
            username: form.username,
            password: form.password,
            role: form.role,
          }),
        })
        setUsers((current) => [...current, created])
        toast.success(t("created"))
      }
      setDialogOpen(false)
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleDelete() {
    if (!deleteUser) {
      return
    }

    try {
      await apiFetch(`/users/${deleteUser.id}`, { method: "DELETE" })
      setUsers((current) =>
        current.filter((user) => user.id !== deleteUser.id)
      )
      toast.success(t("deleted"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setDeleteUser(null)
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
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("columns.username")}</TableHead>
                <TableHead>{t("columns.role")}</TableHead>
                <TableHead>{t("columns.createdAt")}</TableHead>
                <TableHead className="text-right">{t("columns.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">{user.username}</TableCell>
                  <TableCell>
                    <Badge variant="outline">
                      {user.role === "admin"
                        ? tAuth("roleAdmin")
                        : tAuth("roleViewer")}
                    </Badge>
                  </TableCell>
                  <TableCell>{formatDateTime(user.createdAt)}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => openEdit(user)}
                      >
                        <PencilIcon />
                        {t("edit")}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setDeleteUser(user)}
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
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editingUser ? t("editTitle") : t("createTitle")}
            </DialogTitle>
            <DialogDescription>
              {editingUser ? t("editDescription") : t("createDescription")}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              {!editingUser && (
                <Field>
                  <FieldLabel htmlFor="user-username">
                    {tAuth("username")}
                  </FieldLabel>
                  <Input
                    id="user-username"
                    value={form.username}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        username: event.target.value,
                      }))
                    }
                    required
                  />
                </Field>
              )}
              <Field>
                <FieldLabel htmlFor="user-password">
                  {editingUser ? t("newPasswordOptional") : tAuth("password")}
                </FieldLabel>
                <Input
                  id="user-password"
                  type="password"
                  value={form.password}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      password: event.target.value,
                    }))
                  }
                  required={!editingUser}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="user-role">{t("fields.role")}</FieldLabel>
                <Select
                  value={form.role}
                  onValueChange={(value) =>
                    setForm((current) => ({
                      ...current,
                      role: value as "admin" | "viewer",
                    }))
                  }
                  items={[
                    { label: tAuth("roleAdmin"), value: "admin" },
                    { label: tAuth("roleViewer"), value: "viewer" },
                  ]}
                >
                  <SelectTrigger id="user-role" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="admin">{tAuth("roleAdmin")}</SelectItem>
                      <SelectItem value="viewer">
                        {tAuth("roleViewer")}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
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
        open={deleteUser != null}
        onOpenChange={(open) => !open && setDeleteUser(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDescription", { username: deleteUser?.username ?? "" })}
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
