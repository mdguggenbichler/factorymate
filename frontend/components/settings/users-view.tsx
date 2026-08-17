"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import {
  CheckIcon,
  CopyIcon,
  PencilIcon,
  PlusIcon,
  ShieldIcon,
  Trash2Icon,
  XIcon,
} from "lucide-react"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
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
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { apiFetch } from "@/lib/api"
import { discordDefaultAvatarUrl } from "@/lib/discord-avatar"
import type {
  AppUser,
  Invite,
  PendingRegistration,
  Player,
  UnmappedPlayer,
} from "@/lib/api-types"
import { useFormatDateTime } from "@/hooks/use-format-datetime"

type UserFormState = {
  password: string
  role: "admin" | "viewer"
  playerId: string
  externalPlatform: string
  externalUserId: string
  externalUsername: string
  externalDisplayName: string
}

const emptyForm: UserFormState = {
  password: "",
  role: "viewer",
  playerId: "",
  externalPlatform: "",
  externalUserId: "",
  externalUsername: "",
  externalDisplayName: "",
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
  initialPending: PendingRegistration[]
  initialUnmapped: UnmappedPlayer[]
}

export function UsersView({
  initialUsers,
  initialInvites,
  initialPending,
  initialUnmapped,
}: UsersViewProps) {
  const t = useTranslations("settings.users")
  const tAuth = useTranslations("auth")
  const tCommon = useTranslations("common")
  const { formatDateTime } = useFormatDateTime()
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
  const [pending, setPending] = useState(initialPending)
  const [unmapped, setUnmapped] = useState(initialUnmapped)
  const [linkSelections, setLinkSelections] = useState<Record<string, string>>({})
  const [linkingPlayerId, setLinkingPlayerId] = useState<string | null>(null)
  const [rejectRegistration, setRejectRegistration] =
    useState<PendingRegistration | null>(null)
  const [rejectComment, setRejectComment] = useState("")
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [unlinkUser, setUnlinkUser] = useState<AppUser | null>(null)

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
      toast.error(t("playersLoadFailed"))
    }
  }, [t])

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
      externalPlatform: user.externalPlatform ?? "",
      externalUserId: user.externalUserId ?? "",
      externalUsername: user.externalUsername ?? "",
      externalDisplayName: user.externalDisplayName ?? "",
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
      let updated = await apiFetch<AppUser>(`/users/${editingUser.id}`, {
        method: "PUT",
        body: JSON.stringify(body),
      })

      const externalChanged =
        form.externalPlatform !== (editingUser.externalPlatform ?? "") ||
        form.externalUserId !== (editingUser.externalUserId ?? "") ||
        form.externalUsername !== (editingUser.externalUsername ?? "") ||
        form.externalDisplayName !== (editingUser.externalDisplayName ?? "")

      if (externalChanged) {
        updated = await apiFetch<AppUser>(`/users/${editingUser.id}/external`, {
          method: "PUT",
          body: JSON.stringify({
            externalPlatform: form.externalPlatform.trim() || null,
            externalUserId: form.externalUserId.trim() || null,
            externalUsername: form.externalUsername.trim() || null,
            externalDisplayName: form.externalDisplayName.trim() || null,
          }),
        })
        toast.success(t("externalUpdated"))
      }
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

  async function handleApproveRegistration(registration: PendingRegistration) {
    try {
      const approved = await apiFetch<AppUser>(
        `/registrations/${registration.id}/approve`,
        { method: "POST" }
      )
      setPending((current) =>
        current.filter((item) => item.id !== registration.id)
      )
      setUsers((current) => [approved, ...current])
      toast.success(t("registrationApproved"))
    } catch {
      toast.error(tCommon("error"))
    }
  }

  async function handleRejectRegistration() {
    if (!rejectRegistration) {
      return
    }

    try {
      await apiFetch(`/registrations/${rejectRegistration.id}/reject`, {
        method: "POST",
        body: JSON.stringify({
          comment: rejectComment.trim() || undefined,
        }),
      })
      setPending((current) =>
        current.filter((item) => item.id !== rejectRegistration.id)
      )
      toast.success(t("registrationRejected"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setRejectRegistration(null)
      setRejectComment("")
    }
  }

  const linkableUsers = useMemo(
    () => users.filter((user) => !user.playerId && user.status === "active"),
    [users]
  )

  async function handleLinkUnmappedPlayer(player: UnmappedPlayer) {
    const selectedUserId = linkSelections[player.playerId]
    if (!selectedUserId) {
      toast.error(t("linkUserRequired"))
      return
    }

    setLinkingPlayerId(player.playerId)
    try {
      const updated = await apiFetch<AppUser>(`/users/${selectedUserId}`, {
        method: "PUT",
        body: JSON.stringify({ playerId: player.playerId }),
      })
      setUsers((current) =>
        current.map((user) =>
          user.id === updated.id ? { ...user, ...updated } : user
        )
      )
      setUnmapped((current) =>
        current.filter((item) => item.playerId !== player.playerId)
      )
      toast.success(t("playerLinked", { name: player.name }))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setLinkingPlayerId(null)
    }
  }

  async function handleUnlinkDiscord() {
    if (!unlinkUser) {
      return
    }

    try {
      const updated = await apiFetch<AppUser>(`/users/${unlinkUser.id}/external`, {
        method: "PUT",
        body: JSON.stringify({ unlink: true }),
      })
      setUsers((current) =>
        current.map((user) =>
          user.id === unlinkUser.id ? { ...user, ...updated } : user
        )
      )
      if (editingUser?.id === unlinkUser.id) {
        setForm((current) => ({
          ...current,
          externalPlatform: "",
          externalUserId: "",
          externalUsername: "",
          externalDisplayName: "",
        }))
      }
      toast.success(t("discordUnlinked"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setUnlinkUser(null)
    }
  }

  function renderDiscordIdentity(user: AppUser) {
    if (!user.externalUserId) {
      return "—"
    }
    const label =
      user.externalDisplayName ||
      user.externalUsername ||
      user.externalUserId
    const handle = user.externalUsername ? `@${user.externalUsername}` : null

    return (
      <div className="flex items-center gap-2">
        <Avatar size="sm">
          <AvatarImage
            src={discordDefaultAvatarUrl(user.externalUserId)}
            alt=""
          />
          <AvatarFallback>{label.charAt(0).toUpperCase()}</AvatarFallback>
        </Avatar>
        <div className="min-w-0">
          <p className="truncate font-medium">{label}</p>
          {handle ? (
            <p className="truncate text-xs text-muted-foreground">{handle}</p>
          ) : null}
        </div>
      </div>
    )
  }

  function stateLabel(status: string) {
    switch (status) {
      case "pending":
        return t("state.pending")
      case "pending_approval":
        return t("state.pendingApproval")
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

  const playerSelectItems = useMemo(() => {
    const items = [
      { label: t("fields.noPlayer"), value: "__none__" },
      ...players.map((player) => ({
        label: player.name,
        value: player.id,
      })),
    ]
    if (
      form.playerId &&
      !players.some((player) => player.id === form.playerId)
    ) {
      items.push({
        label: editingUser?.playerName ?? form.playerId,
        value: form.playerId,
      })
    }
    return items
  }, [editingUser, form.playerId, players, t])

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
          <p className="text-muted-foreground">{t("description")}</p>
        </div>
      </div>

      <Alert>
        <AlertTitle>{t("discordFirstTitle")}</AlertTitle>
        <AlertDescription>{t("discordFirstDescription")}</AlertDescription>
      </Alert>

      {pending.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("pendingApprovalsTitle")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.username")}</TableHead>
                  <TableHead>{t("columns.discord")}</TableHead>
                  <TableHead>{t("columns.pendingPlayer")}</TableHead>
                  <TableHead>{t("columns.createdAt")}</TableHead>
                  <TableHead className="text-right">{t("columns.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pending.map((registration) => (
                  <TableRow key={`pending-${registration.id}`}>
                    <TableCell className="font-medium">
                      {registration.username}
                    </TableCell>
                    <TableCell>
                      {registration.externalDisplayName ||
                        registration.externalUsername ||
                        "—"}
                    </TableCell>
                    <TableCell>{registration.pendingPlayerName || "—"}</TableCell>
                    <TableCell>{formatDateTime(registration.createdAt)}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => void handleApproveRegistration(registration)}
                        >
                          <CheckIcon />
                          {t("approveRegistration")}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setRejectRegistration(registration)}
                        >
                          <XIcon />
                          {t("rejectRegistration")}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      {unmapped.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("unmappedPlayersTitle")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.player")}</TableHead>
                  <TableHead>{t("columns.status")}</TableHead>
                  <TableHead>{t("columns.lastSeen")}</TableHead>
                  <TableHead className="text-right">{t("columns.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {unmapped.map((player) => (
                  <TableRow key={player.playerId}>
                    <TableCell className="font-medium">{player.name}</TableCell>
                    <TableCell>
                      <Badge variant={player.online ? "default" : "secondary"}>
                        {player.online ? t("statusOnline") : t("statusOffline")}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {player.lastSeenAt ? formatDateTime(player.lastSeenAt) : "—"}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex flex-wrap items-center justify-end gap-2">
                        <Select
                          value={linkSelections[player.playerId] ?? ""}
                          onValueChange={(value) =>
                            setLinkSelections((current) => ({
                              ...current,
                              [player.playerId]: value ?? "",
                            }))
                          }
                          items={linkableUsers.map((user) => ({
                            label: user.username,
                            value: String(user.id),
                          }))}
                        >
                          <SelectTrigger className="w-[180px]">
                            <SelectValue placeholder={t("linkUserPlaceholder")} />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              {linkableUsers.map((user) => (
                                <SelectItem key={user.id} value={String(user.id)}>
                                  {user.username}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={linkingPlayerId === player.playerId}
                          onClick={() => void handleLinkUnmappedPlayer(player)}
                        >
                          {t("linkToUser")}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <CardTitle>{t("breakGlassTitle")}</CardTitle>
              <p className="text-sm text-muted-foreground">
                {t("breakGlassDescription")}
              </p>
            </div>
            <CollapsibleTrigger render={<Button variant="outline" />}>
              {advancedOpen ? t("hideBreakGlass") : t("showBreakGlass")}
            </CollapsibleTrigger>
          </CardHeader>
          <CollapsibleContent>
            <CardContent>
              <Button onClick={openInvite}>
                <PlusIcon data-icon="inline-start" />
                {t("createInvite")}
              </Button>
            </CardContent>
          </CollapsibleContent>
        </Card>
      </Collapsible>

      <Card>
        <CardHeader>
          <CardTitle>{t("tableTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("columns.username")}</TableHead>
                <TableHead>{t("columns.discord")}</TableHead>
                <TableHead>{t("columns.registrationSource")}</TableHead>
                <TableHead>{t("columns.linkedAt")}</TableHead>
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
                      <TableCell>—</TableCell>
                      <TableCell>—</TableCell>
                      <TableCell>—</TableCell>
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
                    <TableCell>{renderDiscordIdentity(user)}</TableCell>
                    <TableCell>{user.registrationSource ?? "—"}</TableCell>
                    <TableCell>
                      {user.externalLinkedAt
                        ? formatDateTime(user.externalLinkedAt)
                        : "—"}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{roleLabel(user.role)}</Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant="secondary">
                          {stateLabel(user.status)}
                        </Badge>
                        {user.pendingPlayerName && !user.playerId ? (
                          <Badge variant="outline">
                            {t("pendingPlayerBadge", {
                              name: user.pendingPlayerName,
                            })}
                          </Badge>
                        ) : null}
                      </div>
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
                  items={playerSelectItems}
                >
                  <SelectTrigger id="user-player" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {playerSelectItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <div className="mt-4 space-y-3 rounded-lg border p-4">
                <div>
                  <p className="text-sm font-medium">{t("externalTitle")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("externalDescription")}
                  </p>
                </div>
                <Field>
                  <FieldLabel htmlFor="external-platform">
                    {t("fields.externalPlatform")}
                  </FieldLabel>
                  <Input
                    id="external-platform"
                    value={form.externalPlatform}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        externalPlatform: event.target.value,
                      }))
                    }
                    placeholder="discord"
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="external-user-id">
                    {t("fields.externalUserId")}
                  </FieldLabel>
                  <Input
                    id="external-user-id"
                    value={form.externalUserId}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        externalUserId: event.target.value,
                      }))
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="external-username">
                    {t("fields.externalUsername")}
                  </FieldLabel>
                  <Input
                    id="external-username"
                    value={form.externalUsername}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        externalUsername: event.target.value,
                      }))
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="external-display-name">
                    {t("fields.externalDisplayName")}
                  </FieldLabel>
                  <Input
                    id="external-display-name"
                    value={form.externalDisplayName}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        externalDisplayName: event.target.value,
                      }))
                    }
                  />
                </Field>
                {editingUser?.externalUserId ? (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setUnlinkUser(editingUser)}
                  >
                    {t("unlinkDiscord")}
                  </Button>
                ) : null}
              </div>
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
        open={unlinkUser != null}
        onOpenChange={(open) => !open && setUnlinkUser(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("unlinkDiscordTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("unlinkDiscordDescription", {
                username: unlinkUser?.username ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleUnlinkDiscord()}>
              {t("unlinkDiscordConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

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

      <Dialog
        open={rejectRegistration != null}
        onOpenChange={(open) => {
          if (!open) {
            setRejectRegistration(null)
            setRejectComment("")
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("rejectRegistrationTitle")}</DialogTitle>
            <DialogDescription>
              {t("rejectRegistrationDescription", {
                username: rejectRegistration?.username ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="reject-comment">
                {t("rejectCommentLabel")}
              </FieldLabel>
              <Textarea
                id="reject-comment"
                value={rejectComment}
                onChange={(event) => setRejectComment(event.target.value)}
                placeholder={t("rejectCommentPlaceholder")}
                rows={3}
              />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setRejectRegistration(null)
                setRejectComment("")
              }}
            >
              {tCommon("cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => void handleRejectRegistration()}
            >
              {t("rejectRegistrationConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={revokeInvite != null}
        onOpenChange={(open) => !open && setRevokeInvite(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("revokeInviteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {revokeInvite
                ? t("revokeInviteDescription", {
                    role: roleLabel(revokeInvite.role),
                    createdAt: formatDateTime(revokeInvite.createdAt),
                  })
                : t("revokeInviteDescriptionFallback")}
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
