import { getTranslations } from "next-intl/server"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatNumber } from "@/lib/format"
import type { Drone } from "@/lib/api-types"

type DronesTableProps = {
  drones: Drone[]
}

export async function DronesTable({ drones }: DronesTableProps) {
  const t = await getTranslations("drones")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("tableTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {drones.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.id")}</TableHead>
                  <TableHead>{t("columns.home")}</TableHead>
                  <TableHead>{t("columns.paired")}</TableHead>
                  <TableHead>{t("columns.destination")}</TableHead>
                  <TableHead>{t("columns.mode")}</TableHead>
                  <TableHead>{t("columns.speed")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {drones.map((drone) => (
                  <TableRow key={drone.droneId}>
                    <TableCell className="font-medium">{drone.droneId}</TableCell>
                    <TableCell>{drone.homeStation ?? "—"}</TableCell>
                    <TableCell>
                      {drone.hasPairedStation
                        ? (drone.pairedStation ?? "—")
                        : t("notPaired")}
                    </TableCell>
                    <TableCell>{drone.currentDestination ?? "—"}</TableCell>
                    <TableCell>
                      {drone.currentFlyingMode ? (
                        <Badge variant="outline">{drone.currentFlyingMode}</Badge>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {t("speedValue", {
                        current: formatNumber(drone.flyingSpeed),
                        max: formatNumber(drone.maxSpeed),
                      })}
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
