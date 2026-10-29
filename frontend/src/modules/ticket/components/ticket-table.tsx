"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { LifeBuoyIcon } from "lucide-react"
import { DataTable, DateText, EmptyState, FilterSelect, SearchInput, StatCard, StatusBadge, CardsSkeleton } from "@/components/shared"
import type { TicketFilter, TicketKind, TicketListFieldsFragment, TicketPriority, TicketStatus } from "@/gql/graphql"
import { arrayPageInfo, useCursorPagination } from "@/hooks/use-cursor-pagination"
import { TICKET_KINDS, TICKET_PRIORITIES, TICKET_STATUSES } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import { useTickets, useTicketStats } from "../hooks/use-tickets"

const PAGE = 25

export function TicketStatsCards() {
  const t = useT()
  const [{ data, fetching }] = useTicketStats()
  if (!data) return fetching ? <CardsSkeleton /> : null
  const s = data.ticketStats
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <StatCard label={t.tickets.stats.open} value={s.open} tone="warning" />
      <StatCard label={t.tickets.stats.inProgress} value={s.inProgress} />
      <StatCard label={t.tickets.stats.resolvedThisMonth} value={s.resolvedThisMonth} tone="success" />
      <StatCard label={t.tickets.stats.replacementsThisMonth} value={s.replacementsThisMonth} tone="money" />
    </div>
  )
}

export function TicketTable({
  customerId,
  onRowClick,
  emptyAction,
}: {
  customerId?: string
  onRowClick?: (ticket: TicketListFieldsFragment) => void
  emptyAction?: React.ReactNode
}) {
  const t = useT()
  const [filter, setFilter] = useState<TicketFilter>({})
  const pager = useCursorPagination(PAGE)
  const effective = useMemo(() => ({ ...filter, customerId }), [filter, customerId])
  const [{ data, fetching, error }, refetch] = useTickets(effective, pager.page)
  const rows = data?.tickets
  const update = (patch: Partial<TicketFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  const columns = useMemo<ColumnDef<TicketListFieldsFragment>[]>(
    () => [
      {
        id: "title",
        header: t.tickets.ticketTitle,
        cell: ({ row }) => (
          <div>
            <p className="font-medium">
              <span className="text-muted-foreground">#{row.original.number}</span> {row.original.title}
            </p>
            <p className="text-xs text-muted-foreground">
              {row.original.customer.name}
              {row.original.subscription ? ` · ${row.original.subscription.product.name}` : ""}
            </p>
          </div>
        ),
      },
      { id: "kind", header: t.tickets.kind, cell: ({ row }) => enumLabel(t, "TicketKind", row.original.kind) },
      { id: "priority", header: t.tickets.priority, cell: ({ row }) => <StatusBadge enum="TicketPriority" value={row.original.priority} /> },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="TicketStatus" value={row.original.status} /> },
      { id: "assigned", header: t.tickets.assignedTo, cell: ({ row }) => row.original.assignedTo?.name ?? "—" },
      { id: "created", header: t.common.createdAt, cell: ({ row }) => <DateText value={row.original.createdAt} relative /> },
    ],
    [t],
  )

  return (
    <DataTable
      columns={columns}
      data={rows}
      fetching={fetching}
      error={error}
      onRetry={() => refetch({ requestPolicy: "network-only" })}
      pageInfo={arrayPageInfo(rows, PAGE)}
      pagination={{ canPrev: pager.canPrev, onPrev: pager.prev, onNext: pager.next }}
      onRowClick={onRowClick}
      getRowId={(r) => r.id}
      toolbar={
        <>
          <SearchInput value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} placeholder={t.tickets.searchPlaceholder} />
          <FilterSelect<TicketStatus>
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={TICKET_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "TicketStatus", s) }))}
          />
          <FilterSelect<TicketKind>
            label={t.tickets.kind}
            value={filter.kind}
            onChange={(kind) => update({ kind })}
            options={TICKET_KINDS.map((s) => ({ value: s, label: enumLabel(t, "TicketKind", s) }))}
          />
          <FilterSelect<TicketPriority>
            label={t.tickets.priority}
            value={filter.priority}
            onChange={(priority) => update({ priority })}
            options={TICKET_PRIORITIES.map((s) => ({ value: s, label: enumLabel(t, "TicketPriority", s) }))}
          />
        </>
      }
      empty={<EmptyState icon={LifeBuoyIcon} title={t.tickets.empty} description={t.tickets.emptyHint} action={emptyAction} />}
      mobileCard={(tk) => (
        <div className="space-y-1.5">
          <div className="flex items-start justify-between gap-2">
            <p className="font-medium">
              <span className="text-muted-foreground">#{tk.number}</span> {tk.title}
            </p>
            <StatusBadge enum="TicketStatus" value={tk.status} />
          </div>
          <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span className="truncate">
              {tk.customer.name} · {enumLabel(t, "TicketKind", tk.kind)}
            </span>
            <StatusBadge enum="TicketPriority" value={tk.priority} />
          </div>
        </div>
      )}
    />
  )
}
