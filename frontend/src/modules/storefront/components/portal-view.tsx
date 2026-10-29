"use client"

import { useState } from "react"
import { LinkIcon, RefreshCwIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { DateText, DaysLeft, EmptyState, ErrorState, LanguageToggle, MoneyText, ResponsiveDialog, StatusBadge } from "@/components/shared"
import type { PaymentMethod, PortalQuery, PublicOrderFieldsFragment } from "@/gql/graphql"
import { MOBILE_WALLETS } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import { usePortal, usePortalRenewal } from "../hooks/use-storefront"
import { themeStyle } from "../lib"
import { ContactLinks, PaymentNumbersList, PublicOrderSummary } from "./payment-instructions"

type Portal = NonNullable<PortalQuery["portal"]>
type PortalSub = Portal["subscriptions"][number]

function RenewDialog({ token, sub, portal, onOpenChange }: { token: string; sub: PortalSub | null; portal: Portal; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [renew, busy] = usePortalRenewal()
  const [method, setMethod] = useState<PaymentMethod>("BKASH")
  const [trxId, setTrxId] = useState("")
  const [order, setOrder] = useState<PublicOrderFieldsFragment | null>(null)
  const close = (o: boolean) => {
    if (!o) {
      setOrder(null)
      setTrxId("")
    }
    onOpenChange(o)
  }
  return (
    <ResponsiveDialog
      open={sub !== null}
      onOpenChange={close}
      title={t.portal.renewTitle}
      description={sub ? `${sub.productName}${sub.planName ? ` · ${sub.planName}` : ""}` : undefined}
      footer={
        order ? (
          <Button onClick={() => close(false)}>{t.common.close}</Button>
        ) : (
          <Button
            size="lg"
            disabled={busy}
            onClick={async () => {
              if (!sub) return
              const res = await renew({ token, subscriptionId: sub.id, paymentMethod: method, trxId: trxId.trim().toUpperCase() || null })
              if (res) setOrder(res.portalRequestRenewal)
            }}
          >
            <RefreshCwIcon /> {t.portal.renew}
          </Button>
        )
      }
    >
      {order ? (
        <PublicOrderSummary order={order} />
      ) : (
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">{t.portal.renewHint}</p>
          <PaymentNumbersList paymentNumbers={portal.storefront.paymentNumbers} instructions={portal.storefront.paymentInstructions} />
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="renew-method">{t.billing.method}</Label>
              <Select value={method} onValueChange={(m) => setMethod(m as PaymentMethod)}>
                <SelectTrigger id="renew-method" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MOBILE_WALLETS.map((m) => (
                    <SelectItem key={m} value={m}>
                      {enumLabel(t, "PaymentMethod", m)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="renew-trx">{t.billing.trxId}</Label>
              <Input id="renew-trx" value={trxId} onChange={(e) => setTrxId(e.target.value)} placeholder={t.billing.trxIdPlaceholder} />
            </div>
          </div>
        </div>
      )}
    </ResponsiveDialog>
  )
}

/** Magic-link customer portal `/p/[token]`: subscriptions, dues, one-tap renew. */
export function PortalView({ token }: { token: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = usePortal(token)
  const [renewing, setRenewing] = useState<PortalSub | null>(null)
  const portal = data?.portal

  if (!data) {
    if (error)
      return (
        <div className="mx-auto max-w-lg p-4 pt-16">
          <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
        </div>
      )
    return (
      <div className="mx-auto max-w-lg space-y-3 p-4 pt-10" aria-busy={fetching}>
        <Skeleton className="h-24 rounded-2xl" />
        <Skeleton className="h-32 rounded-2xl" />
        <Skeleton className="h-32 rounded-2xl" />
      </div>
    )
  }
  if (!portal) {
    return (
      <div className="mx-auto max-w-md p-4 pt-20">
        <EmptyState icon={LinkIcon} title={t.portal.invalidLink} description={t.portal.invalidLinkHint} />
      </div>
    )
  }
  const sf = portal.storefront
  return (
    <div style={themeStyle(sf.theme)} className="min-h-dvh bg-background pb-10">
      <header className="bg-primary px-4 pt-6 pb-16 text-primary-foreground">
        <div className="mx-auto flex max-w-lg items-center justify-between gap-2">
          <div>
            <p className="text-sm opacity-80">{sf.displayName}</p>
            <h1 className="font-heading text-xl font-semibold">{t.portal.hello(portal.customer.name)}</h1>
          </div>
          <LanguageToggle className="text-primary-foreground hover:bg-white/10" />
        </div>
      </header>
      <main className="mx-auto -mt-10 max-w-lg space-y-4 px-4">
        <div className="flex items-center justify-between rounded-2xl border bg-card p-4 shadow-sm">
          <span className="text-sm text-muted-foreground">{t.portal.due}</span>
          <MoneyText amount={portal.customer.dueBalance} tone={portal.customer.dueBalance > 0 ? "due" : "muted"} className="text-2xl" />
        </div>
        <section className="space-y-2">
          <h2 className="font-semibold">{t.portal.subscriptions}</h2>
          {portal.subscriptions.length === 0 ? (
            <EmptyState compact title={t.portal.noSubscriptions} />
          ) : (
            <ul className="space-y-2">
              {portal.subscriptions.map((s) => (
                <li key={s.id} className="space-y-2 rounded-2xl border bg-card p-4">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="font-semibold">{s.productName}</p>
                      <p className="text-xs text-muted-foreground">{s.planName}</p>
                    </div>
                    <StatusBadge enum="SubscriptionStatus" value={s.status} />
                  </div>
                  <div className="flex items-center justify-between gap-2 text-sm">
                    <span>
                      {t.common.expiresAt}: <DateText value={s.expiresAt} className="font-medium" />
                    </span>
                    <DaysLeft days={s.daysLeft} />
                  </div>
                  {s.deliveryNote ? <p className="rounded-lg bg-muted p-2 text-xs whitespace-pre-wrap">{s.deliveryNote}</p> : null}
                  {s.status !== "CANCELLED" && s.status !== "REPLACED" ? (
                    <Button className="w-full" variant={s.daysLeft <= 3 ? "default" : "outline"} onClick={() => setRenewing(s)}>
                      <RefreshCwIcon /> {t.portal.renew}
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </section>
        <section className="space-y-2">
          <h2 className="font-semibold">{t.portal.invoices}</h2>
          {portal.invoices.length === 0 ? (
            <EmptyState compact title={t.portal.noInvoices} />
          ) : (
            <ul className="divide-y rounded-2xl border bg-card">
              {portal.invoices.map((inv) => (
                <li key={inv.id} className="flex items-center justify-between gap-2 p-3">
                  <div>
                    <p className="font-mono text-sm">{inv.number}</p>
                    <DateText value={inv.issuedAt} className="text-xs text-muted-foreground" />
                  </div>
                  <div className="text-right">
                    <MoneyText amount={inv.total} />
                    <div>
                      <StatusBadge enum="InvoiceStatus" value={inv.status} />
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
        <ContactLinks whatsapp={sf.whatsapp} messengerUrl={sf.messengerUrl} telegram={sf.telegram} />
      </main>
      <RenewDialog token={token} sub={renewing} portal={portal} onOpenChange={(o) => !o && setRenewing(null)} />
    </div>
  )
}
