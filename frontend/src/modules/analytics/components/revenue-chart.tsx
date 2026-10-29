"use client"

import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipContentProps } from "recharts"
import { TrendingUpIcon } from "lucide-react"
import { EmptyState, ErrorState } from "@/components/shared"
import { Skeleton } from "@/components/ui/skeleton"
import { formatDate } from "@/lib/dates"
import { formatMoney } from "@/lib/money"
import { useDigits, useLocale, useT } from "@/i18n"
import { useRevenueSeries } from "../hooks/use-analytics"

type Point = { date: string; value: number; count: number }

function ChartTooltip({ active, payload }: Partial<TooltipContentProps<number, string>>) {
  const locale = useLocale()
  const digits = useDigits()
  const t = useT()
  const p = payload?.[0]?.payload as Point | undefined
  if (!active || !p) return null
  return (
    <div className="rounded-lg border bg-popover px-3 py-2 text-xs shadow-md">
      <p className="text-muted-foreground">{formatDate(p.date, locale)}</p>
      <p className="text-sm font-semibold tabular">{formatMoney(p.value, { digits })}</p>
      <p className="text-muted-foreground">
        {t.dashboard.orders}: {p.count}
      </p>
    </div>
  )
}

/** Single-series daily revenue (one hue, 2px line, soft fill, crosshair tooltip, recessive axes). */
export function RevenueChart({ days = 30 }: { days?: number }) {
  const t = useT()
  const locale = useLocale()
  const digits = useDigits()
  const [{ data, fetching, error }, refetch] = useRevenueSeries(days)
  if (!data) {
    if (error) return <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <Skeleton className="h-56 w-full rounded-xl" /> : null
  }
  const points: Point[] = data.revenueSeries.map((p) => ({ date: p.date, value: p.value, count: p.count }))
  if (points.length === 0 || points.every((p) => p.value === 0)) {
    return <EmptyState compact icon={TrendingUpIcon} title={t.dashboard.revenueChartEmpty} />
  }
  return (
    <figure className="h-56 w-full" aria-label={t.dashboard.revenueChart}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={points} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id="rev-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--chart-1)" stopOpacity={0.28} />
              <stop offset="100%" stopColor="var(--chart-1)" stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid vertical={false} stroke="var(--border)" strokeDasharray="3 3" />
          <XAxis
            dataKey="date"
            tickLine={false}
            axisLine={false}
            minTickGap={24}
            tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
            tickFormatter={(d: string) => formatDate(d, locale).replace(/,?\s*\d{4}$/, "")}
          />
          <YAxis
            width={56}
            tickLine={false}
            axisLine={false}
            tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
            tickFormatter={(v: number) => formatMoney(v, { digits, compact: true })}
          />
          <Tooltip content={<ChartTooltip />} cursor={{ stroke: "var(--muted-foreground)", strokeDasharray: "3 3" }} />
          <Area type="monotone" dataKey="value" stroke="var(--chart-1)" strokeWidth={2} fill="url(#rev-fill)" activeDot={{ r: 4, strokeWidth: 2, stroke: "var(--card)" }} />
        </AreaChart>
      </ResponsiveContainer>
    </figure>
  )
}
