"use client"

import { CopyButton, MoneyText, PlatformIcon, StatusBadge } from "@/components/shared"
import type { PublicOrderFieldsFragment } from "@/gql/graphql"
import { useLabel, useT } from "@/i18n"
import { parsePaymentNumbers } from "../lib"

/** Numbers to send money to + free-text instructions (storefront or order). */
export function PaymentNumbersList({ paymentNumbers, instructions }: { paymentNumbers: unknown; instructions?: string | null }) {
  const t = useT()
  const label = useLabel()
  const numbers = parsePaymentNumbers(paymentNumbers)
  return (
    <div className="space-y-3">
      {numbers.length ? (
        <div className="space-y-2">
          <p className="text-sm font-medium">{t.storefront.payTo}</p>
          <ul className="space-y-2">
            {numbers.map((n, i) => (
              <li key={`${n.method}-${i}`} className="flex items-center justify-between gap-2 rounded-2xl border border-border/80 bg-card p-3 shadow-xs">
                <div className="min-w-0">
                  <p className="text-xs text-muted-foreground">
                    {label("PaymentMethod", n.method)}
                    {n.type ? ` · ${label("PaymentNumberType", n.type)}` : ""}
                  </p>
                  <p className="font-mono text-lg font-semibold tracking-wide">{n.number}</p>
                </div>
                <CopyButton value={n.number} size="icon-sm" />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {instructions ? <p className="rounded-xl bg-muted p-3 text-sm whitespace-pre-wrap">{instructions}</p> : null}
    </div>
  )
}

export function PublicOrderSummary({ order }: { order: PublicOrderFieldsFragment }) {
  const t = useT()
  return (
    <div className="space-y-4">
      <div className="rounded-xl border-2 border-success/40 bg-success/8 p-4 text-center">
        <p className="text-sm text-muted-foreground">{t.storefront.orderNumber(order.number)}</p>
        <p className="font-semibold">{order.productName}</p>
        <MoneyText amount={order.amount} currency={order.currency} tone="money" className="text-2xl" />
        <div className="mt-2">
          <StatusBadge enum="OrderStatus" value={order.status} />
        </div>
      </div>
      {order.status === "NEW" || order.status === "AWAITING_PAYMENT" ? (
        <PaymentNumbersList paymentNumbers={order.paymentNumbers} instructions={order.paymentInstructions} />
      ) : null}
      <p className="text-center text-xs text-muted-foreground">{t.storefront.keepOrderNumber}</p>
    </div>
  )
}

export function ContactLinks({ whatsapp, messengerUrl, telegram }: { whatsapp?: string | null; messengerUrl?: string | null; telegram?: string | null }) {
  const links = [
    whatsapp ? { href: `https://wa.me/${whatsapp.replace(/^\+?0/, "880").replace(/\D/g, "")}`, platform: "WHATSAPP" as const, label: whatsapp } : null,
    messengerUrl && /^https:\/\//.test(messengerUrl) ? { href: messengerUrl, platform: "FACEBOOK" as const, label: "Messenger" } : null,
    telegram ? { href: `https://t.me/${telegram.replace(/^@/, "")}`, platform: "TELEGRAM" as const, label: telegram } : null,
  ].filter((x): x is NonNullable<typeof x> => x !== null)
  if (!links.length) return null
  return (
    <div className="flex flex-wrap justify-center gap-2">
      {links.map((l) => (
        <a key={l.platform} href={l.href} target="_blank" rel="noreferrer noopener" className="inline-flex h-10 items-center gap-2 rounded-full border bg-card px-3 text-sm font-medium">
          <PlatformIcon platform={l.platform} className="size-6" /> {l.label}
        </a>
      ))}
    </div>
  )
}
