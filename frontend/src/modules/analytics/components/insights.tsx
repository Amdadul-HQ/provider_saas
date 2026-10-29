"use client"

import Link from "next/link"
import { ShieldAlertIcon } from "lucide-react"
import { DateText, EmptyState, ErrorState, ListSkeleton, MoneyText, PlatformIcon } from "@/components/shared"
import { Progress } from "@/components/ui/progress"
import { enumLabel, useFormat, useLabel, useT } from "@/i18n"
import { useAtRiskCustomers, useChannelStats, useProductStats } from "../hooks/use-analytics"

export function AtRiskList({ limit = 8 }: { limit?: number }) {
  const t = useT()
  const label = useLabel()
  const [{ data, fetching, error }, refetch] = useAtRiskCustomers(limit)
  const items = data?.atRiskCustomers
  if (!items) {
    if (error) return <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={4} /> : null
  }
  if (items.length === 0) return <EmptyState compact icon={ShieldAlertIcon} title={t.dashboard.atRiskEmpty} />
  return (
    <ul className="divide-y">
      {items.map((r) => (
        <li key={r.customer.id} className="flex items-center gap-3 py-2.5">
          <div className="min-w-0 flex-1">
            <Link href={`/customers/${r.customer.id}`} className="block truncate font-medium hover:underline">
              {r.customer.name}
            </Link>
            <p className="truncate text-xs text-muted-foreground">
              {label("AtRiskReason", r.reason)}
              {r.expiresAt ? (
                <>
                  {" · "}
                  <DateText value={r.expiresAt} />
                </>
              ) : null}
            </p>
          </div>
          {r.dueBalance > 0 ? <MoneyText amount={r.dueBalance} tone="due" /> : null}
        </li>
      ))}
    </ul>
  )
}

export function ProductStatsList({ days = 30 }: { days?: number }) {
  const t = useT()
  const f = useFormat()
  const [{ data, fetching, error }, refetch] = useProductStats(days)
  const rows = data?.productStats
  if (!rows) {
    if (error) return <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={4} /> : null
  }
  if (rows.length === 0) return <EmptyState compact title={t.catalog.empty} />
  const sorted = [...rows].sort((a, b) => b.revenue - a.revenue)
  return (
    <ul className="divide-y">
      {sorted.map((r) => (
        <li key={r.product.id} className="space-y-1 py-2.5">
          <div className="flex items-center justify-between gap-2">
            <p className="truncate font-medium">{r.product.name}</p>
            <MoneyText amount={r.revenue} />
          </div>
          <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>
              {t.catalog.activeSubs(r.activeSubscriptions)} · {t.common.profit}: <MoneyText amount={r.profit} tone={r.profit < 0 ? "due" : "profit"} className="font-normal" />
            </span>
            <span className="flex w-28 items-center gap-1.5">
              <Progress value={Math.round(r.renewalRate * 100)} className="h-1.5" aria-label={t.dashboard.renewalRate} />
              {f.percent(r.renewalRate)}
            </span>
          </div>
        </li>
      ))}
    </ul>
  )
}

export function ChannelStatsList({ days = 30 }: { days?: number }) {
  const t = useT()
  const f = useFormat()
  const [{ data, fetching, error }, refetch] = useChannelStats(days)
  const rows = data?.channelStats
  if (!rows) {
    if (error) return <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={4} /> : null
  }
  if (rows.length === 0) return <EmptyState compact title={t.social.empty} />
  return (
    <ul className="grid grid-cols-2 gap-2">
      {rows.map((r) => (
        <li key={r.platform} className="space-y-1 rounded-xl bg-muted/60 p-3">
          <p className="flex items-center gap-2 text-sm font-medium">
            <PlatformIcon platform={r.platform} className="size-5" /> {enumLabel(t, "Platform", r.platform)}
          </p>
          <p className="text-xs text-muted-foreground">
            {t.dashboard.conversations}: {f.number(r.conversations)} · {t.nav.customers}: {f.number(r.customers)} · {t.dashboard.orders}: {f.number(r.orders)}
          </p>
        </li>
      ))}
    </ul>
  )
}
