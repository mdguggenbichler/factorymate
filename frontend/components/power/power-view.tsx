"use client"

import { useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { AlertTriangleIcon } from "lucide-react"

import { IntervalPicker } from "@/components/interval-picker"
import { TimeSeriesChart } from "@/components/time-series-chart"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
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
import { apiFetch } from "@/lib/api"
import {
  buildDateRangeQuery,
  isShortTimePreset,
  presetToDateRange,
  type DateRangePreset,
} from "@/lib/date-range"
import { useFormatDateTime } from "@/hooks/use-format-datetime"
import { formatMw, formatNumber, formatPercent } from "@/lib/format"
import type {
  Circuit,
  PowerHistoryEvent,
  PowerMetricsResponse,
  PowerResponse,
} from "@/lib/api-types"
import type { ChartConfig } from "@/components/ui/chart"

type PowerViewProps = {
  initialPower: PowerResponse
  initialHistory: PowerHistoryEvent[]
  initialMetrics: PowerMetricsResponse["items"]
}

const chartConfig = {
  production: { label: "Production", color: "var(--chart-1)" },
  consumed: { label: "Consumption", color: "var(--chart-2)" },
  battery: { label: "Battery", color: "var(--chart-3)" },
} satisfies ChartConfig

export function PowerView({
  initialPower,
  initialHistory,
  initialMetrics,
}: PowerViewProps) {
  const t = useTranslations("power")
  const tCharts = useTranslations("charts")
  const { formatDateTime } = useFormatDateTime()
  const [circuits] = useState(initialPower.circuits)
  const [history] = useState(initialHistory)
  const [selectedCircuit, setSelectedCircuit] = useState<string>(
    () => String(initialPower.circuits[0]?.circuitId ?? "")
  )
  const [interval, setInterval] = useState<DateRangePreset>("7d")
  const [metrics, setMetrics] = useState<PowerMetricsResponse["items"]>(
    initialMetrics
  )
  const [loadingChart, setLoadingChart] = useState(false)

  const loadMetrics = useCallback(
    async (circuit: string, rangePreset: DateRangePreset) => {
      if (!circuit) {
        setMetrics([])
        return
      }

      setLoadingChart(true)
      try {
        const range = presetToDateRange(rangePreset)
        const query = buildDateRangeQuery(range)
        const response = await apiFetch<PowerMetricsResponse>(
          `/power/metrics?circuit=${circuit}${query ? query.replace("?", "&") : ""}`
        )
        setMetrics(response.items)
      } catch {
        setMetrics([])
      } finally {
        setLoadingChart(false)
      }
    },
    []
  )

  const handleCircuitChange = (circuit: string | null) => {
    if (!circuit) {
      return
    }
    setSelectedCircuit(circuit)
    void loadMetrics(circuit, interval)
  }

  const handleIntervalChange = (next: DateRangePreset) => {
    setInterval(next)
    void loadMetrics(selectedCircuit, next)
  }

  const chartData = useMemo(
    () =>
      metrics.map((point) => ({
        timestamp: point.capturedAt,
        production: point.powerProduction,
        consumed: point.powerConsumed,
        battery: point.batteryPercent,
      })),
    [metrics]
  )

  const trippedCircuits = circuits.filter((circuit) => circuit.tripped)

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      {trippedCircuits.length > 0 ? (
        <Alert variant="destructive">
          <AlertTriangleIcon />
          <AlertTitle>{t("fuseAlertTitle")}</AlertTitle>
          <AlertDescription>
            {t("fuseAlertDescription", {
              circuits: trippedCircuits.map((c) => c.circuitId).join(", "),
            })}
          </AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("circuitsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {circuits.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("columns.circuit")}</TableHead>
                  <TableHead>{t("columns.status")}</TableHead>
                  <TableHead>{t("columns.production")}</TableHead>
                  <TableHead>{t("columns.consumed")}</TableHead>
                  <TableHead>{t("columns.capacity")}</TableHead>
                  <TableHead>{t("columns.battery")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {circuits.map((circuit) => (
                  <CircuitRow key={circuit.circuitId} circuit={circuit} t={t} />
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-4">
          <CardTitle>{t("chartTitle")}</CardTitle>
          <div className="flex flex-wrap items-center gap-2">
            <Select value={selectedCircuit} onValueChange={handleCircuitChange}>
              <SelectTrigger className="w-[140px]" size="sm" aria-label={t("circuitFilter")}>
                <SelectValue placeholder={t("circuitFilter")} />
              </SelectTrigger>
              <SelectContent>
                {circuits.map((circuit) => (
                  <SelectItem key={circuit.circuitId} value={String(circuit.circuitId)}>
                    {t("circuitLabel", { id: circuit.circuitId })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <IntervalPicker value={interval} onChange={handleIntervalChange} />
          </div>
        </CardHeader>
        <CardContent>
          {loadingChart ? (
            <p className="text-sm text-muted-foreground">{tCharts("loading")}</p>
          ) : chartData.length === 0 ? (
            <p className="text-sm text-muted-foreground">{tCharts("noData")}</p>
          ) : (
            <TimeSeriesChart
              data={chartData}
              config={chartConfig}
              series={[
                { key: "production" },
                { key: "consumed" },
                { key: "battery" },
              ]}
              yAxisLabel="MW / %"
              compactTimeAxis={isShortTimePreset(interval)}
            />
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
            <div className="space-y-3">
              {history.map((event) => (
                <div
                  key={event.id}
                  className="flex flex-col gap-1 border-b pb-3 last:border-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="flex items-center gap-2">
                    <Badge
                      variant={
                        event.eventType === "fuse_tripped"
                          ? "destructive"
                          : "outline"
                      }
                    >
                      {event.eventType === "fuse_tripped"
                        ? t("event.tripped")
                        : t("event.restored")}
                    </Badge>
                    <span>{t("circuitLabel", { id: event.circuitId })}</span>
                  </div>
                  <time
                    className="text-sm text-muted-foreground"
                    dateTime={event.occurredAt}
                  >
                    {formatDateTime(event.occurredAt)}
                  </time>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function CircuitRow({
  circuit,
  t,
}: {
  circuit: Circuit
  t: ReturnType<typeof useTranslations<"power">>
}) {
  const capacityUsed =
    circuit.powerCapacity && circuit.powerCapacity > 0 && circuit.powerConsumed != null
      ? (circuit.powerConsumed / circuit.powerCapacity) * 100
      : null

  return (
    <TableRow>
      <TableCell className="font-medium">
        {t("circuitLabel", { id: circuit.circuitId })}
      </TableCell>
      <TableCell>
        <Badge variant={circuit.tripped ? "destructive" : "outline"}>
          {circuit.tripped ? t("status.tripped") : t("status.ok")}
        </Badge>
      </TableCell>
      <TableCell>{formatMw(circuit.powerProduction)}</TableCell>
      <TableCell>{formatMw(circuit.powerConsumed)}</TableCell>
      <TableCell>
        <div className="space-y-1">
          <span>{formatMw(circuit.powerCapacity)}</span>
          {capacityUsed != null ? (
            <Progress value={capacityUsed} aria-label={t("columns.capacity")} />
          ) : null}
        </div>
      </TableCell>
      <TableCell>
        <div className="space-y-1 text-sm">
          <div>{formatPercent(circuit.batteryPercent)}</div>
          <div className="text-muted-foreground">
            {t("batteryDetail", {
              differential: formatNumber(circuit.batteryDifferential),
              empty: circuit.batteryTimeEmpty ?? "—",
              full: circuit.batteryTimeFull ?? "—",
            })}
          </div>
        </div>
      </TableCell>
    </TableRow>
  )
}
