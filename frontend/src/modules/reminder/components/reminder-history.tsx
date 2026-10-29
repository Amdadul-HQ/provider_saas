"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { HistoryIcon, XIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DataTable, DateText, EmptyState, FilterSelect, StatusBadge } from "@/components/shared"
import type { ReminderStatus, RemindersQuery } from "@/gql/graphql"
import { arrayPageInfo, useCursorPagination } from "@/hooks/use-cursor-pagination"
import { REMINDER_STATUSES } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import { useCancelReminder, useReminders } from "../hooks/use-reminders"

type Reminder = RemindersQuery["reminders"][number]
const PAGE = 25

export function ReminderHistory({ customerId }: { customerId?: string }) {
  const t = useT()
  const [status, setStatus] = useState<ReminderStatus | undefined>()
  const pager = useCursorPagination(PAGE)
  const [{ data, fetching, error }, refetch] = useReminders({ status, customerId, page: pager.page })
  const [cancel] = useCancelReminder()
  const rows = data?.reminders

  const columns = useMemo<ColumnDef<Reminder>[]>(() => {
    const cols: ColumnDef<Reminder>[] = [
      { id: "customer", header: t.common.customer, cell: ({ row }) => <span className="font-medium">{row.original.customer?.name ?? "—"}</span> },
      { id: "rule", header: t.reminders.tabs.rules, cell: ({ row }) => row.original.rule?.name ?? "—" },
      { id: "product", header: t.common.product, cell: ({ row }) => row.original.subscription?.product.name ?? "—" },
      { id: "channel", header: t.reminders.channel, cell: ({ row }) => row.original.channel },
      { id: "scheduled", header: t.reminders.scheduledAt, cell: ({ row }) => <DateText value={row.original.scheduledAt} withTime /> },
      { id: "sent", header: t.reminders.sentAt, cell: ({ row }) => <DateText value={row.original.sentAt} withTime /> },
      {
        id: "status",
        header: t.common.status,
        cell: ({ row }) => (
          <div>
            <StatusBadge enum="ReminderStatus" value={row.original.status} />
          </div>
        ),
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) =>
          row.original.status === "SCHEDULED" ? (
            <Button variant="ghost" size="icon-sm" aria-label={t.reminders.cancel} onClick={() => void cancel({ id: row.original.id })}>
              <XIcon />
            </Button>
          ) : null,
      },
    ]
    return customerId ? cols.filter((c) => c.id !== "customer") : cols
  }, [t, customerId, cancel])

  return (
    <DataTable
      columns={columns}
      data={rows}
      fetching={fetching}
      error={error}
      onRetry={() => refetch({ requestPolicy: "network-only" })}
      pageInfo={arrayPageInfo(rows, PAGE)}
      pagination={{ canPrev: pager.canPrev, onPrev: pager.prev, onNext: pager.next }}
      getRowId={(r) => r.id}
      toolbar={
        <FilterSelect<ReminderStatus>
          label={t.common.status}
          value={status}
          onChange={(s) => {
            pager.reset()
            setStatus(s)
          }}
          options={REMINDER_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "ReminderStatus", s) }))}
        />
      }
      empty={<EmptyState compact icon={HistoryIcon} title={t.reminders.historyEmpty} />}
      mobileCard={(r) => (
        <div className="space-y-1">
          <div className="flex items-center justify-between gap-2">
            <p className="truncate font-medium">{customerId ? (r.subscription?.product.name ?? r.rule?.name) : (r.customer?.name ?? "—")}</p>
            <StatusBadge enum="ReminderStatus" value={r.status} />
          </div>
          <p className="text-xs text-muted-foreground">
            {r.channel} · <DateText value={r.sentAt ?? r.scheduledAt} withTime />
          </p>
          {r.renderedBody ? <p className="line-clamp-2 text-xs text-muted-foreground">{r.renderedBody}</p> : null}
        </div>
      )}
    />
  )
}
