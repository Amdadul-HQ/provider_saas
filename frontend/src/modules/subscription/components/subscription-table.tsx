"use client"

import { useMemo, useState } from "react"
import Link from "next/link"
import type { ColumnDef } from "@tanstack/react-table"
import { RepeatIcon, TruckIcon } from "lucide-react"
import { DataTable, DateText, DaysLeft, EmptyState, FilterSelect, MoneyText, SearchInput, StatusBadge, ToneBadge } from "@/components/shared"
import type { SubscriptionFilter, SubscriptionListFieldsFragment, SubscriptionStatus } from "@/gql/graphql"
import { useCursorPagination } from "@/hooks/use-cursor-pagination"
import { SUBSCRIPTION_STATUSES } from "@/lib/enums"
import { ProductSelect } from "@/modules/catalog"
import { enumLabel, useT } from "@/i18n"
import { useSubscriptions } from "../hooks/use-subscriptions"
import { SubscriptionActions } from "./subscription-actions"

/**
 * Subscriptions list mapped to `SubscriptionFilter`. Pass `customerId` to scope it to one customer
 * (customer detail tab) — the customer column/filter are then hidden.
 */
export function SubscriptionTable({ customerId, emptyAction }: { customerId?: string; emptyAction?: React.ReactNode }) {
  const t = useT()
  const [filter, setFilter] = useState<SubscriptionFilter>({})
  const pager = useCursorPagination(25)
  const effective = useMemo(() => ({ ...filter, customerId }), [filter, customerId])
  const [{ data, fetching, error }, refetch] = useSubscriptions(effective, pager.page)
  const conn = data?.subscriptions
  const update = (patch: Partial<SubscriptionFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  const columns = useMemo<ColumnDef<SubscriptionListFieldsFragment>[]>(() => {
    const cols: ColumnDef<SubscriptionListFieldsFragment>[] = [
      {
        id: "customer",
        header: t.common.customer,
        cell: ({ row }) => (
          <Link href={`/customers/${row.original.customer.id}`} className="font-medium hover:underline" onClick={(e) => e.stopPropagation()}>
            {row.original.customer.name}
            <span className="block text-xs font-normal text-muted-foreground">{row.original.customer.phone}</span>
          </Link>
        ),
      },
      {
        id: "product",
        header: t.common.product,
        cell: ({ row }) => (
          <div>
            <p>{row.original.product.name}</p>
            <p className="text-xs text-muted-foreground">
              {row.original.plan?.name ?? "—"}
              {row.original.slot ? ` · ${t.inventory.slotNo(row.original.slot.slotNo)}` : ""}
            </p>
          </div>
        ),
      },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="SubscriptionStatus" value={row.original.status} /> },
      {
        id: "expires",
        header: t.common.expiresAt,
        cell: ({ row }) => (
          <div>
            <DateText value={row.original.expiresAt} />
            <DaysLeft days={row.original.daysLeft} className="block" />
          </div>
        ),
      },
      { id: "price", header: t.common.price, cell: ({ row }) => <MoneyText amount={row.original.price} currency={row.original.currency} /> },
      {
        id: "delivered",
        header: "",
        cell: ({ row }) => (!row.original.deliveredAt ? <ToneBadge tone="warning">{t.subscriptions.notDelivered}</ToneBadge> : null),
      },
      { id: "actions", header: "", cell: ({ row }) => <SubscriptionActions subscription={row.original} /> },
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
      getRowId={(s) => s.id}
      toolbar={
        <>
          {!customerId ? (
            <SearchInput value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} placeholder={t.subscriptions.searchPlaceholder} />
          ) : null}
          <FilterSelect<SubscriptionStatus>
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={SUBSCRIPTION_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "SubscriptionStatus", s) }))}
          />
          <div className="w-full md:w-52">
            <ProductSelect aria-label={t.common.product} value={filter.productId} includeAllOption={`${t.common.product}: ${t.common.all}`} onChange={(p) => update({ productId: p?.id })} />
          </div>
          <FilterSelect<string>
            label={t.subscriptions.expiringWithin}
            value={filter.expiringWithinDays ? String(filter.expiringWithinDays) : undefined}
            onChange={(d) => update({ expiringWithinDays: d ? Number(d) : undefined })}
            options={[1, 3, 7, 15, 30].map((d) => ({ value: String(d), label: t.common.days(d) }))}
          />
        </>
      }
      empty={<EmptyState icon={RepeatIcon} title={t.subscriptions.empty} description={t.subscriptions.emptyHint} action={emptyAction} />}
      mobileCard={(s) => (
        <div className="flex items-start gap-2">
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex items-center justify-between gap-2">
              <p className="truncate font-medium">{customerId ? s.product.name : s.customer.name}</p>
              <StatusBadge enum="SubscriptionStatus" value={s.status} />
            </div>
            <p className="truncate text-xs text-muted-foreground">
              {customerId ? s.plan?.name : `${s.product.name} · ${s.plan?.name ?? ""}`}
              {s.slot ? ` · ${t.inventory.slotNo(s.slot.slotNo)}` : ""}
            </p>
            <div className="flex items-center justify-between gap-2">
              <DaysLeft days={s.daysLeft} />
              <span className="flex items-center gap-2">
                {!s.deliveredAt ? <TruckIcon className="size-3.5 text-warning-foreground dark:text-warning" aria-label={t.subscriptions.notDelivered} /> : null}
                <MoneyText amount={s.price} currency={s.currency} className="text-sm" />
              </span>
            </div>
          </div>
          <SubscriptionActions subscription={s} />
        </div>
      )}
    />
  )
}
