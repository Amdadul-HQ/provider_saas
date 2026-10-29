"use client"

import { useMemo, useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { CheckIcon, ShoppingBagIcon, XIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { ConfirmDialog, DataTable, DateText, EmptyState, FilterSelect, MoneyText, ResponsiveDialog, SearchInput, StatusBadge } from "@/components/shared"
import type { OrderFieldsFragment, OrderFilter, OrderSource, OrderStatus } from "@/gql/graphql"
import { arrayPageInfo, useCursorPagination } from "@/hooks/use-cursor-pagination"
import { ORDER_SOURCES, ORDER_STATUSES } from "@/lib/enums"
import { SlotSelect } from "@/modules/inventory"
import { enumLabel, useT } from "@/i18n"
import { useCancelOrder, useConfirmOrder, useOrders } from "../hooks/use-storefront"

const PAGE = 25

function ConfirmOrderDialog({ order, onOpenChange }: { order: OrderFieldsFragment | null; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [confirm, busy] = useConfirmOrder()
  const [slotId, setSlotId] = useState<string | null>(null)
  const [note, setNote] = useState("")
  return (
    <ResponsiveDialog
      open={order !== null}
      onOpenChange={onOpenChange}
      title={t.storefront.confirmOrder}
      description={order ? `#${order.number} · ${order.customerName ?? order.customer?.name ?? ""} · ${order.product.name}` : undefined}
      footer={
        <Button
          disabled={busy}
          onClick={async () => {
            if (order && (await confirm({ id: order.id, slotId, deliveryNote: note || null }))) onOpenChange(false)
          }}
        >
          <CheckIcon /> {t.storefront.confirmOrder}
        </Button>
      }
    >
      {order ? (
        <div className="space-y-4">
          <Field>
            <FieldLabel htmlFor="order-slot">{t.subscriptions.slot}</FieldLabel>
            <SlotSelect id="order-slot" productId={order.product.id} value={slotId} onChange={setSlotId} />
          </Field>
          <Field>
            <FieldLabel htmlFor="order-note">{t.subscriptions.deliveryNote}</FieldLabel>
            <Textarea id="order-note" rows={3} value={note} onChange={(e) => setNote(e.target.value)} />
          </Field>
        </div>
      ) : null}
    </ResponsiveDialog>
  )
}

export function OrdersTable() {
  const t = useT()
  const [filter, setFilter] = useState<OrderFilter>({})
  const pager = useCursorPagination(PAGE)
  const [{ data, fetching, error }, refetch] = useOrders(filter, pager.page)
  const [cancel, cancelling] = useCancelOrder()
  const [confirming, setConfirming] = useState<OrderFieldsFragment | null>(null)
  const [cancelling_, setCancelling] = useState<OrderFieldsFragment | null>(null)
  const rows = data?.orders
  const update = (patch: Partial<OrderFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }
  const open = (o: OrderFieldsFragment) => o.status === "NEW" || o.status === "AWAITING_PAYMENT" || o.status === "PAID"

  const columns = useMemo<ColumnDef<OrderFieldsFragment>[]>(
    () => [
      { id: "number", header: "#", cell: ({ row }) => <span className="font-mono">#{row.original.number}</span> },
      {
        id: "customer",
        header: t.common.customer,
        cell: ({ row }) => (
          <div>
            <p className="font-medium">{row.original.customer?.name ?? row.original.customerName ?? "—"}</p>
            <p className="text-xs text-muted-foreground">{row.original.customerPhone}</p>
          </div>
        ),
      },
      { id: "product", header: t.common.product, cell: ({ row }) => `${row.original.product.name}${row.original.plan ? ` · ${row.original.plan.name}` : ""}` },
      { id: "amount", header: t.common.amount, cell: ({ row }) => <MoneyText amount={row.original.amount} currency={row.original.currency} /> },
      {
        id: "payment",
        header: t.billing.payments,
        cell: ({ row }) =>
          row.original.payment ? (
            <div className="text-xs">
              {enumLabel(t, "PaymentMethod", row.original.payment.method)} <span className="font-mono">{row.original.payment.trxId}</span>
            </div>
          ) : (
            "—"
          ),
      },
      { id: "source", header: t.customers.source, cell: ({ row }) => enumLabel(t, "OrderSource", row.original.source) },
      { id: "status", header: t.common.status, cell: ({ row }) => <StatusBadge enum="OrderStatus" value={row.original.status} /> },
      { id: "created", header: t.common.createdAt, cell: ({ row }) => <DateText value={row.original.createdAt} relative /> },
      {
        id: "actions",
        header: "",
        cell: ({ row }) =>
          open(row.original) ? (
            <div className="flex gap-1">
              <Button size="sm" onClick={() => setConfirming(row.original)}>
                <CheckIcon /> {t.storefront.confirmOrder}
              </Button>
              <Button size="icon-sm" variant="ghost" aria-label={t.storefront.cancelOrder} onClick={() => setCancelling(row.original)}>
                <XIcon />
              </Button>
            </div>
          ) : null,
      },
    ],
    [t],
  )

  return (
    <>
      <DataTable
        columns={columns}
        data={rows}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        pageInfo={arrayPageInfo(rows, PAGE)}
        pagination={{ canPrev: pager.canPrev, onPrev: pager.prev, onNext: pager.next }}
        getRowId={(o) => o.id}
        toolbar={
          <>
            <SearchInput value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} />
            <FilterSelect<OrderStatus>
              label={t.common.status}
              value={filter.status}
              onChange={(status) => update({ status })}
              options={ORDER_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "OrderStatus", s) }))}
            />
            <FilterSelect<OrderSource>
              label={t.customers.source}
              value={filter.source}
              onChange={(source) => update({ source })}
              options={ORDER_SOURCES.map((s) => ({ value: s, label: enumLabel(t, "OrderSource", s) }))}
            />
          </>
        }
        empty={<EmptyState icon={ShoppingBagIcon} title={t.storefront.ordersEmpty} />}
        mobileCard={(o) => (
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <p className="truncate font-medium">
                <span className="font-mono text-muted-foreground">#{o.number}</span> {o.customer?.name ?? o.customerName}
              </p>
              <StatusBadge enum="OrderStatus" value={o.status} />
            </div>
            <div className="flex items-center justify-between gap-2 text-sm">
              <span className="truncate text-muted-foreground">{o.product.name}</span>
              <MoneyText amount={o.amount} currency={o.currency} />
            </div>
            {open(o) ? (
              <div className="flex gap-2">
                <Button size="sm" className="flex-1" onClick={() => setConfirming(o)}>
                  <CheckIcon /> {t.storefront.confirmOrder}
                </Button>
                <Button size="sm" variant="outline" onClick={() => setCancelling(o)}>
                  {t.storefront.cancelOrder}
                </Button>
              </div>
            ) : null}
          </div>
        )}
      />
      <ConfirmOrderDialog order={confirming} onOpenChange={(o) => !o && setConfirming(null)} />
      <ConfirmDialog
        open={cancelling_ !== null}
        onOpenChange={(o) => !o && setCancelling(null)}
        title={t.storefront.cancelOrder}
        destructive
        loading={cancelling}
        onConfirm={async () => {
          if (cancelling_ && (await cancel({ id: cancelling_.id }))) setCancelling(null)
        }}
      />
    </>
  )
}
