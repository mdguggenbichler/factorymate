"use client"

import { useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { DownloadIcon, ExternalLinkIcon, RefreshCwIcon } from "lucide-react"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { apiFetch, apiUrl } from "@/lib/api"
import type { ModsResponse } from "@/lib/api-types"
import { useFormatDateTime } from "@/hooks/use-format-datetime"

type ModsViewProps = {
  initialData: ModsResponse
  isAdmin: boolean
}

type SortKey = "name" | "version"

export function ModsView({ initialData, isAdmin }: ModsViewProps) {
  const t = useTranslations("mods")
  const { formatDateTime } = useFormatDateTime()
  const tCommon = useTranslations("common")
  const [data, setData] = useState(initialData)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [isDownloading, setIsDownloading] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")
  const [sortBy, setSortBy] = useState<SortKey>("name")

  const filteredMods = useMemo(() => {
    const query = searchQuery.trim().toLowerCase()
    let mods = data.mods
    if (query) {
      mods = mods.filter((mod) => mod.name.toLowerCase().includes(query))
    }
    return [...mods].sort((a, b) => {
      if (sortBy === "version") {
        return a.version.localeCompare(b.version, undefined, { numeric: true })
      }
      return a.name.localeCompare(b.name)
    })
  }, [data.mods, searchQuery, sortBy])

  async function handleRefresh() {
    setIsRefreshing(true)
    try {
      const refreshed = await apiFetch<ModsResponse>("/mods/refresh", {
        method: "POST",
      })
      setData(refreshed)
      toast.success(t("refreshed"))
    } catch {
      toast.error(tCommon("error"))
    } finally {
      setIsRefreshing(false)
    }
  }

  async function handleDownload() {
    setIsDownloading(true)
    try {
      const response = await fetch(apiUrl("/mods/smmprofile"), {
        credentials: "include",
      })
      if (!response.ok) {
        throw new Error("download failed")
      }
      const blob = await response.blob()
      const disposition = response.headers.get("Content-Disposition") ?? ""
      const match = disposition.match(/filename="([^"]+)"/)
      const filename = match?.[1] ?? "factorymate-server.smmprofile"
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = url
      anchor.download = filename
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success(t("downloadStarted"))
    } catch {
      toast.error(t("downloadFailed"))
    } finally {
      setIsDownloading(false)
    }
  }

  return (
    <TooltipProvider>
      <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
          <p className="text-muted-foreground">{t("description")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => void handleDownload()}
            disabled={isDownloading || !data.frmReachable}
          >
            <DownloadIcon data-icon="inline-start" />
            {t("downloadProfile")}
          </Button>
          {isAdmin ? (
            <Button
              variant="outline"
              onClick={() => void handleRefresh()}
              disabled={isRefreshing}
            >
              <RefreshCwIcon data-icon="inline-start" />
              {t("refresh")}
            </Button>
          ) : null}
        </div>
      </div>

      <Alert>
        <AlertTitle>{t("disclaimerTitle")}</AlertTitle>
        <AlertDescription>{t("disclaimerDescription")}</AlertDescription>
      </Alert>

      {!data.frmReachable ? (
        <Alert variant="destructive">
          <AlertTitle>{t("frmUnreachableTitle")}</AlertTitle>
          <AlertDescription>{t("frmUnreachableDescription")}</AlertDescription>
        </Alert>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>{t("gameBuild")}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{data.gameBuild || "—"}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("smlVersion")}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{data.smlVersion || "—"}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("cachedAt")}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              {data.cachedAt ? formatDateTime(data.cachedAt) : "—"}
            </p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <CardTitle>{t("tableTitle")}</CardTitle>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder={t("searchPlaceholder")}
              className="max-w-sm"
            />
            <Select
              value={sortBy}
              onValueChange={(value) => setSortBy((value as SortKey) ?? "name")}
              items={[
                { label: t("sortName"), value: "name" },
                { label: t("sortVersion"), value: "version" },
              ]}
            >
              <SelectTrigger className="w-[140px]" aria-label={t("sortLabel")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="name">{t("sortName")}</SelectItem>
                  <SelectItem value="version">{t("sortVersion")}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {data.mods.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : filteredMods.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("searchEmpty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.name")}</TableHead>
                  <TableHead>{t("columns.version")}</TableHead>
                  <TableHead>{t("columns.remoteVersionRange")}</TableHead>
                  <TableHead>{t("columns.createdBy")}</TableHead>
                  <TableHead>{t("columns.description")}</TableHead>
                  <TableHead>{t("columns.docs")}</TableHead>
                  <TableHead>{t("columns.support")}</TableHead>
                  <TableHead>{t("columns.smrName")}</TableHead>
                  <TableHead>{t("columns.required")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMods.map((mod) => (
                  <TableRow key={`${mod.smrName}-${mod.version}`}>
                    <TableCell className="font-medium">{mod.name}</TableCell>
                    <TableCell>{mod.version}</TableCell>
                    <TableCell>{mod.remoteVersionRange || "—"}</TableCell>
                    <TableCell>{mod.createdBy || "—"}</TableCell>
                    <TableCell className="max-w-xs">
                      {mod.description?.trim() ? (
                        <Tooltip>
                          <TooltipTrigger
                            className="block max-w-xs truncate text-left text-muted-foreground"
                          >
                            {mod.description.trim()}
                          </TooltipTrigger>
                          <TooltipContent className="max-w-sm">
                            {mod.description.trim()}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {mod.docsUrl ? (
                        <a
                          href={mod.docsUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-sm text-primary underline-offset-4 hover:underline"
                        >
                          <ExternalLinkIcon className="size-3.5" />
                          {t("docsLink")}
                        </a>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {mod.supportUrl ? (
                        <a
                          href={mod.supportUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-sm text-primary underline-offset-4 hover:underline"
                        >
                          <ExternalLinkIcon className="size-3.5" />
                          {t("supportLink")}
                        </a>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {mod.smrName}
                    </TableCell>
                    <TableCell>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Badge variant={mod.requiredOnRemote ? "default" : "secondary"}>
                              {mod.requiredOnRemote ? t("requiredYes") : t("requiredNo")}
                            </Badge>
                          }
                        />
                        <TooltipContent className="max-w-xs">
                          {t("requiredTooltip")}
                        </TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
    </TooltipProvider>
  )
}
