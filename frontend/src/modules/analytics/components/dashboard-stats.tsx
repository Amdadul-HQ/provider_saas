"use client"

import {
  AlertTriangleIcon,
  BadgeDollarSignIcon,
  BoxesIcon,
  CalendarClockIcon,
  CreditCardIcon,
  HandCoinsIcon,
  LifeBuoyIcon,
  MessagesSquareIcon,
  RepeatIcon,
  TrendingUpIcon,
  UsersIcon,
} from "lucide-react"
import { CardsSkeleton, ErrorState, MoneyText, StatCard } from "@/components/shared"
import { formatNumber } from "@/lib/money"
import { useDigits, useT } from "@/i18n"
import { useDashboardStats } from "../hooks/use-analytics"

/** KPI tiles from `dashboardStats`, ordered by what a seller acts on first. */
export function DashboardStatsCards() {
  const t = useT()
  const digits = useDigits()
  const [{ data, fetching, error }, refetch] = useDashboardStats()
  if (!data) {
    if (error) return <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <CardsSkeleton count={8} /> : null
  }
  const s = data.dashboardStats
  const n = (v: number) => formatNumber(v, digits)
  return (
    <div className="space-y-3">
      <div className="stagger grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard label={t.dashboard.revenueThisMonth} value={<MoneyText amount={s.revenueThisMonth} currency={s.currency} />} icon={BadgeDollarSignIcon} tone="money" />
        <StatCard
          label={t.dashboard.profitThisMonth}
          value={<MoneyText amount={s.profitThisMonth} currency={s.currency} tone={s.profitThisMonth < 0 ? "due" : "default"} />}
          hint={
            <>
              {t.dashboard.costThisMonth}: <MoneyText amount={s.costThisMonth} currency={s.currency} tone="muted" className="font-normal" />
            </>
          }
          icon={TrendingUpIcon}
          tone="success"
        />
        <StatCard label={t.dashboard.totalDue} value={<MoneyText amount={s.totalDue} currency={s.currency} tone={s.totalDue > 0 ? "due" : "default"} />} icon={HandCoinsIcon} tone="danger" href="/customers" />
        <StatCard label={t.dashboard.mrr} value={<MoneyText amount={s.mrr} currency={s.currency} />} icon={RepeatIcon} />
      </div>
      <div className="stagger grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard label={t.dashboard.activeSubscriptions} value={n(s.activeSubscriptions)} icon={RepeatIcon} href="/subscriptions" />
        <StatCard label={t.dashboard.expiringIn7Days} value={n(s.expiringIn7Days)} hint={`${t.dashboard.expiredUnrenewed}: ${n(s.expiredUnrenewed30Days)}`} icon={CalendarClockIcon} tone="warning" href="/subscriptions" />
        <StatCard label={t.dashboard.totalCustomers} value={n(s.totalCustomers)} hint={t.dashboard.newCustomers(s.newCustomersThisMonth)} icon={UsersIcon} href="/customers" />
        <StatCard label={t.dashboard.pendingPayments} value={n(s.pendingPayments)} icon={CreditCardIcon} tone={s.pendingPayments > 0 ? "warning" : "default"} href="/payments" />
        <StatCard label={t.dashboard.openConversations} value={n(s.openConversations)} icon={MessagesSquareIcon} href="/inbox" />
        <StatCard label={t.dashboard.openTickets} value={n(s.openTickets)} icon={LifeBuoyIcon} href="/tickets" />
        <StatCard label={t.dashboard.freeSlots} value={n(s.freeSlots)} icon={BoxesIcon} tone={s.freeSlots === 0 ? "danger" : "default"} href="/inventory" />
        <StatCard label={t.dashboard.masterExpiring} value={n(s.masterAccountsExpiringIn7Days)} icon={AlertTriangleIcon} tone={s.masterAccountsExpiringIn7Days > 0 ? "warning" : "default"} href="/inventory" />
      </div>
    </div>
  )
}
