import { getTranslations } from "next-intl/server"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatDateTime } from "@/lib/format"
import type { Player, PlayerHistoryEvent } from "@/lib/api-types"

type PlayersContentProps = {
  players: Player[]
  history: PlayerHistoryEvent[]
}

function playerInitials(name: string): string {
  return name
    .split(/\s+/)
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase()
}

export async function PlayersContent({ players, history }: PlayersContentProps) {
  const t = await getTranslations("players")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("rosterTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {players.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.player")}</TableHead>
                  <TableHead>{t("columns.status")}</TableHead>
                  <TableHead>{t("columns.lastSeen")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {players.map((player) => (
                  <TableRow key={player.id}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Avatar className="size-8">
                          <AvatarFallback>{playerInitials(player.name)}</AvatarFallback>
                        </Avatar>
                        <span className="font-medium">{player.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={player.online ? "default" : "secondary"}>
                        {player.online ? t("status.online") : t("status.offline")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDateTime(player.lastSeenAt)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("historyTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {history.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("historyEmpty")}</p>
          ) : (
            <div className="space-y-0">
              {history.map((event, index) => (
                <div key={event.id}>
                  <div className="flex flex-col gap-1 py-4 sm:flex-row sm:items-center sm:justify-between">
                    <div className="space-y-1">
                      <p className="font-medium">
                        {event.eventType === "joined"
                          ? t("event.joined", { name: event.playerName })
                          : t("event.left", { name: event.playerName })}
                      </p>
                      <p className="text-sm text-muted-foreground">
                        {t("event.onlineCount", { count: event.onlineCount })}
                      </p>
                    </div>
                    <time
                      className="text-sm text-muted-foreground"
                      dateTime={event.occurredAt}
                    >
                      {formatDateTime(event.occurredAt)}
                    </time>
                  </div>
                  {index < history.length - 1 ? <Separator /> : null}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
