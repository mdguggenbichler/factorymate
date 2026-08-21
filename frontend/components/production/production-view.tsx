"use client"

import { Fragment, useCallback, useMemo, useState } from "react"
import { useTranslations } from "next-intl"

import { IntervalPicker } from "@/components/interval-picker"
import { ItemIcon } from "@/components/item-icon"
import { ItemWithLabel } from "@/components/item-with-label"
import { TimeSeriesChart } from "@/components/time-series-chart"
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
import { apiFetch } from "@/lib/api"
import {
  buildDateRangeQuery,
  isShortTimePreset,
  presetToDateRange,
  type DateRangePreset,
} from "@/lib/date-range"
import { formatNumber, formatPercent } from "@/lib/format"
import { machineClassNameFromId } from "@/lib/item-icon"
import type {
  FactoryItem,
  ProductionHistoryResponse,
  ProductionItem,
  ProductionMachine,
} from "@/lib/api-types"
import type { ChartConfig } from "@/components/ui/chart"

type ProductionViewProps = {
  overallItems: ProductionItem[]
  machines: ProductionMachine[]
}

const chartConfig = {
  produced: { label: "Produced", color: "var(--chart-1)" },
  consumed: { label: "Consumed", color: "var(--chart-2)" },
} satisfies ChartConfig

export function ProductionView({ overallItems, machines }: ProductionViewProps) {
  const t = useTranslations("production")

  return (
    <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </div>

      <Tabs defaultValue="overall">
        <TabsList>
          <TabsTrigger value="overall">{t("tabs.overall")}</TabsTrigger>
          <TabsTrigger value="detailed">{t("tabs.detailed")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overall">
          <OverallTab items={overallItems} />
        </TabsContent>
        <TabsContent value="detailed">
          <DetailedTab machines={machines} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function OverallTab({ items }: { items: ProductionItem[] }) {
  const t = useTranslations("production")
  const tCharts = useTranslations("charts")
  const [expandedClass, setExpandedClass] = useState<string | null>(null)
  const [interval, setInterval] = useState<DateRangePreset>("7d")
  const [history, setHistory] = useState<ProductionHistoryResponse["items"]>([])
  const [loadingChart, setLoadingChart] = useState(false)

  const loadHistory = useCallback(
    async (itemClassName: string, rangePreset: DateRangePreset) => {
      setLoadingChart(true)
      try {
        const range = presetToDateRange(rangePreset)
        const query = buildDateRangeQuery(range)
        const response = await apiFetch<ProductionHistoryResponse>(
          `/production?item=${encodeURIComponent(itemClassName)}${query ? query.replace("?", "&") : ""}`
        )
        setHistory(response.items)
      } catch {
        setHistory([])
      } finally {
        setLoadingChart(false)
      }
    },
    []
  )

  const handleRowToggle = (itemClassName: string) => {
    const next = expandedClass === itemClassName ? null : itemClassName
    setExpandedClass(next)
    if (next) {
      void loadHistory(next, interval)
    } else {
      setHistory([])
    }
  }

  const handleIntervalChange = (next: DateRangePreset) => {
    setInterval(next)
    if (expandedClass) {
      void loadHistory(expandedClass, next)
    }
  }

  const chartData = useMemo(
    () =>
      history.map((point) => ({
        timestamp: point.capturedAt,
        produced: point.producedPerMin,
        consumed: point.consumedPerMin,
      })),
    [history]
  )

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("overallTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("overallEmpty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("columns.item")}</TableHead>
                <TableHead>{t("columns.rate")}</TableHead>
                <TableHead>{t("columns.prodPercent")}</TableHead>
                <TableHead>{t("columns.consPercent")}</TableHead>
                <TableHead>{t("columns.current")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => {
                const isExpanded = expandedClass === item.itemClassName
                return (
                  <Fragment key={item.itemClassName}>
                    <TableRow
                      className="cursor-pointer"
                      data-state={isExpanded ? "selected" : undefined}
                      onClick={() => handleRowToggle(item.itemClassName)}
                    >
                      <TableCell className="font-medium">
                        <ItemWithLabel
                          className={item.itemClassName}
                          label={item.itemDisplayName}
                          size={20}
                        />
                      </TableCell>
                      <TableCell>{item.prodPerMinLabel}</TableCell>
                      <TableCell>{formatPercent(item.prodPercent)}</TableCell>
                      <TableCell>{formatPercent(item.consPercent)}</TableCell>
                      <TableCell>
                        {t("currentValues", {
                          prod: formatNumber(item.currentProd),
                          maxProd: formatNumber(item.maxProd),
                          cons: formatNumber(item.currentConsumed),
                          maxCons: formatNumber(item.maxConsumed),
                        })}
                      </TableCell>
                    </TableRow>
                    {isExpanded ? (
                      <TableRow>
                        <TableCell colSpan={5} className="bg-muted/30 p-4">
                          <div className="mb-3 flex justify-end">
                            <IntervalPicker
                              value={interval}
                              onChange={handleIntervalChange}
                            />
                          </div>
                          {loadingChart ? (
                            <p className="text-sm text-muted-foreground">
                              {tCharts("loading")}
                            </p>
                          ) : chartData.length === 0 ? (
                            <p className="text-sm text-muted-foreground">
                              {tCharts("noData")}
                            </p>
                          ) : (
                            <TimeSeriesChart
                              data={chartData}
                              config={chartConfig}
                              series={[
                                { key: "produced" },
                                { key: "consumed" },
                              ]}
                              yAxisLabel="/min"
                              compactTimeAxis={isShortTimePreset(interval)}
                            />
                          )}
                        </TableCell>
                      </TableRow>
                    ) : null}
                  </Fragment>
                )
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function DetailedTab({ machines }: { machines: ProductionMachine[] }) {
  const t = useTranslations("production")
  const [expandedId, setExpandedId] = useState<string | null>(null)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("detailedTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {machines.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("detailedEmpty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("columns.building")}</TableHead>
                <TableHead>{t("columns.recipe")}</TableHead>
                <TableHead>{t("columns.speed")}</TableHead>
                <TableHead>{t("columns.status")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {machines.map((machine) => {
                const isExpanded = expandedId === machine.machineId
                return (
                  <Fragment key={machine.machineId}>
                    <TableRow
                      className="cursor-pointer"
                      data-state={isExpanded ? "selected" : undefined}
                      onClick={() =>
                        setExpandedId(
                          isExpanded ? null : machine.machineId
                        )
                      }
                    >
                      <TableCell className="font-medium">
                        <ItemWithLabel
                          className={machineClassNameFromId(machine.machineId)}
                          label={machine.buildingType}
                          size={20}
                        />
                      </TableCell>
                      <TableCell>{machine.recipe || "—"}</TableCell>
                      <TableCell>{formatPercent(machine.manuSpeed)}</TableCell>
                      <TableCell>
                        <MachineStatus machine={machine} />
                      </TableCell>
                    </TableRow>
                    {isExpanded ? (
                      <TableRow>
                        <TableCell colSpan={4} className="bg-muted/30 p-4">
                          <div className="grid gap-4 md:grid-cols-2">
                            <ItemList
                              title={t("ingredients")}
                              items={machine.ingredients}
                            />
                            <ItemList
                              title={t("outputs")}
                              items={machine.production}
                            />
                          </div>
                        </TableCell>
                      </TableRow>
                    ) : null}
                  </Fragment>
                )
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function MachineStatus({ machine }: { machine: ProductionMachine }) {
  const t = useTranslations("production")

  if (!machine.isConfigured) {
    return <Badge variant="secondary">{t("status.notConfigured")}</Badge>
  }
  if (machine.isPaused) {
    return <Badge variant="outline">{t("status.paused")}</Badge>
  }
  if (machine.isProducing) {
    return <Badge>{t("status.producing")}</Badge>
  }
  return <Badge variant="secondary">{t("status.idle")}</Badge>
}

function ItemList({ title, items }: { title: string; items: FactoryItem[] }) {
  return (
    <div>
      <p className="mb-2 text-sm font-medium">{title}</p>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">—</p>
      ) : (
        <ul className="space-y-1 text-sm">
          {items.map((item, index) => (
            <li
              key={`${item.className ?? item.name ?? index}-${index}`}
              className="flex items-center gap-2"
            >
              <ItemIcon className={item.className} size={16} />
              <span>{itemLabel(item)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function itemLabel(item: FactoryItem): string {
  const name = item.name ?? item.className ?? "?"
  const amount = item.amount != null ? ` × ${item.amount}` : ""
  return `${name}${amount}`
}
