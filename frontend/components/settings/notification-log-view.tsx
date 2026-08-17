"use client"

import { useTranslations } from "next-intl"

import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatDateTime } from "@/lib/format"
import type { NotificationLogEntry } from "@/lib/api-types"

type NotificationLogViewProps = {
  entries: NotificationLogEntry[]
  total: number
}

export function NotificationLogView({
  entries,
  total,
}: NotificationLogViewProps) {
  const t = useTranslations("settings.log")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("tableTitle", { total })}</CardTitle>
        </CardHeader>
        <CardContent>
          {entries.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.sentAt")}</TableHead>
                  <TableHead>{t("columns.messageType")}</TableHead>
                  <TableHead>{t("columns.delivery")}</TableHead>
                  <TableHead>{t("columns.target")}</TableHead>
                  <TableHead>{t("columns.preview")}</TableHead>
                  <TableHead>{t("columns.status")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>{formatDateTime(entry.sentAt)}</TableCell>
                    <TableCell className="font-mono text-xs">
                      {entry.messageTypeKey}
                    </TableCell>
                    <TableCell>
                      {entry.deliveryMode === "dm"
                        ? t("deliveryDM")
                        : t("deliveryChannel")}
                    </TableCell>
                    <TableCell>
                      {entry.deliveryMode === "dm"
                        ? entry.recipientExternalUserId
                          ? t("dmRecipient", {
                              id: entry.recipientExternalUserId,
                            })
                          : "—"
                        : entry.targetName ??
                          (entry.targetId != null
                            ? t("deletedTarget")
                            : "—")}
                    </TableCell>
                    <TableCell className="max-w-xs truncate text-sm">
                      {entry.renderedPreview}
                    </TableCell>
                    <TableCell>
                      {entry.success ? (
                        <Badge>{t("statusSuccess")}</Badge>
                      ) : (
                        <Badge variant="destructive">
                          {entry.error ?? t("statusFailed")}
                        </Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
