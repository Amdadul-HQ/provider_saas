"use client"

import Link from "next/link"
import { ExternalLinkIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DaysLeft, ErrorState, ListSkeleton, MoneyText, StatusBadge } from "@/components/shared"
import { useT } from "@/i18n"
import { useCustomer } from "../hooks/use-customers"
import { CustomerIdentities } from "./customer-profile"

/** Compact customer context used beside chats (inbox) — subscriptions, due, identities. */
export function CustomerSidePanel({ customerId, actions }: { customerId: string; actions?: React.ReactNode }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useCustomer(customerId)
  const c = data?.customer
  if (!c) return error ? <ErrorState compact error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} /> : fetching ? <ListSkeleton rows={4} /> : null
  return (
    <div className="space-y-4 text-sm">
      <div className="space-y-1">
        <div className="flex items-center justify-between gap-2">
          <p className="text-base font-semibold">{c.name}</p>
          <StatusBadge enum="CustomerStatus" value={c.status} />
        </div>
        <p className="text-muted-foreground">{c.phone ?? c.email ?? "—"}</p>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="rounded-lg bg-muted p-2.5">
          <p className="text-xs text-muted-foreground">{t.customers.dueBalance}</p>
          <MoneyText amount={c.dueBalance} tone={c.dueBalance > 0 ? "due" : "muted"} />
        </div>
        <div className="rounded-lg bg-muted p-2.5">
          <p className="text-xs text-muted-foreground">{t.customers.lifetimeValue}</p>
          <MoneyText amount={c.lifetimeValue} />
        </div>
      </div>
      <div className="space-y-2">
        <p className="text-xs font-semibold text-muted-foreground uppercase">{t.customers.tabs.subscriptions}</p>
        {c.subscriptions.length === 0 ? (
          <p className="text-muted-foreground">—</p>
        ) : (
          <ul className="space-y-1.5">
            {c.subscriptions.slice(0, 5).map((s) => (
              <li key={s.id} className="flex items-center justify-between gap-2 rounded-lg border p-2">
                <div className="min-w-0">
                  <p className="truncate font-medium">{s.product.name}</p>
                  <DaysLeft days={s.daysLeft} />
                </div>
                <StatusBadge enum="SubscriptionStatus" value={s.status} />
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="space-y-2">
        <p className="text-xs font-semibold text-muted-foreground uppercase">{t.customers.identities}</p>
        <CustomerIdentities identities={c.identities} />
      </div>
      <div className="flex flex-col gap-2">
        {actions}
        <Button variant="outline" asChild>
          <Link href={`/customers/${c.id}`}>
            <ExternalLinkIcon /> {t.inbox.viewCustomer}
          </Link>
        </Button>
      </div>
    </div>
  )
}
