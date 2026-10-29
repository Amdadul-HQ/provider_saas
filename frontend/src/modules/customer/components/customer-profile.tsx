"use client"

import type { ReactNode } from "react"
import { MailIcon, MapPinIcon, PhoneIcon, SendIcon } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { DateText, MoneyText, PlatformIcon, StatusBadge } from "@/components/shared"
import type { CustomerDetailFieldsFragment } from "@/gql/graphql"
import { enumLabel, useT } from "@/i18n"

/** Header card for a customer: contact, money, identities. Pure/presentational. */
export function CustomerProfile({ customer, actions }: { customer: CustomerDetailFieldsFragment; actions?: ReactNode }) {
  const t = useT()
  return (
    <div className="rounded-xl border bg-card">
      <div className="flex flex-col gap-4 p-4 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="font-heading text-xl font-semibold">{customer.name}</h1>
            <StatusBadge enum="CustomerStatus" value={customer.status} />
            <Badge variant="outline">{enumLabel(t, "CustomerSource", customer.source)}</Badge>
          </div>
          <ul className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
            {customer.phone ? (
              <li className="flex items-center gap-1.5">
                <PhoneIcon className="size-3.5" />
                <a href={`tel:${customer.phone}`} className="hover:text-foreground">
                  {customer.phone}
                </a>
              </li>
            ) : null}
            {customer.whatsapp ? (
              <li className="flex items-center gap-1.5">
                <PlatformIcon platform="WHATSAPP" className="size-4" />
                <a href={`https://wa.me/${customer.whatsapp.replace(/^0/, "880")}`} target="_blank" rel="noreferrer" className="hover:text-foreground">
                  {customer.whatsapp}
                </a>
              </li>
            ) : null}
            {customer.email ? (
              <li className="flex items-center gap-1.5">
                <MailIcon className="size-3.5" /> {customer.email}
              </li>
            ) : null}
            {customer.telegram ? (
              <li className="flex items-center gap-1.5">
                <SendIcon className="size-3.5" /> {customer.telegram}
              </li>
            ) : null}
            {customer.address ? (
              <li className="flex items-center gap-1.5">
                <MapPinIcon className="size-3.5" /> {customer.address}
              </li>
            ) : null}
          </ul>
          {customer.tags.length ? (
            <div className="flex flex-wrap gap-1">
              {customer.tags.map((tag) => (
                <Badge key={tag} variant="secondary">
                  {tag}
                </Badge>
              ))}
            </div>
          ) : null}
        </div>
        {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
      </div>
      <dl className="grid grid-cols-2 border-t md:grid-cols-4">
        <div className="border-r p-3">
          <dt className="text-xs text-muted-foreground">{t.customers.dueBalance}</dt>
          <dd>
            <MoneyText amount={customer.dueBalance} tone={customer.dueBalance > 0 ? "due" : "muted"} className="text-lg" />
          </dd>
        </div>
        <div className="p-3 md:border-r">
          <dt className="text-xs text-muted-foreground">{t.customers.lifetimeValue}</dt>
          <dd>
            <MoneyText amount={customer.lifetimeValue} tone="money" className="text-lg" />
          </dd>
        </div>
        <div className="border-t border-r p-3 md:border-t-0">
          <dt className="text-xs text-muted-foreground">{t.customers.activeSubs}</dt>
          <dd className="tabular text-lg font-semibold">{customer.activeSubscriptionCount}</dd>
        </div>
        <div className="border-t p-3 md:border-t-0">
          <dt className="text-xs text-muted-foreground">{t.customers.lastContact}</dt>
          <dd className="text-sm font-medium">
            <DateText value={customer.lastContactAt} relative />
          </dd>
        </div>
      </dl>
    </div>
  )
}

export function CustomerIdentities({ identities }: { identities: CustomerDetailFieldsFragment["identities"] }) {
  const t = useT()
  if (!identities.length) return <p className="text-sm text-muted-foreground">{t.customers.noIdentities}</p>
  return (
    <ul className="space-y-2">
      {identities.map((i) => (
        <li key={i.id} className="flex items-center gap-2 text-sm">
          <PlatformIcon platform={i.platform} />
          <span className="font-medium">{i.handle ?? i.externalId}</span>
          <span className="text-xs text-muted-foreground">{enumLabel(t, "Platform", i.platform)}</span>
        </li>
      ))}
    </ul>
  )
}
