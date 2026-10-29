"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { ColumnDef } from "@tanstack/react-table"
import { BookOpenIcon, PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { DataTable, DateText, EmptyState, MoneyField, MoneyText, ResponsiveDialog, SelectField, StatusBadge, TextField } from "@/components/shared"
import type { LedgerEntryFieldsFragment, LedgerEntryType } from "@/gql/graphql"
import { LEDGER_ENTRY_TYPES } from "@/lib/enums"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useAddLedgerAdjustment, useLedger } from "../hooks/use-billing"

function AdjustDialog({ customerId, open, onOpenChange }: { customerId: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [add, saving] = useAddLedgerAdjustment()
  const schema = useMemo(
    () => z.object({ entryType: z.enum(LEDGER_ENTRY_TYPES as [LedgerEntryType, ...LedgerEntryType[]]), amount: v.positiveMoney(t), note: v.optionalString() }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { entryType: "DEBIT", amount: 0, note: "" } })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.billing.adjustTitle}
      footer={
        <Button type="submit" form="ledger-adjust" disabled={saving}>
          {t.common.save}
        </Button>
      }
    >
      <form
        id="ledger-adjust"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          if (await add({ customerId, entryType: values.entryType, amount: values.amount, note: values.note || null })) {
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <SelectField
            control={form.control}
            name="entryType"
            label={t.common.status}
            options={LEDGER_ENTRY_TYPES.map((e) => ({ value: e, label: e === "DEBIT" ? t.billing.debit : t.billing.credit }))}
          />
          <MoneyField control={form.control} name="amount" label={t.common.amount} required />
          <TextField control={form.control} name="note" label={t.common.notes} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

/** Baki khata for one customer, oldest→newest with running balance (debit = owes more). */
export function LedgerTable({ customerId }: { customerId: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useLedger(customerId)
  const [open, setOpen] = useState(false)
  const rows = useMemo(() => {
    const entries = [...(data?.ledger ?? [])].sort((a, b) => a.createdAt.localeCompare(b.createdAt))
    let balance = 0
    return entries
      .map((e) => {
        balance += e.entryType === "DEBIT" ? e.amount : -e.amount
        return { ...e, balance }
      })
      .reverse()
  }, [data])

  const columns = useMemo<ColumnDef<LedgerEntryFieldsFragment & { balance: number }>[]>(
    () => [
      { id: "date", header: t.common.date, cell: ({ row }) => <DateText value={row.original.createdAt} withTime /> },
      { id: "type", header: t.common.status, cell: ({ row }) => <StatusBadge enum="LedgerEntryType" value={row.original.entryType} /> },
      { id: "note", header: t.common.notes, cell: ({ row }) => row.original.note ?? row.original.refType ?? "—" },
      {
        id: "amount",
        header: t.common.amount,
        cell: ({ row }) => <MoneyText amount={row.original.entryType === "DEBIT" ? row.original.amount : -row.original.amount} tone={row.original.entryType === "DEBIT" ? "due" : "profit"} />,
      },
      { id: "balance", header: t.billing.balance, cell: ({ row }) => <MoneyText amount={row.original.balance} tone={row.original.balance > 0 ? "due" : "muted"} /> },
      { id: "by", header: "", cell: ({ row }) => <span className="text-xs text-muted-foreground">{row.original.createdBy?.name}</span> },
    ],
    [t],
  )

  return (
    <div className="space-y-3">
      <RoleGate roles={["OWNER", "ADMIN"]}>
        <Button variant="outline" onClick={() => setOpen(true)}>
          <PlusIcon /> {t.billing.adjust}
        </Button>
      </RoleGate>
      <DataTable
        columns={columns}
        data={data ? rows : undefined}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        empty={<EmptyState compact icon={BookOpenIcon} title={t.billing.ledgerEmpty} />}
        mobileCard={(e) => (
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">{e.note ?? enumLabel(t, "LedgerEntryType", e.entryType)}</p>
              <DateText value={e.createdAt} className="text-xs text-muted-foreground" />
            </div>
            <div className="text-right">
              <MoneyText amount={e.entryType === "DEBIT" ? e.amount : -e.amount} tone={e.entryType === "DEBIT" ? "due" : "profit"} />
              <p className="text-xs text-muted-foreground">
                {t.billing.balance}: <MoneyText amount={e.balance} className="font-normal" />
              </p>
            </div>
          </div>
        )}
      />
      <AdjustDialog customerId={customerId} open={open} onOpenChange={setOpen} />
    </div>
  )
}
