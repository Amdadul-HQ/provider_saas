"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { BoxesIcon } from "lucide-react"
import { Progress } from "@/components/ui/progress"
import { DataTable, DateText, EmptyState, FilterSelect, MoneyText, StatusBadge } from "@/components/shared"
import type { MasterAccountFieldsFragment, MasterAccountFilter, MasterAccountStatus } from "@/gql/graphql"
import { arrayPageInfo, useCursorPagination } from "@/hooks/use-cursor-pagination"
import { MASTER_ACCOUNT_STATUSES } from "@/lib/enums"
import { ProductSelect } from "@/modules/catalog"
import { enumLabel, useT } from "@/i18n"
import { useMasterAccounts, useSuppliers } from "../hooks/use-inventory"

const PAGE = 25

export function MasterAccountTable({
  onSelect,
  selectedId,
  emptyAction,
}: {
  onSelect?: (a: MasterAccountFieldsFragment) => void
  selectedId?: string | null
  emptyAction?: React.ReactNode
}) {
  const t = useT()
  const [filter, setFilter] = useState<MasterAccountFilter>({})
  const pager = useCursorPagination(PAGE)
  const [{ data, fetching, error }, refetch] = useMasterAccounts(filter, pager.page)
  const [{ data: suppliers }] = useSuppliers()
  const rows = data?.masterAccounts
  const update = (patch: Partial<MasterAccountFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  const columns = useMemo<ColumnDef<MasterAccountFieldsFragment>[]>(
    () => [
      {
        id: "label",
        header: t.inventory.label,
        cell: ({ row }) => (
          <div className={row.original.id === selectedId ? "font-semibold text-primary" : "font-medium"}>
            {row.original.label}
            <p className="text-xs font-normal text-muted-foreground">{row.original.product.name}</p>
          </div>
        ),
      },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="MasterAccountStatus" value={row.original.status} /> },
      {
        id: "slots",
        header: t.inventory.slots,
        cell: ({ row }) => {
          const used = row.original.totalSlots - row.original.freeSlots
          return (
            <div className="w-28 space-y-1">
              <span className="text-xs">{t.inventory.occupancy(used, row.original.totalSlots)}</span>
              <Progress value={row.original.totalSlots ? (used / row.original.totalSlots) * 100 : 0} className="h-1.5" />
            </div>
          )
        },
      },
      { id: "supplier", header: t.inventory.supplier, cell: ({ row }) => row.original.supplier?.name ?? "—" },
      { id: "cost", header: t.inventory.purchaseCost, cell: ({ row }) => <MoneyText amount={row.original.purchaseCost} currency={row.original.currency} /> },
      { id: "expires", header: t.inventory.expiresAt, cell: ({ row }) => <DateText value={row.original.expiresAt} /> },
    ],
    [t, selectedId],
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
      onRowClick={onSelect}
      getRowId={(a) => a.id}
      toolbar={
        <>
          <div className="w-full md:w-56">
            <ProductSelect value={filter.productId} includeAllOption={`${t.common.product}: ${t.common.all}`} onChange={(p) => update({ productId: p?.id })} />
          </div>
          <FilterSelect<string>
            label={t.inventory.supplier}
            value={filter.supplierId}
            onChange={(supplierId) => update({ supplierId })}
            options={(suppliers?.suppliers ?? []).map((s) => ({ value: s.id, label: s.name }))}
          />
          <FilterSelect<MasterAccountStatus>
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={MASTER_ACCOUNT_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "MasterAccountStatus", s) }))}
          />
          <FilterSelect<string>
            label={t.inventory.expiringFilter}
            value={filter.expiringWithinDays ? String(filter.expiringWithinDays) : undefined}
            onChange={(d) => update({ expiringWithinDays: d ? Number(d) : undefined })}
            options={[3, 7, 15, 30].map((d) => ({ value: String(d), label: t.common.days(d) }))}
          />
        </>
      }
      empty={<EmptyState icon={BoxesIcon} title={t.inventory.accountsEmpty} description={t.inventory.accountsEmptyHint} action={emptyAction} />}
      mobileCard={(a) => {
        const used = a.totalSlots - a.freeSlots
        return (
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0">
                <p className="truncate font-medium">{a.label}</p>
                <p className="truncate text-xs text-muted-foreground">{a.product.name}</p>
              </div>
              <StatusBadge enum="MasterAccountStatus" value={a.status} />
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Progress value={a.totalSlots ? (used / a.totalSlots) * 100 : 0} className="h-1.5 flex-1" />
              {t.inventory.occupancy(used, a.totalSlots)}
            </div>
          </div>
        )
      }}
    />
  )
}
