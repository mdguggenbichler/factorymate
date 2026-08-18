"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { CopyIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
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
import type { DiscordSettings, RoleMappingEntry, RoleMappingsConfig } from "@/lib/api-types"

const COMMAND_GROUPS = [
  "admin",
  "register",
  "player",
  "connection",
  "mods",
] as const

function defaultRoleMappings(guildId: string): RoleMappingsConfig {
  return {
    guild_id: guildId,
    role_mappings: [],
    default_fm_role: "viewer",
    default_bot_commands: [],
    allow_self_register: true,
    admin_discord_role_ids: [],
  }
}

function parseRoleMappings(
  raw: RoleMappingsConfig | Record<string, unknown>,
  guildId: string
): RoleMappingsConfig {
  const base = defaultRoleMappings(guildId)
  if (!raw || typeof raw !== "object") {
    return base
  }
  const config = raw as RoleMappingsConfig
  return {
    guild_id: config.guild_id || guildId,
    role_mappings: Array.isArray(config.role_mappings) ? config.role_mappings : [],
    default_fm_role: config.default_fm_role === "admin" ? "admin" : "viewer",
    default_bot_commands: Array.isArray(config.default_bot_commands)
      ? config.default_bot_commands
      : [],
    allow_self_register: config.allow_self_register !== false,
    admin_discord_role_ids: Array.isArray(config.admin_discord_role_ids)
      ? config.admin_discord_role_ids
      : [],
  }
}

type DiscordSettingsFormProps = {
  initialSettings: DiscordSettings
  initialInviteUrl: string | null
}

export function DiscordSettingsForm({
  initialSettings,
  initialInviteUrl,
}: DiscordSettingsFormProps) {
  const t = useTranslations("settings.discord")
  const tAuth = useTranslations("auth")
  const tCommon = useTranslations("common")
  const [settings, setSettings] = useState(initialSettings)
  const [inviteUrl, setInviteUrl] = useState(initialInviteUrl)
  const [guildId, setGuildId] = useState(initialSettings.guildId)
  const [autoApprove, setAutoApprove] = useState(initialSettings.autoApprove)
  const [roleMappings, setRoleMappings] = useState<RoleMappingsConfig>(() =>
    parseRoleMappings(initialSettings.roleMappings, initialSettings.guildId)
  )
  const [adminRoleIds, setAdminRoleIds] = useState(
    () => roleMappings.admin_discord_role_ids.join(", ")
  )
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isLoadingInvite, setIsLoadingInvite] = useState(false)

  const botStatusLabel = useMemo(() => {
    if (!settings.tokenConfigured) {
      return t("status.tokenMissing")
    }
    if (!settings.botEnabled) {
      return t("status.disabled")
    }
    if (settings.botConnected) {
      return t("status.connected")
    }
    return t("status.disconnected")
  }, [settings, t])

  const updateMapping = useCallback(
    (index: number, patch: Partial<RoleMappingEntry>) => {
      setRoleMappings((current) => ({
        ...current,
        role_mappings: current.role_mappings.map((entry, i) =>
          i === index ? { ...entry, ...patch } : entry
        ),
      }))
    },
    []
  )

  const toggleMappingCommand = useCallback(
    (index: number, command: string, checked: boolean) => {
      setRoleMappings((current) => ({
        ...current,
        role_mappings: current.role_mappings.map((entry, i) => {
          if (i !== index) {
            return entry
          }
          const commands = new Set(entry.bot_commands)
          if (checked) {
            commands.add(command)
          } else {
            commands.delete(command)
          }
          return { ...entry, bot_commands: Array.from(commands) }
        }),
      }))
    },
    []
  )

  async function loadInviteUrl() {
    setIsLoadingInvite(true)
    try {
      const data = await apiFetch<{ inviteUrl: string }>("/discord/invite-url")
      setInviteUrl(data.inviteUrl)
    } catch {
      toast.error(t("inviteLoadFailed"))
    } finally {
      setIsLoadingInvite(false)
    }
  }

  async function copyInviteUrl() {
    if (!inviteUrl) {
      return
    }
    try {
      await navigator.clipboard.writeText(inviteUrl)
      toast.success(t("inviteCopied"))
    } catch {
      toast.error(tCommon("error"))
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)

    const adminIds = adminRoleIds
      .split(",")
      .map((id) => id.trim())
      .filter(Boolean)

    const payload = {
      guildId,
      autoApprove,
      roleMappings: {
        ...roleMappings,
        guild_id: guildId,
        admin_discord_role_ids: adminIds,
      },
    }

    try {
      const updated = await apiFetch<DiscordSettings>("/discord/settings", {
        method: "PUT",
        body: JSON.stringify(payload),
      })
      setSettings(updated)
      setRoleMappings(parseRoleMappings(updated.roleMappings, updated.guildId))
      toast.success(t("saved"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      {!settings.tokenConfigured ? (
        <Alert>
          <AlertTitle>{t("tokenWarningTitle")}</AlertTitle>
          <AlertDescription>{t("tokenWarningDescription")}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("statusTitle")}</CardTitle>
          <CardDescription>{t("statusDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <Badge
              variant={
                settings.botConnected && settings.tokenConfigured
                  ? "default"
                  : "secondary"
              }
            >
              {botStatusLabel}
            </Badge>
            {settings.botEnabled ? (
              <Badge variant="outline">{t("status.enabled")}</Badge>
            ) : (
              <Badge variant="outline">{t("status.botDisabled")}</Badge>
            )}
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
            <Field className="flex-1">
              <FieldLabel>{t("inviteUrl")}</FieldLabel>
              <Input readOnly value={inviteUrl ?? ""} placeholder={t("inviteUnavailable")} />
            </Field>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => void loadInviteUrl()}
                disabled={isLoadingInvite}
              >
                {t("loadInvite")}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => void copyInviteUrl()}
                disabled={!inviteUrl}
              >
                <CopyIcon />
                {t("copyInvite")}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        <Card>
          <CardHeader>
            <CardTitle>{t("configTitle")}</CardTitle>
            <CardDescription>{t("configDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="guild-id">{t("fields.guildId")}</FieldLabel>
                <Input
                  id="guild-id"
                  value={guildId}
                  onChange={(event) => setGuildId(event.target.value)}
                  placeholder={t("fields.guildIdPlaceholder")}
                />
              </Field>
              <Field className="flex items-center gap-3">
                <Switch
                  id="auto-approve"
                  checked={autoApprove}
                  onCheckedChange={setAutoApprove}
                />
                <FieldLabel htmlFor="auto-approve" className="mb-0">
                  {t("fields.autoApprove")}
                </FieldLabel>
              </Field>
              <Field className="flex items-center gap-3">
                <Switch
                  id="allow-self-register"
                  checked={roleMappings.allow_self_register}
                  onCheckedChange={(checked) =>
                    setRoleMappings((current) => ({
                      ...current,
                      allow_self_register: checked,
                    }))
                  }
                />
                <FieldLabel htmlFor="allow-self-register" className="mb-0">
                  {t("fields.allowSelfRegister")}
                </FieldLabel>
              </Field>
              <Field>
                <FieldLabel htmlFor="default-fm-role">
                  {t("fields.defaultFmRole")}
                </FieldLabel>
                <Select
                  value={roleMappings.default_fm_role}
                  onValueChange={(value) =>
                    setRoleMappings((current) => ({
                      ...current,
                      default_fm_role: value as "admin" | "viewer",
                    }))
                  }
                  items={[
                    { label: tAuth("roleAdmin"), value: "admin" },
                    { label: tAuth("roleViewer"), value: "viewer" },
                  ]}
                >
                  <SelectTrigger id="default-fm-role" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="admin">{tAuth("roleAdmin")}</SelectItem>
                      <SelectItem value="viewer">{tAuth("roleViewer")}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="admin-role-ids">
                  {t("fields.adminDiscordRoleIds")}
                </FieldLabel>
                <Input
                  id="admin-role-ids"
                  value={adminRoleIds}
                  onChange={(event) => setAdminRoleIds(event.target.value)}
                  placeholder={t("fields.adminDiscordRoleIdsPlaceholder")}
                />
              </Field>
            </FieldGroup>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <CardTitle>{t("roleMappingsTitle")}</CardTitle>
              <CardDescription>{t("roleMappingsDescription")}</CardDescription>
            </div>
            <Button
              type="button"
              variant="outline"
              onClick={() =>
                setRoleMappings((current) => ({
                  ...current,
                  role_mappings: [
                    ...current.role_mappings,
                    {
                      discord_role_id: "",
                      fm_role: "viewer",
                      bot_commands: ["register", "player", "connection", "mods"],
                    },
                  ],
                }))
              }
            >
              <PlusIcon data-icon="inline-start" />
              {t("addRoleMapping")}
            </Button>
          </CardHeader>
          <CardContent>
            {roleMappings.role_mappings.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("roleMappingsEmpty")}</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("columns.discordRoleId")}</TableHead>
                    <TableHead>{t("columns.fmRole")}</TableHead>
                    <TableHead>{t("columns.commands")}</TableHead>
                    <TableHead className="text-right">{t("columns.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {roleMappings.role_mappings.map((entry, index) => (
                    <TableRow key={`mapping-${index}`}>
                      <TableCell>
                        <div className="space-y-1">
                          <Input
                            value={entry.discord_role_id}
                            onChange={(event) =>
                              updateMapping(index, {
                                discord_role_id: event.target.value,
                              })
                            }
                            placeholder={t("fields.discordRoleIdPlaceholder")}
                          />
                          <p className="text-xs text-muted-foreground">
                            {t("fields.discordRoleIdHint")}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Select
                          value={entry.fm_role}
                          onValueChange={(value) =>
                            updateMapping(index, {
                              fm_role: value as "admin" | "viewer",
                            })
                          }
                          items={[
                            { label: tAuth("roleAdmin"), value: "admin" },
                            { label: tAuth("roleViewer"), value: "viewer" },
                          ]}
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              <SelectItem value="admin">{tAuth("roleAdmin")}</SelectItem>
                              <SelectItem value="viewer">{tAuth("roleViewer")}</SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-3">
                          {COMMAND_GROUPS.map((command) => (
                            <label
                              key={command}
                              className="flex items-center gap-2 text-sm"
                            >
                              <Checkbox
                                checked={entry.bot_commands.includes(command)}
                                onCheckedChange={(checked) =>
                                  toggleMappingCommand(index, command, checked === true)
                                }
                              />
                              {t(`commandGroups.${command}`)}
                            </label>
                          ))}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() =>
                            setRoleMappings((current) => ({
                              ...current,
                              role_mappings: current.role_mappings.filter(
                                (_, i) => i !== index
                              ),
                            }))
                          }
                        >
                          <Trash2Icon />
                          {t("removeRoleMapping")}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <div className="flex justify-end">
          <Button type="submit" disabled={isSubmitting}>
            {tCommon("save")}
          </Button>
        </div>
      </form>
    </div>
  )
}
