"use client"

import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts"

import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart"
import { formatDateTime, formatTime } from "@/lib/format"

export type TimeSeriesPoint = {
  timestamp: string
  [key: string]: string | number | null
}

type TimeSeriesChartProps = {
  data: TimeSeriesPoint[]
  config: ChartConfig
  series: { key: string; label?: string }[]
  className?: string
  yAxisLabel?: string
  compactTimeAxis?: boolean
}

export function TimeSeriesChart({
  data,
  config,
  series,
  className,
  yAxisLabel,
  compactTimeAxis = false,
}: TimeSeriesChartProps) {
  return (
    <ChartContainer config={config} className={className ?? "aspect-auto h-[280px] w-full"}>
      <LineChart data={data} margin={{ left: 8, right: 8, top: 8, bottom: 0 }}>
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey="timestamp"
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          minTickGap={32}
          tickFormatter={(value) => {
            const iso = String(value)
            return compactTimeAxis ? formatTime(iso) : formatDateTime(iso)
          }}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={56}
          tickFormatter={(value) => String(value)}
          label={
            yAxisLabel
              ? {
                  value: yAxisLabel,
                  angle: -90,
                  position: "insideLeft",
                  style: { textAnchor: "middle" },
                }
              : undefined
          }
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(value) => formatDateTime(String(value))}
            />
          }
        />
        <ChartLegend content={<ChartLegendContent />} />
        {series.map((item) => (
          <Line
            key={item.key}
            type="monotone"
            dataKey={item.key}
            stroke={`var(--color-${item.key})`}
            dot={false}
            strokeWidth={2}
            connectNulls
          />
        ))}
      </LineChart>
    </ChartContainer>
  )
}
