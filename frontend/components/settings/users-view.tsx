"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import {
  CopyIcon,
  PencilIcon,
  PlusIcon,
  ShieldIcon,
  Trash2Icon,
} from "lucide-react"
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
import type { AppUser, Invite, Player } from "@/lib/api-types"
import { formatDateTime } from "@/lib/format"

type UserFormState = {
  password: string
  role: "admin" | "viewer"
  playerId: string
}

const emptyForm: UserFormState = {
  password: "",
  role: "viewer",
  playerId: "",
}

type RosterRow =
  | {
      kind: "user"
      user: AppUser
    }
  | {
      kind: "invite"
      invite: Invite
    }

type UsersViewProps = {
  initialUsers: AppUser[]
  initialInvites: Invite[]
}

export function UsersView({ initialUsers, initialInvites }: UsersViewProps) {
  const t = useTranslations("settings.users")
  const tAuth = useTranslations("auth")
  const tCommon = useTranslations("common")
  const [users, setUsers] = useState(initialUsers)
  const [invites, setInvites] = useState(initialInvites)
  const [players, setPlayers] = useState<Player[]>([])
  const [inviteDialogOpen, setInviteDialogOpen] = useState(false)
  const [inviteRole, setInviteRole] = useState<"admin" | "viewer">("viewer")
  const [createdInviteUrl, setCreatedInviteUrl] = useState<string | null>(null)
  const [isSubmittingInvite, setIsSubmittingInvite] = useState(false)
  const [editDialogOpen, setEditDialogOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<AppUser | null>(null)
  const [form, setForm] = useState<UserFormState>(emptyForm)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [deleteUser, setDeleteUser] = useState<AppUser | null>(null)
  const [promoteUser, setPromoteUser] = useState<AppUser | null>(null)
  const [revokeInvite, setRevokeInvite] = useState<Invite | null>(null)

  const roster = useMemo<RosterRow[]>(() => {
    const inviteRows: RosterRow[] = invites.map((invite) => ({
      kind: "invite",
      invite,
    }))
    const userRows: RosterRow[] = users.map((user) => ({
      kind: "user",
      user,
    }))
    return [...inviteRows, ...userRows]
  }, [invites, users])

  const loadPlayers = useCallback(async () => {
    try {
      const data = await apiFetch<{ players: Player[] }>("/players")
      setPlayers(data.players)
    } catch {
      setPlayers([])
    }
  }, [])

  function openInvite() {
    setInviteRole("viewer")
    setCreatedInviteUrl(null)
    setInviteDialogOpen(true)
  }

  function openEdit(user: AppUser) {
    setEditingUser(user)
    setForm({
      password: "",
      role: user.role as "admin" | "viewer",
      playerId: user.playerId ?? "",
    })
    setEditDialogOpen(true)
    void loadPlayers()
  }

  async function handleCreateInvite(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmittingInvite(true)

    try {
      const created = await apiFetch<Invite>("/invites", {
        method: "POST",
        body: JSON.stringify({ role: inviteRole }),
      })
      const url = `${window.location.origin}${created.invitePath}`
      setCreatedInviteUrl(url)
      setInvites((current) => [created, ...current])
      toast.success(t("inviteCreated"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmittingInvite(false)
    }
  }

  async function copyInviteUrl(url: string) {
    try {
      await navigator.clipboard.writeText(url)
      toast.success(t("linkCopied"))
    } catch {
      toast.error(tCommon("error"))
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!editingUser) {
      return
    }

    setIsSubmitting(true)

    try {
      const body: {
        role?: string
        password?: string
        playerId?: string | null
      } = {
        role: form.role,
        playerId: form.playerId || null,
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
      setEditDialogOpen(false)
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handlePromote() {
    if (!promoteUser) {
      return
    }

    try {
      const updated = await apiFetch<AppUser>(`/users/${promoteUser.id}`, {
        method: "PUT",
        body: JSON.stringify({ role: "admin" }),
      })
      setUsers((current) =>
        current.map((user) =>
          user.id === promoteUser.id ? { ...user, ...updated } : user
        )
      )
      toast.success(t("promoted"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setPromoteUser(null)
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

  async function handleRevokeInvite() {
    if (!revokeInvite) {
      return
    }

    try {
      await apiFetch(`/invites/${revokeInvite.id}`, { method: "DELETE" })
      setInvites((current) =>
        current.map((invite) =>
          invite.id === revokeInvite.id
            ? { ...invite, status: "revoked" }
            : invite
        )
      )
      toast.success(t("inviteRevoked"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setRevokeInvite(null)
    }
  }

  function stateLabel(status: string) {
    switch (status) {
      case "pending":
        return t("state.pending")
      case "active":
        return t("state.active")
      case "accepted":
        return t("state.accepted")
      case "expired":
        return t("state.expired")
      case "revoked":
        return t("state.revoked")
      default:
        return status
    }
  }

  function roleLabel(role: string) {
    return role === "admin" ? tAuth("roleAdmin") : tAuth("roleViewer")
  }

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
          <p className="text-muted-foreground">{t("description")}</p>
        </div>
        <Button onClick={openInvite}>
          <PlusIcon data-icon="inline-start" />
          {t("createInvite")}
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
                <TableHead>{t("columns.state")}</TableHead>
                <TableHead>{t("columns.player")}</TableHead>
                <TableHead>{t("columns.createdAt")}</TableHead>
                <TableHead className="text-right">{t("columns.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {roster.map((row) => {
                if (row.kind === "invite") {
                  const invite = row.invite
                  const inviteUrl = `${typeof window !== "undefined" ? window.location.origin : ""}${invite.invitePath}`
                  const isHistorical = invite.status !== "pending"
                  return (
                    <TableRow
                      key={`invite-${invite.id}`}
                      className={isHistorical ? "text-muted-foreground" : undefined}
                    >
                      <TableCell className="text-muted-foreground">
                        {invite.acceptedUsername ?? "—"}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{roleLabel(invite.role)}</Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary">
                          {stateLabel(invite.status)}
                        </Badge>
                      </TableCell>
                      <TableCell>—</TableCell>
                      <TableCell>{formatDateTime(invite.createdAt)}</TableCell>
                      <TableCell className="text-right">
                        {invite.status === "pending" ? (
                          <div className="flex justify-end gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => void copyInviteUrl(inviteUrl)}
                            >
                              <CopyIcon />
                              {t("copyLink")}
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setRevokeInvite(invite)}
                            >
                              {t("revokeInvite")}
                            </Button>
                          </div>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  )
                }

                const user = row.user
                return (
                  <TableRow key={`user-${user.id}`}>
                    <TableCell className="font-medium">{user.username}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{roleLabel(user.role)}</Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">
                        {stateLabel(user.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>{user.playerName ?? "—"}</TableCell>
                    <TableCell>{formatDateTime(user.createdAt)}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        {user.role === "viewer" ? (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setPromoteUser(user)}
                          >
                            <ShieldIcon />
                            {t("promote")}
                          </Button>
                        ) : null}
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
                )
              })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={inviteDialogOpen} onOpenChange={setInviteDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("createInviteTitle")}</DialogTitle>
            <DialogDescription>{t("createInviteDescription")}</DialogDescription>
          </DialogHeader>
          {createdInviteUrl ? (
            <div className="flex flex-col gap-4">
              <Input readOnly value={createdInviteUrl} />
              <Button onClick={() => void copyInviteUrl(createdInviteUrl)}>
                <CopyIcon data-icon="inline-start" />
                {t("copyLink")}
              </Button>
            </div>
          ) : (
            <form onSubmit={handleCreateInvite}>
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="invite-role">{t("fields.role")}</FieldLabel>
                  <Select
                    value={inviteRole}
                    onValueChange={(value) =>
                      setInviteRole(value as "admin" | "viewer")
                    }
                    items={[
                      { label: tAuth("roleAdmin"), value: "admin" },
                      { label: tAuth("roleViewer"), value: "viewer" },
                    ]}
                  >
                    <SelectTrigger id="invite-role" className="w-full">
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
                  onClick={() => setInviteDialogOpen(false)}
                >
                  {tCommon("cancel")}
                </Button>
                <Button type="submit" disabled={isSubmittingInvite}>
                  {t("createInvite")}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("editTitle")}</DialogTitle>
            <DialogDescription>{t("editDescription")}</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="user-password">
                  {t("newPasswordOptional")}
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
              <Field>
                <FieldLabel htmlFor="user-player">{t("fields.player")}</FieldLabel>
                <Select
                  value={form.playerId || "__none__"}
                  onValueChange={(value) =>
                    setForm((current) => ({
                      ...current,
                      playerId: !value || value === "__none__" ? "" : value,
                    }))
                  }
                  items={[
                    { label: t("fields.noPlayer"), value: "__none__" },
                    ...players.map((player) => ({
                      label: player.name,
                      value: player.id,
                    })),
                  ]}
                >
                  <SelectTrigger id="user-player" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="__none__">
                        {t("fields.noPlayer")}
                      </SelectItem>
                      {players.map((player) => (
                        <SelectItem key={player.id} value={player.id}>
                          {player.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            </FieldGroup>
            <DialogFooter className="mt-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditDialogOpen(false)}
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
            <AlertDialogAction onClick={() => void handleDelete()}>
              {t("deleteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={promoteUser != null}
        onOpenChange={(open) => !open && setPromoteUser(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("promoteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("promoteDescription", {
                username: promoteUser?.username ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handlePromote()}>
              {t("promoteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={revokeInvite != null}
        onOpenChange={(open) => !open && setRevokeInvite(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("revokeInviteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("revokeInviteDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleRevokeInvite()}>
              {t("revokeInviteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
