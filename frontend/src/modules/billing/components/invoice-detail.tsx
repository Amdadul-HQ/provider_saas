"use client"

import { useState } from "react"
import Link from "next/link"
import { BanIcon, CreditCardIcon, SendIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { ConfirmDialog, DateText, DetailRow, ErrorState, MoneyText, PageSkeleton, SectionCard, StatusBadge, EmptyState } from "@/components/shared"
import { RoleGate } from "@/lib/auth"
import { enumLabel, useT } from "@/i18n"
import { useInvoice, useSendInvoice, useVoidInvoice } from "../hooks/use-billing"
import { PaymentRowActions } from "./payment-table"
import { RecordPaymentDialog } from "./record-payment-dialog"

export function InvoiceDetail({ invoiceId }: { invoiceId: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useInvoice(invoiceId)
  const [send, sending] = useSendInvoice()
  const [voidInvoice, voiding] = useVoidInvoice()
  const [payOpen, setPayOpen] = useState(false)
  const [voidOpen, setVoidOpen] = useState(false)
  const inv = data?.invoice

  if (!inv) {
    if (error) return <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <PageSkeleton /> : <EmptyState title={t.billing.notFound} />
  }
  const closed = inv.status === "VOID" || inv.status === "PAID"

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h1 className="font-mono text-xl font-semibold">{inv.number}</h1>
            <StatusBadge enum="InvoiceStatus" value={inv.status} />
          </div>
          <Link href={`/customers/${inv.customer.id}`} className="text-sm text-muted-foreground hover:underline">
            {inv.customer.name} {inv.customer.phone ? `· ${inv.customer.phone}` : ""}
          </Link>
        </div>
        <div className="flex flex-wrap gap-2">
          {!closed ? (
            <Button onClick={() => setPayOpen(true)}>
              <CreditCardIcon /> {t.billing.recordPayment}
            </Button>
          ) : null}
          {inv.status !== "VOID" ? (
            <Button variant="outline" disabled={sending} onClick={() => void send({ id: inv.id })}>
              <SendIcon /> {t.billing.send}
            </Button>
          ) : null}
          {inv.status !== "VOID" ? (
            <RoleGate roles={["OWNER", "ADMIN"]}>
              <Button variant="destructive" onClick={() => setVoidOpen(true)}>
                <BanIcon /> {t.billing.void}
              </Button>
            </RoleGate>
          ) : null}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1fr_20rem] 2xl:grid-cols-[1fr_24rem]">
        <SectionCard title={t.billing.items} bodyClassName="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t.billing.itemDescription}</TableHead>
                <TableHead className="text-right">{t.billing.qty}</TableHead>
                <TableHead className="text-right">{t.billing.unitPrice}</TableHead>
                <TableHead className="text-right">{t.common.total}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {inv.items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell className="whitespace-normal">{it.description}</TableCell>
                  <TableCell className="text-right tabular">{it.quantity}</TableCell>
                  <TableCell className="text-right">
                    <MoneyText amount={it.unitPrice} currency={inv.currency} />
                  </TableCell>
                  <TableCell className="text-right">
                    <MoneyText amount={it.total} currency={inv.currency} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </SectionCard>
        <SectionCard>
          <dl className="divide-y">
            <DetailRow label={t.billing.subtotal}>
              <MoneyText amount={inv.subtotal} currency={inv.currency} />
            </DetailRow>
            <DetailRow label={t.billing.discount}>
              <MoneyText amount={-inv.discount} currency={inv.currency} tone="muted" />
            </DetailRow>
            <DetailRow label={t.common.total}>
              <MoneyText amount={inv.total} currency={inv.currency} className="text-base" />
            </DetailRow>
            <DetailRow label={t.billing.paidAmount}>
              <MoneyText amount={inv.paidAmount} currency={inv.currency} tone="profit" />
            </DetailRow>
            <DetailRow label={t.billing.dueAmount}>
              <MoneyText amount={inv.dueAmount} currency={inv.currency} tone={inv.dueAmount > 0 ? "due" : "muted"} className="text-base" />
            </DetailRow>
            <DetailRow label={t.billing.issuedAt}>
              <DateText value={inv.issuedAt} />
            </DetailRow>
            <DetailRow label={t.billing.dueAt}>
              <DateText value={inv.dueAt} />
            </DetailRow>
            {inv.paidAt ? (
              <DetailRow label={t.billing.paidAt}>
                <DateText value={inv.paidAt} />
              </DetailRow>
            ) : null}
          </dl>
          {inv.notes ? <p className="mt-3 text-sm whitespace-pre-wrap text-muted-foreground">{inv.notes}</p> : null}
        </SectionCard>
      </div>

      <SectionCard title={t.billing.payments}>
        {inv.payments.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t.billing.paymentsEmpty}</p>
        ) : (
          <ul className="divide-y">
            {inv.payments.map((p) => (
              <li key={p.id} className="flex items-center gap-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">
                    {enumLabel(t, "PaymentMethod", p.method)} {p.trxId ? <span className="font-mono text-xs">· {p.trxId}</span> : null}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    <DateText value={p.receivedAt} withTime /> {p.senderNumber ? `· ${p.senderNumber}` : ""}
                  </p>
                </div>
                <MoneyText amount={p.amount} currency={p.currency} />
                <StatusBadge enum="PaymentStatus" value={p.status} />
                <PaymentRowActions payment={p} />
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <RecordPaymentDialog
        open={payOpen}
        onOpenChange={setPayOpen}
        invoiceId={inv.id}
        customerId={inv.customer.id}
        defaultAmount={inv.dueAmount}
        description={`${inv.number} · ${inv.customer.name}`}
      />
      <ConfirmDialog
        open={voidOpen}
        onOpenChange={setVoidOpen}
        title={t.billing.void}
        description={t.billing.voidConfirm}
        destructive
        loading={voiding}
        onConfirm={async () => {
          if (await voidInvoice({ id: inv.id })) setVoidOpen(false)
        }}
      />
    </div>
  )
}
