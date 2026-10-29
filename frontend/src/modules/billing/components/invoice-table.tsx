"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { ReceiptTextIcon } from "lucide-react"
import { DataTable, DateText, EmptyState, FilterSelect, FilterToggle, MoneyText, SearchInput, StatusBadge } from "@/components/shared"
import type { InvoiceFilter, InvoiceListFieldsFragment, InvoiceStatus } from "@/gql/graphql"
import { useCursorPagination } from "@/hooks/use-cursor-pagination"
import { INVOICE_STATUSES } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import { useInvoices } from "../hooks/use-billing"

export function InvoiceTable({
  customerId,
  onRowClick,
  emptyAction,
}: {
  customerId?: string
  onRowClick?: (invoice: InvoiceListFieldsFragment) => void
  emptyAction?: React.ReactNode
}) {
  const t = useT()
  const [filter, setFilter] = useState<InvoiceFilter>({})
  const pager = useCursorPagination(25)
  const effective = useMemo(() => ({ ...filter, customerId }), [filter, customerId])
  const [{ data, fetching, error }, refetch] = useInvoices(effective, pager.page)
  const conn = data?.invoices
  const update = (patch: Partial<InvoiceFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  const columns = useMemo<ColumnDef<InvoiceListFieldsFragment>[]>(() => {
    const cols: ColumnDef<InvoiceListFieldsFragment>[] = [
      { id: "number", header: t.billing.invoiceNo, cell: ({ row }) => <span className="font-mono text-sm font-medium">{row.original.number}</span> },
      { id: "customer", header: t.common.customer, cell: ({ row }) => row.original.customer.name },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="InvoiceStatus" value={row.original.status} /> },
      { id: "total", header: t.common.total, cell: ({ row }) => <MoneyText amount={row.original.total} currency={row.original.currency} /> },
      {
        id: "due",
        header: t.billing.dueAmount,
        cell: ({ row }) => <MoneyText amount={row.original.dueAmount} currency={row.original.currency} tone={row.original.dueAmount > 0 ? "due" : "muted"} />,
      },
      { id: "issued", header: t.billing.issuedAt, cell: ({ row }) => <DateText value={row.original.issuedAt} /> },
      { id: "dueAt", header: t.billing.dueAt, cell: ({ row }) => <DateText value={row.original.dueAt} /> },
    ]
    return customerId ? cols.filter((c) => c.id !== "customer") : cols
  }, [t, customerId])

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
      onRowClick={onRowClick}
      getRowId={(i) => i.id}
      toolbar={
        <>
          {!customerId ? <SearchInput value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} /> : null}
          <FilterSelect<InvoiceStatus>
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={INVOICE_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "InvoiceStatus", s) }))}
          />
          <FilterToggle id="inv-overdue" label={t.billing.overdueOnly} checked={Boolean(filter.overdueOnly)} onChange={(o) => update({ overdueOnly: o || undefined })} />
        </>
      }
      empty={<EmptyState icon={ReceiptTextIcon} title={t.billing.invoicesEmpty} action={emptyAction} />}
      mobileCard={(i) => (
        <div className="space-y-1">
          <div className="flex items-center justify-between gap-2">
            <span className="font-mono text-sm font-medium">{i.number}</span>
            <StatusBadge enum="InvoiceStatus" value={i.status} />
          </div>
          <div className="flex items-center justify-between gap-2 text-sm">
            <span className="truncate text-muted-foreground">{customerId ? <DateText value={i.issuedAt} /> : i.customer.name}</span>
            <span className="flex gap-2">
              {i.dueAmount > 0 ? <MoneyText amount={i.dueAmount} currency={i.currency} tone="due" /> : <MoneyText amount={i.total} currency={i.currency} />}
            </span>
          </div>
        </div>
      )}
    />
  )
}
