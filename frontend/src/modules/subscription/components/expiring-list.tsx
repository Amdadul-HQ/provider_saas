"use client"

import Link from "next/link"
import { CalendarClockIcon } from "lucide-react"
import { DaysLeft, EmptyState, ErrorState, ListSkeleton, MoneyText } from "@/components/shared"
import { useT } from "@/i18n"
import { useExpiringSubscriptions } from "../hooks/use-subscriptions"
import { SubscriptionActions } from "./subscription-actions"

/** "Expiring soon" card list for the dashboard; renew straight from the row. */
export function ExpiringList({ days = 7, limit = 8 }: { days?: number; limit?: number }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useExpiringSubscriptions(days)
  const items = data?.expiringSubscriptions
  if (!items) {
    if (error) return <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={4} /> : null
  }
  if (items.length === 0) return <EmptyState compact icon={CalendarClockIcon} title={t.dashboard.expiringEmpty} />
  return (
    <ul className="divide-y">
      {[...items]
        .sort((a, b) => a.daysLeft - b.daysLeft)
        .slice(0, limit)
        .map((s) => (
          <li key={s.id} className="flex items-center gap-3 py-2.5">
            <div className="min-w-0 flex-1">
              <Link href={`/customers/${s.customer.id}`} className="block truncate font-medium hover:underline">
                {s.customer.name}
              </Link>
              <p className="truncate text-xs text-muted-foreground">
                {s.product.name} · <MoneyText amount={s.price} currency={s.currency} tone="muted" className="font-normal" />
              </p>
            </div>
            <DaysLeft days={s.daysLeft} />
            <SubscriptionActions subscription={s} />
          </li>
        ))}
    </ul>
  )
}
