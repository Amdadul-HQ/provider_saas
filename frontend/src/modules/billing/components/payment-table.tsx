"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { CheckIcon, CreditCardIcon, XIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { ConfirmDialog, DataTable, DateText, EmptyState, FilterSelect, MoneyText, SearchInput, StatusBadge } from "@/components/shared"
import type { PaymentFieldsFragment, PaymentFilter, PaymentMethod, PaymentStatus } from "@/gql/graphql"
import { useCursorPagination } from "@/hooks/use-cursor-pagination"
import { PAYMENT_METHODS, PAYMENT_STATUSES } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import { usePayments, useRejectPayment, useVerifyPayment } from "../hooks/use-billing"

/** Verify / reject buttons for a pending payment. */
export function PaymentRowActions({ payment }: { payment: Pick<PaymentFieldsFragment, "id" | "status" | "trxId"> }) {
  const t = useT()
  const [verify, verifying] = useVerifyPayment()
  const [reject, rejecting] = useRejectPayment()
  const [rejectOpen, setRejectOpen] = useState(false)
  const [reason, setReason] = useState("")
  if (payment.status !== "PENDING") return null
  return (
    <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
      <Button size="sm" variant="outline" className="border-success/40 text-success" disabled={verifying} onClick={() => void verify({ id: payment.id })}>
        <CheckIcon /> {t.billing.verify}
      </Button>
      <Button size="icon-sm" variant="ghost" aria-label={t.billing.reject} onClick={() => setRejectOpen(true)}>
        <XIcon />
      </Button>
      <ConfirmDialog
        open={rejectOpen}
        onOpenChange={setRejectOpen}
        title={t.billing.reject}
        description={payment.trxId ?? undefined}
        destructive
        loading={rejecting}
        confirmLabel={t.billing.reject}
        onConfirm={async () => {
          if (await reject({ id: payment.id, reason: reason || null })) {
            setReason("")
            setRejectOpen(false)
          }
        }}
      >
        <Textarea placeholder={t.billing.rejectReason} aria-label={t.billing.rejectReason} value={reason} onChange={(e) => setReason(e.target.value)} rows={2} />
      </ConfirmDialog>
    </div>
  )
}

export function PaymentTable({ customerId, initialStatus }: { customerId?: string; initialStatus?: PaymentStatus }) {
  const t = useT()
  const [filter, setFilter] = useState<PaymentFilter>({ status: initialStatus })
  const pager = useCursorPagination(25)
  const effective = useMemo(() => ({ ...filter, customerId }), [filter, customerId])
  const [{ data, fetching, error }, refetch] = usePayments(effective, pager.page)
  const conn = data?.payments
  const update = (patch: Partial<PaymentFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  const columns = useMemo<ColumnDef<PaymentFieldsFragment>[]>(
    () => [
      {
        id: "method",
        header: t.billing.method,
        cell: ({ row }) => (
          <div>
            <p className="font-medium">{enumLabel(t, "PaymentMethod", row.original.method)}</p>
            {row.original.trxId ? <p className="font-mono text-xs text-muted-foreground">{row.original.trxId}</p> : null}
          </div>
        ),
      },
      { id: "customer", header: t.common.customer, cell: ({ row }) => row.original.customer?.name ?? "—" },
      { id: "sender", header: t.billing.senderNumber, cell: ({ row }) => row.original.senderNumber ?? "—" },
      { id: "amount", header: t.common.amount, cell: ({ row }) => <MoneyText amount={row.original.amount} currency={row.original.currency} tone="money" /> },
      { id: "invoice", header: t.billing.invoice, cell: ({ row }) => row.original.invoice?.number ?? "—" },
      { id: "received", header: t.billing.receivedAt, cell: ({ row }) => <DateText value={row.original.receivedAt} withTime /> },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="PaymentStatus" value={row.original.status} /> },
      { id: "actions", header: "", cell: ({ row }) => <PaymentRowActions payment={row.original} /> },
    ],
    [t],
  )

  return (
    <DataTable
      columns={columns}
      data={conn?.edges.map((e) => e.node)}
      fetching={fetching}
      error={error}
      onRetry={() => refetch({ requestPolicy: "network-only" })}
      pageInfo={conn?.pageInfo}
      totalCount={conn?.totalCount}
      pagination={{ canPrev: pager.canPrev, onPrev: pager.prev, onNext: pager.next }}
      getRowId={(p) => p.id}
      toolbar={
        <>
          <SearchInput value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} placeholder={`${t.billing.trxId} / ${t.common.phone}`} />
          <FilterSelect<PaymentStatus>
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={PAYMENT_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "PaymentStatus", s) }))}
          />
          <FilterSelect<PaymentMethod>
            label={t.billing.method}
            value={filter.method}
            onChange={(method) => update({ method })}
            options={PAYMENT_METHODS.map((m) => ({ value: m, label: enumLabel(t, "PaymentMethod", m) }))}
          />
        </>
      }
      empty={<EmptyState icon={CreditCardIcon} title={t.billing.paymentsEmpty} />}
      mobileCard={(p) => (
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate font-medium">
                {enumLabel(t, "PaymentMethod", p.method)} · {p.customer?.name ?? p.senderNumber ?? "—"}
              </p>
              <p className="truncate font-mono text-xs text-muted-foreground">{p.trxId ?? "—"}</p>
            </div>
            <MoneyText amount={p.amount} currency={p.currency} tone="money" />
          </div>
          <div className="flex items-center justify-between gap-2">
            <StatusBadge enum="PaymentStatus" value={p.status} />
            <PaymentRowActions payment={p} />
          </div>
        </div>
      )}
    />
  )
}
