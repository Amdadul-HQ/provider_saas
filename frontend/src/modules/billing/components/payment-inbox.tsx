"use client"

import { useState } from "react"
import { CheckIcon, EyeOffIcon, MessageSquareTextIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { DateText, EmptyState, ErrorState, ListSkeleton, MoneyText, ResponsiveDialog, ToneBadge } from "@/components/shared"
import type { PaymentInboxQuery } from "@/gql/graphql"
import { enumLabel, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useIgnorePaymentInbox, useMatchPaymentInbox, usePaymentInbox, usePayments } from "../hooks/use-billing"

type InboxItem = PaymentInboxQuery["paymentInbox"][number]

function MatchDialog({ item, onOpenChange }: { item: InboxItem | null; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [match, matching] = useMatchPaymentInbox()
  const [chosen, setChosen] = useState<string | null>(null)
  // candidates: pending payments searched by TrxID (falls back to all pending)
  const [{ data, fetching }] = usePayments({ status: "PENDING", search: item?.trxId ?? undefined }, { first: 20 })
  const candidates = (data?.payments.edges.map((e) => e.node) ?? []).sort((a, b) => {
    const score = (p: (typeof candidates)[number]) => (p.trxId && p.trxId === item?.trxId ? 0 : p.amount === item?.amount ? 1 : 2)
    return score(a) - score(b)
  })
  return (
    <ResponsiveDialog
      open={item !== null}
      onOpenChange={onOpenChange}
      title={t.billing.matchTitle}
      description={t.billing.matchHint}
      footer={
        <Button
          disabled={!chosen || matching}
          onClick={async () => {
            if (item && chosen && (await match({ inboxId: item.id, paymentId: chosen }))) onOpenChange(false)
          }}
        >
          <CheckIcon /> {t.billing.match}
        </Button>
      }
    >
      {item ? (
        <Alert className="mb-3">
          <MessageSquareTextIcon />
          <AlertDescription className="font-mono text-xs whitespace-pre-wrap">{item.rawText}</AlertDescription>
        </Alert>
      ) : null}
      {fetching && candidates.length === 0 ? (
        <ListSkeleton rows={3} />
      ) : candidates.length === 0 ? (
        <p className="py-4 text-center text-sm text-muted-foreground">{t.billing.noCandidates}</p>
      ) : (
        <ul role="radiogroup" className="space-y-2">
          {candidates.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                role="radio"
                aria-checked={chosen === p.id}
                onClick={() => setChosen(p.id)}
                className={cn("flex w-full items-center justify-between gap-2 rounded-lg border-2 p-3 text-left", chosen === p.id ? "border-primary bg-primary/5" : "border-transparent bg-muted")}
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{p.customer?.name ?? "—"}</p>
                  <p className="truncate font-mono text-xs text-muted-foreground">
                    {p.trxId ?? "—"} · {p.senderNumber ?? ""}
                  </p>
                </div>
                <div className="text-right">
                  <MoneyText amount={p.amount} currency={p.currency} />
                  {p.trxId && p.trxId === item?.trxId ? <ToneBadge tone="success">TrxID ✓</ToneBadge> : null}
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
    </ResponsiveDialog>
  )
}

/** SMS-forwarder inbox: unmatched bKash/Nagad messages to reconcile against pending payments. */
export function PaymentInbox() {
  const t = useT()
  const [{ data, fetching, error }, refetch] = usePaymentInbox(undefined, { first: 50 })
  const [ignore] = useIgnorePaymentInbox()
  const [matching, setMatching] = useState<InboxItem | null>(null)
  const items = data?.paymentInbox

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">{t.billing.inboxHint}</p>
      {!items ? (
        error ? (
          <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
        ) : fetching ? (
          <ListSkeleton />
        ) : null
      ) : items.length === 0 ? (
        <EmptyState icon={MessageSquareTextIcon} title={t.billing.inboxEmpty} />
      ) : (
        <ul className="space-y-2">
          {items.map((item) => {
            const done = Boolean(item.matchedPaymentId)
            return (
              <li key={item.id} className={cn("rounded-xl border bg-card p-3", done && "opacity-60")}>
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 space-y-0.5">
                    <p className="text-sm font-medium">
                      {item.method ? enumLabel(t, "PaymentMethod", item.method) : item.source}
                      {item.trxId ? <span className="ml-1 font-mono text-xs">· {item.trxId}</span> : null}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {item.senderNumber ?? "—"} · <DateText value={item.receivedAt} withTime />
                    </p>
                  </div>
                  {item.amount !== null && item.amount !== undefined ? <MoneyText amount={item.amount} tone="money" /> : null}
                </div>
                <p className="mt-2 line-clamp-2 font-mono text-xs text-muted-foreground">{item.rawText}</p>
                <div className="mt-2 flex items-center justify-between gap-2">
                  <ToneBadge tone={done ? "success" : "warning"}>{item.status}</ToneBadge>
                  {!done ? (
                    <div className="flex gap-2">
                      <Button size="sm" variant="ghost" onClick={() => void ignore({ inboxId: item.id })}>
                        <EyeOffIcon /> {t.billing.ignore}
                      </Button>
                      <Button size="sm" onClick={() => setMatching(item)}>
                        <CheckIcon /> {t.billing.match}
                      </Button>
                    </div>
                  ) : null}
                </div>
              </li>
            )
          })}
        </ul>
      )}
      {matching ? <MatchDialog item={matching} onOpenChange={(o) => !o && setMatching(null)} /> : null}
    </div>
  )
}
