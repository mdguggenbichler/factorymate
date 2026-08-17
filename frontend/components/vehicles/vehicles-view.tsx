"use client"

import { useTranslations } from "next-intl"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatNumber } from "@/lib/format"
import type { Train, WheeledVehicle } from "@/lib/api-types"

type VehiclesViewProps = {
  trains: Train[]
  vehicles: WheeledVehicle[]
}

export function VehiclesView({ trains, vehicles }: VehiclesViewProps) {
  const t = useTranslations("vehicles")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Tabs defaultValue="trains">
        <TabsList>
          <TabsTrigger value="trains">{t("tabs.trains")}</TabsTrigger>
          <TabsTrigger value="wheeled">{t("tabs.wheeled")}</TabsTrigger>
        </TabsList>

        <TabsContent value="trains">
          <Card>
            <CardHeader>
              <CardTitle>{t("trainsTitle")}</CardTitle>
            </CardHeader>
            <CardContent>
              {trains.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("trainsEmpty")}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.status")}</TableHead>
                      <TableHead>{t("columns.station")}</TableHead>
                      <TableHead>{t("columns.flags")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {trains.map((train) => (
                      <TableRow key={train.trainId}>
                        <TableCell className="font-medium">
                          {train.name ?? train.trainId}
                        </TableCell>
                        <TableCell>{train.status ?? "—"}</TableCell>
                        <TableCell>{train.station ?? "—"}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {train.derailed ? (
                              <Badge variant="destructive">{t("badges.derailed")}</Badge>
                            ) : null}
                            {train.pendingDerail ? (
                              <Badge variant="destructive">{t("badges.pendingDerail")}</Badge>
                            ) : null}
                            {train.selfDrivingError ? (
                              <Badge variant="outline">{train.selfDrivingError}</Badge>
                            ) : null}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="wheeled">
          <Card>
            <CardHeader>
              <CardTitle>{t("wheeledTitle")}</CardTitle>
            </CardHeader>
            <CardContent>
              {vehicles.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("wheeledEmpty")}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.type")}</TableHead>
                      <TableHead>{t("columns.driver")}</TableHead>
                      <TableHead>{t("columns.speed")}</TableHead>
                      <TableHead>{t("columns.flags")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {vehicles.map((vehicle) => (
                      <TableRow key={vehicle.vehicleId}>
                        <TableCell className="font-medium">
                          {vehicle.displayName}
                        </TableCell>
                        <TableCell>{vehicle.vehicleType}</TableCell>
                        <TableCell>{vehicle.driver ?? "—"}</TableCell>
                        <TableCell>{formatNumber(vehicle.forwardSpeed)}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {vehicle.fuelEmpty ? (
                              <Badge variant="destructive">{t("badges.fuelEmpty")}</Badge>
                            ) : null}
                            {vehicle.stuck ? (
                              <Badge variant="destructive">{t("badges.stuck")}</Badge>
                            ) : null}
                            {vehicle.autopilot ? (
                              <Badge variant="outline">{t("badges.autopilot")}</Badge>
                            ) : null}
                            {vehicle.followingPath ? (
                              <Badge variant="outline">{t("badges.followingPath")}</Badge>
                            ) : null}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
