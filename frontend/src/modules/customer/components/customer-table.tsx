"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { UsersIcon } from "lucide-react"
import {
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  FilterToggle,
  MoneyText,
  PlatformIcon,
  SearchInput,
  StatusBadge,
} from "@/components/shared"
import type { CustomerFilter, CustomerListFieldsFragment, CustomerSource, CustomerStatus } from "@/gql/graphql"
import { useCursorPagination } from "@/hooks/use-cursor-pagination"
import { CUSTOMER_SOURCES, CUSTOMER_STATUSES } from "@/lib/enums"
import { enumLabel, useT, useFormat } from "@/i18n"
import { useCustomers } from "../hooks/use-customers"

/** Server-paginated customer list with filters mapped 1:1 to `CustomerFilter`. */
export function CustomerTable({
  onRowClick,
  rowHref,
  emptyAction,
  initialFilter,
}: {
  onRowClick?: (customer: CustomerListFieldsFragment) => void
  /** Detail URL per row: rows render as real links (BUG-10). */
  rowHref?: (customer: CustomerListFieldsFragment) => string
  emptyAction?: React.ReactNode
  initialFilter?: CustomerFilter
}) {
  const t = useT()
  const f = useFormat()
  const [filter, setFilter] = useState<CustomerFilter>(initialFilter ?? {})
  const pager = useCursorPagination(25)
  const [{ data, fetching, error }, refetch] = useCustomers(filter, pager.page)
  const conn = data?.customers

  const update = (patch: Partial<CustomerFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  const columns = useMemo<ColumnDef<CustomerListFieldsFragment>[]>(
    () => [
      {
        id: "name",
        header: t.common.name,
        cell: ({ row }) => (
          <div className="flex items-center gap-2">
            <PlatformIcon platform={row.original.preferredChannel} />
            <div className="min-w-0">
              <p className="truncate font-medium">{row.original.name}</p>
              <p className="truncate text-xs text-muted-foreground">{row.original.phone ?? row.original.email ?? "—"}</p>
            </div>
          </div>
        ),
      },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="CustomerStatus" value={row.original.status} /> },
      { id: "source", header: t.customers.source, cell: ({ row }) => enumLabel(t, "CustomerSource", row.original.source) },
      { id: "subs", header: t.customers.activeSubs, cell: ({ row }) => <span className="tabular">{f.number(row.original.activeSubscriptionCount)}</span> },
      {
        id: "due",
        header: t.customers.dueBalance,
        cell: ({ row }) => <MoneyText amount={row.original.dueBalance} tone={row.original.dueBalance > 0 ? "due" : "muted"} />,
      },
      { id: "ltv", header: t.customers.lifetimeValue, cell: ({ row }) => <MoneyText amount={row.original.lifetimeValue} /> },
      { id: "last", header: t.customers.lastContact, cell: ({ row }) => <DateText value={row.original.lastContactAt} relative /> },
    ],
    [t, f],
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
      onRowClick={onRowClick}
      rowHref={rowHref}
      getRowId={(c) => c.id}
      toolbar={
        <>
          <SearchInput value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} placeholder={t.customers.searchPlaceholder} />
          <FilterSelect<CustomerStatus>
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={CUSTOMER_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "CustomerStatus", s) }))}
          />
          <FilterSelect<CustomerSource>
            label={t.customers.source}
            value={filter.source}
            onChange={(source) => update({ source })}
            options={CUSTOMER_SOURCES.map((s) => ({ value: s, label: enumLabel(t, "CustomerSource", s) }))}
          />
          <FilterSelect<string>
            label={t.customers.expiringWithin}
            value={filter.expiringWithinDays ? String(filter.expiringWithinDays) : undefined}
            onChange={(d) => update({ expiringWithinDays: d ? Number(d) : undefined })}
            options={[3, 7, 15, 30].map((d) => ({ value: String(d), label: t.common.days(d) }))}
          />
          <FilterToggle id="cust-has-due" label={t.customers.hasDue} checked={Boolean(filter.hasDue)} onChange={(hasDue) => update({ hasDue: hasDue || undefined })} />
        </>
      }
      empty={<EmptyState icon={UsersIcon} title={t.customers.empty} description={t.customers.emptyHint} action={emptyAction} />}
      mobileCard={(c) => (
        <div className="flex items-center gap-3">
          <PlatformIcon platform={c.preferredChannel} className="size-9" />
          <div className="min-w-0 flex-1">
            <div className="flex items-center justify-between gap-2">
              <p className="truncate font-medium">{c.name}</p>
              {c.dueBalance > 0 ? <MoneyText amount={c.dueBalance} tone="due" className="text-sm" /> : null}
            </div>
            <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
              <span className="truncate">{c.phone ?? c.email ?? "—"}</span>
              <span>
                {t.customers.activeSubs}: {c.activeSubscriptionCount}
              </span>
            </div>
          </div>
        </div>
      )}
    />
  )
}
